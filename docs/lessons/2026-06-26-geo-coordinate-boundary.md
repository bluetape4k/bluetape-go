# Issue #276 Lessons

- Geo work must be split by ownership. Pure geohash values, provider
  geocoding, GeoIP database lookup, projection, and GIS file formats have
  different dependency and lifecycle contracts.
- A broad source module is not enough evidence for a broad Go package. Start
  from the caller's data shape, precision needs, and coordinate-system rules.
- Provider-backed geo APIs need explicit credential, quota, retry, timeout,
  and error contracts before they enter a library package.
