# Benchmark artifact retention

Issue #401은 recommendation 작업을 시작하기 전에 benchmark output을 추적 가능하게 만든다.

## 교훈

- Raw output, command, environment metadata는 함께 보존한다. OS, CPU, Go version, git
  SHA, dirty-tree state가 없는 benchmark row는 cross-repo recommendation에 충분하지 않다.
- Local snapshot은 ranking이 아니라 evidence로 저장한다. Report는 파일이 무엇을
  측정했는지 말할 수 있지만, production default는 caller constraint와 security boundary를
  반영한 별도 결정이 필요하다.
- Issue별 stable output directory를 사용한다. Downstream report는 pasted benchmark
  excerpt에 의존하지 말고 file을 cite해야 한다.

## 증거

- `docs/research/2026-07-07-issue-401-benchmark-artifact-retention.md`
- `docs/research/benchmark-artifact-template.md`
- `docs/research/outputs/issue-401/`
