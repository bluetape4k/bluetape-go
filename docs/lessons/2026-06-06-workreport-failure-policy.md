# Workreport Failure Policy

Issue #28 keeps the workflow result model independent from runner execution.

The useful split is:

- `workreport` owns statuses, failure policies, report trees, predicates, and
  deterministic aggregation.
- `workflow` owns branch execution, context propagation, and runner lifecycle in
  issue #27.

This avoids baking sequential or parallel runner behavior into the shared result
model. Unknown failure-policy values should return an `errors.Is`-compatible
error before a runner can silently treat them as success.
