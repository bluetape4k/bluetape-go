package textsearch

// NormalizeMode selects Unicode normalization applied before matching.
type NormalizeMode int

const (
	// NormalizeNone leaves input and patterns unchanged.
	NormalizeNone NormalizeMode = iota
	// NormalizeNFC applies canonical composition.
	NormalizeNFC
	// NormalizeNFKC applies compatibility composition.
	NormalizeNFKC
)

// BoundaryMode controls optional word-boundary filtering.
type BoundaryMode int

const (
	// BoundaryNone allows matches anywhere.
	BoundaryNone BoundaryMode = iota
	// BoundaryASCIIWord requires ASCII word boundaries around matches.
	BoundaryASCIIWord
	// BoundaryUnicodeWord requires Unicode letter, digit, or underscore
	// boundaries around matches.
	BoundaryUnicodeWord
)

// OverlapMode controls how FindAll returns overlapping matches.
type OverlapMode int

const (
	// OverlapAll returns every match emitted by the automaton.
	OverlapAll OverlapMode = iota
	// OverlapLeftmostLongest returns non-overlapping matches, preferring the
	// longest match at each leftmost start position.
	OverlapLeftmostLongest
)

// Pattern is one dictionary entry.
type Pattern struct {
	// ID is caller-owned metadata copied into Match values. When ID is empty,
	// Compile assigns a stable decimal index string.
	ID string
	// Text is the pattern text after caller-owned loading or filtering.
	Text string
}

// Config controls dictionary compilation and match filtering.
type Config struct {
	// IgnoreCase folds input and pattern text with strings.ToLower before
	// matching. Locale-specific casing is outside this package's scope.
	IgnoreCase bool
	// Normalize applies Unicode normalization before matching. Offset reporting
	// maps normalized segments back to original byte spans.
	Normalize NormalizeMode
	// Boundary filters matches that are not surrounded by word boundaries.
	Boundary BoundaryMode
	// Overlap controls FindAll output. The zero value returns all matches.
	Overlap OverlapMode
}

// Match describes one pattern occurrence in the original input string.
type Match struct {
	Pattern Pattern
	// Start and End are byte offsets in the original input. End is exclusive.
	Start int
	End   int
	// Text is the original input slice for Start:End.
	Text string
}
