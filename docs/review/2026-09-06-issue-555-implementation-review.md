# Issue #555 Graph Backend Conformance 구현 리뷰

## 검토 기준

- Base: `origin/develop`
- 최초 구현 검토 HEAD: `05818b7bbf74cda729f63f3d011dd02c065f06d0`
- 리뷰 수정 HEAD: `ec98e00276ffacd98e559ea4177f16fb58df31d0`
- 범위: `graph/graphtest`, Neo4j/Memgraph conformance adapter, 관련 README와
  변경 기록
- 판정 규칙: P0/P1은 완료 전 반드시 0이어야 한다. 독립 lane이 hard-stall이면
  `lane timed out; main integration fallback performed`로 기록하고 main이 같은 관점을
  exact diff에서 직접 검토한다.

## 7-Tier 결과

| 관점 | 실행 형태 | 발견 | 조치 후 판정 |
| --- | --- | --- | --- |
| Performance | 독립 lane timeout 뒤 main fallback | Materialization 전에 provider가 `limit+1`을 적용하고 runner가 sort 전에 다시 상한을 검사한다. 세 독립 provider suite는 16.65초, 16.70초, 16.70초로 10분 budget 안에 끝났다. | P0=0, P1=0, P2=0, P3=0 |
| Stability | 독립 lane timeout 뒤 main fallback | Cancellation precedence, `Started` handshake, callback join, fixture cleanup, adapter close 순서를 race 및 fail-stop subprocess로 확인했다. Startup deadline 뒤 늦게 성공 반환된 adapter가 닫히지 않는 P1을 발견했다. | P1은 `ad9c54f`에서 수정. P0=0, P1=0, P2=0, P3=0 |
| Security | 독립 lane timeout 뒤 main fallback | Query는 고정 문자열이고 값은 bound parameter로 전달된다. Provider metadata와 image reference를 fail-closed 검증하며 raw query, URI, credential, panic payload를 출력하지 않는다. | P0=0, P1=0, P2=0, P3=0 |
| Operator/Ops | 독립 reviewer | Stability와 같은 late adapter P1을 독립 확인했다. Startup failure 로그에 provider/digest/status/category/timeout/duration이 없는 P2를 추가 보고했다. | P1은 `ad9c54f`, P2는 `ec98e00`에서 수정. P0=0, P1=0, P2=0, P3=0 |
| Developer/API | 독립 reviewer | Public API, invalid-state rejection, defensive clone, capability snapshot, examples를 검토해 수정 요청 없이 승인했다. | P0=0, P1=0, P2=0, P3=0 |
| User/Caller | 독립 reviewer | 예제 표현, provider timeout 명령, graph recipe의 graphtest 누락에 P2 3건을 보고했다. | locale pair를 `ec98e00`에서 함께 수정. P0=0, P1=0, P2=0, P3=0 |
| Main integration | main exact-diff review | 여섯 관점의 근거, source/tests/docs, 로컬 실행 결과를 통합했다. | P0=0, P1=0, P2=0, P3=0 |

독립 Performance, Stability, Security lane은 5분 동안 fresh evidence를 반환하지
않아 각각 `lane timed out; main integration fallback performed`로 종료했다. 해당 이름의
독립 provenance는 주장하지 않으며 위 표의 판정은 main exact-diff fallback 결과다.
별도 verifier는 `05818b7..ec98e00` delta에서 late adapter close, redacted startup
diagnostic, README 수정과 targeted race/vet/diff-check를 확인해
`P0=0, P1=0, P2=0, P3=0`으로 승인했다.

## 발견과 수정

### P1 — startup deadline 뒤 adapter 누수

`call`은 callback을 abandon하지 않으므로 factory가 deadline 뒤 adapter를 반환할 수
있다. 기존 runner는 factory timeout을 먼저 반환해 이 adapter의 `Close`를 등록하지
않았다. `TestRunClosesAdapterReturnedAfterStartupDeadline`은 수정 전
`close count = 0, want 1`로 실패했다. `ad9c54f`는 adapter의 non-nil `Close`를 factory
결과 판정보다 먼저 defer해 timeout 오류를 보존하면서 owner를 정확히 한 번 회수한다.

### P2 — startup failure 운영 진단 부족

Factory 실패 경로는 안전한 오류만 반환했지만 provider, digest, category, timeout과
duration을 남기지 않았다. `TestRunLogsRedactedStartupFailure`은 수정 전
`provider=fake`가 없어 실패했다. `ec98e00`은 validated metadata와 allowlist 상태만
기록하고 raw `factory-secret-marker`가 출력되지 않음을 subprocess로 검증한다.

### P2 — caller 문서의 실행 경로 불일치

`graph/graphtest` README는 전체 compile-checked fake를 `example_test.go`로 직접
연결한다. Graph/Neo4j locale pair는 graphtest self-test와 Testcontainers process
timeout을 같은 명령으로 안내한다.

## 검증 증거

- `go test -race -count=10 ./graph/graphtest`: PASS
- `go test -count=1 ./graph/neo4j -run '^TestBackendConformance$' -timeout=10m`:
  PASS
- `make fmt-check`: PASS
- `make lint`: `0 issues`
- `git diff --check`: PASS
- 구현 source HEAD `b0900bc` 기준 `make test`, `make ci`: PASS

리뷰 수정 이후 exact-head `make ci` 결과는 최종 확인 뒤 `WIP.md`에 고정한다. 원격
GitHub CI와 Testcontainers Nightly는 PR 및 workflow dispatch 별도 권한 게이트이므로
`PENDING`이다.

## 최종 판정

현재 source review 판정은 `P0=0`, `P1=0`, `P2=0`, `P3=0`이다. Exact-head 로컬
canonical 검증이 통과하면 Step 6-R을 완료한다.
