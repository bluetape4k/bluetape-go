package postgis

import (
	"fmt"
	"math"
	"strings"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

// CreateSpatialTableSQL 는 SRID 제약이 있는 PostGIS Point 테이블 DDL을 만든다.
func CreateSpatialTableSQL(table, column string, srid int) (string, error) {
	quotedTable, quotedColumn, err := quoteTableColumn(table, column)
	if err != nil {
		return "", err
	}
	if err := validateSRID(srid); err != nil {
		return "", err
	}
	return fmt.Sprintf("CREATE TABLE %s (%s geometry(Point, %d) NOT NULL)", quotedTable, quotedColumn, srid), nil
}

// CreateSpatialIndexSQL 는 PostGIS geometry column에 대한 GIST index DDL을 만든다.
func CreateSpatialIndexSQL(table, column string) (string, error) {
	quotedTable, quotedColumn, err := quoteTableColumn(table, column)
	if err != nil {
		return "", err
	}
	indexName := strings.ReplaceAll(table+"_"+column, ".", "_") + "_gist_idx"
	quotedIndex, err := quoteIdentifier(indexName)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("CREATE INDEX %s ON %s USING GIST (%s)", quotedIndex, quotedTable, quotedColumn), nil
}

// InsertPoint 는 EWKB를 bind argument로 사용하는 point insert statement를 만든다.
func InsertPoint(table, column string, point Point) (sqlkit.Statement, error) {
	quotedTable, quotedColumn, err := quoteTableColumn(table, column)
	if err != nil {
		return sqlkit.Statement{}, err
	}
	raw, err := point.MarshalEWKB()
	if err != nil {
		return sqlkit.Statement{}, err
	}
	return sqlkit.NewStatement(
		fmt.Sprintf("INSERT INTO %s (%s) VALUES (ST_GeomFromEWKB($1))", quotedTable, quotedColumn),
		raw,
	), nil
}

// WithinDistance 는 SRID가 같은 geometry에서 거리 단위의 indexed predicate를 만든다.
func WithinDistance(table, column string, center Point, distance float64) (sqlkit.Statement, error) {
	quotedTable, quotedColumn, err := quoteTableColumn(table, column)
	if err != nil {
		return sqlkit.Statement{}, err
	}
	if math.IsNaN(distance) || math.IsInf(distance, 0) || distance < 0 {
		return sqlkit.Statement{}, ErrInvalidDistance
	}
	if !center.Valid {
		return sqlkit.Statement{}, fmt.Errorf("%w: center is invalid", ErrInvalidPoint)
	}
	raw, err := center.MarshalEWKB()
	if err != nil {
		return sqlkit.Statement{}, err
	}
	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE ST_DWithin(%s, ST_SetSRID(ST_GeomFromEWKB($1), %d), $2)",
		quotedTable, quotedColumn, center.SRID,
	)
	return sqlkit.NewStatement(query, raw, distance), nil
}

// WithinBounds 는 inclusive bounding box predicate statement를 만든다.
func WithinBounds(table, column string, minX, minY, maxX, maxY float64, srid int) (sqlkit.Statement, error) {
	quotedTable, quotedColumn, err := quoteTableColumn(table, column)
	if err != nil {
		return sqlkit.Statement{}, err
	}
	if err := validateSRID(srid); err != nil {
		return sqlkit.Statement{}, err
	}
	if err := validateCoordinates(minX, minY); err != nil {
		return sqlkit.Statement{}, err
	}
	if err := validateCoordinates(maxX, maxY); err != nil {
		return sqlkit.Statement{}, err
	}
	if minX > maxX || minY > maxY {
		return sqlkit.Statement{}, fmt.Errorf("%w: bounds are inverted", ErrInvalidArgument)
	}
	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s && ST_MakeEnvelope($1, $2, $3, $4, %d)",
		quotedTable, quotedColumn, srid,
	)
	return sqlkit.NewStatement(query, minX, minY, maxX, maxY), nil
}

func quoteTableColumn(table, column string) (string, string, error) {
	quotedTable, err := quoteIdentifier(table)
	if err != nil {
		return "", "", err
	}
	quotedColumn, err := quoteIdentifier(column)
	if err != nil {
		return "", "", err
	}
	return quotedTable, quotedColumn, nil
}

func quoteIdentifier(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("%w: empty identifier", ErrInvalidArgument)
	}
	segments := strings.Split(value, ".")
	quoted := make([]string, len(segments))
	for index, segment := range segments {
		if segment == "" {
			return "", fmt.Errorf("%w: invalid identifier", ErrInvalidArgument)
		}
		for position, r := range segment {
			valid := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (position > 0 && r >= '0' && r <= '9')
			if !valid {
				return "", fmt.Errorf("%w: invalid identifier", ErrInvalidArgument)
			}
		}
		quoted[index] = `"` + segment + `"`
	}
	return strings.Join(quoted, "."), nil
}
