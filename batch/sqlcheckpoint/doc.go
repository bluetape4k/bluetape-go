// Package sqlcheckpoint SQL 기반 batch checkpoint 저장과 atomic writer 계약을 제공한다.
// 공개 API 주석은 호출자가 입력, 반환값, 오류, 취소, deadline, zero value 계약을 한국어로 확인할 수 있도록 유지한다.
//
// RetryPolicy와 SkipPolicy는 processor failures 이후 checkpoint 처리 방식을 구분한다.
// AtomicCheckpointWriter.Commit은 callback 실행과 checkpoint 갱신을 하나의 소유권 경계로 묶으며,
// 저장소 구현은 CAS 조건을 사용해 unknown-outcome 복구 경로를 판별할 수 있어야 한다.
package sqlcheckpoint
