package falkordb

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func validGraphName(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for i, r := range value {
		if !(r == '_' || r == '-' || unicode.IsLetter(r) || i > 0 && unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

func buildQueryCommand(graphName, query string, params map[string]any, timeout time.Duration) ([]any, error) {
	query = strings.TrimSpace(query)
	if query == "" || len(query) > maxQueryBytes {
		return nil, fmt.Errorf("%w: query is empty or too large", ErrInvalidQuery)
	}
	for key := range params {
		if !validParameterName(key) {
			return nil, fmt.Errorf("%w: parameter name is invalid", ErrInvalidQuery)
		}
	}
	if len(params) > 0 {
		keys := make([]string, 0, len(params))
		for key := range params {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var header strings.Builder
		header.WriteString("CYPHER ")
		for _, key := range keys {
			value, err := formatValue(params[key])
			if err != nil {
				return nil, err
			}
			header.WriteString(key)
			header.WriteByte('=')
			header.WriteString(value)
			header.WriteByte(' ')
		}
		query = header.String() + query
	}
	command := []any{"GRAPH.QUERY", graphName, query, "--compact"}
	if milliseconds, ok := serverTimeout(timeout); ok {
		command = append(command, "timeout", milliseconds)
	}
	return command, nil
}

func validParameterName(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if !(r == '_' || unicode.IsLetter(r) || i > 0 && unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

func formatValue(value any) (string, error) {
	switch value := value.(type) {
	case nil:
		return "null", nil
	case string:
		return strconv.Quote(value), nil
	case bool:
		return strconv.FormatBool(value), nil
	case int:
		return strconv.Itoa(value), nil
	case int8:
		return strconv.FormatInt(int64(value), 10), nil
	case int16:
		return strconv.FormatInt(int64(value), 10), nil
	case int32:
		return strconv.FormatInt(int64(value), 10), nil
	case int64:
		return strconv.FormatInt(value, 10), nil
	case uint:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(value), 10), nil
	case uint64:
		return strconv.FormatUint(value, 10), nil
	case float32:
		return strconv.FormatFloat(float64(value), 'g', -1, 32), nil
	case float64:
		return strconv.FormatFloat(value, 'g', -1, 64), nil
	case []string:
		parts := make([]string, len(value))
		for i, item := range value {
			parts[i] = strconv.Quote(item)
		}
		return "[" + strings.Join(parts, ",") + "]", nil
	case []any:
		parts := make([]string, len(value))
		for i, item := range value {
			formatted, err := formatValue(item)
			if err != nil {
				return "", err
			}
			parts[i] = formatted
		}
		return "[" + strings.Join(parts, ",") + "]", nil
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			if !validParameterName(key) {
				return "", fmt.Errorf("%w: map key is invalid", ErrInvalidQuery)
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			formatted, err := formatValue(value[key])
			if err != nil {
				return "", err
			}
			parts = append(parts, key+":"+formatted)
		}
		return "{" + strings.Join(parts, ",") + "}", nil
	default:
		return "", fmt.Errorf("%w: unsupported parameter type", ErrInvalidQuery)
	}
}

func parseResult(value any, maxRows int) (Result, error) {
	outer, ok := asSlice(value)
	if !ok || len(outer) == 0 {
		return Result{}, classified(ErrInvalidResult, nil)
	}
	if len(outer) == 1 {
		statsRaw, ok := asSlice(outer[0])
		if !ok {
			return Result{}, classified(ErrInvalidResult, nil)
		}
		statistics, ok := parseStatistics(statsRaw)
		if !ok {
			return Result{}, classified(ErrInvalidResult, nil)
		}
		return Result{Statistics: statistics}, nil
	}
	if len(outer) < 2 {
		return Result{}, classified(ErrInvalidResult, nil)
	}
	headerRaw, ok := asSlice(outer[0])
	if !ok {
		return Result{}, classified(ErrInvalidResult, nil)
	}
	columns := make([]string, len(headerRaw))
	for i, item := range headerRaw {
		pair, ok := asSlice(item)
		if !ok || len(pair) < 2 {
			return Result{}, classified(ErrInvalidResult, nil)
		}
		columns[i] = fmt.Sprint(pair[len(pair)-1])
	}
	rowsRaw, ok := asSlice(outer[1])
	if !ok || len(rowsRaw) > maxRows {
		return Result{}, classified(ErrInvalidResult, nil)
	}
	rows := make([][]any, len(rowsRaw))
	for i, item := range rowsRaw {
		row, ok := asSlice(item)
		if !ok || len(row) != len(columns) {
			return Result{}, classified(ErrInvalidResult, nil)
		}
		rows[i] = append([]any(nil), row...)
	}
	var statistics []string
	if len(outer) > 2 {
		statsRaw, ok := asSlice(outer[2])
		if !ok {
			return Result{}, classified(ErrInvalidResult, nil)
		}
		statistics, ok = parseStatistics(statsRaw)
		if !ok {
			return Result{}, classified(ErrInvalidResult, nil)
		}
	}
	return Result{Columns: columns, Rows: rows, Statistics: statistics}, nil
}

func parseStatistics(raw []any) ([]string, bool) {
	statistics := make([]string, len(raw))
	for i, item := range raw {
		value, ok := item.(string)
		if !ok {
			return nil, false
		}
		statistics[i] = value
	}
	return statistics, true
}

func asSlice(value any) ([]any, bool) {
	switch value := value.(type) {
	case []interface{}:
		return value, true
	default:
		return nil, false
	}
}
