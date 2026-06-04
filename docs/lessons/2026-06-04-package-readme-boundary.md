# Package README 경계

Root README는 package index와 프로젝트 상태를 보여주는 역할에 머물러야 한다.
각 package의 사용법, 동작 경계, 테스트 명령, benchmark 결과는 package directory의
`README.md`에 둔다.

이렇게 해야 module이 늘어나도 root README가 API reference처럼 커지지 않고,
package별 변경을 해당 package 근처에서 검토할 수 있다. Benchmark chart도 결과를
해석하는 package README에 두고, root에는 링크만 둔다.
