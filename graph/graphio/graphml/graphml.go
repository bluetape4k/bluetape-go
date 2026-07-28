package graphml

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bluetape4k/bluetape-go/graph"
	"github.com/bluetape4k/bluetape-go/graph/graphio"
)

const defaultMaxInputBytes int64 = 4 << 20

// FormatGraphML graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
const FormatGraphML graphio.Format = "graphml"

// ReadOptions graph IO Neo4j backend에서 설정값과 기본값 적용 방식을 설명한다.
type ReadOptions struct {
	graphio.ReadOptions
	MaxInputBytes int64
}

// WriteOptions graph IO Neo4j backend에서 설정값과 기본값 적용 방식을 설명한다.
type WriteOptions struct {
	graphio.WriteOptions
}

type keyScope string

const (
	scopeAll  keyScope = "all"
	scopeNode keyScope = "node"
	scopeEdge keyScope = "edge"
)

type keyDef struct {
	ID    string
	Scope keyScope
	Name  string
	Type  string
}

type nodeBuilder struct {
	ID         string
	Label      string
	Properties graph.Properties
}

type edgeBuilder struct {
	ID         string
	Source     string
	Target     string
	Label      string
	Properties graph.Properties
}

type dataBuilder struct {
	Key    keyDef
	Owner  keyScope
	Record string
	Text   strings.Builder
}

type parser struct {
	decoder      *xml.Decoder
	options      ReadOptions
	keys         map[string]keyDef
	seenVertices map[string]struct{}
	seenEdges    map[string]struct{}
	records      []graphio.Record
	report       graphio.Report
	rootSeen     bool
	graphOpen    bool
	graphSeen    bool
	node         *nodeBuilder
	edge         *edgeBuilder
	data         *dataBuilder
	stack        []string
}

// Read graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func Read(ctx context.Context, reader io.Reader, options ReadOptions) ([]graphio.Record, graphio.Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	started := time.Now()
	normalized, err := normalizeReadOptions(options)
	if err != nil {
		return nil, graphio.Report{Format: FormatGraphML, Elapsed: time.Since(started)}, err
	}
	if reader == nil {
		return nil, graphio.Report{Format: FormatGraphML, Elapsed: time.Since(started)}, newError(graphio.ErrInvalidOptions, graphio.PhaseValidate, graphio.Location{FileRole: graphio.FileRoleStream}, "reader", "", "reader must not be nil", nil)
	}
	if err := ctx.Err(); err != nil {
		return nil, graphio.Report{Format: FormatGraphML, Elapsed: time.Since(started)}, err
	}

	data, err := readBounded(reader, normalized.MaxInputBytes)
	if err != nil {
		return nil, graphio.Report{Format: FormatGraphML, Elapsed: time.Since(started)}, err
	}
	if err := ctx.Err(); err != nil {
		return nil, graphio.Report{Format: FormatGraphML, Elapsed: time.Since(started)}, err
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	p := &parser{
		decoder:      decoder,
		options:      normalized,
		keys:         map[string]keyDef{},
		seenVertices: map[string]struct{}{},
		seenEdges:    map[string]struct{}{},
		records:      make([]graphio.Record, 0),
		report:       graphio.Report{Format: FormatGraphML},
	}
	if err := p.parse(ctx); err != nil {
		p.report.Elapsed = time.Since(started)
		return p.records, p.report, err
	}
	p.report.Elapsed = time.Since(started)
	return p.records, p.report, nil
}

// Write graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
func Write(ctx context.Context, writer io.Writer, records []graphio.Record, options WriteOptions) (graphio.Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	started := time.Now()
	_, err := normalizeWriteOptions(options)
	report := graphio.Report{Format: FormatGraphML}
	if err != nil {
		report.Elapsed = time.Since(started)
		return report, err
	}
	if writer == nil {
		report.Elapsed = time.Since(started)
		return report, newError(graphio.ErrInvalidOptions, graphio.PhaseValidate, graphio.Location{}, "writer", "", "writer must not be nil", nil)
	}
	if err := ctx.Err(); err != nil {
		report.Elapsed = time.Since(started)
		return report, err
	}
	if err := validateRecords(records); err != nil {
		report.Elapsed = time.Since(started)
		return report, err
	}

	nodeKeys, edgeKeys := discoverKeys(records)
	encoder := xml.NewEncoder(writer)
	encoder.Indent("", "  ")
	if _, err := writer.Write([]byte(xml.Header)); err != nil {
		return report, fmt.Errorf("write graphml header: %w", err)
	}
	if err := encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "graphml"}, Attr: []xml.Attr{
		{Name: xml.Name{Local: "xmlns"}, Value: "http://graphml.graphdrawing.org/xmlns"},
	}}); err != nil {
		return report, fmt.Errorf("write graphml root: %w", err)
	}
	if err := writeKey(encoder, "node_label", scopeNode, "label", "string"); err != nil {
		return report, err
	}
	if err := writeKey(encoder, "edge_label", scopeEdge, "label", "string"); err != nil {
		return report, err
	}
	for _, key := range nodeKeys {
		if err := writeKey(encoder, "node_"+key, scopeNode, key, inferGraphMLType(records, graphio.RecordVertex, key)); err != nil {
			return report, err
		}
	}
	for _, key := range edgeKeys {
		if err := writeKey(encoder, "edge_"+key, scopeEdge, key, inferGraphMLType(records, graphio.RecordEdge, key)); err != nil {
			return report, err
		}
	}

	if err := encoder.EncodeToken(xml.StartElement{Name: xml.Name{Local: "graph"}, Attr: []xml.Attr{
		{Name: xml.Name{Local: "id"}, Value: "G"},
		{Name: xml.Name{Local: "edgedefault"}, Value: "directed"},
	}}); err != nil {
		return report, fmt.Errorf("write graphml graph: %w", err)
	}
	vertices, edges := orderedRecords(records)
	for _, record := range vertices {
		if record.Kind != graphio.RecordVertex {
			continue
		}
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if err := writeVertex(encoder, record.Vertex, nodeKeys); err != nil {
			return report, err
		}
		report.VerticesWritten++
	}
	for _, record := range edges {
		if record.Kind != graphio.RecordEdge {
			continue
		}
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if err := writeEdge(encoder, record.Edge, edgeKeys); err != nil {
			return report, err
		}
		report.EdgesWritten++
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "graph"}}); err != nil {
		return report, fmt.Errorf("close graphml graph: %w", err)
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "graphml"}}); err != nil {
		return report, fmt.Errorf("close graphml root: %w", err)
	}
	if err := encoder.Flush(); err != nil {
		return report, fmt.Errorf("flush graphml: %w", err)
	}
	report.Elapsed = time.Since(started)
	return report, nil
}

func normalizeReadOptions(options ReadOptions) (ReadOptions, error) {
	read, err := graphio.NormalizeReadOptions(options.ReadOptions)
	if err != nil {
		return options, err
	}
	options.ReadOptions = read
	if options.MaxInputBytes == 0 {
		options.MaxInputBytes = defaultMaxInputBytes
	}
	if options.MaxInputBytes < 0 {
		return options, newError(graphio.ErrInvalidOptions, graphio.PhaseValidate, graphio.Location{}, "max_input_bytes", "", "max input bytes must not be negative", nil)
	}
	return options, nil
}

func normalizeWriteOptions(options WriteOptions) (WriteOptions, error) {
	if options.MaxFailures < 0 {
		return options, newError(graphio.ErrInvalidOptions, graphio.PhaseValidate, graphio.Location{}, "max_failures", "", "max failures must not be negative", nil)
	}
	return options, nil
}

func readBounded(reader io.Reader, maxBytes int64) ([]byte, error) {
	limited := io.LimitReader(reader, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read graphml: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, newError(graphio.ErrMalformedInput, graphio.PhaseValidate, graphio.Location{FileRole: graphio.FileRoleStream}, "document", "", "input too large", nil)
	}
	return data, nil
}

func (p *parser) parse(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		token, err := p.decoder.Token()
		if errors.Is(err, io.EOF) {
			if len(p.stack) != 0 {
				return p.malformed("document", "", "unexpected end of graphml document", err)
			}
			if !p.graphSeen {
				return p.malformed("graph", "", "missing graph element", nil)
			}
			return nil
		}
		if err != nil {
			return p.malformed("", "", "xml decode failed", err)
		}
		switch typed := token.(type) {
		case xml.StartElement:
			if err := p.start(typed); err != nil {
				return err
			}
		case xml.EndElement:
			if err := p.end(typed); err != nil {
				return err
			}
		case xml.CharData:
			if err := p.text([]byte(typed)); err != nil {
				return err
			}
		case xml.Directive:
			return p.malformed("xml", "", "xml directives and processing instructions are not supported", nil)
		case xml.ProcInst:
			if typed.Target != "xml" || len(p.stack) != 0 || p.graphSeen {
				return p.malformed("xml", "", "xml processing instructions are not supported", nil)
			}
		}
	}
}

func (p *parser) start(start xml.StartElement) error {
	name := start.Name.Local
	parent := p.parent()
	switch name {
	case "graphml":
		if len(p.stack) != 0 {
			return p.malformed("graphml", "", "nested graphml is not supported", nil)
		}
		if p.rootSeen {
			return p.malformed("graphml", "", "multiple graphml roots are not supported", nil)
		}
		p.rootSeen = true
	case "key":
		if parent != "graphml" {
			return p.malformed("key", "", "key must be declared under graphml", nil)
		}
		key, err := parseKey(start)
		if err != nil {
			return p.malformed("key", "", "invalid key declaration", err)
		}
		if _, exists := p.keys[key.ID]; exists {
			return p.malformed("key", key.ID, "duplicate key id", nil)
		}
		p.keys[key.ID] = key
	case "graph":
		if parent != "graphml" {
			return p.malformed("graph", "", "nested graphs are not supported", nil)
		}
		if p.graphSeen {
			return p.malformed("graph", "", "multiple graph elements are not supported", nil)
		}
		if edgedefault := attr(start, "edgedefault"); edgedefault != "directed" {
			return p.malformed("edgedefault", "", "only directed graphs are supported", nil)
		}
		p.graphOpen = true
		p.graphSeen = true
	case "node":
		if parent != "graph" || !p.graphOpen {
			return p.malformed("node", "", "node must be inside graph", nil)
		}
		id := attr(start, "id")
		if strings.TrimSpace(id) == "" {
			return p.malformed("id", "", "node id is required", nil)
		}
		p.node = &nodeBuilder{ID: id, Properties: graph.Properties{}}
	case "edge":
		if parent != "graph" || !p.graphOpen {
			return p.malformed("edge", "", "edge must be inside graph", nil)
		}
		if directed := attr(start, "directed"); directed != "" && directed != "true" {
			return p.malformed("directed", attr(start, "id"), "only directed edges are supported", nil)
		}
		id, source, target := attr(start, "id"), attr(start, "source"), attr(start, "target")
		if strings.TrimSpace(id) == "" || strings.TrimSpace(source) == "" || strings.TrimSpace(target) == "" {
			return p.malformed("edge", id, "edge id, source, and target are required", nil)
		}
		p.edge = &edgeBuilder{ID: id, Source: source, Target: target, Properties: graph.Properties{}}
	case "data":
		if parent != "node" && parent != "edge" {
			return p.malformed("data", "", "data must be inside node or edge", nil)
		}
		key, ok := p.keys[attr(start, "key")]
		if !ok {
			return p.malformed("key", "", "unknown data key", nil)
		}
		owner := scopeNode
		recordID := ""
		if parent == "edge" {
			owner = scopeEdge
		}
		if owner == scopeNode && p.node != nil {
			recordID = p.node.ID
		}
		if owner == scopeEdge && p.edge != nil {
			recordID = p.edge.ID
		}
		if key.Scope != scopeAll && key.Scope != owner {
			return p.malformed("key", recordID, "data key does not apply to element", nil)
		}
		if p.data != nil {
			return p.malformed("data", recordID, "nested data is not supported", nil)
		}
		p.data = &dataBuilder{Key: key, Owner: owner, Record: recordID}
	default:
		return p.malformed(name, "", "unsupported graphml element", nil)
	}
	p.stack = append(p.stack, name)
	return nil
}

func (p *parser) end(end xml.EndElement) error {
	name := end.Name.Local
	if len(p.stack) == 0 || p.stack[len(p.stack)-1] != name {
		return p.malformed(name, "", "unexpected closing element", nil)
	}
	switch name {
	case "data":
		if err := p.finishData(); err != nil {
			return err
		}
	case "node":
		if err := p.finishNode(); err != nil {
			return err
		}
	case "edge":
		if err := p.finishEdge(); err != nil {
			return err
		}
	case "graph":
		p.graphOpen = false
	}
	p.stack = p.stack[:len(p.stack)-1]
	return nil
}

func (p *parser) text(data []byte) error {
	if p.data != nil {
		_, _ = p.data.Text.Write(data)
		return nil
	}
	if strings.TrimSpace(string(data)) != "" {
		return p.malformed("text", "", "text outside data is not supported", nil)
	}
	return nil
}

func (p *parser) finishData() error {
	if p.data == nil {
		return p.malformed("data", "", "data end without data start", nil)
	}
	value, err := convertScalar(p.data.Key, p.data.Text.String())
	if err != nil {
		return p.malformed(p.data.Key.Name, p.data.Record, "invalid scalar value", err)
	}
	if p.data.Owner == scopeNode {
		if p.node == nil {
			return p.malformed("node", p.data.Record, "node data without node", nil)
		}
		assignNodeData(p.node, p.data.Key.Name, value)
	} else {
		if p.edge == nil {
			return p.malformed("edge", p.data.Record, "edge data without edge", nil)
		}
		assignEdgeData(p.edge, p.data.Key.Name, value)
	}
	p.data = nil
	return nil
}

func (p *parser) finishNode() error {
	if p.node == nil {
		return p.malformed("node", "", "node end without node start", nil)
	}
	if _, exists := p.seenVertices[p.node.ID]; exists {
		return newError(graphio.ErrDuplicateVertex, graphio.PhaseReadVertex, p.location(), "id", p.node.ID, "duplicate vertex", nil)
	}
	vertex, err := graph.ParseVertex(p.node.ID, p.node.Label, nilIfEmpty(p.node.Properties))
	if err != nil {
		return newError(graphio.ErrInvalidRecord, graphio.PhaseReadVertex, p.location(), "vertex", p.node.ID, "invalid vertex", err)
	}
	record, err := graphio.VertexRecord(vertex)
	if err != nil {
		return err
	}
	p.seenVertices[p.node.ID] = struct{}{}
	p.records = append(p.records, record)
	p.report.VerticesRead++
	p.node = nil
	if err := checkRecordLimit(p.options.ReadOptions, p.report.VerticesRead+p.report.EdgesRead); err != nil {
		return err
	}
	return nil
}

func (p *parser) finishEdge() error {
	if p.edge == nil {
		return p.malformed("edge", "", "edge end without edge start", nil)
	}
	if _, exists := p.seenEdges[p.edge.ID]; exists {
		return newError(graphio.ErrInvalidRecord, graphio.PhaseReadEdge, p.location(), "id", p.edge.ID, "duplicate edge", nil)
	}
	if _, ok := p.seenVertices[p.edge.Source]; !ok {
		return newError(graphio.ErrMissingEndpoint, graphio.PhaseReadEdge, p.location(), "endpoint", p.edge.ID, "missing endpoint", nil)
	}
	if _, ok := p.seenVertices[p.edge.Target]; !ok {
		return newError(graphio.ErrMissingEndpoint, graphio.PhaseReadEdge, p.location(), "endpoint", p.edge.ID, "missing endpoint", nil)
	}
	edge, err := graph.ParseEdge(p.edge.ID, p.edge.Label, graph.RawEdgeEndpoints{Start: p.edge.Source, End: p.edge.Target}, nilIfEmpty(p.edge.Properties))
	if err != nil {
		return newError(graphio.ErrInvalidRecord, graphio.PhaseReadEdge, p.location(), "edge", p.edge.ID, "invalid edge", err)
	}
	record, err := graphio.EdgeRecord(edge)
	if err != nil {
		return err
	}
	p.seenEdges[p.edge.ID] = struct{}{}
	p.records = append(p.records, record)
	p.report.EdgesRead++
	p.edge = nil
	if err := checkRecordLimit(p.options.ReadOptions, p.report.VerticesRead+p.report.EdgesRead); err != nil {
		return err
	}
	return nil
}

func (p *parser) parent() string {
	if len(p.stack) == 0 {
		return ""
	}
	return p.stack[len(p.stack)-1]
}

func (p *parser) malformed(field string, recordID string, summary string, cause error) error {
	return newError(graphio.ErrMalformedInput, graphio.PhaseValidate, p.location(), field, recordID, summary, cause)
}

func (p *parser) location() graphio.Location {
	line, col := p.decoder.InputPos()
	return graphio.Location{Line: int64(line), Column: strconv.Itoa(col), FileRole: graphio.FileRoleStream}
}

func parseKey(start xml.StartElement) (keyDef, error) {
	key := keyDef{
		ID:    attr(start, "id"),
		Scope: keyScope(attr(start, "for")),
		Name:  attr(start, "attr.name"),
		Type:  attr(start, "attr.type"),
	}
	if key.Scope == "" {
		key.Scope = scopeAll
	}
	if key.Type == "" {
		key.Type = "string"
	}
	if strings.TrimSpace(key.ID) == "" || strings.TrimSpace(key.Name) == "" {
		return key, errors.New("key id and attr.name are required")
	}
	switch key.Scope {
	case scopeAll, scopeNode, scopeEdge:
	default:
		return key, errors.New("unsupported key scope")
	}
	if !supportedType(key.Type) {
		return key, errors.New("unsupported attr.type")
	}
	return key, nil
}

func attr(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func supportedType(value string) bool {
	switch value {
	case "string", "boolean", "int", "long", "float", "double":
		return true
	default:
		return false
	}
}

func convertScalar(key keyDef, raw string) (any, error) {
	value := strings.TrimSpace(raw)
	switch key.Type {
	case "boolean":
		return strconv.ParseBool(value)
	case "int", "long":
		return strconv.ParseInt(value, 10, 64)
	case "float", "double":
		return strconv.ParseFloat(value, 64)
	default:
		return value, nil
	}
}

func assignNodeData(node *nodeBuilder, name string, value any) {
	if name == "label" {
		node.Label = fmt.Sprint(value)
		return
	}
	node.Properties[name] = value
}

func assignEdgeData(edge *edgeBuilder, name string, value any) {
	if name == "label" {
		edge.Label = fmt.Sprint(value)
		return
	}
	edge.Properties[name] = value
}

func nilIfEmpty(properties graph.Properties) graph.Properties {
	if len(properties) == 0 {
		return nil
	}
	return properties
}

func checkRecordLimit(options graphio.ReadOptions, count int64) error {
	if options.MaxRecords != graphio.UnlimitedRecords && count > options.MaxRecords {
		return newError(graphio.ErrMalformedInput, graphio.PhaseValidate, graphio.Location{FileRole: graphio.FileRoleStream}, "record", "", "record limit exceeded", nil)
	}
	return nil
}

func validateRecords(records []graphio.Record) error {
	for _, record := range records {
		if err := record.Validate(); err != nil {
			return newError(graphio.ErrInvalidRecord, graphio.PhaseValidate, graphio.Location{}, "record", "", "invalid graphio record", err)
		}
	}
	return nil
}

func writeKey(encoder *xml.Encoder, id string, scope keyScope, name string, valueType string) error {
	start := xml.StartElement{Name: xml.Name{Local: "key"}, Attr: []xml.Attr{
		{Name: xml.Name{Local: "id"}, Value: id},
		{Name: xml.Name{Local: "for"}, Value: string(scope)},
		{Name: xml.Name{Local: "attr.name"}, Value: name},
		{Name: xml.Name{Local: "attr.type"}, Value: valueType},
	}}
	if err := encoder.EncodeToken(start); err != nil {
		return fmt.Errorf("write graphml key: %w", err)
	}
	return encoder.EncodeToken(xml.EndElement{Name: start.Name})
}

func writeVertex(encoder *xml.Encoder, vertex graph.Vertex, keys []string) error {
	if err := vertex.Validate(); err != nil {
		return newError(graphio.ErrInvalidRecord, graphio.PhaseWriteVertex, graphio.Location{}, "vertex", vertex.ID().String(), "invalid vertex", err)
	}
	start := xml.StartElement{Name: xml.Name{Local: "node"}, Attr: []xml.Attr{{Name: xml.Name{Local: "id"}, Value: vertex.ID().String()}}}
	if err := encoder.EncodeToken(start); err != nil {
		return fmt.Errorf("write graphml node: %w", err)
	}
	if err := writeData(encoder, "node_label", vertex.Label().String()); err != nil {
		return err
	}
	properties := vertex.Properties()
	for _, key := range keys {
		if value, ok := properties[key]; ok {
			if err := writeData(encoder, "node_"+key, fmt.Sprint(value)); err != nil {
				return err
			}
		}
	}
	return encoder.EncodeToken(xml.EndElement{Name: start.Name})
}

func writeEdge(encoder *xml.Encoder, edge graph.Edge, keys []string) error {
	if err := edge.Validate(); err != nil {
		return newError(graphio.ErrInvalidRecord, graphio.PhaseWriteEdge, graphio.Location{}, "edge", edge.ID().String(), "invalid edge", err)
	}
	start := xml.StartElement{Name: xml.Name{Local: "edge"}, Attr: []xml.Attr{
		{Name: xml.Name{Local: "id"}, Value: edge.ID().String()},
		{Name: xml.Name{Local: "source"}, Value: edge.StartID().String()},
		{Name: xml.Name{Local: "target"}, Value: edge.EndID().String()},
	}}
	if err := encoder.EncodeToken(start); err != nil {
		return fmt.Errorf("write graphml edge: %w", err)
	}
	if err := writeData(encoder, "edge_label", edge.Label().String()); err != nil {
		return err
	}
	properties := edge.Properties()
	for _, key := range keys {
		if value, ok := properties[key]; ok {
			if err := writeData(encoder, "edge_"+key, fmt.Sprint(value)); err != nil {
				return err
			}
		}
	}
	return encoder.EncodeToken(xml.EndElement{Name: start.Name})
}

func writeData(encoder *xml.Encoder, key string, value string) error {
	start := xml.StartElement{Name: xml.Name{Local: "data"}, Attr: []xml.Attr{{Name: xml.Name{Local: "key"}, Value: key}}}
	if err := encoder.EncodeToken(start); err != nil {
		return fmt.Errorf("write graphml data: %w", err)
	}
	if err := encoder.EncodeToken(xml.CharData([]byte(value))); err != nil {
		return fmt.Errorf("write graphml data value: %w", err)
	}
	return encoder.EncodeToken(xml.EndElement{Name: start.Name})
}

func discoverKeys(records []graphio.Record) ([]string, []string) {
	nodeSet, edgeSet := map[string]struct{}{}, map[string]struct{}{}
	for _, record := range records {
		switch record.Kind {
		case graphio.RecordVertex:
			for key := range record.Vertex.Properties() {
				if key != "label" {
					nodeSet[key] = struct{}{}
				}
			}
		case graphio.RecordEdge:
			for key := range record.Edge.Properties() {
				if key != "label" {
					edgeSet[key] = struct{}{}
				}
			}
		}
	}
	return sortedKeys(nodeSet), sortedKeys(edgeSet)
}

func orderedRecords(records []graphio.Record) ([]graphio.Record, []graphio.Record) {
	vertices := make([]graphio.Record, 0)
	edges := make([]graphio.Record, 0)
	for _, record := range records {
		switch record.Kind {
		case graphio.RecordVertex:
			vertices = append(vertices, record)
		case graphio.RecordEdge:
			edges = append(edges, record)
		}
	}
	sort.Slice(vertices, func(i, j int) bool {
		return vertices[i].Vertex.ID().String() < vertices[j].Vertex.ID().String()
	})
	sort.Slice(edges, func(i, j int) bool {
		return edges[i].Edge.ID().String() < edges[j].Edge.ID().String()
	})
	return vertices, edges
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func inferGraphMLType(records []graphio.Record, kind graphio.RecordKind, key string) string {
	valueType := ""
	for _, record := range records {
		var properties graph.Properties
		if kind == graphio.RecordVertex && record.Kind == graphio.RecordVertex {
			properties = record.Vertex.Properties()
		}
		if kind == graphio.RecordEdge && record.Kind == graphio.RecordEdge {
			properties = record.Edge.Properties()
		}
		value, ok := properties[key]
		if !ok || value == nil {
			continue
		}
		current := typeForValue(value)
		if valueType == "" {
			valueType = current
			continue
		}
		if valueType != current {
			return "string"
		}
	}
	if valueType == "" {
		return "string"
	}
	return valueType
}

func typeForValue(value any) string {
	switch value.(type) {
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32:
		return "long"
	case float32, float64:
		return "double"
	default:
		return "string"
	}
}

func newError(kind error, phase graphio.Phase, loc graphio.Location, field string, recordID string, summary string, cause error) error {
	return graphio.NewError(kind, FormatGraphML, phase, loc, field, recordID, summary, cause)
}
