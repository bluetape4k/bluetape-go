# CloudWatch Metrics and Logs examples

This package contains compile-checked AWS SDK for Go v2 examples for
`PutMetricData` and `PutLogEvents`. The tests use in-memory fakes, so the normal
build never needs AWS credentials or a live endpoint.

The caller owns `aws.Config`, credentials, client lifecycle, retries, timeout
policy, logger, and any batching worker. Metrics and Logs remain separate
decisions; this package does not install a global registry or logger.

## Metrics

`PutMetricData` accepts at most 1,000 metrics per request and 30 dimensions per
metric. The HTTP request payload must remain below 1 MiB. Metric names,
namespaces, dimensions, and values are validated before dispatch; `NaN` and
infinite values are rejected. Dimension values identify a time series, so
high-cardinality values such as request IDs should not be used as default
observability labels.

## Logs

`PutLogEvents` requires chronological events. A batch contains at most 10,000
events, each event is at most 1 MiB, and the batch is at most 1 MiB including
the documented 26-byte per-event overhead. Events in a batch must span no more
than 24 hours. The legacy sequence token is intentionally omitted: CloudWatch
Logs now ignores it and accepts parallel puts to the same stream.

Both examples check `context.Context` before dispatch and after the SDK returns.
Caller cancellation therefore wins over a successful or failed provider
response. Provider error text and raw metric/log payloads are not copied into
the example errors. Attach low-cardinality telemetry through a caller-owned
hook or logger when needed.

For a real application, construct `cloudwatch.NewFromConfig(cfg)` or
`cloudwatchlogs.NewFromConfig(cfg)` after loading configuration and pass that
client to the same request shape. Live AWS calls, IAM, log-group/stream
provisioning, retention, alarms, and dashboards are outside this example.
