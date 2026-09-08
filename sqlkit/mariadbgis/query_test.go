package mariadbgis_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/sqlkit/mariadbgis"
)

func TestMariaDBSpatialDDLAndQueries(t *testing.T) {
	create, err := mariadbgis.CreateSpatialTableSQL("places", "location", 4326)
	if err != nil {
		t.Fatal(err)
	}
	if want := "CREATE TABLE `places` (`location` POINT REF_SYSTEM_ID=4326 NOT NULL)"; create != want {
		t.Fatalf("create=%q want=%q", create, want)
	}
	index, err := mariadbgis.CreateSpatialIndexSQL("places", "location")
	if err != nil {
		t.Fatal(err)
	}
	if want := "CREATE SPATIAL INDEX `places_location_spatial_idx` ON `places` (`location`)"; index != want {
		t.Fatalf("index=%q want=%q", index, want)
	}
	point, _ := mariadbgis.NewPoint(127, 37, 4326)
	insert, err := mariadbgis.InsertPoint("places", "location", point)
	if err != nil || !strings.Contains(insert.SQL, "ST_PointFromWKB(CAST(? AS BINARY), 4326)") || len(insert.Args) != 1 {
		t.Fatalf("insert=%#v err=%v", insert, err)
	}
	distance, err := mariadbgis.WithinDistance("places", "location", point, 100)
	if err != nil || !strings.Contains(distance.SQL, "ST_Distance_Sphere") || len(distance.Args) != 2 {
		t.Fatalf("distance=%#v err=%v", distance, err)
	}
	bounds, err := mariadbgis.WithinBounds("places", "location", 126, 36, 128, 38, 4326)
	if err != nil || !strings.Contains(bounds.SQL, "ST_PolyFromWKB") || len(bounds.Args) != 1 {
		t.Fatalf("bounds=%#v err=%v", bounds, err)
	}
}

func TestMariaDBSpatialRejectsUnsafeInputs(t *testing.T) {
	if _, err := mariadbgis.CreateSpatialTableSQL("places;drop", "location", 4326); !errors.Is(err, mariadbgis.ErrInvalidArgument) {
		t.Fatalf("unsafe identifier err=%v", err)
	}
	if _, err := mariadbgis.WithinDistance("places", "location", mariadbgis.Point{}, -1); !errors.Is(err, mariadbgis.ErrInvalidDistance) {
		t.Fatalf("invalid distance err=%v", err)
	}
}
