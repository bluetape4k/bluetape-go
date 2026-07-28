package sqlkit

import (
	"context"
	stdsql "database/sql"
	"fmt"
)

// Statement 검토 가능한 SQL statement와 database/sql에 순서대로 전달할 argument를 함께 보관한다.
type Statement struct {
	SQL  string
	Args []any
}

// NewStatement args를 defensive copy한 stmt를 반환한다.
func NewStatement(query string, args ...any) Statement {
	return Statement{
		SQL:  query,
		Args: copyArgs(args),
	}
}

// Exec context-aware database/sql Exec boundary를 통해 stmt를 실행한다.
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
