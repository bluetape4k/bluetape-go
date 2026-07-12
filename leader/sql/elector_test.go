package sqlleader

import (
	"database/sql"
	"testing"
	"time"

	"github.com/bluetape4k/bluetape-go/leader"
)

func TestNewValidatesInputs(t *testing.T) {
	valid := leader.Options{
		Group:         "billing",
		MemberID:      "worker-1",
		Lease:         time.Second,
		RenewInterval: 100 * time.Millisecond,
	}
	if _, err := New(nil, valid); err == nil {
		t.Fatal("New(nil) succeeded")
	}

	db := &sql.DB{}
	if _, err := New(db, leader.Options{}); err == nil {
		t.Fatal("New accepted invalid identities")
	}
	for _, renew := range []time.Duration{time.Second, 2 * time.Second} {
		opts := valid
		opts.RenewInterval = renew
		if _, err := New(db, opts); err == nil {
			t.Fatalf("New accepted renew=%s lease=%s", renew, opts.Lease)
		}
	}
}

func TestNewDoesNotTouchDatabase(t *testing.T) {
	db := &sql.DB{}
	elector, err := New(db, leader.Options{Group: "billing", MemberID: "worker-1"})
	if err != nil || elector == nil {
		t.Fatalf("New() elector=%v err=%v", elector, err)
	}
}
