# README Diagram Batch

Issue #133과 #134는 API implementation issue가 아니라 documentation coverage
gate다.

유용한 closure pattern은 다음과 같다.

- accepted diagram이 gate 일부를 이미 만족하면 재사용한다.
- `.dot`, `.plain`, `-graphviz.svg`, `-graphviz.png`, final `.svg`, final `.png`를
  함께 생성해 evidence와 README asset이 drift하지 않게 한다.
- README에는 PNG만 embed하고, inspection과 future edit를 위해 adjacent SVG source를
  유지한다.
- Graphviz edge label은 layout generation이 성공해도 overlap할 수 있으므로 commit 전
  rendered PNG contact sheet를 검사한다.

향후 milestone에서는 orthogonal README diagram이 복잡해질 때 edge meaning을 node
text에 둔다. node와 충돌하는 source-grounded label보다 unlabeled connector가 보통 더
읽기 쉽다.
