package textsearch

// NormalizeMode textsearch language image example에서 leader election 선택과 조정 계약을 설명한다.
type NormalizeMode int

const (
	// NormalizeNone textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	NormalizeNone NormalizeMode = iota
	// NormalizeNFC textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	NormalizeNFC
	// NormalizeNFKC textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	NormalizeNFKC
)

// BoundaryMode textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type BoundaryMode int

const (
	// BoundaryNone textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	BoundaryNone BoundaryMode = iota
	// BoundaryASCIIWord textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	BoundaryASCIIWord
	// BoundaryUnicodeWord textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
	BoundaryUnicodeWord
)

// OverlapMode textsearch language image example에서 반환값과 오류 의미를 설명한다.
type OverlapMode int

const (
	// OverlapAll textsearch language image example에서 반환값과 오류 의미를 설명한다.
	OverlapAll OverlapMode = iota
	// OverlapLeftmostLongest textsearch language image example에서 반환값과 오류 의미를 설명한다.
	// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
	OverlapLeftmostLongest
)

// Pattern textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
type Pattern struct {
	// ID textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	// Compile textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	ID string
	// Text textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Text string
}

// Config textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type Config struct {
	// IgnoreCase textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	// 이 주석은 textsearch language image example의 backend 요구사항, cancellation, timeout, 오류 처리 세부사항을 설명한다.
	IgnoreCase bool
	// Normalize textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	// 세부 조건은 language script, tokenizer, normalization, example ownership 계약을 따른다.
	Normalize NormalizeMode
	// Boundary textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Boundary BoundaryMode
	// Overlap textsearch language image example에서 반환값과 오류 의미를 설명한다.
	Overlap OverlapMode
}

// Match textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type Match struct {
	Pattern Pattern
	// Start textsearch language image example에서 설정값과 기본값 적용 방식을 설명한다.
	Start int
	End   int
	// Text textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Text string
}
