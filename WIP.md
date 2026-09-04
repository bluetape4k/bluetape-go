# 진행 상황

스냅샷: 2026-09-04 KST
범위: `0.20.0` Go stable release 준비.

## 현재 대상 릴리스

`v0.20.0`은 AWS 통합 provider와 compile-checked 예제, DynamoDB leader,
공유 Redis primitive, Apache Fory 기반 cache, 그리고 공개 Go doc 한국어화를
묶는 기능 릴리스입니다. AWS client/credential/IAM/provisioning, Redis
connection/ACL/TLS, database lifecycle 및 downstream idempotency는 계속
caller/operator가 소유합니다.

완료된 milestone 이슈는 #517, #518, #519, #520, #522, #523, #524, #525,
#538, #539, #559, #568, #572, #573, #596, #597, #598, #599, #628, #629,
#630과 해당 child issue들입니다. 모든 42개 milestone issue가 `CLOSED`이며
현재 열린 milestone issue는 0개입니다.

## 현재 상태

- `develop` 기준 후보는 `aba16affd96d9f5008caec97b6108f4effbf47f6`이며,
  release-preparation branch의 현재 문서 커밋은
  `cd951124fa1f13d4a761814ecb948d26a718e690`입니다.
- release 기준 `origin/main`은 기존 `v0.19.0` commit
  `75a5d0fe2f2862c6d64d2934a2da987c08645fe0`입니다.
- `CHANGELOG.md`에 `## [v0.20.0] - 2026-09-04` 섹션을 추가했고, `WIP.md`를
  현재 release scope로 갱신했습니다.
- `0.20.0` milestone은 open issue 0개이지만 아직 열려 있습니다. 문서·검증이
  완료되면 close합니다.
- exact-head GitHub CI는 성공했습니다: run `33869096004`.
- PostgreSQL Testcontainers fixture는 `ForListeningPort`의 TCP 수락만으로
  반환하지 않고 native `pgx` 연결·ping을 bounded retry하도록 보강했습니다.
  회귀 테스트는 transient failure 3회 시도와 context deadline을 검증합니다.
- 로컬 `make ci` 첫 재실행에서는 `leader/etcd` fixture가 15초 readiness budget을
  초과했지만, 잔여 Testcontainers 없이 두 번째 전체 실행은 종료 코드 0으로
  정상/`-race` 전체 패키지와 benchmark contract self-test를 통과했습니다.
  `leader/sql`은 readiness 보강 후 정상 3회와 `-race` 1회를 모두 통과했습니다.
- `v0.20.0` tag와 GitHub Release는 아직 생성하지 않았습니다.

## 릴리스 체크리스트

1. 이 브랜치의 `CHANGELOG.md`/`WIP.md`를 검토하고 release-preparation PR로
   `develop`에 반영합니다.
2. `0.20.0` milestone을 close하고 live read-back합니다.
3. exact-head local validation, Go P0/P1 review 및 required GitHub/Nightly
   evidence를 모두 수렴합니다.
4. 검증된 `develop` tree를 `main`으로 promotion합니다. direct PR이
   conflicting이면 protected-branch projection fallback을 사용합니다.
5. `main`의 exact commit에 서명된 annotated `v0.20.0` tag를 생성·push합니다.
6. matching tag를 대상으로 validation evidence와 `CHANGELOG.md`를 사용해
   GitHub Release를 생성합니다.
7. downstream module 업데이트는 release closeout 이후 별도 작업으로 처리합니다.

## 릴리스 지원 참고

현재 작업은 tag·merge·GitHub Release를 실행하지 않는 release-preparation
단계입니다. 각 외부 side effect는 exact target, fresh CI/review/read-back 및
별도 authority hold를 요구합니다. `#521`의 deferred 상태와 `0.21.0` 이후
milestone work는 `v0.20.0` 범위에 포함하지 않습니다.
