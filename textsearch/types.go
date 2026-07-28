package textsearch

// NormalizeMode는 textsearch language image example에서 leader election 선택과 조정 계약을 설명한다.
type NormalizeMode int

const (
	// NormalizeNone는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	NormalizeNone NormalizeMode = iota
	// NormalizeNFC는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	NormalizeNFC
	// NormalizeNFKC는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	NormalizeNFKC
)

// BoundaryMode는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type BoundaryMode int

const (
	// BoundaryNone는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	BoundaryNone BoundaryMode = iota
	// BoundaryASCIIWord는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	BoundaryASCIIWord
	// BoundaryUnicodeWord는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
	BoundaryUnicodeWord
)

// OverlapMode는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
type OverlapMode int

const (
	// OverlapAll는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
	OverlapAll OverlapMode = iota
	// OverlapLeftmostLongest는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
	// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
	OverlapLeftmostLongest
)

// Pattern는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type Pattern struct {
	// ID는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	// Compile는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	ID string
	// Text는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Text string
}

// Config는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type Config struct {
	// IgnoreCase는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	// 이 주석은 textsearch language image example의 backend 요구사항, cancellation, timeout, 오류 처리 세부사항을 설명한다.
	IgnoreCase bool
	// Normalize는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
	Normalize NormalizeMode
	// Boundary는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Boundary BoundaryMode
	// Overlap는 textsearch language image example에서 반환값과 오류 의미를 설명한다.
	Overlap OverlapMode
}

// Match는 textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type Match struct {
	Pattern Pattern
	// Start는 textsearch language image example에서 설정값과 기본값 적용 방식을 설명한다.
	Start int
	End   int
	// Text는 textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Text string
}
