# Workflow Runners

Issue #27 keeps lightweight workflow execution separate from durable orchestration.

Useful rules for this package:

- `workflow` owns branch execution and context propagation.
- `workreport` owns status values, failure policies, and report aggregation.
- Parallel runners should preserve child reports in input order, but the parent
  stop cause should come from the child that actually triggered cancellation.
- Ordinary Go closures are enough for the 0.4.0 runner API. A mutable
  `WorkContext` map would add shared-state and typing risk without solving a
  current problem.
