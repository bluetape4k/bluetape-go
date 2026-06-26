package sqlkit

import (
	"context"
	stdsql "database/sql"
	"fmt"
)

// Statement is an inspectable SQL statement plus the ordered arguments that
// should be passed to database/sql.
type Statement struct {
	SQL  string
	Args []any
}

// NewStatement returns stmt with a defensive copy of args.
func NewStatement(query string, args ...any) Statement {
	return Statement{
		SQL:  query,
		Args: copyArgs(args),
	}
}

// Exec executes stmt through the context-aware database/sql Exec boundary.
func (stmt Statement) Exec(ctx context.Context, db Execer) (stdsql.Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if db == nil {
		return nil, fmt.Errorf("%w: execer is nil", ErrInvalidArgument)
	}
	return db.ExecContext(ctx, stmt.SQL, stmt.Args...)
}

func copyArgs(args []any) []any {
	if len(args) == 0 {
		return nil
	}
	copied := make([]any, len(args))
	copy(copied, args)
	return copied
}
