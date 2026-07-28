# Probabilistic Redis 공유 key builder migration 구현 계획

> 한국어 재작성 범위: 이 계획 문서는 한국어 운영 문서로 읽히도록 제목, 판단, 작업 설명, 위험, 검증, 롤백 문맥을 한국어로 정리한다. 명령, 경로, API 이름, 이슈/PR 번호, 브랜치명, 코드 블록, 테스트 출력 같은 증거 문자열은 정확성을 위해 원문 그대로 보존한다.


> **에이전트 작업자용:** 필수 하위 스킬: 사용 superpowers:subagent-driven-development (권장) 또는 superpowers:executing-plans to 이 계획을 작업 단위로 구현. 단계는 checkbox (`- [ ]`) 추적 문법을 사용.

**목표:** Reuse the 공유 Redis structural key builder in `probabilistic/redis`
변경하지 않고 stored-key bytes, local namespace validation, 또는 the provider's
공개 redacted 오류 계약.

**아키텍처:** 유지 the probabilistic 패키지 as the owner of namespace
validation 및 its short redacted key ID. 추가 private helpers that create a
공유 `redis.KeyBuilder` 만 후 local validation 및 convert any impossible
fixed-configuration builder failure into an opaque local 오류. 사용 the helper
for Bloom slot/bits/config 및 HyperLogLog keys; retain every 호출자-visible
오류 mapping 및 Redis operation unchanged.

**기술 스택:** Go, `github.com/bluetape4k/bluetape-go/redis`,
`github.com/redis/go-redis/v9`, Testcontainers Redis, standard library.

---

## 파일 지도

| 파일 | 책임 |
|---|---|
| `probabilistic/redis/keys.go` | 사용 공유 structural key construction behind local validation 및 local opaque configuration 오류. |
| `probabilistic/redis/options_test.go` | 고정 Bloom/HLL key bytes, invalid-input boundary, 및 short local redaction ID. |
| `docs/lessons/2026-07-10-issue-592-probabilistic-redis-keybuilder.md` | 기록 why a 공유 construction helper does 아님 imply a 공유 공개 오류 또는 redaction 계약. |
| `docs/review/2026-07-10-issue-592-probabilistic-redis-keybuilder-review.md` | 단계 6-R integrated review evidence. |

## 작업 0: 커밋 Approved Design Artifacts

**복잡도:** 낮음

**파일:** 커밋 the two specs, 단계 2-R review, this plan, 및 단계 3-R
review 전에 source 또는 테스트 implementation begins.

- [ ] **단계 1:** 실행 `git diff --check` 및 verify 만 the five planning
  artifacts are staged.
- [ ] **단계 2:** 커밋 them 함께 Lore trailers. The intent is to preserve the
  compatible 공유-construction boundary; record that 공유 오류/redaction
  adoption was rejected, confidence is 높음, scope risk is narrow, 및 source
  validation is 아님 yet run.
- [ ] **단계 3:** 실행 `git status --short` 및 require a clean worktree 전에
  the RED 테스트 step.

## 작업 1: RED Exact-Key And Error-Boundary Tests

**복잡도:** 낮음

**파일:** Modify `probabilistic/redis/options_test.go`; read
`probabilistic/redis/keys.go`, `probabilistic/redis/options.go`, 및
`redis/key.go`.

**패턴:** Apply `$bluetape-go-patterns`: table-driven exact 계약 테스트,
없음 new dependencies, 없음 parallel Testcontainers work, 및 preserve
`errors.Is`/`errors.As` behavior. This change owns 없음 concurrency 계약, so
기존 serial provider 테스트 및 the repository race gate are sufficient;
없음 new stress helper is applicable.

- [ ] **단계 1:** 추가 `TestKeyBuilderForNamespaceKeepsClusterHashTag`. It must
  call the new private adapter directly, so the current source fails to compile
  until the 공유-builder migration exists:

  ```go
  builder, err := keyBuilderForNamespace(keyPrefix, "tenant-a:emails")
  if err != nil {
      t.Fatalf("keyBuilderForNamespace failed: %v", err)
  }
  key, err := builder.StructuralKey("bits")
  if err != nil {
      t.Fatalf("StructuralKey failed: %v", err)
  }
  if got, want := key.Value, "bluetape:probabilistic:bloom:v1:{tenant-a:emails}:bits"; got != want {
      t.Fatalf("key = %q, want %q", got, want)
  }
  ```

- [ ] **단계 2:** 추가 `TestBuildKeysKeepsSharedBuilderCompatibleLayout` 및
  `TestBuildHyperLogLogKeyKeepsSharedBuilderCompatibleLayout` 함께 exact
  Bloom slot/bits/config values 및 HLL expected value
  `bluetape:probabilistic:hll:v1:{tenant-a:emails}` 및 a marker namespace
  check that `redactedID` has the 기존 `redis-key:[0-9a-f]{12}` shape 및
  contains neither marker text nor the full key.
- [ ] **단계 3:** 추가 `TestKeyBuilderForNamespaceRetainsLocal검증` 함께
  `tenant:{bad}`, whitespace-만, 및 `tenant-secret`; require an 오류 및
  reject 오류 strings containing `redis: invalid key` 또는 `invalid hash tag`.
- [ ] **단계 4:** 실행
  `go test -count=1 ./probabilistic/redis -run 'KeyBuilderForNamespace|KeepsSharedBuilderCompatibleLayout'`.
  예상: RED 함께 `undefined: keyBuilderForNamespace` 후 the 테스트 are
  added 및 전에 작업 2 implements the adapter.

## 작업 2: Minimal Shared-Builder Adapter

**복잡도:** 보통

**파일:** Modify `probabilistic/redis/keys.go`; 테스트
`probabilistic/redis/options_test.go`.

**패턴:** Apply `$bluetape-go-patterns`: retain local 공개 오류 behavior,
use idiomatic private helpers, do 아님 wrap 공유 validation 오류 through a
호출자-visible path, 및 avoid changes to scripts, commands, 또는 exported APIs.

- [ ] **단계 1:** 가져오기 `github.com/bluetape4k/bluetape-go/redis` as
  `btredis`. 추가 `keyBuilderForNamespace(prefix, namespace string)`; it calls
  `validateNamespace(namespace)`, creates `btredis.NewKeyBuilder(prefix)`, applies
  `builder.WithHashTag(namespace)`, 및 maps every non-nil builder 오류 to a
  local opaque 오류 such as `fmt.Errorf("probabilistic redis key configuration")`.
  다음을 하지 않는다: wrap the 공유 오류.
- [ ] **단계 2:** 추가 a private helper that calls
  `builder.StructuralKey(parts...)`, returns `key.Value`, 및 maps a non-nil
  오류 to the same local opaque 오류. The 호출자 must 아님 observe a 공유
  `ErrInvalidKey` 또는 `ErrInvalidHashTag` 원인.
- [ ] **단계 3:** Rewrite `buildKeys` to derive `slot`, `bits`, 및 `config`
  through the adapter 및 `StructuralKey`, then retain
  `redactedRedisKeyID(slot)`. Rewrite `buildHyperLogLogKey` through the same
  adapter 및 retain its local `redactedRedisKeyID(key)`.
- [ ] **단계 4:** 다음을 하지 않는다: modify `errors.go`, Bloom Lua scripts, filter/HLL
  command calls, configuration metadata, 또는 README files.
- [ ] **단계 5:** 실행
  `gofmt -w probabilistic/redis/keys.go probabilistic/redis/options_test.go`
  fol낮음ed by
  `go test -p 1 -count=1 ./probabilistic/redis -run 'KeepsSharedBuilderCompatibleLayout|BuildKeysUsesClusterHashTag|UnsafeNamespaces|RedisError'`.
  예상: PASS.

## 작업 3: Focused Contract 검증

**복잡도:** 보통

**파일:** 검증 `probabilistic/redis/{keys.go,options_test.go,filter_test.go,hyperloglog_test.go,config_test.go}` 및 `redis/key.go`.

- [ ] **단계 1:** 실행 `make fmt-check`, `make tidy-check`, 및
  `go vet ./probabilistic/redis ./redis`.
- [ ] **단계 2:** 실행 serial provider 테스트:

  ```bash
  go 테스트 -p 1 -count=1 ./probabilistic/redis ./redis
  go 테스트 -p 1 -race -count=1 ./probabilistic/redis
  ```

  예상: PASS. The 패키지 uses Testcontainers, so 없음 패키지 테스트 command
  runs in parallel 함께 another Docker-backed command.
- [ ] **단계 3:** 실행 `git diff --check` 및 inspect `git diff --stat`.
  Confirm 없음 documentation 또는 benchmark artifact changed because 공개 behavior
  및 성능 claims did 아님 change.

## 작업 4: Verifier, 리뷰, Lesson, And Full CI

**복잡도:** 보통

**파일:** 생성 `docs/review/2026-07-10-issue-592-probabilistic-redis-keybuilder-verification.md`,
`docs/review/2026-07-10-issue-592-probabilistic-redis-keybuilder-review.md`,
및 `docs/lessons/2026-07-10-issue-592-probabilistic-redis-keybuilder.md`.

- [ ] **단계 1:** Before code review, run the 단계 5 verifier. It must inspect
  the implementation diff 및 focused 테스트 outputs, confirm every spec
  invariant has evidence, 및 explicitly verify local 오류/redaction remain
  the 호출자-visible 계약.
- [ ] **단계 2:** 실행 단계 6 local six-perspective review 및 record
  성능, 안정성, 보안, 운영자/Ops, 개발자/API, 및
  사용자/호출자 evidence. Require `P0=0 P1=0`; explicitly reject changing
  `RedisError`, `redactedRedisKeyID`, validation, scripts, 및 benchmark scope.
- [ ] **단계 3:** Write the lesson: 공유 Redis key construction can be
  compatible while 공유 validation/오류/redaction contracts are 아님. 기록
  the exact-key rule 및 #560 benchmark ownership.
- [ ] **단계 4:** 실행 the full gate serially:

  ```bash
  TESTCONTAINERS_REUSE_ENABLE=false TESTCONTAINERS_RYUK_DISABLED=false make ci
  git diff --check
  git status --short
  ```

  예상: PASS. Retain these explicit Testcontainers overrides 만 for the
  command; do 아님 mutate machine configuration.

## 롤백

Revert the implementation commit. Stored Redis keys, scripts, state, 공개
API, 및 configuration remain unchanged, so 없음 data migration 또는 cleanup is
needed.
