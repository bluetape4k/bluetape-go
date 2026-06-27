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
	// MetadataLanguage identifies tokens produced by this Japanese adapter.
	MetadataLanguage = "language"
	// MetadataDictionary records the Kagome dictionary selected at construction.
	MetadataDictionary = "dictionary"
	// MetadataTokenClass records Kagome's token class.
	MetadataTokenClass = "kagome.class"
	// MetadataPOS records Kagome's POS hierarchy as a slash-separated string.
	MetadataPOS = "kagome.pos"
	// MetadataBaseForm records Kagome's base-form feature when present.
	MetadataBaseForm = "kagome.base_form"
	// MetadataReading records Kagome's reading feature when present.
	MetadataReading = "kagome.reading"
	// MetadataPronunciation records Kagome's pronunciation feature when present.
	MetadataPronunciation = "kagome.pronunciation"
)

// Option configures a Japanese Tokenizer.
type Option func(*config) error

// TokenizeMode selects Kagome's segmentation mode.
type TokenizeMode = ktokenizer.TokenizeMode

const (
	// Normal is regular Kagome segmentation.
	Normal TokenizeMode = ktokenizer.Normal
	// Search adds segmentation useful for search indexes.
	Search TokenizeMode = ktokenizer.Search
	// Extended is Kagome's extended search segmentation mode.
	Extended TokenizeMode = ktokenizer.Extended
)

type config struct {
	dictionary     *dict.Dict
	dictionaryName string
	mode           TokenizeMode
	options        []ktokenizer.Option
}

// WithDictionary selects a Kagome dictionary. The zero value uses IPA.
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

// WithMode selects Kagome's tokenize mode. The zero value uses Normal.
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

// WithKagomeOptions appends raw Kagome tokenizer options.
func WithKagomeOptions(options ...ktokenizer.Option) Option {
	return func(cfg *config) error {
		cfg.options = append(cfg.options, options...)
		return nil
	}
}

// Tokenizer adapts Kagome v2 to textsearch.Tokenizer.
type Tokenizer struct {
	inner          *ktokenizer.Tokenizer
	dictionaryName string
	mode           TokenizeMode
}

var _ textsearch.Tokenizer = (*Tokenizer)(nil)

// NewTokenizer creates a Japanese tokenizer backed by Kagome and IPA.
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

// Tokenize validates a textsearch request and returns Kagome tokens.
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

// IsNoun reports whether token metadata came from a Japanese noun.
func IsNoun(token textsearch.Token) bool {
	return hasPOSPrefix(token, "名詞")
}

// IsVerb reports whether token metadata came from a Japanese verb.
func IsVerb(token textsearch.Token) bool {
	return hasPOSPrefix(token, "動詞")
}

// Filter returns tokens that satisfy predicate.
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

// FilterNouns returns Japanese noun tokens.
func FilterNouns(tokens []textsearch.Token) []textsearch.Token {
	return Filter(tokens, IsNoun)
}

// FilterVerbs returns Japanese verb tokens.
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
