package graphio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/graph"
)

// CSVWriterStreams CSV writer가 사용할 출력 stream 묶음이다.
type CSVWriterStreams struct {
	Vertices io.Writer
	Edges    io.Writer
}

// CSVReaderStreams CSV reader가 사용할 입력 stream 묶음이다.
type CSVReaderStreams struct {
	Vertices io.Reader
	Edges    io.Reader
}

// CSVWriter graph를 CSV vertex/edge stream으로 직렬화한다.
type CSVWriter struct {
	ctx           context.Context
	options       CSVWriteOptions
	vertexWriter  *csv.Writer
	edgeWriter    *csv.Writer
	vertexHeader  bool
	edgeHeader    bool
	report        Report
	started       time.Time
	closed        bool
	final         Report
	terminalError error
}

// NewCSVWriter graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func NewCSVWriter(ctx context.Context, streams CSVWriterStreams, options CSVWriteOptions) *CSVWriter {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, err := normalizeCSVWriteOptions(options)
	writer := &CSVWriter{
		ctx:     ctx,
		options: normalized,
		report:  Report{Format: FormatCSV},
		started: time.Now(),
	}
	if streams.Vertices != nil {
		writer.vertexWriter = csv.NewWriter(streams.Vertices)
	}
	if streams.Edges != nil {
		writer.edgeWriter = csv.NewWriter(streams.Edges)
	}
	writer.terminalError = err
	return writer
}

// WriteVertex graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (w *CSVWriter) WriteVertex(vertex graph.Vertex) error {
	if w.closed {
		return ErrStreamClosed
	}
	if err := w.ready(PhaseWriteVertex); err != nil {
		return err
	}
	if err := vertex.Validate(); err != nil {
		return wrap(ErrInvalidRecord, FormatCSV, PhaseWriteVertex, Location{FileRole: FileRoleVertices}, "vertex", vertex.ID().String(), "invalid vertex", err)
	}
	if !w.vertexHeader {
		if err := w.writeHeader(w.vertexWriter, FileRoleVertices); err != nil {
			return err
		}
		w.vertexHeader = true
	}
	row, err := w.vertexRow(vertex)
	if err != nil {
		return err
	}
	if err := w.vertexWriter.Write(row); err != nil {
		return fmt.Errorf("write vertex csv: %w", err)
	}
	w.report.VerticesWritten++
	return nil
}

// WriteEdge graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (w *CSVWriter) WriteEdge(edge graph.Edge) error {
	if w.closed {
		return ErrStreamClosed
	}
	if err := w.ready(PhaseWriteEdge); err != nil {
		return err
	}
	if err := edge.Validate(); err != nil {
		return wrap(ErrInvalidRecord, FormatCSV, PhaseWriteEdge, Location{FileRole: FileRoleEdges}, "edge", edge.ID().String(), "invalid edge", err)
	}
	if !w.edgeHeader {
		if err := w.writeHeader(w.edgeWriter, FileRoleEdges); err != nil {
			return err
		}
		w.edgeHeader = true
	}
	row, err := w.edgeRow(edge)
	if err != nil {
		return err
	}
	if err := w.edgeWriter.Write(row); err != nil {
		return fmt.Errorf("write edge csv: %w", err)
	}
	w.report.EdgesWritten++
	return nil
}

// Close graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (w *CSVWriter) Close() (Report, error) {
	if w.closed {
		return w.final, w.terminalError
	}
	w.closed = true
	if w.vertexWriter != nil {
		w.vertexWriter.Flush()
		if err := w.vertexWriter.Error(); err != nil && w.terminalError == nil {
			w.terminalError = fmt.Errorf("flush vertex csv: %w", err)
		}
	}
	if w.edgeWriter != nil {
		w.edgeWriter.Flush()
		if err := w.edgeWriter.Error(); err != nil && w.terminalError == nil {
			w.terminalError = fmt.Errorf("flush edge csv: %w", err)
		}
	}
	w.final = w.report
	w.final.Elapsed = time.Since(w.started)
	return w.final, w.terminalError
}

func (w *CSVWriter) ready(phase Phase) error {
	if w.terminalError != nil {
		return w.terminalError
	}
	if err := w.ctx.Err(); err != nil {
		return err
	}
	if w.vertexWriter == nil || w.edgeWriter == nil {
		return wrap(ErrInvalidOptions, FormatCSV, phase, Location{}, "streams", "", "csv streams must not be nil", nil)
	}
	if w.options.PropertyMode == CSVPropertiesPrefixedColumns && len(w.options.PropertyColumns) == 0 {
		return wrap(ErrInvalidOptions, FormatCSV, phase, Location{}, "property_columns", "", "property columns required for streaming csv writer", nil)
	}
	return nil
}

func (w *CSVWriter) writeHeader(writer *csv.Writer, role FileRole) error {
	header := []string{"id", "label"}
	if role == FileRoleEdges {
		header = append(header, "from", "to")
	}
	switch w.options.PropertyMode {
	case CSVPropertiesPrefixedColumns:
		for _, column := range w.options.PropertyColumns {
			header = append(header, prefixedColumn(w.options.PropertyPrefix, column))
		}
	case CSVPropertiesRawJSONColumn:
		header = append(header, w.options.RawPropertiesColumn)
	case CSVPropertiesNone:
	}
	return writer.Write(header)
}

func (w *CSVWriter) vertexRow(vertex graph.Vertex) ([]string, error) {
	row := []string{w.escape(vertex.ID().String()), w.escape(vertex.Label().String())}
	props, err := w.propertyCells(vertex.Properties())
	if err != nil {
		return nil, err
	}
	return append(row, props...), nil
}

func (w *CSVWriter) edgeRow(edge graph.Edge) ([]string, error) {
	row := []string{w.escape(edge.ID().String()), w.escape(edge.Label().String()), w.escape(edge.StartID().String()), w.escape(edge.EndID().String())}
	props, err := w.propertyCells(edge.Properties())
	if err != nil {
		return nil, err
	}
	return append(row, props...), nil
}

func (w *CSVWriter) propertyCells(properties graph.Properties) ([]string, error) {
	switch w.options.PropertyMode {
	case CSVPropertiesNone:
		return nil, nil
	case CSVPropertiesRawJSONColumn:
		if properties == nil && !w.options.IncludeEmptyProperties {
			return []string{""}, nil
		}
		data, err := json.Marshal(properties)
		if err != nil {
			return nil, wrap(ErrInvalidRecord, FormatCSV, PhaseValidate, Location{}, "properties", "", "properties are not json serializable", err)
		}
		return []string{string(data)}, nil
	default:
		cells := make([]string, 0, len(w.options.PropertyColumns))
		for _, column := range w.options.PropertyColumns {
			value := ""
			if properties != nil {
				if raw, ok := properties[column]; ok && raw != nil {
					value = fmt.Sprint(raw)
				}
			}
			cells = append(cells, w.escape(value))
		}
		return cells, nil
	}
}

func (w *CSVWriter) escape(value string) string {
	if w.options.FormulaPolicy != CSVFormulaEscape || value == "" {
		return value
	}
	if strings.HasPrefix(value, "=") || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "@") ||
		strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") || strings.HasPrefix(value, "\n") {
		return "'" + value
	}
	return value
}

// WriteCSV graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func WriteCSV(ctx context.Context, streams CSVWriterStreams, records []Record, options CSVWriteOptions) (Report, error) {
	normalized, err := normalizeCSVWriteOptions(options)
	if err != nil {
		return Report{Format: FormatCSV}, err
	}
	if normalized.PropertyMode == CSVPropertiesPrefixedColumns && len(normalized.PropertyColumns) == 0 {
		normalized.PropertyColumns = discoverPropertyColumns(records)
	}
	writer := NewCSVWriter(ctx, streams, normalized)
	for _, record := range records {
		if record.Kind == RecordVertex {
			if err := writer.WriteVertex(record.Vertex); err != nil {
				report, _ := writer.Close()
				return report, err
			}
		}
	}
	for _, record := range records {
		if record.Kind == RecordEdge {
			if err := writer.WriteEdge(record.Edge); err != nil {
				report, _ := writer.Close()
				return report, err
			}
		}
	}
	return writer.Close()
}

// CSVReader CSV vertex/edge stream에서 graph를 복원한다.
type CSVReader struct {
	ctx          context.Context
	options      CSVReadOptions
	vertices     *csvRecordReader
	edges        *csvRecordReader
	vertexHeader csvHeader
	edgeHeader   csvHeader
	vertexReady  bool
	edgeReady    bool
	vertexEOF    bool
	edgeEOF      bool
	records      int64
	seen         map[string]struct{}
	report       Report
	started      time.Time
	closed       bool
	final        Report
	setupErr     error
}

// NewCSVReader graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func NewCSVReader(ctx context.Context, streams CSVReaderStreams, options CSVReadOptions) *CSVReader {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, err := normalizeCSVReadOptions(options)
	reader := &CSVReader{
		ctx:      ctx,
		options:  normalized,
		seen:     map[string]struct{}{},
		report:   Report{Format: FormatCSV},
		started:  time.Now(),
		setupErr: err,
	}
	if streams.Vertices != nil {
		reader.vertices = newCSVRecordReader(streams.Vertices, normalized.ReadOptions, FileRoleVertices)
	}
	if streams.Edges != nil {
		reader.edges = newCSVRecordReader(streams.Edges, normalized.ReadOptions, FileRoleEdges)
	}
	return reader
}

// ReadVertex graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (r *CSVReader) ReadVertex() (graph.Vertex, error) {
	if r.closed {
		return graph.Vertex{}, ErrStreamClosed
	}
	if r.vertexEOF {
		return graph.Vertex{}, io.EOF
	}
	if err := r.ready(FileRoleVertices); err != nil {
		return graph.Vertex{}, err
	}
	if !r.vertexReady {
		header, err := r.readHeader(r.vertices, FileRoleVertices)
		if err != nil {
			return graph.Vertex{}, err
		}
		r.vertexHeader = header
		r.vertexReady = true
	}
	for {
		row, loc, err := r.vertices.Read()
		if errors.Is(err, io.EOF) {
			r.vertexEOF = true
			return graph.Vertex{}, io.EOF
		}
		if err != nil {
			return graph.Vertex{}, err
		}
		vertex, skipped, err := r.vertexFromRow(row, loc)
		if err != nil {
			return graph.Vertex{}, err
		}
		if skipped {
			continue
		}
		return vertex, nil
	}
}

// ReadEdge graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (r *CSVReader) ReadEdge() (graph.Edge, error) {
	if r.closed {
		return graph.Edge{}, ErrStreamClosed
	}
	if r.edgeEOF {
		return graph.Edge{}, io.EOF
	}
	if !r.vertexEOF {
		return graph.Edge{}, wrap(ErrInvalidRecord, FormatCSV, PhaseReadEdge, Location{FileRole: FileRoleEdges}, "edge", "", "read vertices before edges", nil)
	}
	if err := r.ready(FileRoleEdges); err != nil {
		return graph.Edge{}, err
	}
	if !r.edgeReady {
		header, err := r.readHeader(r.edges, FileRoleEdges)
		if err != nil {
			return graph.Edge{}, err
		}
		r.edgeHeader = header
		r.edgeReady = true
	}
	for {
		row, loc, err := r.edges.Read()
		if errors.Is(err, io.EOF) {
			r.edgeEOF = true
			return graph.Edge{}, io.EOF
		}
		if err != nil {
			return graph.Edge{}, err
		}
		edge, skipped, err := r.edgeFromRow(row, loc)
		if err != nil {
			return graph.Edge{}, err
		}
		if skipped {
			continue
		}
		return edge, nil
	}
}

// Close graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (r *CSVReader) Close() (Report, error) {
	if r.closed {
		return r.final, nil
	}
	r.closed = true
	r.final = r.report
	r.final.Elapsed = time.Since(r.started)
	return r.final, nil
}

func (r *CSVReader) ready(role FileRole) error {
	if r.setupErr != nil {
		return r.setupErr
	}
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if role == FileRoleVertices && r.vertices == nil {
		return wrap(ErrInvalidOptions, FormatCSV, PhaseReadVertex, Location{FileRole: role}, "vertices", "", "vertices stream must not be nil", nil)
	}
	if role == FileRoleEdges && r.edges == nil {
		return wrap(ErrInvalidOptions, FormatCSV, PhaseReadEdge, Location{FileRole: role}, "edges", "", "edges stream must not be nil", nil)
	}
	return nil
}

func (r *CSVReader) readHeader(reader *csvRecordReader, role FileRole) (csvHeader, error) {
	row, loc, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return csvHeader{}, wrap(ErrMalformedInput, FormatCSV, phaseForRole(role), loc, "header", "", "missing csv header", err)
		}
		return csvHeader{}, err
	}
	header, err := parseHeader(row, role, r.options)
	if err != nil {
		return csvHeader{}, wrap(ErrMalformedInput, FormatCSV, phaseForRole(role), loc, "header", "", "invalid csv header", err)
	}
	return header, nil
}

func (r *CSVReader) vertexFromRow(row []string, loc Location) (graph.Vertex, bool, error) {
	if err := r.countRecord(PhaseReadVertex, loc); err != nil {
		return graph.Vertex{}, false, err
	}
	if len(row) != len(r.vertexHeader.columns) {
		return graph.Vertex{}, false, wrap(ErrMalformedInput, FormatCSV, PhaseReadVertex, loc, "row", "", "vertex row width mismatch", nil)
	}
	properties, err := r.propertiesFromRow(row, r.vertexHeader, loc, PhaseReadVertex)
	if err != nil {
		return graph.Vertex{}, false, err
	}
	vertex, err := graph.ParseVertex(row[r.vertexHeader.id], row[r.vertexHeader.label], properties)
	if err != nil {
		return graph.Vertex{}, false, wrap(ErrInvalidRecord, FormatCSV, PhaseReadVertex, loc, "vertex", row[r.vertexHeader.id], "invalid vertex", err)
	}
	id := vertex.ID().String()
	if _, ok := r.seen[id]; ok {
		if r.options.DuplicateVertexPolicy == DuplicateVertexSkip {
			r.report.SkippedVertices++
			return graph.Vertex{}, true, nil
		}
		return graph.Vertex{}, false, wrap(ErrDuplicateVertex, FormatCSV, PhaseReadVertex, loc, "id", id, "duplicate vertex", nil)
	}
	r.seen[id] = struct{}{}
	r.report.VerticesRead++
	return vertex, false, nil
}

func (r *CSVReader) edgeFromRow(row []string, loc Location) (graph.Edge, bool, error) {
	if err := r.countRecord(PhaseReadEdge, loc); err != nil {
		return graph.Edge{}, false, err
	}
	if len(row) != len(r.edgeHeader.columns) {
		return graph.Edge{}, false, wrap(ErrMalformedInput, FormatCSV, PhaseReadEdge, loc, "row", "", "edge row width mismatch", nil)
	}
	properties, err := r.propertiesFromRow(row, r.edgeHeader, loc, PhaseReadEdge)
	if err != nil {
		return graph.Edge{}, false, err
	}
	edge, err := graph.ParseEdge(row[r.edgeHeader.id], row[r.edgeHeader.label], graph.RawEdgeEndpoints{Start: row[r.edgeHeader.from], End: row[r.edgeHeader.to]}, properties)
	if err != nil {
		return graph.Edge{}, false, wrap(ErrInvalidRecord, FormatCSV, PhaseReadEdge, loc, "edge", row[r.edgeHeader.id], "invalid edge", err)
	}
	if !r.hasVertex(edge.StartID().String()) || !r.hasVertex(edge.EndID().String()) {
		if r.options.MissingEndpointPolicy == MissingEndpointSkipEdge {
			r.report.SkippedEdges++
			return graph.Edge{}, true, nil
		}
		return graph.Edge{}, false, wrap(ErrMissingEndpoint, FormatCSV, PhaseReadEdge, loc, "endpoint", edge.ID().String(), "missing endpoint", nil)
	}
	r.report.EdgesRead++
	return edge, false, nil
}

func (r *CSVReader) hasVertex(id string) bool {
	_, ok := r.seen[id]
	return ok
}

func (r *CSVReader) countRecord(phase Phase, loc Location) error {
	r.records++
	if r.options.MaxRecords != UnlimitedRecords && r.records > r.options.MaxRecords {
		return wrap(ErrMalformedInput, FormatCSV, phase, loc, "record", "", "record limit exceeded", nil)
	}
	return nil
}

func (r *CSVReader) propertiesFromRow(row []string, header csvHeader, loc Location, phase Phase) (graph.Properties, error) {
	switch r.options.PropertyMode {
	case CSVPropertiesNone:
		return nil, nil
	case CSVPropertiesRawJSONColumn:
		if header.rawProperties < 0 || row[header.rawProperties] == "" {
			return nil, nil
		}
		var properties graph.Properties
		if err := json.Unmarshal([]byte(row[header.rawProperties]), &properties); err != nil {
			return nil, wrap(ErrMalformedInput, FormatCSV, phase, loc, r.options.RawPropertiesColumn, "", "raw properties json decode failed", err)
		}
		return properties, nil
	default:
		properties := graph.Properties{}
		for name, idx := range header.properties {
			properties[name] = row[idx]
		}
		if len(properties) == 0 {
			return nil, nil
		}
		return properties, nil
	}
}

// ReadCSV graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
func ReadCSV(ctx context.Context, streams CSVReaderStreams, options CSVReadOptions) ([]Record, Report, error) {
	reader := NewCSVReader(ctx, streams, options)
	records := make([]Record, 0)
	for {
		vertex, err := reader.ReadVertex()
		if err == nil {
			record, recordErr := VertexRecord(vertex)
			if recordErr != nil {
				report, _ := reader.Close()
				return records, report, recordErr
			}
			records = append(records, record)
			continue
		}
		if errors.Is(err, io.EOF) {
			break
		}
		report, _ := reader.Close()
		return records, report, err
	}
	for {
		edge, err := reader.ReadEdge()
		if err == nil {
			record, recordErr := EdgeRecord(edge)
			if recordErr != nil {
				report, _ := reader.Close()
				return records, report, recordErr
			}
			records = append(records, record)
			continue
		}
		if errors.Is(err, io.EOF) {
			report, closeErr := reader.Close()
			return records, report, closeErr
		}
		report, _ := reader.Close()
		return records, report, err
	}
}

type csvRecordReader struct {
	reader  *bufio.Reader
	options ReadOptions
	role    FileRole
	row     int64
}

func newCSVRecordReader(reader io.Reader, options ReadOptions, role FileRole) *csvRecordReader {
	return &csvRecordReader{reader: bufio.NewReader(reader), options: options, role: role}
}

func (r *csvRecordReader) Read() ([]string, Location, error) {
	raw, loc, err := r.readRaw()
	if err != nil {
		return nil, loc, err
	}
	parser := csv.NewReader(bytes.NewReader(raw))
	parser.FieldsPerRecord = -1
	row, err := parser.Read()
	if err != nil {
		return nil, loc, wrap(ErrMalformedInput, FormatCSV, phaseForRole(r.role), loc, "row", "", "csv parse failed", err)
	}
	if len(row) > r.options.MaxColumns {
		return nil, loc, wrap(ErrMalformedInput, FormatCSV, phaseForRole(r.role), loc, "columns", "", "too many columns", nil)
	}
	for _, field := range row {
		if len(field) > r.options.MaxFieldBytes {
			return nil, loc, wrap(ErrMalformedInput, FormatCSV, phaseForRole(r.role), loc, "field", "", "field too large", nil)
		}
	}
	return row, loc, nil
}

func (r *csvRecordReader) readRaw() ([]byte, Location, error) {
	var buf []byte
	inQuotes := false
	for {
		b, err := r.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) && len(buf) > 0 {
				r.row++
				return buf, Location{Row: r.row, FileRole: r.role}, nil
			}
			return nil, Location{Row: r.row + 1, FileRole: r.role}, err
		}
		buf = append(buf, b)
		if len(buf) > r.options.MaxRecordBytes {
			return nil, Location{Row: r.row + 1, FileRole: r.role}, wrap(ErrMalformedInput, FormatCSV, phaseForRole(r.role), Location{Row: r.row + 1, FileRole: r.role}, "record", "", "record too large", nil)
		}
		if b == '"' {
			next, err := r.reader.Peek(1)
			if inQuotes && err == nil && len(next) == 1 && next[0] == '"' {
				escaped, _ := r.reader.ReadByte()
				buf = append(buf, escaped)
				continue
			}
			inQuotes = !inQuotes
		}
		if b == '\n' && !inQuotes {
			r.row++
			return buf, Location{Row: r.row, FileRole: r.role}, nil
		}
	}
}

type csvHeader struct {
	columns       []string
	id            int
	label         int
	from          int
	to            int
	properties    map[string]int
	rawProperties int
}

func parseHeader(row []string, role FileRole, options CSVReadOptions) (csvHeader, error) {
	header := csvHeader{id: -1, label: -1, from: -1, to: -1, rawProperties: -1, properties: map[string]int{}, columns: row}
	for idx, column := range row {
		switch column {
		case "id":
			header.id = idx
		case "label":
			header.label = idx
		case "from":
			header.from = idx
		case "to":
			header.to = idx
		case options.RawPropertiesColumn:
			header.rawProperties = idx
		default:
			if strings.HasPrefix(column, options.PropertyPrefix) {
				header.properties[strings.TrimPrefix(column, options.PropertyPrefix)] = idx
			}
		}
	}
	if header.id < 0 || header.label < 0 {
		return header, errors.New("missing id or label")
	}
	if role == FileRoleEdges && (header.from < 0 || header.to < 0) {
		return header, errors.New("missing edge endpoints")
	}
	if options.PropertyMode == CSVPropertiesRawJSONColumn && header.rawProperties < 0 {
		return header, errors.New("missing raw properties column")
	}
	return header, nil
}

func phaseForRole(role FileRole) Phase {
	if role == FileRoleEdges {
		return PhaseReadEdge
	}
	return PhaseReadVertex
}

func prefixedColumn(prefix, column string) string {
	if strings.HasPrefix(column, prefix) {
		return column
	}
	return prefix + column
}

func discoverPropertyColumns(records []Record) []string {
	set := map[string]struct{}{}
	for _, record := range records {
		var properties graph.Properties
		switch record.Kind {
		case RecordVertex:
			properties = record.Vertex.Properties()
		case RecordEdge:
			properties = record.Edge.Properties()
		}
		for key := range properties {
			set[key] = struct{}{}
		}
	}
	columns := make([]string, 0, len(set))
	for key := range set {
		columns = append(columns, key)
	}
	sort.Strings(columns)
	return columns
}
