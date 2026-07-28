# Workreport Failure Policy 교훈

Issue #28은 workflow result model을 runner 실행에서 독립시킨다.

유지해야 할 분리는 다음과 같다.

- `workreport`는 status, failure policy, report tree, predicate, deterministic
  aggregation을 소유한다.
- `workflow`는 issue #27의 branch 실행, context 전파, runner lifecycle을
  소유한다.

이렇게 하면 sequential 또는 parallel runner 동작이 shared result model에
고정되지 않는다. 알 수 없는 failure-policy 값은 runner가 이를 success로
조용히 처리하기 전에 `errors.Is`와 호환되는 error를 반환해야 한다.
