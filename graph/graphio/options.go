package graphio

const (
	defaultMaxLineBytes   = 1 << 20
	defaultMaxRecordBytes = 1 << 20
	defaultMaxFieldBytes  = 256 << 10
	defaultMaxColumns     = 1024
	defaultMaxRecords     = 1_000_000
	defaultMaxFailures    = 100
)

// UnlimitedRecords graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
const UnlimitedRecords int64 = -1

// Format graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
type Format string

const (
	// FormatNDJSON graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	FormatNDJSON Format = "ndjson"
	// FormatCSV graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
	FormatCSV Format = "csv"
)

// DuplicateVertexPolicy graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
type DuplicateVertexPolicy string

const (
	// DuplicateVertexFail graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
	DuplicateVertexFail DuplicateVertexPolicy = "fail"
	// DuplicateVertexSkip graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
	DuplicateVertexSkip DuplicateVertexPolicy = "skip"
)

// MissingEndpointPolicy graph IO Neo4j backend에서 caller-visible 상태와 의미를 설명한다.
type MissingEndpointPolicy string

const (
	// MissingEndpointFail graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
	MissingEndpointFail MissingEndpointPolicy = "fail"
	// MissingEndpointSkipEdge graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
	MissingEndpointSkipEdge MissingEndpointPolicy = "skip_edge"
)

// ReadOptions graph IO Neo4j backend에서 설정값과 기본값 적용 방식을 설명한다.
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

// WriteOptions graph IO Neo4j backend에서 설정값과 기본값 적용 방식을 설명한다.
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

// NormalizeReadOptions graph IO Neo4j backend에서 동작과 caller-visible 계약을 설명한다.
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

// CSVPropertyMode CSV property column을 쓰고 읽는 방식을 선택한다.
type CSVPropertyMode string

const (
	// CSVPropertiesPrefixedColumns property를 prefix column으로 펼친다.
	CSVPropertiesPrefixedColumns CSVPropertyMode = "prefixed_columns"
	// CSVPropertiesRawJSONColumn property를 JSON column 하나에 보관한다.
	CSVPropertiesRawJSONColumn CSVPropertyMode = "raw_json_column"
	// CSVPropertiesNone property column을 쓰지 않는다.
	CSVPropertiesNone CSVPropertyMode = "none"
)

// CSVFormulaPolicy spreadsheet formula injection 방지 방식을 선택한다.
type CSVFormulaPolicy string

const (
	// CSVFormulaEscape formula처럼 보이는 cell을 escape한다.
	CSVFormulaEscape CSVFormulaPolicy = "escape"
	// CSVFormulaRaw cell 값을 escape하지 않고 그대로 쓴다.
	CSVFormulaRaw CSVFormulaPolicy = "raw"
)

// CSVWriteOptions CSV writer 동작을 조정한다.
type CSVWriteOptions struct {
	WriteOptions
	PropertyMode        CSVPropertyMode
	PropertyPrefix      string
	RawPropertiesColumn string
	FormulaPolicy       CSVFormulaPolicy
	PropertyColumns     []string
}

// CSVReadOptions CSV reader 동작을 조정한다.
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
