// Package probabilistic 확률적 자료구조를 위한 작은 first-party API를 제공합니다.
//
// 현재 패키지는 인메모리 Bloom filter를 제공합니다. Bloom filter는 삭제를
// 지원하지 않고 false positive가 가능하지만, 성공적으로 삽입되고 이후 Clear로
// 지워지지 않은 값에 대해서는 false negative를 만들지 않아야 합니다.
//
// Go 구현은 Kotlin utils/probabilistic의 핵심 계약을 따르되, 서비스 코드에서
// 공유하기 쉽도록 goroutine-safe하게 구현합니다. Redis-backed Bloom, Cuckoo,
// HyperLogLog 별도 후속 이슈에서 다룹니다.
package probabilistic
