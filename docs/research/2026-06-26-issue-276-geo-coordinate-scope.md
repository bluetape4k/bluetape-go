# Issue #276 Geo And Coordinate Scope

Issue: #276
Parent: #7
Date: 2026-06-26

## Decision

Close #276 as research-only. Do not add a `geo`, `geohash`,
`geocode`, `geoip`, `projection`, or GIS package during the 0.7.0 research
gate.

The useful Go direction is intentionally split:

- pure coordinate/geohash helpers may be revisited only when a concrete
  bluetape-go package needs spatial indexing or location values;
- provider-backed Google/Bing reverse-geocoding belongs in application examples
  or provider-specific packages, not a shared utility facade;
- GeoIP2 support belongs with a concrete security, analytics, or networking
  consumer that owns database download/licensing and lookup lifecycle;
- projection, shapefile, NetCDF, and geometry operations are domain/GIS work
  and should not be pulled into a general utility milestone.

No implementation follow-up is filed from this issue. Future consumers should
open a narrow issue with input data, precision needs, coordinate-system rules,
and dependency candidates before any package is added.

## Source Inventory

| Source module | Capability | Go decision |
|---|---|---|
| `bluetape4k-projects/utils/geo/geohash` | WGS84 point, geohash encode/decode, neighbors, bounding-box and circle queries, Vincenty distance helpers | Defer. This is the only potentially reusable pure slice, but no current bluetape-go caller needs it. |
| `bluetape4k-projects/utils/geo/geocode` | Google and Bing reverse geocoding over HTTP/Feign/coroutines | Defer to provider-specific application work. It requires credentials, quotas, retry/timeout/error contracts, and service-specific mapping. |
| `bluetape4k-projects/utils/geo/geoip2` | MaxMind database readers for city/country/ASN lookup | Defer to a concrete networking/security/analytics consumer. The owning package must define database lifecycle, licensing, update, and lookup failure behavior. |
| `bluetape4k-projects/utils/science` | WGS84/UTM projection, EPSG handling, shapefile/NetCDF, geometry operations, Exposed persistence | Reject for generic utility scope. This is domain GIS/data work, not a small Go helper port. |

## Go Ecosystem Candidates

| Candidate | Use if future consumer appears | Current decision |
|---|---|---|
| `github.com/mmcloughlin/geohash` | String and integer geohash encode/decode when only geohash cells are needed | Candidate only. Review architecture support, maintenance, and precision behavior before adopting. |
| `github.com/paulmach/orb` | 2D geo and planar/projected geometry values, GeoJSON subpackages, simple spatial types | Candidate only. Prefer for geometry value interoperability if a real package needs it. |
| `github.com/twpayne/go-geom` | OGC-style geometry types, GeoJSON/WKB/KML and database integration | Candidate only. Heavier than the current repo needs. |
| `github.com/twpayne/go-geos` | GEOS-backed topology operations | Reject until a hard GIS requirement exists because it requires native GEOS headers/libraries. |

## Rejected

- Porting the Kotlin `utils/geo` module as one Go package.
- Adding provider clients for Google Maps, Bing Maps, or MaxMind without a
  caller-owned use case and credential/lifecycle contract.
- Adding projection or NetCDF/shapefile support in a general utilities
  milestone.
- Adding coordinate math helpers just because the source JVM module has them.
- Creating spatial SQL integration before the SQL milestone defines a spatial
  storage/query requirement.

## Required Pattern For Future Geo Work

Any future issue must state:

1. whether the scope is pure value/geohash, provider IO, GeoIP database lookup,
   projection, or full GIS geometry;
2. coordinate ordering and naming rules (`lat/lon` versus `x/y`);
3. validation behavior for invalid latitude/longitude, empty geometry, and
   out-of-range precision;
4. boundary cases for poles, antimeridian, bounding boxes, and radius queries
   where relevant;
5. dependency, license, native-library, and database-file lifecycle decisions;
6. examples that prove callers can use the package without hidden credentials
   or global state.

## Validation

- Current bluetape-go search shows no package needing geo/coordinate helpers.
- Source inventory confirms `utils/geo` mixes pure geohash, provider IO, and
  local GeoIP database work that should not share one Go utility facade.
- `utils/science` projection and GIS functions are broader than a first-party
  helper package and belong to future domain-specific issues only.
