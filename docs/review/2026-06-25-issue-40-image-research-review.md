# Issue 40 Image Research Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

날짜: 2026-06-25
범위: issue #40 research note, follow-up image issues, and preserved external
research evidence.

## 판정

P0: 0
P1: 0

This is a documentation and tracker-alignment change. It does not add Go
package code, exported APIs, dependencies, benchmark claims, or runtime
behavior.

## 7-Tier 검토

### 성능

P0: 0
P1: 0

The research separates measured libvips acceleration from default package
adoption. Native-backed performance claims are blocked on Go benchmarks against
the pure-Go path.

### 안정성

P0: 0
P1: 0

The first package path avoids global native runtime state, cgo, and codec
availability variance. Optional libvips work requires lifecycle, cleanup, and
concurrent caller tests before adoption.

### 보안

P0: 0
P1: 0

The first implementation issue requires bounded decode behavior and explicit
format support. CAPTCHA remains example-only and is not described as an
authentication or abuse-prevention boundary.

### 운영/Ops

P0: 0
P1: 0

Native libvips, codec support, and OCR/Tesseract deployment are kept out of the
default image package. The optional adapter issue requires detection and
operator guidance before merge.

### 개발자/API

P0: 0
P1: 0

The recommendation is Go-shaped: `context.Context`, small first-party helper
contracts, stdlib-compatible formats, and optional native acceleration. It does
not port Kotlin/JVM framework facades.

### 사용자/호출자

P0: 0
P1: 0

The first useful caller value is predictable thumbnail, resize, and conversion
behavior. Broader image analysis, OCR, and CAPTCHA surfaces are deferred until
caller workflows justify them.

### 통합

P0: 0
P1: 0

Evidence sources include current `bluetape4k-image` module inventory, Go image
and libvips library metadata, the repo's `go 1.26.3` module contract, duplicate
issue search, and preserved wiki research notes.
