# #525 S3 Vectors 구현 리뷰

## 2026-09-04 보강 검토

security/performance 리뷰의 oversized request·clone amplification 지적을
반영했다. AWS documented bound를 기준으로 PutVectors 500개/4,096 dimension,
GetVectors 100개 key, 63-byte key, List page/segment, Query TopK 10,000,
metadata 40 KiB와 request 20 MiB preflight를 clone 전에 적용한다. QueryVectors
fake의 post-response hook과 root README EN/KO package row도 보강했으며, oversized
입력의 SDK 호출 0회·targeted race/vet가 PASS다. `Error.GoString`과 `%#v` redaction
검증도 포함한다. opaque `document.Interface`는 provider-specific deep-copy를
시도하지 않고 dispatch 동안 caller 불변 계약을 README/spec/lesson에 명시했다.
보강 판정은 `P0=0, P1=0`이며 exact-head 원격 CI를 별도 확인한다.

## 검토 범위

- 기준점: `develop`의 `906a68fdb41551ccaa6ce1394a2370e654ade10e`
- 대상: `s3vectors/*.go`, bilingual README, `go.mod`/`go.sum` 및 #525 설계·계획·lesson
- 계약: AWS SDK eight operation thin forwarding, caller-owned client/credentials/
  retry/lifecycle, fake-first cancellation·redaction·malformed output 검증

## 7-Tier 통합 검토

| 관점 | 결과 | 근거 |
|---|---|---|
| 성능 | P0/P1 없음 | `s3vectors.go:140-219`에서 SDK 호출당 bounded clone만 수행하고 paginator·retry·embedding worker를 추가하지 않음 |
| 안정성 | P0/P1 없음 | `s3vectors.go:222-246`의 dispatch 전/응답 직후 cancellation과 `s3vectors_test.go:380-485`의 경계 테스트 |
| 보안 | P0/P1 없음 | `errors.go:18-63`의 redaction, `s3vectors.go:311-374`의 finite vector·식별자 검증 |
| 운영 | P0/P1 없음 | README에서 client, timeout, retry, logging, metrics, live endpoint 소유권을 caller에게 명시 |
| 개발/API | P0/P1 없음 | `s3vectors.go:12-48`의 최소 SDK interface와 `doc.go`의 zero-value 계약 |
| 사용자/호출자 | P0/P1 없음 | SDK request/response와 `document.Interface`를 보존하고 README에서 emulator/live 제한을 명시 |

## 발견 사항

- P0: 0
- P1: 0
- P2: 0
- P3: 0

`document.Interface` metadata/filter는 opaque 값으로 유지한다. deep-copy 가능한
SDK union과 key slice는 복사하지만 문서 값을 임의로 직렬화·정규화하지 않는 선택은
설계의 caller ownership과 일치한다. S3 Vectors의 dimension, distance metric,
permission과 live endpoint 동작은 AWS service 계약에 남기며 기본 CI가 추측하지 않는다.

## 검증 증거

- `go test -count=1 ./s3vectors` PASS
- `go test -race -count=1 ./s3vectors` PASS
- `go test -p 1 -count=1 ./...` PASS
- `go vet ./s3vectors` PASS
- `make fmt-check` PASS
- `make vet` PASS
- `make lint` PASS (`0 issues`)
- `go mod tidy` idempotence PASS
- `git diff --check` PASS

## 결론

승인된 #525 범위와 Go pattern을 충족한다. live AWS credential 및 검증된 S3
Vectors emulator가 없어 해당 통합 검증은 N/A이며, fake/compile 증거를 live 성공으로
간주하지 않는다. 병합 전 exact-head 원격 CI와 PR thread 재확인이 남아 있다.
