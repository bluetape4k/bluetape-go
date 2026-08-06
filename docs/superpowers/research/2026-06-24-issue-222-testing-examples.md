# Issue #222 Testing Examples

> 한국어 연구 요약: 이 문서는 사용자 협업용 조사/결정 기록이다. 아래 표와 목록의 URL, package name, command, issue number, version, source path는 evidence이므로 그대로 보존한다. 의사결정, 선택/보류/거절 사유, 후속 이슈 경계는 한국어 독자가 바로 이해할 수 있도록 이 요약을 우선 적용한다.


Issue: #222
Parent Epic: #221
Date: 2026-06-24

## 결정

Assertion DSL, faker dependency, JUnit-style parameter-source API를 추가하지
않고, 기존 `testing` package에 focused compile-checked example을 추가한다.

이는 #214의 research decision을 따른다. Go-native table test, 일반 subtest,
명시적 builder, package-local `testdata`, seeded `math/rand/v2`, `cmp.Diff`,
기존 cancellation helper가 추가 test framework 없이도 유용한 developer
experience를 제공한다.

## 구현 범위

- `testing/patterns_example_test.go`
  - 이름 있는 subtest를 가진 table-driven test;
  - package-local domain builder;
  - `cmp.Diff` structured comparison;
  - `testing/testdata`의 golden-file check;
  - `TempOutputPath`를 통한 generated temp output;
  - deterministic seeded random data;
  - `RequireContextCanceled`와 `RequireCleanupOnCancel`을 쓰는 cancellation
    assertion example;
  - compile-checked example function.
- `testing/testdata/order.golden.json`
  - canonical expected fixture.
- `testing/README.md`와 `testing/README.ko.md`
  - focused pattern과 non-goal을 설명한다.
- Root README pair
  - test-support reader를 focused example로 연결한다.

## 거절 항목

- General assertion DSL: standard `testing`, `cmp.Diff`, 기존 helper를
  중복한다.
- Faker dependency: #214는 explicit builder 대신 realistic random data를
  정당화할 구체 consumer를 찾지 못했다.
- JUnit-style parameter source: Go table literal이 더 명확하고
  type-checked된다.

## 검증

- PASS `go test -count=1 ./testing`
- PASS `go test -race -count=1 ./testing`
- PASS `make fmt-check`
- PASS `make vet`
- PASS `golangci-lint cache clean && make lint`
- PASS `git diff --check`
- PASS staged `make tidy-check`
- PASS `make test`
- PASS `make race`
