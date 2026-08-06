# SQLKit builder 경계 교훈 (2026-06-26)

관련 이슈: #318
영향 모듈: `sqlkit`

## L1: 첫 builder는 표현력보다 inspectable한 계약을 우선한다

### 문제

relational SQL milestone에는 유용한 repository prototype이 필요했지만, 넓은 query
DSL은 빠르게 ORM 형태의 표면으로 커질 수 있었다. Builder가 생기면 join, dialect
switching, metadata, code generation을 추가하려는 유혹도 함께 생긴다.

### 교훈

첫 범위는 expression coverage 확장보다 explicit SQL과 args를 증명해야 한다.
`Statement`는 SQL을 눈에 보이게 유지하고, test는 생성된 문자열을 exact하게 검증하며,
`Where`는 subquery를 위한 caller-owned escape hatch로 남긴다. 동시에 identifier와
bind value는 계속 보호한다.

### 증거

- Builder test가 SELECT/INSERT/UPDATE/DELETE의 exact SQL과 args를 검증한다.
- Repository prototype이 PostgreSQL Testcontainers에서 CRUD, rollback, relational
  `exists` query를 증명했다.
- README가 PostgreSQL `$n` placeholder 경계와 non-goal을 문서화한다.

### 향후 지침

First-class JOIN, dialect, generator 표면은 follow-up issue가 repository example과
review evidence로 필요성을 증명한 뒤에만 추가한다. 그 전까지는 숨은 ORM 동작보다
명시적인 raw SQL fragment를 선호한다.
