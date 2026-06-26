# 언어 감지

`textsearch/language`는 Lingua-Go 기반 optional language detection adapter입니다.
Lingua model 크기, loading mode, memory behavior는 운영 선택이므로 core
`textsearch` package 밖에 둡니다.

## 설치

```bash
go get github.com/bluetape4k/bluetape-go/textsearch/language
```

## Detector lifecycle

Detector는 한 번 만들고 재사용하세요.

```go
detector, err := language.NewDetector([]language.Language{
    language.English,
    language.German,
    language.Japanese,
}, language.WithLowAccuracyMode())
if err != nil {
    return err
}

result, err := detector.Detect("This text is written in English.")
```

모든 Lingua 언어에는 `NewAllDetector`, spoken language에는 `NewSpokenDetector`,
Latin script 언어에는 `NewLatinScriptDetector`, caller-selected subset에는
`NewDetector`를 사용합니다. 작은 subset은 보통 속도, memory behavior, 짧은 문장
ambiguity에 유리합니다.

Lingua는 기본적으로 language model을 lazy-load합니다.
첫 요청 latency보다 startup memory가 덜 중요하면 `WithPreloadedLanguageModels`를
사용하세요. 짧은 문장 정확도보다 memory가 중요하면 `WithLowAccuracyMode`를
사용하세요.

## Helper

- `Detect`는 unknown text를 operational error로 만들지 않고 `Detected` boolean으로
  반환합니다.
- `Confidences`는 Lingua confidence 값을 내림차순으로 반환합니다.
- `DetectMultiple`은 mixed-language section을 byte offset으로 노출합니다.
- `ContainsKorean`, `ContainsJapanese`, `ContainsChinese`, `ContainsThai`,
  `ContainsLatin`은 작은 Unicode script helper입니다.

## 경계

- Language detection은 preprocessing hint이며 security, moderation,
  compliance, authorization boundary가 아닙니다.
- Whatlanggo는 research note에 남긴 lightweight fallback/comparison path이고,
  이 package는 Lingua-Go parity path를 구현합니다.
- Language subset별 detector를 만들고 goroutine 사이에서 재사용하세요.

## 검증

```bash
go test -count=1 ./textsearch/language
go test -race -count=1 ./textsearch/language
```
