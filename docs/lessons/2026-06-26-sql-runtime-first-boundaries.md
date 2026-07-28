# SQL runtime-first 경계

bluetape-go SQL 작업은 DSL보다 runtime contract에서 시작한다. `database/sql`
transaction, row mapping, typed error, resource cleanup이 먼저 안정되어야 이후
builder나 optional generator가 안전하게 조합된다.

sqlc와 Jet 같은 generated-code 도구는 훌륭한 project workflow가 될 수 있다. 다만
mandatory generation이 가장 작은 안전 경로라는 증거가 나오기 전까지는 optional
example로 남긴다.

Kotlin Exposed를 ORM clone으로 이식하지 않는다. JSON column, encrypted column,
measured column, cache-backed repository, CTE, batch helper, dialect module은
base SQL runtime이 PostgreSQL에서 검증된 뒤 구체적인 package consumer가 필요하다.
