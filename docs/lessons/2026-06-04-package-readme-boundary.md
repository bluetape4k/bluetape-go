# Package README 경계

Root README는 package index와 project state를 보여주는 역할에 머물러야 한다. 각
package의 사용법, behavior boundary, test command, benchmark result는 package
directory의 `README.md`에 둔다.

이렇게 해야 module이 늘어나도 root README가 API reference처럼 커지지 않고,
package별 변경을 해당 package 근처에서 검토할 수 있다. Benchmark chart도 결과를
해석하는 package README에 두고 root에는 link만 둔다.
