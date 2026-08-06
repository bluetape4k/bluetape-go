# Codec Compatibility 교훈

Issue #187은 codec parity가 단일 "compatible" 주장으로는 부족하고 matrix가
필요하다는 점을 보여줬다. Base58은 Kotlin alphabet과 leading-zero 동작을
직접 맞출 수 있지만, Base62와 URL62는 경계를 넘는다. Kotlin helper는
numeric `BigInteger`/UUID 중심이고, bluetape-go는 byte API를 노출한다.

향후 codec 작업에서는 다음을 지킨다.

- 문서를 바꾸기 전에 upstream vector를 test로 고정한다.
- Kotlin이 numeric 값을 normalize하는 경우 Go-specific byte 동작을 문서화한다.
- empty input, blank whitespace, high-order zero byte, bit-limit check를 별도
  compatibility row로 다룬다.
- package-level immutable encoder가 public helper 사이에서 공유되면 bounded
  goroutine stress를 추가하고, 같은 target을 `-race`로 실행한다.
- 실제 UUID API를 의도적으로 추가하기 전까지 URL62는 Base62 alias로 유지한다.
