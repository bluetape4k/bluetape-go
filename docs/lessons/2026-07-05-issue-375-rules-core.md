# Issue #375 rules core primitive

일자: 2026-07-05

첫 `rules` 범위는 의도적으로 dependency-free로 유지했다. `Facts`, plain Go `Rule`
contract, deterministic `RuleSet`, sequential `Engine`, typed error, context-aware
result reporting만 포함했다. Expression reader, composite, forward chaining은 후속
범위로 남겼다.

교훈: rule engine은 failure를 detail에만 기록하면 쉽게 fail open한다.
`StopOnFirstFailed`가 false여도 어떤 rule이 실패했다면 error를 반환하고, 호출자가 mutated
facts를 신뢰하기 전에 `Result.Failed`를 확인하게 해야 한다.

교훈: public `Rule` implementation은 caller code다. Rule name과 priority는 registration
시점에 capture하고, internal lock을 잡거나 engine run을 sort하는 동안 metadata method를
호출하지 않는다.

예방: 미래 rules 작업은 composite이나 reader로 확장하기 전에 failure-after-fact mutation,
rule-returned context cancellation, zero-value typed error, cached registration
metadata regression test를 추가해야 한다.
