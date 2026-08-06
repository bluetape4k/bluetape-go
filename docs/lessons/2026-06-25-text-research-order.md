# Text Research Ordering 교훈

text 작업은 broad tokenizer-core port로 시작하지 않는다. Go value는 deterministic하고
dependency-light하며 testable한 영역에서 가장 강하다. compiled multi-pattern search,
masking, normalization, narrow dictionary model이 여기에 해당한다.

Korean NLP와 Japanese NLP는 동등한 porting target이 아니다. Japanese는 Kagome을 통한
credible Go-native adoption path가 있지만, Korean full POS, stemming, phrase
extraction, sentence splitting은 initial bluetape-go helper package로는 너무 크다.
language detection은 model loading과 memory shape가 operational concern이므로 optional로
유지해야 한다.

향후 text issue는 작은 deterministic helper와 heavy model-backed NLP를 분리해야 한다. 전자는
unit/property test와 benchmark로 바로 검증할 수 있지만, 후자는 model provenance, memory budget,
runtime packaging, language-specific quality metric이 먼저 필요하다. Kotlin package 이름이 같다는
이유만으로 Go package boundary를 만들지 않는다.
