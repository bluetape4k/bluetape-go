package postgis_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/sqlkit/postgis"
)

func TestSpatialDDLQuotesIdentifiersAndBindsSRID(t *testing.T) {
	if got, want := postgis.CreateExtensionSQL(), "CREATE EXTENSION IF NOT EXISTS postgis"; got != want {
		t.Fatalf("extension SQL = %q, want %q", got, want)
	}

	create, err := postgis.CreateSpatialTableSQL("places", "location", 4326)
	if err != nil {
		t.Fatalf("CreateSpatialTableSQL failed: %v", err)
	}
	if want := `CREATE TABLE "places" ("location" geometry(Point, 4326) NOT NULL)`; create != want {
		t.Fatalf("create SQL = %q, want %q", create, want)
	}

	index, err := postgis.CreateSpatialIndexSQL("places", "location")
	if err != nil {
		t.Fatalf("CreateSpatialIndexSQL failed: %v", err)
	}
	if want := `CREATE INDEX "places_location_gist_idx" ON "places" USING GIST ("location")`; index != want {
		t.Fatalf("index SQL = %q, want %q", index, want)
	}
	selectSQL, err := postgis.SelectPointSQL("places", "location")
	if err != nil {
		t.Fatalf("SelectPointSQL failed: %v", err)
	}
	if want := `SELECT ST_AsEWKB("location"), ST_SRID("location") FROM "places"`; selectSQL != want {
		t.Fatalf("select SQL = %q, want %q", selectSQL, want)
	}
}

func TestSpatialQueriesUseEWKBExpressionAndCopiedArgs(t *testing.T) {
	point, err := postgis.NewPoint(127, 37, 4326)
	if err != nil {
		t.Fatalf("NewPoint failed: %v", err)
	}

	insert, err := postgis.InsertPoint("places", "location", point)
	if err != nil {
		t.Fatalf("InsertPoint failed: %v", err)
	}
	if !strings.Contains(insert.SQL, "ST_GeomFromEWKB($1)") || len(insert.Args) != 1 {
		t.Fatalf("insert = %#v, want bound EWKB", insert)
	}
	insert.Args[0] = nil
	if !point.Valid {
		t.Fatalf("statement mutation changed point")
	}

	query, err := postgis.WithinDistance("places", "location", point, 500)
	if err != nil {
		t.Fatalf("WithinDistance failed: %v", err)
	}
	if !strings.Contains(query.SQL, "ST_DWithin(\"location\", ST_SetSRID(ST_GeomFromEWKB($1), 4326), $2)") {
		t.Fatalf("distance SQL = %q", query.SQL)
	}
	if len(query.Args) != 2 || query.Args[1] != 500.0 {
		t.Fatalf("distance args = %#v, want point and radius", query.Args)
	}
}

func TestSpatialHelpersRejectUnsafeIdentifiersAndValues(t *testing.T) {
	if _, err := postgis.CreateSpatialTableSQL("places;drop", "location", 4326); !errors.Is(err, postgis.ErrInvalidArgument) {
		t.Fatalf("unsafe table error = %v, want ErrInvalidArgument", err)
	}
	if _, err := postgis.CreateSpatialTableSQL("places", "location", -1); !errors.Is(err, postgis.ErrInvalidSRID) {
		t.Fatalf("invalid SRID error = %v, want ErrInvalidSRID", err)
	}
	if _, err := postgis.WithinDistance("places", "location", postgis.Point{}, -1); !errors.Is(err, postgis.ErrInvalidDistance) {
		t.Fatalf("invalid distance error = %v, want ErrInvalidDistance", err)
	}
}
