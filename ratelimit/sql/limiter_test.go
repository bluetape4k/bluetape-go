package sqlratelimit

import (
	"database/sql"
	"testing"
)

func TestNewValidatesDatabaseAndDoesNotTouchIt(t *testing.T) {
	if _, err := New(nil, Options{RatePerSecond: 1, Burst: 1}); err == nil {
		t.Fatal("New(nil) succeeded")
	}
	db := &sql.DB{}
	limiter, err := New(db, Options{RatePerSecond: 1, Burst: 1})
	if err != nil || limiter == nil {
		t.Fatalf("New() = %v, %v", limiter, err)
	}
	if limiter.db != db {
		t.Fatal("New did not retain caller-owned database")
	}
}
