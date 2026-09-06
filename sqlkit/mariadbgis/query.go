package mariadbgis

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

// InsertPoint 는 ST_PointFromWKB와 bind argument를 사용하는 MariaDB insert statement를 만든다.
func InsertPoint(table, column string, point Point) (sqlkit.Statement, error) {
	quotedTable, quotedColumn, err := quoteTableColumn(table, column)
	if err != nil {
		return sqlkit.Statement{}, err
	}
	if !point.Valid {
		return sqlkit.Statement{}, fmt.Errorf("%w: point is invalid", ErrInvalidPoint)
	}
	raw, err := point.MarshalWKB()
	if err != nil {
		return sqlkit.Statement{}, err
	}
	return sqlkit.NewStatement(fmt.Sprintf("INSERT INTO %s (%s) VALUES (ST_PointFromWKB(CAST(? AS BINARY), %d))", quotedTable, quotedColumn, point.SRID), raw), nil
}

// SelectPointSQL 는 ST_AsBinary와 ST_SRID를 함께 읽어 SRID를 잃지 않게 한다.
func SelectPointSQL(table, column string) (string, error) {
	quotedTable, quotedColumn, err := quoteTableColumn(table, column)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("SELECT ST_AsBinary(%s), ST_SRID(%s) FROM %s", quotedColumn, quotedColumn, quotedTable), nil
}

// WithinDistance 는 미터 단위 ST_Distance_Sphere predicate를 만든다.
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
	raw, err := center.MarshalWKB()
	if err != nil {
		return sqlkit.Statement{}, err
	}
	return sqlkit.NewStatement(fmt.Sprintf("SELECT * FROM %s WHERE ST_Distance_Sphere(%s, ST_PointFromWKB(CAST(? AS BINARY), %d)) <= ?", quotedTable, quotedColumn, center.SRID), raw, distance), nil
}

// WithinBounds 는 MBRContains와 bind된 polygon WKB를 사용하는 spatial index predicate를 만든다.
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
	return sqlkit.NewStatement(fmt.Sprintf("SELECT * FROM %s WHERE MBRContains(ST_PolyFromWKB(CAST(? AS BINARY), %d), %s)", quotedTable, srid, quotedColumn), envelopeWKB(minX, minY, maxX, maxY)), nil
}

func envelopeWKB(minX, minY, maxX, maxY float64) []byte {
	raw := make([]byte, 1+4+4+4+5*16)
	raw[0] = 1
	binary.LittleEndian.PutUint32(raw[1:5], 3)
	binary.LittleEndian.PutUint32(raw[5:9], 1)
	binary.LittleEndian.PutUint32(raw[9:13], 5)
	coords := [][2]float64{{minX, minY}, {maxX, minY}, {maxX, maxY}, {minX, maxY}, {minX, minY}}
	offset := 13
	for _, coordinate := range coords {
		binary.LittleEndian.PutUint64(raw[offset:offset+8], math.Float64bits(coordinate[0]))
		binary.LittleEndian.PutUint64(raw[offset+8:offset+16], math.Float64bits(coordinate[1]))
		offset += 16
	}
	return raw
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
	for i, segment := range segments {
		if segment == "" {
			return "", fmt.Errorf("%w: invalid identifier", ErrInvalidArgument)
		}
		for pos, r := range segment {
			valid := r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || pos > 0 && r >= '0' && r <= '9'
			if !valid {
				return "", fmt.Errorf("%w: invalid identifier", ErrInvalidArgument)
			}
		}
		quoted[i] = "`" + segment + "`"
	}
	return strings.Join(quoted, "."), nil
}
