# Issue #276 Geo Research Review

Issue: #276
Branch: `research/issue-276-geo-scope`
Date: 2026-06-26

## Scope

Docs-only research boundary for geo, coordinate, geohash, provider geocoding,
GeoIP2, projection, and GIS/data helper candidates.

## 7-Tier Review

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

## Residual P2/P3

- P2: If the SQL milestone later adds spatial columns or spatial queries, open
  a narrow SQL-spatial issue instead of reviving this broad utility scope.
