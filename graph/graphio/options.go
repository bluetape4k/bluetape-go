package graphio

const (
	defaultMaxLineBytes   = 1 << 20
	defaultMaxRecordBytes = 1 << 20
	defaultMaxFieldBytes  = 256 << 10
	defaultMaxColumns     = 1024
	defaultMaxRecords     = 1_000_000
	defaultMaxFailures    = 100
)

// UnlimitedRecords opts into no record-count cap for trusted bounded inputs.
const UnlimitedRecords int64 = -1

// Format identifies a graph wire format.
type Format string

const (
	// FormatNDJSON identifies newline-delimited JSON graph records.
	FormatNDJSON Format = "ndjson"
	// FormatCSV identifies paired vertex and edge CSV records.
	FormatCSV Format = "csv"
)

// DuplicateVertexPolicy controls duplicate vertex handling.
type DuplicateVertexPolicy string

const (
	// DuplicateVertexFail rejects duplicate vertex IDs.
	DuplicateVertexFail DuplicateVertexPolicy = "fail"
	// DuplicateVertexSkip skips duplicate vertex rows.
	DuplicateVertexSkip DuplicateVertexPolicy = "skip"
)

// MissingEndpointPolicy controls missing edge endpoint handling.
type MissingEndpointPolicy string

const (
	// MissingEndpointFail rejects edges whose endpoints were not seen.
	MissingEndpointFail MissingEndpointPolicy = "fail"
	// MissingEndpointSkipEdge skips edges whose endpoints were not seen.
	MissingEndpointSkipEdge MissingEndpointPolicy = "skip_edge"
)

// ReadOptions configures graph imports.
type ReadOptions struct {
	DuplicateVertexPolicy DuplicateVertexPolicy
	MissingEndpointPolicy MissingEndpointPolicy
	MaxLineBytes          int
	MaxRecordBytes        int
	MaxFieldBytes         int
	MaxColumns            int
	MaxRecords            int64
	MaxFailures           int
}

// WriteOptions configures graph exports.
type WriteOptions struct {
	IncludeEmptyProperties bool
	MaxFailures            int
}

func normalizeWriteOptions(options WriteOptions) (WriteOptions, error) {
	if options.MaxFailures == 0 {
		options.MaxFailures = defaultMaxFailures
	}
	if options.MaxFailures < 0 {
		return options, optionError("max failures must not be negative")
	}
	return options, nil
}

// NormalizeReadOptions applies fail-closed safe defaults.
func NormalizeReadOptions(options ReadOptions) (ReadOptions, error) {
	if options.DuplicateVertexPolicy == "" {
		options.DuplicateVertexPolicy = DuplicateVertexFail
	}
	if options.MissingEndpointPolicy == "" {
		options.MissingEndpointPolicy = MissingEndpointFail
	}
	if options.DuplicateVertexPolicy != DuplicateVertexFail && options.DuplicateVertexPolicy != DuplicateVertexSkip {
		return options, optionError("unknown duplicate vertex policy")
	}
	if options.MissingEndpointPolicy != MissingEndpointFail && options.MissingEndpointPolicy != MissingEndpointSkipEdge {
		return options, optionError("unknown missing endpoint policy")
	}
	if options.MaxLineBytes == 0 {
		options.MaxLineBytes = defaultMaxLineBytes
	}
	if options.MaxRecordBytes == 0 {
		options.MaxRecordBytes = defaultMaxRecordBytes
	}
	if options.MaxFieldBytes == 0 {
		options.MaxFieldBytes = defaultMaxFieldBytes
	}
	if options.MaxColumns == 0 {
		options.MaxColumns = defaultMaxColumns
	}
	if options.MaxRecords == 0 {
		options.MaxRecords = defaultMaxRecords
	}
	if options.MaxFailures == 0 {
		options.MaxFailures = defaultMaxFailures
	}
	if options.MaxLineBytes < 0 || options.MaxRecordBytes < 0 || options.MaxFieldBytes < 0 || options.MaxColumns < 0 || options.MaxFailures < 0 {
		return options, optionError("limits must not be negative")
	}
	if options.MaxRecords < 0 && options.MaxRecords != UnlimitedRecords {
		return options, optionError("max records must not be negative")
	}
	return options, nil
}

func checkRecordLimit(options ReadOptions, count int64) error {
	if options.MaxRecords != UnlimitedRecords && count > options.MaxRecords {
		return wrap(ErrMalformedInput, "", PhaseValidate, Location{}, "", "", "record limit exceeded", nil)
	}
	return nil
}

// CSVPropertyMode selects how CSV properties are encoded.
type CSVPropertyMode string

const (
	// CSVPropertiesPrefixedColumns maps properties to columns with a prefix.
	CSVPropertiesPrefixedColumns CSVPropertyMode = "prefixed_columns"
	// CSVPropertiesRawJSONColumn maps all properties to one JSON column.
	CSVPropertiesRawJSONColumn CSVPropertyMode = "raw_json_column"
	// CSVPropertiesNone omits properties.
	CSVPropertiesNone CSVPropertyMode = "none"
)

// CSVFormulaPolicy selects CSV formula escaping behavior.
type CSVFormulaPolicy string

const (
	// CSVFormulaEscape prefixes formula-like cells with a quote.
	CSVFormulaEscape CSVFormulaPolicy = "escape"
	// CSVFormulaRaw writes cells without spreadsheet formula escaping.
	CSVFormulaRaw CSVFormulaPolicy = "raw"
)

// CSVWriteOptions configures paired CSV exports.
type CSVWriteOptions struct {
	WriteOptions
	PropertyMode        CSVPropertyMode
	PropertyPrefix      string
	RawPropertiesColumn string
	FormulaPolicy       CSVFormulaPolicy
	PropertyColumns     []string
}

// CSVReadOptions configures paired CSV imports.
type CSVReadOptions struct {
	ReadOptions
	PropertyMode        CSVPropertyMode
	PropertyPrefix      string
	RawPropertiesColumn string
}

func normalizeCSVWriteOptions(options CSVWriteOptions) (CSVWriteOptions, error) {
	write, err := normalizeWriteOptions(options.WriteOptions)
	if err != nil {
		return options, err
	}
	options.WriteOptions = write
	if options.PropertyMode == "" {
		options.PropertyMode = CSVPropertiesPrefixedColumns
	}
	if options.PropertyMode != CSVPropertiesPrefixedColumns && options.PropertyMode != CSVPropertiesRawJSONColumn && options.PropertyMode != CSVPropertiesNone {
		return options, optionError("unknown csv property mode")
	}
	if options.PropertyPrefix == "" {
		options.PropertyPrefix = "prop."
	}
	if options.RawPropertiesColumn == "" {
		options.RawPropertiesColumn = "properties"
	}
	if options.FormulaPolicy == "" {
		options.FormulaPolicy = CSVFormulaEscape
	}
	if options.FormulaPolicy != CSVFormulaEscape && options.FormulaPolicy != CSVFormulaRaw {
		return options, optionError("unknown csv formula policy")
	}
	return options, nil
}

func normalizeCSVReadOptions(options CSVReadOptions) (CSVReadOptions, error) {
	read, err := NormalizeReadOptions(options.ReadOptions)
	if err != nil {
		return options, err
	}
	options.ReadOptions = read
	if options.PropertyMode == "" {
		options.PropertyMode = CSVPropertiesPrefixedColumns
	}
	if options.PropertyMode != CSVPropertiesPrefixedColumns && options.PropertyMode != CSVPropertiesRawJSONColumn && options.PropertyMode != CSVPropertiesNone {
		return options, optionError("unknown csv property mode")
	}
	if options.PropertyPrefix == "" {
		options.PropertyPrefix = "prop."
	}
	if options.RawPropertiesColumn == "" {
		options.RawPropertiesColumn = "properties"
	}
	return options, nil
}
