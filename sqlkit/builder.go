package sqlkit

import (
	"fmt"
	"strconv"
	"strings"
)

type whereClause struct {
	fragment string
	args     []any
}

// SelectBuilder 단순한 PostgreSQL SELECT statement를 구성한다.
type SelectBuilder struct {
	table   string
	columns []string
	where   []whereClause
	orderBy []string
	limit   *int
}

// SelectFrom은 table을 대상으로 하는 SELECT statement 구성을 시작한다.
func SelectFrom(table string) *SelectBuilder {
	return &SelectBuilder{table: table}
}

// Columns projection column을 설정한다. 생략하면 SELECT는 *를 사용한다.
func (b *SelectBuilder) Columns(columns ...string) *SelectBuilder {
	b.columns = append(b.columns, columns...)
	return b
}

// Where 호출자가 소유한 SQL predicate fragment를 추가한다. 값 자리에는 ? placeholder를 사용하며
// Build가 PostgreSQL $n placeholder로 다시 쓴다.
func (b *SelectBuilder) Where(fragment string, args ...any) *SelectBuilder {
	b.where = append(b.where, whereClause{fragment: fragment, args: copyArgs(args)})
	return b
}

// OrderBy identifier 기반 ascending order clause를 추가한다.
func (b *SelectBuilder) OrderBy(columns ...string) *SelectBuilder {
	b.orderBy = append(b.orderBy, columns...)
	return b
}

// Limit PostgreSQL LIMIT 값을 설정한다.
func (b *SelectBuilder) Limit(limit int) *SelectBuilder {
	b.limit = &limit
	return b
}

// Build 검토 가능한 SELECT statement를 반환한다.
func (b *SelectBuilder) Build() (Statement, error) {
	table, err := quoteIdentifier(b.table)
	if err != nil {
		return Statement{}, err
	}

	columns := []string{"*"}
	if len(b.columns) > 0 {
		columns, err = quoteIdentifiers(b.columns)
		if err != nil {
			return Statement{}, err
		}
	}

	var sql strings.Builder
	sql.WriteString("select ")
	sql.WriteString(strings.Join(columns, ", "))
	sql.WriteString(" from ")
	sql.WriteString(table)

	args, err := appendWhere(&sql, 1, b.where)
	if err != nil {
		return Statement{}, err
	}

	if len(b.orderBy) > 0 {
		orderBy, err := quoteIdentifiers(b.orderBy)
		if err != nil {
			return Statement{}, err
		}
		sql.WriteString(" order by ")
		sql.WriteString(strings.Join(orderBy, ", "))
	}

	if b.limit != nil {
		if *b.limit < 0 {
			return Statement{}, fmt.Errorf("%w: limit must not be negative", ErrInvalidArgument)
		}
		sql.WriteString(" limit ")
		sql.WriteString(strconv.Itoa(*b.limit))
	}

	return NewStatement(sql.String(), args...), nil
}

// InsertBuilder 단순한 PostgreSQL INSERT statement를 구성한다.
type InsertBuilder struct {
	table     string
	columns   []string
	values    []any
	returning []string
}

// InsertInto table을 대상으로 하는 INSERT statement 구성을 시작한다.
func InsertInto(table string) *InsertBuilder {
	return &InsertBuilder{table: table}
}

// Columns insert 대상 column 목록을 설정한다.
func (b *InsertBuilder) Columns(columns ...string) *InsertBuilder {
	b.columns = append(b.columns, columns...)
	return b
}

// Values insert할 단일 row 값을 설정한다.
func (b *InsertBuilder) Values(values ...any) *InsertBuilder {
	b.values = append(b.values, values...)
	return b
}

// Returning은 PostgreSQL RETURNING column 목록을 추가한다.
func (b *InsertBuilder) Returning(columns ...string) *InsertBuilder {
	b.returning = append(b.returning, columns...)
	return b
}

// Build 검토 가능한 INSERT statement를 반환한다.
func (b *InsertBuilder) Build() (Statement, error) {
	table, err := quoteIdentifier(b.table)
	if err != nil {
		return Statement{}, err
	}
	if len(b.columns) == 0 {
		return Statement{}, fmt.Errorf("%w: insert requires columns", ErrInvalidArgument)
	}
	if len(b.columns) != len(b.values) {
		return Statement{}, fmt.Errorf("%w: insert columns and values count differ", ErrInvalidArgument)
	}

	columns, err := quoteIdentifiers(b.columns)
	if err != nil {
		return Statement{}, err
	}
	placeholders := make([]string, len(b.values))
	for i := range b.values {
		placeholders[i] = "$" + strconv.Itoa(i+1)
	}

	var sql strings.Builder
	sql.WriteString("insert into ")
	sql.WriteString(table)
	sql.WriteString(" (")
	sql.WriteString(strings.Join(columns, ", "))
	sql.WriteString(") values (")
	sql.WriteString(strings.Join(placeholders, ", "))
	sql.WriteString(")")

	if len(b.returning) > 0 {
		returning, err := quoteIdentifiers(b.returning)
		if err != nil {
			return Statement{}, err
		}
		sql.WriteString(" returning ")
		sql.WriteString(strings.Join(returning, ", "))
	}

	return NewStatement(sql.String(), b.values...), nil
}

// UpdateBuilder 단순한 PostgreSQL UPDATE statement를 구성한다.
type UpdateBuilder struct {
	table string
	sets  []assignment
	where []whereClause
}

type assignment struct {
	column string
	value  any
}

// Update table을 대상으로 하는 UPDATE statement 구성을 시작한다.
func Update(table string) *UpdateBuilder {
	return &UpdateBuilder{table: table}
}

// Set은 column assignment를 추가한다.
func (b *UpdateBuilder) Set(column string, value any) *UpdateBuilder {
	b.sets = append(b.sets, assignment{column: column, value: value})
	return b
}

// Where 호출자가 소유한 SQL predicate fragment를 추가한다. 값 자리에는 ? placeholder를 사용하며
// Build가 PostgreSQL $n placeholder로 다시 쓴다.
func (b *UpdateBuilder) Where(fragment string, args ...any) *UpdateBuilder {
	b.where = append(b.where, whereClause{fragment: fragment, args: copyArgs(args)})
	return b
}

// Build 검토 가능한 UPDATE statement를 반환한다.
func (b *UpdateBuilder) Build() (Statement, error) {
	table, err := quoteIdentifier(b.table)
	if err != nil {
		return Statement{}, err
	}
	if len(b.sets) == 0 {
		return Statement{}, fmt.Errorf("%w: update requires at least one set", ErrInvalidArgument)
	}
	if len(b.where) == 0 {
		return Statement{}, fmt.Errorf("%w: update requires where", ErrInvalidArgument)
	}

	args := make([]any, 0, len(b.sets))
	setSQL := make([]string, 0, len(b.sets))
	for i, set := range b.sets {
		column, err := quoteIdentifier(set.column)
		if err != nil {
			return Statement{}, err
		}
		setSQL = append(setSQL, column+" = $"+strconv.Itoa(i+1))
		args = append(args, set.value)
	}

	var sql strings.Builder
	sql.WriteString("update ")
	sql.WriteString(table)
	sql.WriteString(" set ")
	sql.WriteString(strings.Join(setSQL, ", "))

	whereArgs, err := appendWhere(&sql, len(args)+1, b.where)
	if err != nil {
		return Statement{}, err
	}
	args = append(args, whereArgs...)

	return NewStatement(sql.String(), args...), nil
}

// DeleteBuilder 단순한 PostgreSQL DELETE statement를 구성한다.
type DeleteBuilder struct {
	table string
	where []whereClause
}

// DeleteFrom은 table을 대상으로 하는 DELETE statement 구성을 시작한다.
func DeleteFrom(table string) *DeleteBuilder {
	return &DeleteBuilder{table: table}
}

// Where 호출자가 소유한 SQL predicate fragment를 추가한다. 값 자리에는 ? placeholder를 사용하며
// Build가 PostgreSQL $n placeholder로 다시 쓴다.
func (b *DeleteBuilder) Where(fragment string, args ...any) *DeleteBuilder {
	b.where = append(b.where, whereClause{fragment: fragment, args: copyArgs(args)})
	return b
}

// Build 검토 가능한 DELETE statement를 반환한다.
func (b *DeleteBuilder) Build() (Statement, error) {
	table, err := quoteIdentifier(b.table)
	if err != nil {
		return Statement{}, err
	}
	if len(b.where) == 0 {
		return Statement{}, fmt.Errorf("%w: delete requires where", ErrInvalidArgument)
	}

	var sql strings.Builder
	sql.WriteString("delete from ")
	sql.WriteString(table)

	args, err := appendWhere(&sql, 1, b.where)
	if err != nil {
		return Statement{}, err
	}

	return NewStatement(sql.String(), args...), nil
}

func appendWhere(sql *strings.Builder, start int, clauses []whereClause) ([]any, error) {
	if len(clauses) == 0 {
		return nil, nil
	}

	sql.WriteString(" where ")
	args := make([]any, 0)
	next := start
	fragments := make([]string, 0, len(clauses))
	for _, clause := range clauses {
		fragment, clauseArgs, newNext, err := rewritePlaceholders(clause.fragment, next, clause.args)
		if err != nil {
			return nil, err
		}
		fragments = append(fragments, fragment)
		args = append(args, clauseArgs...)
		next = newNext
	}
	sql.WriteString(strings.Join(fragments, " and "))
	return args, nil
}

func rewritePlaceholders(fragment string, start int, args []any) (string, []any, int, error) {
	if fragment == "" {
		return "", nil, start, fmt.Errorf("%w: where fragment is empty", ErrInvalidArgument)
	}

	var sql strings.Builder
	next := start
	placeholderCount := 0
	for _, r := range fragment {
		if r != '?' {
			sql.WriteRune(r)
			continue
		}
		placeholderCount++
		sql.WriteString("$")
		sql.WriteString(strconv.Itoa(next))
		next++
	}
	if placeholderCount != len(args) {
		return "", nil, start, fmt.Errorf("%w: placeholder count %d does not match args count %d", ErrInvalidArgument, placeholderCount, len(args))
	}
	return sql.String(), copyArgs(args), next, nil
}
