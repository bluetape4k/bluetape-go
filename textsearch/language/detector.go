package language

import (
	"fmt"
	"strings"
	"unicode/utf8"

	lingua "github.com/pemistahl/lingua-go"
)

// MaxTextLength is the maximum input size accepted by detection helpers.
const MaxTextLength = 100_000

// Language is Lingua's language enum re-exported for callers of this optional
// package.
type Language = lingua.Language

const (
	// Unknown reports that Lingua could not reliably detect a language.
	Unknown Language = lingua.Unknown
	// Chinese is Lingua's Chinese language enum value.
	Chinese Language = lingua.Chinese
	// English is Lingua's English language enum value.
	English Language = lingua.English
	// French is Lingua's French language enum value.
	French Language = lingua.French
	// German is Lingua's German language enum value.
	German Language = lingua.German
	// Japanese is Lingua's Japanese language enum value.
	Japanese Language = lingua.Japanese
	// Korean is Lingua's Korean language enum value.
	Korean Language = lingua.Korean
	// Latin is Lingua's Latin language enum value.
	Latin Language = lingua.Latin
	// Spanish is Lingua's Spanish language enum value.
	Spanish Language = lingua.Spanish
	// Thai is Lingua's Thai language enum value.
	Thai Language = lingua.Thai
)

// AllLanguages returns every language supported by Lingua.
func AllLanguages() []Language {
	return lingua.AllLanguages()
}

// AllSpokenLanguages returns Lingua's non-extinct spoken languages.
func AllSpokenLanguages() []Language {
	return lingua.AllSpokenLanguages()
}

// Option configures detector construction.
type Option func(*config) error

type config struct {
	lowAccuracy             bool
	preload                 bool
	minimumRelativeDistance *float64
}

// WithLowAccuracyMode trades short-text accuracy for lower memory use.
func WithLowAccuracyMode() Option {
	return func(cfg *config) error {
		cfg.lowAccuracy = true
		return nil
	}
}

// WithPreloadedLanguageModels loads all selected models at detector build time.
func WithPreloadedLanguageModels() Option {
	return func(cfg *config) error {
		cfg.preload = true
		return nil
	}
}

// WithMinimumRelativeDistance configures Lingua's ambiguity threshold.
func WithMinimumRelativeDistance(distance float64) Option {
	return func(cfg *config) error {
		if distance < 0 || distance > 0.99 {
			return fmt.Errorf("%w: %.2f", ErrInvalidMinimumRelativeDistance, distance)
		}
		cfg.minimumRelativeDistance = &distance
		return nil
	}
}

// Detector wraps a reusable Lingua detector.
type Detector struct {
	inner     lingua.LanguageDetector
	languages []Language
}

// NewAllDetector builds a detector for all Lingua languages.
func NewAllDetector(options ...Option) (*Detector, error) {
	builder := lingua.NewLanguageDetectorBuilder().FromAllLanguages()
	return buildDetector(builder, lingua.AllLanguages(), options...)
}

// NewSpokenDetector builds a detector for Lingua's non-extinct spoken languages.
func NewSpokenDetector(options ...Option) (*Detector, error) {
	languages := lingua.AllSpokenLanguages()
	builder := lingua.NewLanguageDetectorBuilder().FromLanguages(languages...)
	return buildDetector(builder, languages, options...)
}

// NewLatinScriptDetector builds a detector for Lingua languages using Latin script.
func NewLatinScriptDetector(options ...Option) (*Detector, error) {
	languages := lingua.AllLanguagesWithLatinScript()
	builder := lingua.NewLanguageDetectorBuilder().FromAllLanguagesWithLatinScript()
	return buildDetector(builder, languages, options...)
}

// NewDetector builds a detector for a caller-selected language subset.
func NewDetector(languages []Language, options ...Option) (*Detector, error) {
	languages = normalizeLanguages(languages)
	if len(languages) < 2 {
		return nil, ErrLanguageSubsetTooSmall
	}
	builder := lingua.NewLanguageDetectorBuilder().FromLanguages(languages...)
	return buildDetector(builder, languages, options...)
}

func buildDetector(builder lingua.LanguageDetectorBuilder, languages []Language, options ...Option) (*Detector, error) {
	cfg := config{}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&cfg); err != nil {
			return nil, err
		}
	}
	if cfg.minimumRelativeDistance != nil {
		builder = builder.WithMinimumRelativeDistance(*cfg.minimumRelativeDistance)
	}
	if cfg.lowAccuracy {
		builder = builder.WithLowAccuracyMode()
	}
	if cfg.preload {
		builder = builder.WithPreloadedLanguageModels()
	}
	return &Detector{
		inner:     builder.Build(),
		languages: append([]Language(nil), languages...),
	}, nil
}

// Languages returns a copy of the selected language subset.
func (d *Detector) Languages() []Language {
	if d == nil {
		return nil
	}
	return append([]Language(nil), d.languages...)
}

// Result describes one single-language detection result.
type Result struct {
	Language   Language
	Detected   bool
	Confidence float64
	ISO6391    string
	ISO6393    string
}

// Confidence describes one confidence value returned by Lingua.
type Confidence struct {
	Language Language
	Value    float64
	ISO6391  string
	ISO6393  string
}

// Section describes one contiguous single-language region in mixed text.
type Section struct {
	Language Language
	Start    int
	End      int
	Text     string
	ISO6391  string
	ISO6393  string
}

// Detect validates input and detects its most likely language.
func (d *Detector) Detect(text string) (Result, error) {
	if err := validateText(text); err != nil {
		return Result{}, err
	}
	if d == nil || d.inner == nil {
		return Result{}, fmt.Errorf("language detector is nil")
	}
	detected, ok := d.inner.DetectLanguageOf(text)
	if !ok || detected == Unknown {
		return Result{Language: Unknown, Detected: false}, nil
	}
	return Result{
		Language:   detected,
		Detected:   true,
		Confidence: d.inner.ComputeLanguageConfidence(text, detected),
		ISO6391:    detected.IsoCode639_1().String(),
		ISO6393:    detected.IsoCode639_3().String(),
	}, nil
}

// Confidences returns Lingua confidence values sorted by descending confidence.
func (d *Detector) Confidences(text string) ([]Confidence, error) {
	if err := validateText(text); err != nil {
		return nil, err
	}
	if d == nil || d.inner == nil {
		return nil, fmt.Errorf("language detector is nil")
	}
	values := d.inner.ComputeLanguageConfidenceValues(text)
	confidences := make([]Confidence, len(values))
	for i, value := range values {
		lang := value.Language()
		confidences[i] = Confidence{
			Language: lang,
			Value:    value.Value(),
			ISO6391:  lang.IsoCode639_1().String(),
			ISO6393:  lang.IsoCode639_3().String(),
		}
	}
	return confidences, nil
}

// DetectMultiple validates input and detects contiguous language sections.
func (d *Detector) DetectMultiple(text string) ([]Section, error) {
	if err := validateText(text); err != nil {
		return nil, err
	}
	if d == nil || d.inner == nil {
		return nil, fmt.Errorf("language detector is nil")
	}
	results := d.inner.DetectMultipleLanguagesOf(text)
	sections := make([]Section, 0, len(results))
	for _, result := range results {
		start, end := result.StartIndex(), result.EndIndex()
		if start < 0 || end > len(text) || start > end {
			return nil, fmt.Errorf("language detector emitted invalid section %d:%d", start, end)
		}
		lang := result.Language()
		sections = append(sections, Section{
			Language: lang,
			Start:    start,
			End:      end,
			Text:     text[start:end],
			ISO6391:  lang.IsoCode639_1().String(),
			ISO6393:  lang.IsoCode639_3().String(),
		})
	}
	return sections, nil
}

func validateText(text string) error {
	length := utf8.RuneCountInString(text)
	if length > MaxTextLength {
		return fmt.Errorf("%w: %d chars (max %d)", ErrTextTooLong, length, MaxTextLength)
	}
	if strings.TrimSpace(text) == "" {
		return ErrBlankText
	}
	return nil
}

func normalizeLanguages(languages []Language) []Language {
	seen := make(map[Language]struct{}, len(languages))
	normalized := make([]Language, 0, len(languages))
	for _, lang := range languages {
		if lang == Unknown {
			continue
		}
		if _, ok := seen[lang]; ok {
			continue
		}
		seen[lang] = struct{}{}
		normalized = append(normalized, lang)
	}
	return normalized
}
