package mysqlgis

import (
	"fmt"
	"strings"
)

// CreateSpatialTableSQL은 MySQL SRID 제약과 NOT NULL Point column DDL을 만든다.
func CreateSpatialTableSQL(table, column string, srid int) (string, error) {
	quotedTable, quotedColumn, err := quoteTableColumn(table, column)
	if err != nil {
		return "", err
	}
	if err := validateSRID(srid); err != nil {
		return "", err
	}
	return fmt.Sprintf("CREATE TABLE %s (%s POINT SRID %d NOT NULL)", quotedTable, quotedColumn, srid), nil
}

// CreateSpatialIndexSQL은 MySQL SPATIAL INDEX DDL을 만든다.
func CreateSpatialIndexSQL(table, column string) (string, error) {
	quotedTable, quotedColumn, err := quoteTableColumn(table, column)
	if err != nil {
		return "", err
	}
	indexName, err := quoteIdentifier(strings.ReplaceAll(table+"_"+column, ".", "_") + "_spatial_idx")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("CREATE SPATIAL INDEX %s ON %s (%s)", indexName, quotedTable, quotedColumn), nil
}
