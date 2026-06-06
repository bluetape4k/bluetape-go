# workreport

[English](README.md) | [한국어](README.ko.md)

`workreport`는 lightweight workflow code를 위한 status, failure-policy,
report-tree value를 제공합니다. Runner 실행과 분리되어 ordinary Go function과
향후 `workflow` runner가 같은 result model을 공유할 수 있습니다.

## Diagram

![workreport failure policy flow](../docs/images/readme-diagrams/workreport-failure-policy-flow.png)

## 예제

```go
report, err := workreport.Aggregate(
    "import",
    workreport.ContinueOnFailure,
    workreport.Completed("read"),
    workreport.Failed("write", errors.New("disk full")),
)
if err != nil {
    return err
}

if report.IsPartial() {
    log.Printf("%s finished with %d child reports", report.Name, len(report.Children))
}
```

## 실행 가능한 예제

Aggregation과 cancellation report의 compile-checked 예제는
[`workreport_example_test.go`](workreport_example_test.go)에 있습니다. 다음 명령으로
실행합니다.

```bash
go test ./workreport
```

## 상태

- `StatusCompleted`: failed child 없이 work가 완료되었습니다.
- `StatusFailed`: caller-visible error로 work가 실패했습니다.
- `StatusPartial`: aggregated work에 non-completed child가 하나 이상 있습니다.
- `StatusAborted`: policy 또는 caller-defined reason으로 work가 중단되었습니다.
- `StatusCancelled`: caller cancellation 또는 deadline으로 work가 중단되었습니다.

Zero-value `Report`는 unknown status입니다. Successful, failed, partial,
aborted, cancelled, terminal로 간주하지 않습니다.

## Failure Policies

- `StopOnFailure`는 첫 non-completed child까지 child report를 보존하고 그 child
  status를 parent status로 반환합니다.
- `ContinueOnFailure`는 모든 child report를 보존하고 child 중 하나라도 completed가
  아니면 `StatusPartial`을 반환합니다.

Unknown policy value는 `Aggregate`에서 `ErrUnknownFailurePolicy`와 호환되는 error를 반환합니다.

## 계약

- Constructor는 caller-visible error를 `Report.Err`에 보존합니다.
- Aggregation은 child report order를 보존합니다.
- Child report는 parent report에 복사되므로 caller slice mutation이 기존 report
  tree를 바꾸지 않습니다.
- `workreport`는 goroutine 시작, retry, cancellation ownership, workflow branch
  실행을 담당하지 않습니다. Runner behavior는 `workflow` package의 책임입니다.
