# Workflow Runner 교훈

Issue #27은 가벼운 workflow 실행과 지속성 있는 orchestration을 분리한다.

이 package에서 유지할 규칙은 다음과 같다.

- `workflow`는 branch 실행과 context 전파를 소유한다.
- `workreport`는 status 값, failure policy, report 집계를 소유한다.
- parallel runner는 child report를 입력 순서대로 보존해야 하지만, parent의
  stop cause는 실제 cancellation을 유발한 child에서 가져와야 한다.
- 0.4.0 runner API에는 일반 Go closure만으로 충분하다. 변경 가능한
  `WorkContext` map은 현재 문제를 해결하지 못하면서 shared-state와 typing
  risk만 추가한다.
