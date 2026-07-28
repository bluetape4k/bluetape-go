# Issue #276 Geo Research Review

> 한국어 감사/리뷰 경계: 이 문서는 리뷰 결론과 남은 위험을 한국어 독자가 추적할 수 있도록 정리한다. 심각도 표기, 판정 표기, 파일 경로, 라인 번호, 이슈/PR 링크, 명령, 코드 식별자, 인용 증거는 원문의 증거 앵커로 보존한다.

이슈: #276
브랜치: `research/issue-276-geo-scope`
날짜: 2026-06-26

## 범위

Docs-only research boundary for geo, coordinate, geohash, provider geocoding,
GeoIP2, projection, and GIS/data helper candidates.

## 7-Tier 검토

| Tier | Lens | P0 | P1 | Verdict | Evidence |
|---|---:|---:|---:|---|---|
| 1 | Security | 0 | 0 | PASS | Rejects hidden Google/Bing credentials, MaxMind database lifecycle, and global provider state. |
| 2 | Performance | 0 | 0 | PASS | Avoids unneeded geospatial dependencies and native GEOS/projection costs without a caller. |
| 3 | Stability | 0 | 0 | PASS | Splits pure value helpers from provider IO, database lookup, and GIS projection lifecycles. |
| 4 | Operator/Ops | 0 | 0 | PASS | Defers quota, API key, GeoIP database update, and native-library installation concerns to concrete consumers. |
| 5 | Developer/API | 0 | 0 | PASS | Preserves Go-native narrow package design and records required boundary cases for future APIs. |
| 6 | User/Caller | 0 | 0 | PASS | Avoids publishing a broad utility surface with unclear coordinate ordering, precision, and dependency behavior. |
| 7 | Evidence | 0 | 0 | PASS | Grounded in #223, `utils/geo`, `utils/science`, repo search, and Go ecosystem package candidates. |

P0=0 P1=0

## 잔여 P2/P3

- P2: If the SQL milestone later adds spatial columns or spatial queries, open
  a narrow SQL-spatial issue instead of reviving this broad utility scope.
