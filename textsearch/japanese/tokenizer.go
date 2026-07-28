package japanese

import (
	"fmt"
	"strings"

	"github.com/bluetape4k/bluetape-go/textsearch"
	"github.com/ikawaha/kagome-dict/dict"
	"github.com/ikawaha/kagome-dict/ipa"
	ktokenizer "github.com/ikawaha/kagome/v2/tokenizer"
)

const (
	// MetadataLanguage textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	MetadataLanguage = "language"
	// MetadataDictionary textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	MetadataDictionary = "dictionary"
	// MetadataTokenClass textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	MetadataTokenClass = "kagome.class"
	// MetadataPOS textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	MetadataPOS = "kagome.pos"
	// MetadataBaseForm textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	MetadataBaseForm = "kagome.base_form"
	// MetadataReading textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	MetadataReading = "kagome.reading"
	// MetadataPronunciation textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	MetadataPronunciation = "kagome.pronunciation"
)

// Option textsearch language image example에서 설정값과 기본값 적용 방식을 설명한다.
type Option func(*config) error

// TokenizeMode textsearch language image example에서 leader election 선택과 조정 계약을 설명한다.
type TokenizeMode = ktokenizer.TokenizeMode

const (
	// Normal textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Normal TokenizeMode = ktokenizer.Normal
	// Search textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
	Search TokenizeMode = ktokenizer.Search
	// Extended textsearch language image example에서 caller-visible 상태와 의미를 설명한다.
	Extended TokenizeMode = ktokenizer.Extended
)

type config struct {
	dictionary     *dict.Dict
	dictionaryName string
	mode           TokenizeMode
	options        []ktokenizer.Option
}

// WithDictionary textsearch language image example에서 leader election 선택과 조정 계약을 설명한다.
func WithDictionary(name string, dictionary *dict.Dict) Option {
	return func(cfg *config) error {
		if dictionary == nil {
			return fmt.Errorf("japanese tokenizer dictionary %q is nil", name)
		}
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("japanese tokenizer dictionary name is blank")
		}
		cfg.dictionary = dictionary
		cfg.dictionaryName = name
		return nil
	}
}

// WithMode textsearch language image example에서 leader election 선택과 조정 계약을 설명한다.
func WithMode(mode TokenizeMode) Option {
	return func(cfg *config) error {
		switch mode {
		case Normal, Search, Extended:
			cfg.mode = mode
			return nil
		default:
			return fmt.Errorf("unsupported Japanese tokenize mode: %d", mode)
		}
	}
}

// WithKagomeOptions textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
func WithKagomeOptions(options ...ktokenizer.Option) Option {
	return func(cfg *config) error {
		cfg.options = append(cfg.options, options...)
		return nil
	}
}

// Tokenizer textsearch language image example에서 동작과 caller-visible 계약을 설명한다.
type Tokenizer struct {
	inner          *ktokenizer.Tokenizer
	dictionaryName string
	mode           TokenizeMode
}

var _ textsearch.Tokenizer = (*Tokenizer)(nil)

// NewTokenizer textsearch language image example에서 생성과 초기화 계약을 설명한다.
func NewTokenizer(options ...Option) (*Tokenizer, error) {
	cfg := config{
		dictionary:     ipa.Dict(),
		dictionaryName: ipa.DictName,
		mode:           Normal,
		options:        []ktokenizer.Option{ktokenizer.OmitBosEos()},
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&cfg); err != nil {
			return nil, err
		}
	}

	inner, err := ktokenizer.New(cfg.dictionary, cfg.options...)
	if err != nil {
		return nil, fmt.Errorf("create Japanese tokenizer: %w", err)
	}
	return &Tokenizer{
		inner:          inner,
		dictionaryName: cfg.dictionaryName,
		mode:           cfg.mode,
	}, nil
}

// Tokenize textsearch language image example에서 반환값과 오류 의미를 설명한다.
func (t *Tokenizer) Tokenize(request textsearch.TokenizeRequest) (textsearch.TokenizeResponse, error) {
	if t == nil || t.inner == nil {
		return textsearch.TokenizeResponse{}, fmt.Errorf("japanese tokenizer is nil")
	}
	normalized, err := textsearch.NewTokenizeRequest(request.Text, request.Options)
	if err != nil {
		return textsearch.TokenizeResponse{}, err
	}

	kagomeTokens := t.inner.Analyze(normalized.Text, t.mode)
	tokens := make([]textsearch.Token, 0, len(kagomeTokens))
	for _, token := range kagomeTokens {
		if token.Surface == "" {
			continue
		}
		span := textsearch.TokenSpan{Start: token.Position, End: token.Position + len(token.Surface)}
		if span.Start < 0 || span.End > len(normalized.Text) || span.Start > span.End {
			return textsearch.TokenizeResponse{}, fmt.Errorf("japanese tokenizer emitted invalid span %d:%d for %q", span.Start, span.End, token.Surface)
		}
		text := normalized.Text[span.Start:span.End]
		if text != token.Surface {
			return textsearch.TokenizeResponse{}, fmt.Errorf("japanese tokenizer span %d:%d resolved to %q, want %q", span.Start, span.End, text, token.Surface)
		}
		if !normalized.Options.IncludeWhitespace && strings.TrimSpace(text) == "" {
			continue
		}
		tokens = append(tokens, textsearch.Token{
			Text:       text,
			Span:       span,
			Normalized: textsearch.NormalizeText(text, normalized.Options.Normalize).Normalized,
			POS:        coarsePOS(token.POS()),
			Metadata:   tokenMetadata(token, t.dictionaryName),
		})
	}

	return textsearch.TokenizeResponse{Request: normalized, Tokens: tokens}, nil
}

// IsNoun textsearch language image example에서 반환값과 오류 의미를 설명한다.
func IsNoun(token textsearch.Token) bool {
	return hasPOSPrefix(token, "名詞")
}

// IsVerb textsearch language image example에서 반환값과 오류 의미를 설명한다.
func IsVerb(token textsearch.Token) bool {
	return hasPOSPrefix(token, "動詞")
}

// Filter textsearch language image example에서 반환값과 오류 의미를 설명한다.
func Filter(tokens []textsearch.Token, predicate func(textsearch.Token) bool) []textsearch.Token {
	if predicate == nil {
		return nil
	}
	filtered := make([]textsearch.Token, 0, len(tokens))
	for _, token := range tokens {
		if predicate(token) {
			filtered = append(filtered, token)
		}
	}
	return filtered
}

// FilterNouns textsearch language image example에서 반환값과 오류 의미를 설명한다.
func FilterNouns(tokens []textsearch.Token) []textsearch.Token {
	return Filter(tokens, IsNoun)
}

// FilterVerbs textsearch language image example에서 반환값과 오류 의미를 설명한다.
func FilterVerbs(tokens []textsearch.Token) []textsearch.Token {
	return Filter(tokens, IsVerb)
}

func tokenMetadata(token ktokenizer.Token, dictionaryName string) map[string]string {
	metadata := map[string]string{
		MetadataLanguage:   "ja",
		MetadataDictionary: dictionaryName,
		MetadataTokenClass: token.Class.String(),
	}
	pos := token.POS()
	if len(pos) > 0 {
		metadata[MetadataPOS] = strings.Join(pos, "/")
		for i, value := range pos {
			metadata[fmt.Sprintf("kagome.pos.%d", i)] = value
		}
	}
	if value, ok := token.BaseForm(); ok {
		metadata[MetadataBaseForm] = value
	}
	if value, ok := token.Reading(); ok {
		metadata[MetadataReading] = value
	}
	if value, ok := token.Pronunciation(); ok {
		metadata[MetadataPronunciation] = value
	}
	return metadata
}

func coarsePOS(pos []string) textsearch.PartOfSpeech {
	if len(pos) == 0 {
		return textsearch.POSUnknown
	}
	switch pos[0] {
	case "名詞", "動詞", "形容詞", "副詞", "連体詞", "接頭詞", "接尾詞", "感動詞":
		return textsearch.POSWord
	case "記号":
		return textsearch.POSPunctuation
	default:
		return textsearch.POSUnknown
	}
}

func hasPOSPrefix(token textsearch.Token, prefix string) bool {
	if token.Metadata == nil {
		return false
	}
	pos := token.Metadata[MetadataPOS]
	return pos == prefix || strings.HasPrefix(pos, prefix+"/")
}
