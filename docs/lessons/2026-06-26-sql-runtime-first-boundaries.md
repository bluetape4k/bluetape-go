# SQL Runtime-First Boundaries

For bluetape-go SQL work, start with runtime contracts before building a DSL.
`database/sql` transaction, row mapping, typed errors, and resource cleanup are
the foundation that lets later builders or optional generators compose safely.

Generated-code tools such as sqlc and Jet can be excellent project workflows,
but they should remain optional examples until the repository proves that
mandatory generation is the smallest safe path.

Do not port Kotlin Exposed as an ORM clone. JSON columns, encrypted columns,
measured columns, cache-backed repositories, CTEs, batch helpers, and dialect
modules need concrete package consumers after the base SQL runtime is tested
against PostgreSQL.
