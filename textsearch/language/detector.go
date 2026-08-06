package language

import (
	"fmt"
	"strings"
	"unicode/utf8"

	lingua "github.com/pemistahl/lingua-go"
)

// MaxTextLength textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
const MaxTextLength = 100_000

// Language textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
// 이 주석은 textsearch language image example의 backend 요구사항, cancellation, timeout, 오류 처리 세부사항을 설명한다.
type Language = lingua.Language

const (
	// Unknown textsearch language image example에서 반환값과 오류 의미를 설명한다.
	Unknown Language = lingua.Unknown
	// Chinese textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Chinese Language = lingua.Chinese
	// English 영어 text로 판정된 언어 code다.
	English Language = lingua.English
	// French textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	French Language = lingua.French
	// German textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	German Language = lingua.German
	// Japanese 일본어 text로 판정된 언어 code다.
	Japanese Language = lingua.Japanese
	// Korean 한국어 text로 판정된 언어 code다.
	Korean Language = lingua.Korean
	// Latin textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Latin Language = lingua.Latin
	// Spanish textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Spanish Language = lingua.Spanish
	// Thai textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Thai Language = lingua.Thai
)

// AllLanguages textsearch language image example에서 반환값과 오류 의미를 설명한다.
func AllLanguages() []Language {
	return lingua.AllLanguages()
}

// AllSpokenLanguages textsearch language image example에서 반환값과 오류 의미를 설명한다.
func AllSpokenLanguages() []Language {
	return lingua.AllSpokenLanguages()
}

// Option textsearch language image example에서 설정값과 기본값 적용 방식을 설명한다.
type Option func(*config) error

type config struct {
	lowAccuracy             bool
	preload                 bool
	minimumRelativeDistance *float64
}

// WithLowAccuracyMode textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
func WithLowAccuracyMode() Option {
	return func(cfg *config) error {
		cfg.lowAccuracy = true
		return nil
	}
}

// WithPreloadedLanguageModels textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
func WithPreloadedLanguageModels() Option {
	return func(cfg *config) error {
		cfg.preload = true
		return nil
	}
}

// WithMinimumRelativeDistance textsearch language image example에서 설정값과 기본값 적용 방식을 설명한다.
func WithMinimumRelativeDistance(distance float64) Option {
	return func(cfg *config) error {
		if distance < 0 || distance > 0.99 {
			return fmt.Errorf("%w: %.2f", ErrInvalidMinimumRelativeDistance, distance)
		}
		cfg.minimumRelativeDistance = &distance
		return nil
	}
}

// Detector textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type Detector struct {
	inner     lingua.LanguageDetector
	languages []Language
}

// NewAllDetector textsearch language image example에서 생성과 초기화 계약을 설명한다.
func NewAllDetector(options ...Option) (*Detector, error) {
	builder := lingua.NewLanguageDetectorBuilder().FromAllLanguages()
	return buildDetector(builder, lingua.AllLanguages(), options...)
}

// NewSpokenDetector textsearch language image example에서 생성과 초기화 계약을 설명한다.
func NewSpokenDetector(options ...Option) (*Detector, error) {
	languages := lingua.AllSpokenLanguages()
	builder := lingua.NewLanguageDetectorBuilder().FromLanguages(languages...)
	return buildDetector(builder, languages, options...)
}

// NewLatinScriptDetector textsearch language image example에서 생성과 초기화 계약을 설명한다.
func NewLatinScriptDetector(options ...Option) (*Detector, error) {
	languages := lingua.AllLanguagesWithLatinScript()
	builder := lingua.NewLanguageDetectorBuilder().FromAllLanguagesWithLatinScript()
	return buildDetector(builder, languages, options...)
}

// NewDetector textsearch language image example에서 생성과 초기화 계약을 설명한다.
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

// Languages textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (d *Detector) Languages() []Language {
	if d == nil {
		return nil
	}
	return append([]Language(nil), d.languages...)
}

// Result textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type Result struct {
	Language   Language
	Detected   bool
	Confidence float64
	ISO6391    string
	ISO6393    string
}

// Confidence textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type Confidence struct {
	Language Language
	Value    float64
	ISO6391  string
	ISO6393  string
}

// Section textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type Section struct {
	Language Language
	Start    int
	End      int
	Text     string
	ISO6391  string
	ISO6393  string
}

// Detect textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
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

// Confidences textsearch language image example에서 반환값과 오류 의미를 설명한다.
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

// DetectMultiple textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
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
