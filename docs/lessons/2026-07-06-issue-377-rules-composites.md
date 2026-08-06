# Issue #377 rules composite와 inference primitive

일자: 2026-07-06

Composite group은 별도 DSL을 도입하지 않고 기존 `Rule` interface를 구현하는 방식으로
Go-native하게 유지할 수 있다. Activation, conditional, unit group은 모두 `RuleSet`
ordering을 재사용해 priority/name/registration tie behavior가 core engine과 일관되게
남아야 한다.

교훈: per-run child selection state를 composite rule struct에 저장하지 않는다. Rule value는
동시에 재사용될 수 있으므로 composite `Execute`는 child predicate를 다시 평가해야 하며,
predicate는 side-effect free여야 한다고 문서화한다. 재평가 결과 실행할 child가 없으면
outer engine이 composite를 applied로 세지 않도록 typed error로 fail closed한다.

교훈: forward-chaining-style behavior는 bounded해야 한다. `InferenceEngine`은 positive
`MaxCycles`를 요구하고, matching rule이 configured limit 이후에도 계속 firing하면
`ErrInferenceNonConverged`를 보고한다.

교훈: inference convergence에는 첫 non-triggered rule 뒤에 멈추는 engine option을 사용할 수
없다. `StopOnFirstNotTriggered`는 뒤쪽 matching rule을 숨겨 false convergence를 만들 수
있으므로 bounded inference는 이 configuration을 거부한다.

교훈: inference는 `Facts`를 in place로 mutate하며 transactional하지 않다. Bounded run이
실패한 뒤에도 partial write가 보인다는 점을 non-convergence docs와 test에 명확히 남긴다.

예방: 미래 expression/YAML/JSON reader 작업은 두 번째 execution model을 만들지 말고 같은
first-party rule contract로 compile해야 한다.
