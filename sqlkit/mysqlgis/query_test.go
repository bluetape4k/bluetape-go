package mysqlgis_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/sqlkit/mysqlgis"
)

func TestMySQLSpatialDDLAndQueries(t *testing.T) {
	create, err := mysqlgis.CreateSpatialTableSQL("places", "location", 4326)
	if err != nil {
		t.Fatal(err)
	}
	if want := "CREATE TABLE `places` (`location` POINT SRID 4326 NOT NULL)"; create != want {
		t.Fatalf("create=%q want=%q", create, want)
	}
	index, err := mysqlgis.CreateSpatialIndexSQL("places", "location")
	if err != nil {
		t.Fatal(err)
	}
	if want := "CREATE SPATIAL INDEX `places_location_spatial_idx` ON `places` (`location`)"; index != want {
		t.Fatalf("index=%q want=%q", index, want)
	}
	point, _ := mysqlgis.NewPoint(127, 37, 4326)
	insert, err := mysqlgis.InsertPoint("places", "location", point)
	if err != nil || !strings.Contains(insert.SQL, "ST_GeomFromWKB(?, 4326, 'axis-order=long-lat')") || len(insert.Args) != 1 {
		t.Fatalf("insert=%#v err=%v", insert, err)
	}
	distance, err := mysqlgis.WithinDistance("places", "location", point, 100)
	if err != nil || !strings.Contains(distance.SQL, "ST_Distance_Sphere") || len(distance.Args) != 2 {
		t.Fatalf("distance=%#v err=%v", distance, err)
	}
	bounds, err := mysqlgis.WithinBounds("places", "location", 126, 36, 128, 38, 4326)
	if err != nil || !strings.Contains(bounds.SQL, "MBRContains") || len(bounds.Args) != 1 {
		t.Fatalf("bounds=%#v err=%v", bounds, err)
	}
	selectSQL, err := mysqlgis.SelectPointSQL("places", "location")
	if err != nil || !strings.Contains(selectSQL, "ST_SRID") {
		t.Fatalf("select=%q err=%v", selectSQL, err)
	}
}

func TestMySQLSpatialRejectsUnsafeInputs(t *testing.T) {
	if _, err := mysqlgis.CreateSpatialTableSQL("places;drop", "location", 4326); !errors.Is(err, mysqlgis.ErrInvalidArgument) {
		t.Fatalf("unsafe identifier err=%v", err)
	}
	if _, err := mysqlgis.WithinDistance("places", "location", mysqlgis.Point{}, -1); !errors.Is(err, mysqlgis.ErrInvalidDistance) {
		t.Fatalf("invalid distance err=%v", err)
	}
}
