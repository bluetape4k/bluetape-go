# CloudWatch Metrics 및 Logs 예시

이 패키지는 AWS SDK for Go v2의 `PutMetricData`와 `PutLogEvents` 호출을
compile-checked example로 보여준다. 테스트는 메모리 fake를 사용하므로
기본 빌드에 AWS credentials나 live endpoint가 필요하지 않다.

`aws.Config`, credentials, client lifecycle, retry, timeout, logger와 batching
worker는 호출자가 소유한다. Metrics와 Logs는 서로 다른 계약이므로 하나의
전역 registry나 logger로 합치지 않는다.

## Metrics

`PutMetricData` request는 최대 1,000 metrics, metric 하나당 최대 30
dimensions를 받으며 HTTP payload는 1 MiB 미만이어야 한다. namespace, metric
name, dimension과 값은 dispatch 전에 검증하고 `NaN`/무한대 값은 거부한다.
dimension 값은 시계열 identity이므로 request ID 같은 고카디널리티 값을 기본
관찰성 label로 사용하지 않는다.

## Logs

`PutLogEvents`의 event는 시간순이어야 한다. 한 batch는 최대 10,000 events,
event 하나는 최대 1 MiB, batch 전체는 event당 26-byte overhead를 포함해
1 MiB 이하여야 하며 event 시간 범위는 24시간 이내여야 한다. 예시는 오래된
sequence token을 의도적으로 넣지 않는다. 현재 CloudWatch Logs는 token을
무시하고 동일 stream에 대한 병렬 put을 허용한다.

두 예시는 SDK 호출 전에, 그리고 SDK가 반환한 직후 `context.Context`를
확인한다. 따라서 provider가 성공 또는 실패를 반환해도 호출자 취소가
우선한다. provider 오류 문자열과 raw metric/log payload는 예시 오류에
복사하지 않는다. 필요한 운영 계측은 호출자 소유의 low-cardinality hook이나
logger로 연결한다.

실제 애플리케이션에서는 configuration을 로드한 뒤
`cloudwatch.NewFromConfig(cfg)` 또는 `cloudwatchlogs.NewFromConfig(cfg)`를
생성하고 동일한 request shape를 전달한다. live AWS 호출, IAM, log group/
stream provisioning, retention, alarm과 dashboard는 이 예시 범위가 아니다.
