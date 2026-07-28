# Korean Rewrite Inventory

이 문서는 issue #616의 산출물이다. Epic #615의 후속 이슈들은 이
인벤토리를 기준으로 단일 언어 문서와 Go 주석을 한국어로 재작성한다.
GitHub issue/PR 제목과 본문, commit message, release-facing 메타데이터는
저장소 규칙에 따라 영어로 유지한다.

## 범위 원칙

- 단일 언어 문서의 설명문, 결정 기록, 검토문, 연구 요약, 계획 문서는
  한국어로 재작성한다.
- Go 주석은 한국어로 재작성한다. 공개 API 주석뿐 아니라 struct field,
  option/property, 함수 인자, 반환값, 오류, cancellation, timeout, backend
  요구사항처럼 호출자가 오해하면 문제가 되는 의미를 자세히 설명한다.
- 코드 식별자, 패키지 경로, 명령어, 파일 경로, URL, 버전, issue/PR 번호,
  benchmark 이름, SQL 조각, error 문자열, source excerpt, raw command output은
  정확성이 우선이므로 원문을 유지한다.
- README/README.ko.md 쌍은 이번 primary rewrite 범위가 아니다. 후속 이슈는
  필요할 때 parity 확인만 수행한다.
- `AGENTS.md`, `CLAUDE.md`, prompt, skill, hook, `.omx` 상태 파일처럼
  LLM-facing operating document는 이번 한국어 재작성 범위에서 제외한다.
- `docs/manual/en`과 `docs/manual/ko`처럼 이미 bilingual pair로 운영되는
  manual은 primary rewrite 대상이 아니다. 후속 이슈는 parity 검증 대상으로만
  취급한다.
- 생산 코드 동작, 공개 API 이름, module 구조, dependency, test fixture 동작은
  변경하지 않는다.

## 문서 인벤토리

명령:

```bash
find . -path ./.git -prune -o -type f \( -name '*.md' -o -name '*.mdx' -o -name '*.txt' \) -print
```

관측 결과: doc-like 파일 1,053개.

| 구분 | Issue | 파일 수 | 대표 경로 | 처리 |
|---|---:|---:|---|---|
| README bilingual 보호 범위 | N/A | 153 | `README.md`, `README.ko.md`, `*/README.md`, `*/README.ko.md` | primary rewrite 제외, parity 확인만 수행 |
| LLM-facing operating docs | N/A | 1 | `AGENTS.md` | 영어 유지 |
| root/cross-cutting 문서 | #617 | 6 | `CHANGELOG.md`, `WIP.md`, `docs/package-layout.md`, `docs/release.md`, `docs/sql-generator-migration-guidance*.md` | 한국어 재작성. 단 release-facing 문구와 command literal은 원문 유지 |
| lesson archive | #618 | 142 | `docs/lessons/*.md` | 한국어 재작성 |
| research narrative | #619 | 70 | `docs/research/*.md` | 한국어 재작성. citation, URL, upstream 이름은 원문 유지 |
| raw output triage | #620 | 103 | `docs/research/outputs/**`, `docs/audits/outputs/**` | 사람이 쓴 설명은 한국어화, raw output은 원문 보존 또는 한국어 companion note 추가 |
| superpowers plans/PR/research | #621 | 120 | `docs/superpowers/plans/**`, `docs/superpowers/pr/**`, `docs/superpowers/research/**` | 사용자 협업 문서로 한국어 재작성 |
| superpowers specs | #622 | 83 | `docs/superpowers/specs/**` | 요구사항 의미를 유지하며 한국어 재작성 |
| superpowers reviews | #623 | 234 | `docs/superpowers/reviews/**` | finding, severity, evidence를 유지하며 한국어 재작성 |
| docs/review | #624 | 130 | `docs/review/**` | 검토문을 한국어 재작성 |
| release/audit/benchmark prose | #625 | 11 | `docs/release/**`, `docs/audits/*.md`, `docs/benchmarks/**` | 절차 의미와 raw evidence를 보존하며 한국어 재작성 |

위 표에서 #617부터 #625까지의 합계는 899개다. 후속 작업 중 새 단일 언어
문서가 발견되면 가장 가까운 issue에 편입하고, 편입 근거를 PR 본문에 남긴다.

## Go 주석 인벤토리

명령:

```bash
find . -path ./.git -prune -o -type f -name '*.go' -print
```

관측 결과: Go 파일 732개.

| 구분 | Issue | 파일 수 | 포함 top-level package | 주석 요구사항 |
|---|---:|---:|---|---|
| core utility comments | #626 | 153 | `core`, `collections`, `concurrency`, `codec`, `compression`, `serialization`, `id`, `measure`, `money`, `rules` | 공개 API와 helper semantics를 한국어로 설명 |
| orchestration/testing comments | #627 | 99 | `resilience`, `workflow`, `workreport`, `batch`, `state`, `testing` | cancellation, timeout, state transition, fixture semantics를 한국어로 설명 |
| Redis/cache/lock/rate/probabilistic comments | #628 | 170 | `redis`, `cache`, `lock`, `ratelimit`, `probabilistic` | TTL, lease, fencing, key, owner, permit, backend argument 의미를 한국어로 설명 |
| database/audit/encryption/AWS-adjacent comments | #629 | 57 | `audit`, `sqlkit`, `dynamodb`, `encrypt` | transaction, idempotency, encryption, persistence 의미를 한국어로 설명 |
| remaining public package families | #630 | 253 | `leader`, `jwt`, `graph`, `textsearch`, `imagekit`, `testcontainers`, `examples`, `internal` | provider, protocol, fixture, backend 요구사항을 한국어로 설명 |

위 다섯 이슈는 현재 관측된 top-level Go package를 모두 포함한다. 후속 작업은
identifier나 behavior를 바꾸지 않고 comment-only diff를 목표로 한다.

## 원문 보존 규칙

다음 항목은 한국어 문장 안에서도 원문을 유지한다.

- Go identifier, package path, exported API name, struct field name, option name.
- Shell command, Make target, environment variable, flag, file path.
- URL, issue/PR number, commit SHA, tag, version, module path.
- SQL, JSON key, YAML key, protocol name, HTTP status name, Redis command.
- Benchmark/test output, captured logs, generated command output, environment dump.
- 외부 source의 direct quote. 필요한 경우 짧게 유지하고 요약은 한국어로 쓴다.

## Raw Output 처리 규칙

`docs/research/outputs/**`와 `docs/audits/outputs/**`는 무조건 번역하지 않는다.
후속 issue #620은 각 파일을 다음 셋 중 하나로 분류한다.

| 분류 | 기준 | 처리 |
|---|---|---|
| prose summary | 사람이 작성한 설명, README, environment note | 설명문은 한국어로 재작성 |
| raw evidence | test output, benchmark output, command output, log, dump | 원문 보존 |
| mixed artifact | 설명과 raw evidence가 섞인 문서 | 설명은 한국어화하고 raw block은 원문 보존 |

원문 보존 파일은 `N/A`가 아니라 "evidence preserved"로 PR 본문에 기록한다.

## 후속 이슈 공통 체크리스트

각 후속 PR은 다음을 확인한다.

- [ ] README/README.ko.md primary rewrite가 포함되지 않았다. 포함했다면 parity
      note 또는 link fix처럼 별도 근거가 있다.
- [ ] LLM-facing operating document가 변경되지 않았다.
- [ ] bilingual manual pair가 primary rewrite 대상으로 변경되지 않았다.
- [ ] command, identifier, URL, issue/PR 번호, version, raw output이 번역으로
      손상되지 않았다.
- [ ] 한국어 문장은 직역보다 의미 보존을 우선했다.
- [ ] Go 주석 변경 PR은 `gofmt`를 통과했다.
- [ ] Go 주석 변경 PR은 touched package의 targeted `go test`를 통과했다.
- [ ] 문서 변경 PR은 `git diff --check`와 representative link/path check를
      통과했다.
- [ ] PR 본문은 English로 작성하고, 마지막 `##` heading을 `## DoD Status`로
      유지했다.

## 검증 명령

후속 이슈는 변경 범위에 맞춰 가장 작은 검증부터 실행한다.

```bash
git diff --check
```

```bash
gofmt -w <touched-go-files>
go test ./<touched-package>
```

```bash
rg -n "AGENTS\\.md|CLAUDE\\.md|docs/manual/(en|ko)|README(\\.ko)?\\.md" <changed-files>
```

마지막 명령은 변경 파일 목록을 대상으로 보호 범위가 실수로 포함됐는지
검사하기 위한 예시다. shell glob 한계 때문에 실제 PR에서는 변경 파일 목록을
명시적으로 전달하거나 `git diff --name-only` 결과를 사용한다.
