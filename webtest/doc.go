// Package webtest provides framework-neutral fixtures and a bounded harness for
// net/http middleware 테스트.
//
// 이 package는 테스트 지원 코드이며 production middleware나 framework
// adapter를 소유하지 않는다. 각 scenario는 독립된 request, recorder,
// observation을 사용하고, timeout 뒤에는 request context를 취소해 정리를
// 기다린다.
package webtest
