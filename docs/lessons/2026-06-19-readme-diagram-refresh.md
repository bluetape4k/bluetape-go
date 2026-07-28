# README Diagram Refresh 교훈

README diagram refresh에서는 SVG source만이 아니라 rendered PNG를 acceptance
artifact로 다뤄야 한다.

유용한 closure pattern은 다음과 같다.

- module diagram을 다시 그리기 전에 README와 source package를 다시 읽는다.
- README asset은 `docs/images/readme-diagrams`에 `.svg`와 `.png` pair로 유지한다.
- 최종 README diagram이 source-owned SVG가 되면 obsolete Graphviz script와 중간
  `.dot`/`.plain` asset을 제거한다.
- renderer fallback이 text width를 조용히 바꾸지 않도록 SVG style에는
  `Architects Daughter`와 `Comic Mono`만 사용한다.
- CairoSVG로 render한 뒤 full size PNG에서 card boundary에 닿는 text, unrelated
  arrow 위의 label, lifeline을 가리거나 가로지르는 sequence `alt` region을 검사한다.
- final commit 전에 전체 diagram set의 contact sheet를 만들고, dense diagram은
  개별적으로 다시 연다.

향후 pass에서는 script 성공 뒤 broad batch acceptance를 피한다. 여기서의 failure
mode는 미묘했다. SVG XML과 README link는 valid했지만, rendered PNG inspection은 여전히
card text overflow와 sequence lifeline에 겹친 `alt` note를 찾았다.
