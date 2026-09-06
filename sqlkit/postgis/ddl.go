package postgis

import "fmt"

// CreateExtensionSQL은 PostGIS extension을 명시적으로 활성화하는 SQL을 반환한다.
func CreateExtensionSQL() string {
	return "CREATE EXTENSION IF NOT EXISTS postgis"
}

// SelectPointSQL은 geometry column의 WKB와 SRID를 함께 읽는 SQL statement를 만든다.
func SelectPointSQL(table, column string) (string, error) {
	quotedTable, quotedColumn, err := quoteTableColumn(table, column)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("SELECT ST_AsEWKB(%s), ST_SRID(%s) FROM %s", quotedColumn, quotedColumn, quotedTable), nil
}
