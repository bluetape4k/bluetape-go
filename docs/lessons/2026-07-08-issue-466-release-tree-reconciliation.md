# Release Tree Reconciliation Lessons

## L1: Repair branch drift immediately after fallback release projection

The v0.15.0 release used a protected fallback branch from `main` because direct
`develop -> main` promotion would have deleted main-only release assets. After
that kind of release, the next milestone should first reconcile `develop` back
to the published `main` tree before feature work continues.

Prevention:

- After a fallback release projection, run `git diff --name-status
  origin/develop..origin/main` and create a follow-up issue if the diff contains
  main-only release assets.
- Resolve reconciliation conflicts against the published release tree unless
  live evidence shows `develop` has newer post-release work.
- Prove the resolved sync tree matches `origin/main` before adding review or
  lesson evidence for the reconciliation PR.

## L2: Release evidence is part of the branch contract

Docs under `docs/release`, `docs/review`, and package README pairs are not
incidental release byproducts. If they only exist on `main`, the next direct
promotion can delete them.

Prevention:

- Treat release/readiness evidence and package docs as protected assets during
  branch reconciliation.
- Keep the PR body focused on before/after diff evidence so future release
  operators can see why the branch sync existed.
