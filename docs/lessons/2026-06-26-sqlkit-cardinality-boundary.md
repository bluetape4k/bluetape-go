# sqlkit cardinality 경계

`database/sql` 위에 one-row helper를 추가할 때 exact-one cardinality를 구현하려고
전체 result set을 먼저 읽지 않는다. 최대 두 row만 읽고, 두 번째 row가 관찰되는 즉시
`ErrTooManyRows`를 반환한다.

이 방식은 `limit 1` 누락 같은 caller mistake를 잡아 주면서도 `QueryOne`과
`QueryOptional`을 unbounded allocation 경로로 만들지 않는다. 모든 row가 정말 필요한
호출자는 명시적인 API인 `QueryAll`을 사용해야 한다.
