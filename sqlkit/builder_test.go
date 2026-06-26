package sqlkit_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/bluetape4k/bluetape-go/sqlkit"
)

func TestSelectBuilderBuildsInspectableSQLAndArgs(t *testing.T) {
	stmt, err := sqlkit.SelectFrom("accounts").
		Columns("id", "name").
		Where("id = ?", 42).
		OrderBy("id").
		Limit(1).
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	assertStatement(t, stmt, `select "id", "name" from "accounts" where id = $1 order by "id" limit 1`, []any{42})
}

func TestInsertBuilderBuildsInspectableSQLAndArgs(t *testing.T) {
	stmt, err := sqlkit.InsertInto("accounts").
		Columns("id", "name").
		Values(42, "Ada").
		Returning("id").
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	assertStatement(t, stmt, `insert into "accounts" ("id", "name") values ($1, $2) returning "id"`, []any{42, "Ada"})
}

func TestUpdateBuilderBuildsInspectableSQLAndArgs(t *testing.T) {
	stmt, err := sqlkit.Update("accounts").
		Set("name", "Grace").
		Where("id = ?", 42).
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	assertStatement(t, stmt, `update "accounts" set "name" = $1 where id = $2`, []any{"Grace", 42})
}

func TestDeleteBuilderBuildsInspectableSQLAndArgs(t *testing.T) {
	stmt, err := sqlkit.DeleteFrom("accounts").
		Where("id = ?", 42).
		Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	assertStatement(t, stmt, `delete from "accounts" where id = $1`, []any{42})
}

func TestBuilderRejectsInvalidIdentifiers(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "select table",
			err:  selectErr(sqlkit.SelectFrom(`accounts; drop table accounts`).Build()),
		},
		{
			name: "insert column",
			err:  selectErr(sqlkit.InsertInto("accounts").Columns("name;drop").Values("Ada").Build()),
		},
		{
			name: "update set column",
			err:  selectErr(sqlkit.Update("accounts").Set("bad-name", "Ada").Where("id = ?", 42).Build()),
		},
		{
			name: "delete table",
			err:  selectErr(sqlkit.DeleteFrom("bad table").Where("id = ?", 42).Build()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, sqlkit.ErrInvalidArgument) {
				t.Fatalf("error = %v, want ErrInvalidArgument", tt.err)
			}
		})
	}
}

func TestBuilderRejectsPlaceholderMismatch(t *testing.T) {
	_, err := sqlkit.SelectFrom("accounts").
		Where("id = ? and name = ?", 42).
		Build()
	if !errors.Is(err, sqlkit.ErrInvalidArgument) {
		t.Fatalf("error = %v, want ErrInvalidArgument", err)
	}
}

func TestBuilderRejectsUnsafeFullTableMutations(t *testing.T) {
	if _, err := sqlkit.Update("accounts").Set("name", "Ada").Build(); !errors.Is(err, sqlkit.ErrInvalidArgument) {
		t.Fatalf("update without where error = %v, want ErrInvalidArgument", err)
	}
	if _, err := sqlkit.DeleteFrom("accounts").Build(); !errors.Is(err, sqlkit.ErrInvalidArgument) {
		t.Fatalf("delete without where error = %v, want ErrInvalidArgument", err)
	}
}

func TestStatementCopiesArgs(t *testing.T) {
	args := []any{42}
	stmt := sqlkit.NewStatement("select $1", args...)
	args[0] = 7
	stmt.Args[0] = 9

	again := sqlkit.NewStatement(stmt.SQL, stmt.Args...)
	assertStatement(t, again, "select $1", []any{9})
}

func assertStatement(t *testing.T, stmt sqlkit.Statement, wantSQL string, wantArgs []any) {
	t.Helper()

	if stmt.SQL != wantSQL {
		t.Fatalf("SQL = %q, want %q", stmt.SQL, wantSQL)
	}
	if !reflect.DeepEqual(stmt.Args, wantArgs) {
		t.Fatalf("Args = %#v, want %#v", stmt.Args, wantArgs)
	}
}

func selectErr(_ sqlkit.Statement, err error) error {
	return err
}
