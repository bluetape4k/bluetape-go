# Issue #276 교훈

- geo 작업은 ownership 기준으로 나눠야 한다. pure geohash value, provider geocoding,
  GeoIP database lookup, projection, GIS file format은 서로 다른 dependency와 lifecycle
  contract를 가진다.
- broad source module은 broad Go package를 만들 충분한 evidence가 아니다. caller의 data
  shape, precision need, coordinate-system rule에서 시작한다.
- provider-backed geo API가 library package에 들어가기 전에는 explicit credential,
  quota, retry, timeout, error contract가 필요하다.
