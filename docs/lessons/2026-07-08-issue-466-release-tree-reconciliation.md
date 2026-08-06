# Release Tree Reconciliation 교훈

## L1: fallback release projection 직후 branch drift를 바로 복구한다

v0.15.0 release는 직접 `develop -> main` promotion을 하면 main-only release
asset이 삭제될 수 있어 `main`에서 만든 protected fallback branch를 사용했다. 이런
release 뒤에는 feature work를 계속하기 전에 다음 milestone의 첫 작업으로
`develop`을 published `main` tree와 다시 맞춰야 한다.

Prevention:

- fallback release projection 뒤에는 `git diff --name-status
  origin/develop..origin/main`을 실행하고, diff에 main-only release asset이 있으면
  follow-up issue를 만든다.
- live evidence가 `develop`의 더 최신 post-release work를 보이지 않는 한,
  reconciliation conflict는 published release tree 기준으로 해결한다.
- reconciliation PR에 review나 lesson evidence를 추가하기 전에 resolved sync tree가
  `origin/main`과 일치함을 증명한다.

## L2: Release evidence는 branch contract의 일부다

`docs/release`, `docs/review`, package README pair 아래 문서는 우연한 release
byproduct가 아니다. 이들이 `main`에만 있으면 다음 direct promotion에서 삭제될 수
있다.

Prevention:

- branch reconciliation 중 release/readiness evidence와 package docs를 protected
  asset으로 취급한다.
- 미래 release operator가 branch sync가 필요했던 이유를 볼 수 있도록 PR body는
  before/after diff evidence에 집중한다.
