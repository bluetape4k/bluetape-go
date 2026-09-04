# #538 Secrets Manager 및 SSM provider 구현 리뷰

## 2026-09-04 보강 검토

performance/security 리뷰의 positive-TTL implicit unbounded cache와 `%#v`
redaction 지적을 반영했다. `CacheTTL > 0`이면 caller-owned cache를 반드시
제공해야 하며 provider가 `cache.NewMemory`를 자동 생성하지 않는다. 양 package의
short-TTL expiry와 constructor rejection 회귀 테스트가 추가됐고, `Error.GoString`
및 `%+v %#v` 검증으로 provider cause redaction을 고정했다. Secret example은
raw value 대신 set 여부와 byte 길이만 출력한다. targeted test, race, vet와
`git diff --check`가 PASS하며 보강 판정은 `P0=0, P1=0`이다.

## 판정

- 검토 대상: `feat/issue-538-secrets-ssm` working tree delta
- 기준 head: `906a68fdb41551ccaa6ce1394a2370e654ade10e`
- 대상 범위: `secretsmanager`, `ssm`, AWS service module, EN/KO README index,
  설계/계획/risk/lesson 문서
- 구현 판정: **PASS (P0=0, P1=0, P2=0, P3=0)**
- 원격 PR/CI, AWS credential, live Secrets Manager/SSM 호출은 실행하지 않았다.
- 별도 reviewer lane 없이 main integration fallback으로 여섯 관점과 통합 관점을
  read-only 대조했다. 따라서 독립 human/external review attestation은 없다.

## 구현 근거

두 package는 AWS SDK concrete client 전체를 노출하지 않고 필요한 method subset만
caller-owned interface로 받는다. 조회 값은 immutable `Value`로 복사하고
`String`/`GoString`/`%+v`에서 `[REDACTED]`만 반환한다. `CacheTTL`이 positive일
때만 기존 `cache.LoadingCache`를 사용하며 loader 성공만 저장하고, cache key와
오류 문자열에는 secret/parameter name과 raw payload를 넣지 않는다.

- Secrets Manager API와 output 판정: `secretsmanager/provider.go:17-143`
- SSM API, `WithDecryption`/`GetSecure`, mode별 cache key: `ssm/provider.go:17-156`
- 값 복사 및 formatter redaction: `secretsmanager/value.go:3-72`,
  `ssm/value.go:3-63`
- safe sentinel/error chain: `secretsmanager/errors.go:8-77`,
  `ssm/errors.go:8-77`
- fake-first request/cancellation/cache/race tests:
  `secretsmanager/provider_test.go`, `ssm/provider_test.go`
- compile-checked examples: `secretsmanager/example_test.go`,
  `ssm/example_test.go`
- caller ownership과 no-live-test 경계: 각 package `README.md`, `README.ko.md`

## 여섯 관점 + 통합 관점

| 관점 | P0 | P1 | P2 | P3 | 근거와 결론 |
| --- | ---: | ---: | ---: | ---: | --- |
| Performance | 0 | 0 | 0 | 0 | positive TTL에서 기존 single-flight `GetOrLoad`만 사용하고 별도 goroutine이나 무제한 재시도를 추가하지 않았다. |
| Stability | 0 | 0 | 0 | 0 | 호출 전후 context checkpoint, nil/malformed output fail-closed, typed-nil constructor 검증과 concurrent cache 테스트를 확인했다. |
| Security | 0 | 0 | 0 | 0 | raw value/name/provider error는 formatter와 public error 문자열에서 제외하고, caller가 명시한 `Bytes`/`Text`에서만 원문을 꺼낸다. |
| Operator/Ops | 0 | 0 | 0 | 0 | credentials, IAM, retry, endpoint, lifecycle, rotation, precedence와 logger는 caller/operator 소유로 유지하고 live AWS 경로를 추가하지 않았다. |
| Developer/API | 0 | 0 | 0 | 0 | narrow interface, concrete SDK compile assertion, documented options, sentinel `errors.Is`, EN/KO README와 example을 확인했다. |
| User/Caller | 0 | 0 | 0 | 0 | SecretString/SecretBinary와 SSM decryption mode를 보존하고, empty-but-present 값과 cache hit의 independent 복사 값을 구분한다. |
| Main integration | 0 | 0 | 0 | 0 | 신규 top-level package와 dependency/index 변경으로 범위를 제한했으며 기존 cache 계약과 root README locale pair를 유지했다. |

P0/P1 차단 finding은 없다. `context.Background()`는 nil context 정규화에만
사용하고, production code에 global client, credential, logger, refresh worker를
설치하지 않았다.

## 검증 증적

| 명령 | 결과 |
| --- | --- |
| `go test -count=1 ./secretsmanager ./ssm` | PASS |
| `go test -race -count=1 ./secretsmanager ./ssm` | PASS |
| `go test -run '^Example' -count=1 ./secretsmanager ./ssm` | PASS |
| `go vet ./secretsmanager ./ssm` | PASS |
| `golangci-lint run ./secretsmanager/... ./ssm/...` | `0 issues` |
| `make fmt-check` | PASS |
| `make vet` | PASS |
| `make lint` | PASS, `0 issues` |
| `go test -count=1 ./...` | PASS, repository-wide package tests |
| `git diff --check` | PASS |
| Korean terminology audit | PASS, 6 files, findings=0 |

`make tidy-check`는 미커밋 working tree의 의존성 변경을 clean-tree와 비교하는
저장소 규칙 때문에 실패한다. `go mod tidy`를 먼저 실행했고, 두 AWS service
module의 direct requirement와 checksum만 의도한 변경으로 남아 있다. 커밋 후
clean-tree에서 tidy gate를 다시 실행해야 한다.

## SPW-01..05 및 남은 게이트

- SPW-01: PASS — issue/parent, package/API source와 live 범위를 고정했다.
- SPW-02: PASS — API, zero value, failure, cancellation, cache와 acceptance를
  계획 및 테스트에 매핑했다.
- SPW-03: PASS — Korean reader-facing prose와 API/command token 보존을
  terminology audit로 확인했다.
- SPW-04: PASS — AWS SDK response 및 기존 cache 계약과 각 위험을 source/test에
  연결했다.
- SPW-05: PASS — 구현 read-back, targeted/full tests, static checks와 문서 parity를
  fresh evidence로 확인했다.

원격 branch push, PR exact-head CI, merge 승인, live AWS smoke, release/tag는
별도 workflow gate이며 이 lane에서 실행하지 않았다.
