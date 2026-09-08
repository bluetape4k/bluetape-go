# #534 Redis Cuckoo filter 설계

## 목표와 승인 범위

milestone 0.23.0의 #534를 독립 브랜치 `feat/issue-534-cuckoo`와 `develop` 대상
PR로 전달한다. 2026-09-08 사용자는 기존 Redis probabilistic 패키지에 Cuckoo
명령·오류 검증을 추가하는 범위를 승인했다. 새 Go 의존성, 기존 Bloom/HLL 변경,
머지, 배포, Count-Min Sketch/Top-K/t-digest는 포함하지 않는다.

## 근거와 대안

- 기존 `probabilistic/redis/keys.go`의 namespace 검증·구조적 key 생성과
  `redis.NewOpError`의 원인 보존·문자열 비노출 방식을 재사용한다.
- 기존 #410 조사는 서버 capability와 client 메서드 존재를 구분했고 Cuckoo를
  별도 후속 작업으로 남겼다. GNO docs/github에서 해당 조사와 #534를 확인했다.
- [RedisBloom 원본](https://github.com/RedisBloom/RedisBloom)은 Redis 8부터 해당
  자료구조가 통합되었다고 명시한다. 일반 Redis 7.4 fixture는 unsupported 검증에만 사용한다.
- [CF.RESERVE](https://redis.io/docs/latest/commands/cf.reserve/),
  [CF.INSERT](https://redis.io/docs/latest/commands/cf.insert/),
  [CF.DEL](https://redis.io/docs/latest/commands/cf.del/),
  [CF.COUNT](https://redis.io/docs/latest/commands/cf.count/)가 서버 계약의 기준이다.

기존 패키지에 좁은 Cuckoo API를 추가한다. 모든 RedisBloom 명령을 노출하는
범용 wrapper는 범위가 넓어 제외한다. generic Hasher 계층도 실제 요구가 없어
추가하지 않는다. 원본 byte를 보존하는 Go `string` 입력을 사용하며, binary
입력은 호출자가 명시적으로 string으로 변환할 수 있다.

go-redis v9.20.0 `CFReserveWithArgs`는 `Expansion == 0`이면 옵션을 생략한다.
서버의 명시적 `EXPANSION 0`을 전달하고 RESP 결과를 엄격히 검증하기 위해
`Do(context.Context, ...any) *redis.Cmd`만 포함하는 `CuckooClient`를 주입한다.
이는 자체 Redis transport가 아니며 caller-owned go-redis client를 그대로 사용한다.

## 공개 계약

`NewCuckoo(CuckooOptions) (Cuckoo, error)`는 IO 없이 client와 namespace를 검증한다.
`CuckooOptions`는 `Client CuckooClient`, `Namespace string`만 가진다.
nil과 typed-nil client를 모두 거부한다. 구현체는 비공개이고 생성자를 거쳐 사용한다.
key prefix는 `bluetape:probabilistic:cuckoo:v1`이며 기존 Bloom/HLL key와 분리한다.
Namespace는 기존 validator와 동일하게 1..128 byte ASCII 영숫자와 `._-:`만
허용한다. 경계 colon, 예약 suffix, 민감정보 marker도 기존 규칙대로 거부한다.

| 메서드 | 동작과 결과 |
|---|---|
| `Reserve(ctx, CuckooReserveOptions) error` | CF.RESERVE, 기존 key를 지우거나 성공으로 간주하지 않음 |
| `Add(ctx, item string) error` | CF.INSERT key NOCREATE ITEMS item, 하나의 표본 추가, 자동 생성 금지 |
| `Exists(ctx, item string) (bool, error)` | CF.EXISTS, true는 존재 가능성이지 정확한 membership이 아님 |
| `Count(ctx, item string) (int64, error)` | CF.COUNT, 중복 삽입 수의 근사값 |
| `Delete(ctx, item string) (bool, error)` | CF.DEL, 알려진 삽입 한 번만 제거; membership 조회로 삭제 권한을 추론하지 않음 |

Reserve 옵션은 `Capacity int64`, `BucketSize uint8`, `MaxIterations uint16`,
`Expansion uint16`이다. Capacity는 4 이상 2^30 이하이며 정규화한 BucketSize의
두 배 이상이어야 한다. BucketSize=0은 2, MaxIterations=0은 20이다.
Expansion=0은 **확장 금지**이며 서버 기본값으로 치환하지 않는다. 양수는 32768
이하로 제한하고 서버의 다음 2의 거듭제곱 반올림을 문서화한다.
capacity는 최대 성공 삽입 수가 아니며 충돌로 일찍 포화될 수 있다.
옵션은 reserve 호출의 요청일 뿐 기존 key의 호환성 metadata로 저장하지 않는다.
Reserve 옵션은 신뢰하는 운영 설정이다. 서버 문법 범위가 안전한 tenant quota를
뜻하지 않는다. 호출자는 메모리·CPU 예산과 namespace 생성 수를 별도로 제한하며,
인증되지 않은 외부 요청의 값을 그대로 Reserve에 전달해서는 안 된다.

item은 최대 64 KiB이고 빈 string도 유효한 byte열이다. 변환·정규화·로깅하지 않는다.
key와 item은 각각 별도 RESP argv로 전달한다. 공백·CR/LF·NUL·명령어 형태 문자열도
문자열 결합으로 명령을 만들지 않고 하나의 item으로 보존한다.
동작마다 하나의 명령만 전송하고 batch·자체 retry·확인용 추가 IO는 수행하지 않는다.
Exists/Delete는 RESP2 int64 0/1과 RESP3 bool, Count는 비음수 int64만 허용한다.
Reserve는 정확한 OK, Add는 길이 1의 int64 1 또는 RESP3 true 성공 응답만 허용한다.
Add의 int64 -1은 문서화된 포화이므로 `ErrCuckooFull`이며 commit-unknown으로
취급하지 않는다. 그 밖의 결과는 `ErrCuckooReply`이며 성공으로 반환하지 않는다.
Exists/Count의 false/0은 missing key와 absent item을 구분하지 않는다. Exists는
wrong-type key도 false로 반환할 수 있으므로 type/capability health check로 사용하지 않는다.

## 오류·취소·소유권

- nil context는 `ErrInvalidOptions`이며 pre-cancel은 전송 0회이다. 응답 직후 취소를 다시 확인해
  늦은 성공을 반환하지 않는다. 오류 결과에서는 bool/count의 zero를 반환한다.
- `ErrCuckooUnsupported`는 실제 Redis server error의 unknown CF command일 때만
  매핑한다. ACL 실패, WRONGTYPE, transport 오류를 unsupported로 숨기지 않는다.
- `ErrCuckooReply`는 nil command와 문서에 없는 응답 형태를 나타낸다.
  `ErrCuckooFull`은 CF.INSERT의 명시적 -1이다.
- 전송된 Reserve/Add/Delete의 실패·잘못된 응답·응답 시점 취소는 보수적으로
  `redis.ErrCommitUnknown`을 함께 보존한다. 명확한 unsupported 응답은 실행되지
  않은 것으로 분류한다. 다만 output-plus-error나 응답 시점 취소가 함께 있으면
  이 예외를 적용하지 않고 commit-unknown과 모든 원인을 보존한다. 자동 replay하지 않는다.
- public 오류는 `redis.NewOpError`로 감싸 namespace·item·provider 메시지를 숨긴다.
  `errors.Is/As`로 원인과 context 취소, sentinel을 검사할 수 있다.
  이 비노출 보장은 최상위 오류 문자열에 한정된다. unwrap 또는 errors.As로 꺼낸
  원인은 원본 정보를 포함할 수 있으므로 직접 로깅해서는 안 된다.
- 연결·credential·TLS·ACL·timeout·retry·관측성은 호출자 소유다. 기본 global logger,
  goroutine, timer, client Close는 추가하지 않는다. 안전한 오류를 관찰하는 adapter이므로
  내부 logger는 두지 않는다.
- mutation 전용 standalone client는 `MaxRetries: -1`을 사용한다. Cluster 사용 시
  독립 `MaxRedirects`와 middleware 재시도도 호출자가 제한한다. Cluster transport
  검증은 이번 단일 서버 fixture 범위 밖이다. 공유 client는 동시 호출을 지원해야 한다.
  모든 mutation에서 client·redirect·middleware·호출자 retry의 자동 replay 금지는
  필수 전제다. 좁은 Do 인터페이스가 이 설정을 검사하거나 강제한다고 주장하지 않는다.
  응답 유실 회귀에서 기본 retry client와 no-retry client의 차이를 확인한다.
- 호출자는 deadline 또는 client IO timeout으로 각 요청을 제한한다. adapter는 caller
  client를 강제 종료하거나 비협조적 호출을 별도 goroutine으로 분리하지 않는다.
  Redis endpoint와 client는 신뢰 경계 안에 있으며 raw Do가 응답을 먼저 디코딩하므로
  악의적인 서버의 거대한 RESP 응답 할당까지 제한한다는 보장은 하지 않는다.

## 실패 모드와 방지

1. 예약 전 Add가 기본 설정 filter를 생성함: CF.ADD 대신 원자적 NOCREATE 삽입.
2. Exists의 false positive로 타인의 표본을 삭제함: Delete는 호출자가 알고 있는
   성공 삽입 횟수 안에서만 사용. 읽기 결과를 근거로 안전 삭제를 주장하지 않음.
   commit-unknown 삽입은 성공 장부에 넣지 않는다. never-added item 또는 알려진
   성공 횟수를 초과한 삭제는 다른 표본을 제거해 false negative를 만들 수 있다.
3. 응답 유실 뒤 Add/Delete 재시도로 중복 변경됨: commit-unknown와 client retry 금지.
4. 옵션 생략으로 비확장 의도가 달라짐: raw args에서 EXPANSION 0을 명시적으로 검증.
5. Redis 7.4를 모듈 지원으로 오인함: plain unsupported와 module-positive fixture 분리.
6. key 소실 뒤 자동 재생성으로 설정이 바뀜: Add는 NOCREATE로 실패. 자동 Reserve 없음.

## 검증과 수용 기준

- fake Do client는 요청을 복사하고 typed RESP 값, server error, nil command,
  output-plus-error, 지연 취소, pre-cancel을 주입한다. invalid input 전송 0회,
  method별 명령·args·key 분리, 원인 보존, `%v`/`%+v` 비노출을 확인한다.
  공백·CR/LF·NUL item, unwrap 원인 보존, unknown 결과를 성공 삽입으로 세지 않는
  호출자 예제도 검증한다. adapter가 외부 삽입 장부를 소유한다고 주장하지 않는다.
- default CI에서 fake와 기존 plain Redis 7.4의 unknown command를 검증한다.
- opt-in `BLUETAPE_CUCKOO_INTEGRATION=1`에서 digest-pinned Redis 8.8.1을
  Testcontainers로 시작한다. PING과 CF.RESERVE로 readiness/capability를 확인하고,
  중복 count, known-item delete, 미예약 Add 실패, reserve 충돌, 비확장 포화를 검증한다.
  mutable tag나 테스트 건너뛰기를 positive PASS로 보고하지 않는다.
- concurrent Add는 BucketSize=64, Capacity=512, Expansion=0에서 같은 item의
  64회 추가를 확인한다. 다른 fingerprint를 섞지 않아 근사 Count의 충돌 과대 추정을
  검증값으로 오인하지 않는다. fake 상태 보호와
  실제 fixture를 race로 검증한다. Docker suite는 다른 작업과 직렬로 실행한다.
- Example 테스트와 package README EN/KO에서 동일한 제한과 실행 명령을 제공한다.
- fmt/tidy/lint, 일반/race, local module-positive, exact-head CI를 통과하고
  P0/P1/P2/P3 미해결 0건인 상태에서 이슈별 PR을 전달한다.

## 변경 경계와 되돌리기

기존 Bloom/HLL 구현과 key는 변경하지 않는다. 신규 Cuckoo 코드·테스트·문서만
되돌릴 수 있다. 서버 key 삭제는 라이브러리의 자동 복구가 아니며 운영자 책임이다.
이 기능은 exact set, fencing, quota 또는 인증 자료구조로 사용하지 않는다.

## 문서 검증

독자는 Go 라이브러리 사용자·유지보수자이고 설명은 한국어, API·명령은 원형을 유지한다.
소스·공식 문서·테스트 수용 기준을 대응시켰다. 성능 우위나 아직 실행하지 않은
통합 테스트 성공은 주장하지 않는다. 작성된 spec은 독립 검토와 사용자 확인 대상이다.
