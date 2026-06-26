# sqlkit Cardinality Boundary

When adding one-row helpers over `database/sql`, do not implement exact-one
cardinality by first reading the whole result set. Read at most two rows, then
return `ErrTooManyRows` once the second row is observed.

This keeps `QueryOne` and `QueryOptional` useful for caller mistakes such as
missing `limit 1` clauses without turning the helper into an unbounded
allocation path. Keep `QueryAll` as the explicit API for callers that genuinely
want all rows.
