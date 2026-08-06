# Leader Coordination 예제

Leader election example은 문서용 snippet으로만 두지 말고 `go test`에서 실행되는
smoke test로 유지한다. 그래야 Redis key, token, resign behavior가 실제 container
위에서 계속 검증된다.

0.1.0 범위에서는 batch scheduler와 migration gate가 가장 설명력이 높다. 둘 다
여러 backend replica가 동시에 뜰 때 "한 번만 실행되어야 하는 작업"을 다루며,
현재 Redis leader API의 핵심 contract를 보여준다.
