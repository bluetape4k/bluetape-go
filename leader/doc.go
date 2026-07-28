// Package leader leader election 계약을 정의한다.
//
// 하나의 Elector는 하나의 group에서 하나의 member를 대표한다. Campaign은
// leadership 획득을 시도하고, Resign은 보유 중인 leadership만 해제한다.
// 호출자가 넘긴 context 취소는 해당 호출에만 적용되며, lease renewal은 adapter가
// 관리한다. renewal이 실패하거나 소유권을 잃으면 IsLeader는 false가 된다.
// GroupElector 하나의 group에서 제한된 수의 member가 동시에 leader가 되게 한다.
// StrategicElector 후보 목록에 deterministic strategy를 적용해 winner일 때만
// action을 실행하는 candidate-registry 모델이다.
//
// 공개 에러는 sentinel error로 제공되며 errors.Is로 비교한다.
package leader
