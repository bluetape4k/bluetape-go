# Text Research Ordering

For text work, do not begin with a broad tokenizer-core port. The Go value is
strongest where behavior is deterministic, dependency-light, and testable:
compiled multi-pattern search, masking, normalization, and narrow dictionary
models.

Korean and Japanese NLP are not equivalent porting targets. Japanese has a
credible Go-native adoption path through Kagome, while Korean full POS,
stemming, phrase extraction, and sentence splitting remain too large for an
initial bluetape-go helper package. Language detection should stay optional
because model loading and memory shape are operational concerns.
