package graphio

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/graph"
)

type ndjsonEnvelope struct {
	Type       string           `json:"type"`
	ID         string           `json:"id"`
	Label      string           `json:"label"`
	From       string           `json:"from,omitempty"`
	To         string           `json:"to,omitempty"`
	Properties graph.Properties `json:"properties,omitempty"`
}

// NDJSONWriter graph record를 NDJSON stream으로 직렬화한다.
type NDJSONWriter struct {
	ctx      context.Context
	writer   io.Writer
	options  WriteOptions
	report   Report
	started  time.Time
	closed   bool
	final    Report
	setupErr error
}

// NewNDJSONWriter graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func NewNDJSONWriter(ctx context.Context, writer io.Writer, options WriteOptions) *NDJSONWriter {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, err := normalizeWriteOptions(options)
	return &NDJSONWriter{
		ctx:      ctx,
		writer:   writer,
		options:  normalized,
		report:   Report{Format: FormatNDJSON},
		started:  time.Now(),
		setupErr: err,
	}
}

// WriteRecord graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func (w *NDJSONWriter) WriteRecord(record Record) error {
	if w.closed {
		return ErrStreamClosed
	}
	if err := w.ctx.Err(); err != nil {
		return err
	}
	if w.setupErr != nil {
		return w.setupErr
	}
	if w.writer == nil {
		return wrap(ErrInvalidOptions, FormatNDJSON, PhaseValidate, Location{}, "writer", "", "writer must not be nil", nil)
	}
	if err := record.Validate(); err != nil {
		return err
	}
	envelope, phase, err := envelopeFromRecord(record)
	if err != nil {
		return err
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return wrap(ErrInvalidRecord, FormatNDJSON, phase, Location{}, "record", envelope.ID, "encode failed", err)
	}
	data = append(data, '\n')
	if _, err := w.writer.Write(data); err != nil {
		return fmt.Errorf("write ndjson: %w", err)
	}
	if record.Kind == RecordVertex {
		w.report.VerticesWritten++
	} else {
		w.report.EdgesWritten++
	}
	return nil
}

// Close graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (w *NDJSONWriter) Close() (Report, error) {
	if w.closed {
		return w.final, nil
	}
	w.closed = true
	w.final = w.report
	w.final.Elapsed = time.Since(w.started)
	return w.final, nil
}

// WriteNDJSON graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func WriteNDJSON(ctx context.Context, writer io.Writer, records []Record, options WriteOptions) (Report, error) {
	stream := NewNDJSONWriter(ctx, writer, options)
	for _, record := range records {
		if record.Kind == RecordVertex {
			if err := stream.WriteRecord(record); err != nil {
				report, _ := stream.Close()
				return report, err
			}
		}
	}
	for _, record := range records {
		if record.Kind == RecordEdge {
			if err := stream.WriteRecord(record); err != nil {
				report, _ := stream.Close()
				return report, err
			}
		}
	}
	return stream.Close()
}

// NDJSONReader NDJSON stream에서 graph record를 복원한다.
type NDJSONReader struct {
	ctx       context.Context
	scanner   *bufio.Scanner
	options   ReadOptions
	report    Report
	seen      map[string]struct{}
	started   time.Time
	line      int64
	records   int64
	eof       bool
	closed    bool
	final     Report
	setupErr  error
	lastError error
}

// NewNDJSONReader graph IO Neo4j backend에서 생성과 초기화 계약을 설명한다.
func NewNDJSONReader(ctx context.Context, reader io.Reader, options ReadOptions) *NDJSONReader {
	if ctx == nil {
		ctx = context.Background()
	}
	normalized, err := NormalizeReadOptions(options)
	if err != nil {
		normalized = ReadOptions{}
	}
	if reader == nil {
		if err == nil {
			err = wrap(ErrInvalidOptions, FormatNDJSON, PhaseValidate, Location{FileRole: FileRoleStream}, "reader", "", "reader must not be nil", nil)
		}
		reader = strings.NewReader("")
	}
	scanner := bufio.NewScanner(reader)
	if normalized.MaxLineBytes > 0 {
		scanner.Buffer(make([]byte, 0, min(normalized.MaxLineBytes, 64*1024)), normalized.MaxLineBytes)
	}
	return &NDJSONReader{
		ctx:      ctx,
		scanner:  scanner,
		options:  normalized,
		report:   Report{Format: FormatNDJSON},
		seen:     map[string]struct{}{},
		started:  time.Now(),
		setupErr: err,
	}
}

// ReadRecord graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (r *NDJSONReader) ReadRecord() (Record, error) {
	if r.closed {
		return Record{}, ErrStreamClosed
	}
	if r.eof {
		return Record{}, io.EOF
	}
	if r.setupErr != nil {
		return Record{}, r.setupErr
	}
	if err := r.ctx.Err(); err != nil {
		return Record{}, err
	}
	for {
		if !r.scanner.Scan() {
			if err := r.scanner.Err(); err != nil {
				r.lastError = wrap(ErrMalformedInput, FormatNDJSON, PhaseValidate, Location{Line: r.line + 1, FileRole: FileRoleStream}, "", "", "line too large or unreadable", err)
				return Record{}, r.lastError
			}
			r.eof = true
			return Record{}, io.EOF
		}
		r.line++
		line := r.scanner.Bytes()
		if len(line) > r.options.MaxRecordBytes {
			r.lastError = wrap(ErrMalformedInput, FormatNDJSON, PhaseValidate, Location{Line: r.line, FileRole: FileRoleStream}, "record", "", "record too large", nil)
			return Record{}, r.lastError
		}
		if len(line) == 0 {
			r.lastError = wrap(ErrMalformedInput, FormatNDJSON, PhaseValidate, Location{Line: r.line, FileRole: FileRoleStream}, "", "", "blank line", nil)
			return Record{}, r.lastError
		}
		record, skipped, err := r.decodeLine(line)
		if err != nil {
			r.lastError = err
			return Record{}, err
		}
		if skipped {
			continue
		}
		return record, nil
	}
}

// Close graph IO Neo4j backend에서 반환값과 오류 의미를 설명한다.
func (r *NDJSONReader) Close() (Report, error) {
	if r.closed {
		return r.final, nil
	}
	r.closed = true
	r.final = r.report
	r.final.Elapsed = time.Since(r.started)
	return r.final, nil
}

// ReadNDJSON graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func ReadNDJSON(ctx context.Context, reader io.Reader, options ReadOptions) ([]Record, Report, error) {
	stream := NewNDJSONReader(ctx, reader, options)
	records := make([]Record, 0)
	for {
		record, err := stream.ReadRecord()
		if err == nil {
			records = append(records, record)
			continue
		}
		if errors.Is(err, io.EOF) {
			report, closeErr := stream.Close()
			return records, report, closeErr
		}
		report, _ := stream.Close()
		return records, report, err
	}
}

func (r *NDJSONReader) decodeLine(line []byte) (Record, bool, error) {
	r.records++
	if err := checkRecordLimit(r.options, r.records); err != nil {
		return Record{}, false, err
	}
	var envelope ndjsonEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return Record{}, false, wrap(ErrMalformedInput, FormatNDJSON, PhaseValidate, Location{Line: r.line, FileRole: FileRoleStream}, "", "", "json decode failed", err)
	}
	switch envelope.Type {
	case string(RecordVertex):
		vertex, err := graph.ParseVertex(envelope.ID, envelope.Label, envelope.Properties)
		if err != nil {
			return Record{}, false, wrap(ErrInvalidRecord, FormatNDJSON, PhaseReadVertex, Location{Line: r.line, FileRole: FileRoleStream}, "vertex", envelope.ID, "invalid vertex", err)
		}
		id := vertex.ID().String()
		if _, ok := r.seen[id]; ok {
			if r.options.DuplicateVertexPolicy == DuplicateVertexSkip {
				r.report.SkippedVertices++
				return Record{}, true, nil
			}
			return Record{}, false, wrap(ErrDuplicateVertex, FormatNDJSON, PhaseReadVertex, Location{Line: r.line, FileRole: FileRoleStream}, "id", id, "duplicate vertex", nil)
		}
		r.seen[id] = struct{}{}
		r.report.VerticesRead++
		record, err := VertexRecord(vertex)
		return record, false, err
	case string(RecordEdge):
		edge, err := graph.ParseEdge(envelope.ID, envelope.Label, graph.RawEdgeEndpoints{Start: envelope.From, End: envelope.To}, envelope.Properties)
		if err != nil {
			return Record{}, false, wrap(ErrInvalidRecord, FormatNDJSON, PhaseReadEdge, Location{Line: r.line, FileRole: FileRoleStream}, "edge", envelope.ID, "invalid edge", err)
		}
		if !r.hasEndpoint(edge.StartID().String()) || !r.hasEndpoint(edge.EndID().String()) {
			if r.options.MissingEndpointPolicy == MissingEndpointSkipEdge {
				r.report.SkippedEdges++
				return Record{}, true, nil
			}
			return Record{}, false, wrap(ErrMissingEndpoint, FormatNDJSON, PhaseReadEdge, Location{Line: r.line, FileRole: FileRoleStream}, "endpoint", edge.ID().String(), "missing endpoint", nil)
		}
		r.report.EdgesRead++
		record, err := EdgeRecord(edge)
		return record, false, err
	default:
		return Record{}, false, wrap(ErrMalformedInput, FormatNDJSON, PhaseValidate, Location{Line: r.line, FileRole: FileRoleStream}, "type", envelope.ID, "unknown record type", nil)
	}
}

func (r *NDJSONReader) hasEndpoint(id string) bool {
	_, ok := r.seen[id]
	return ok
}

func envelopeFromRecord(record Record) (ndjsonEnvelope, Phase, error) {
	if err := record.Validate(); err != nil {
		return ndjsonEnvelope{}, PhaseValidate, err
	}
	if record.Kind == RecordVertex {
		vertex := record.Vertex
		return ndjsonEnvelope{
			Type:       string(RecordVertex),
			ID:         vertex.ID().String(),
			Label:      vertex.Label().String(),
			Properties: vertex.Properties(),
		}, PhaseWriteVertex, nil
	}
	edge := record.Edge
	return ndjsonEnvelope{
		Type:       string(RecordEdge),
		ID:         edge.ID().String(),
		Label:      edge.Label().String(),
		From:       edge.StartID().String(),
		To:         edge.EndID().String(),
		Properties: edge.Properties(),
	}, PhaseWriteEdge, nil
}
