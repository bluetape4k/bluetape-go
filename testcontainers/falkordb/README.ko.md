# testcontainers/falkordb

`testcontainers/falkordb`는 immutable
`falkordb/falkordb:latest@sha256:adbddd418916c25618564ff8597a919b08bc76452ebeb74eb985c38d7281df62`
image를 시작하고 Redis port readiness를 기다린 뒤 bounded termination을
등록합니다. `AddressKey`로 caller-owned Redis address를 제공합니다.
