package language_test

import (
	"strings"
	"testing"

	"github.com/bluetape4k/bluetape-go/textsearch/language"
)

var (
	benchmarkDetector       *language.Detector
	benchmarkDetectResult   language.Result
	benchmarkConfidences    []language.Confidence
	benchmarkLanguageRanges []language.Section
)

func BenchmarkDetectorConstruction(b *testing.B) {
	for _, tc := range detectorConstructionCases() {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				detector, err := tc.build()
				if err != nil {
					b.Fatalf("build detector failed: %v", err)
				}
				benchmarkDetector = detector
			}
		})
	}
}

func BenchmarkDetectorConstructionAndFirstUse(b *testing.B) {
	for _, tc := range detectorConstructionCases() {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				detector, err := tc.build()
				if err != nil {
					b.Fatalf("build detector failed: %v", err)
				}
				result, err := detector.Detect(englishBenchmarkText())
				if err != nil {
					b.Fatalf("Detect failed: %v", err)
				}
				benchmarkDetector = detector
				benchmarkDetectResult = result
			}
		})
	}
}

func BenchmarkDetectorDetect(b *testing.B) {
	for _, tc := range steadyDetectorCases() {
		detector, err := tc.build()
		if err != nil {
			b.Fatalf("build detector failed: %v", err)
		}
		if _, err := detector.Detect(tc.text); err != nil {
			b.Fatalf("warm Detect failed: %v", err)
		}
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				result, err := detector.Detect(tc.text)
				if err != nil {
					b.Fatalf("Detect failed: %v", err)
				}
				benchmarkDetectResult = result
			}
		})
	}
}

func BenchmarkDetectorConfidences(b *testing.B) {
	detector, err := language.NewDetector(benchmarkSubsetLanguages(), language.WithLowAccuracyMode())
	if err != nil {
		b.Fatalf("NewDetector failed: %v", err)
	}
	text := englishBenchmarkText()
	if _, err := detector.Confidences(text); err != nil {
		b.Fatalf("warm Confidences failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		confidences, err := detector.Confidences(text)
		if err != nil {
			b.Fatalf("Confidences failed: %v", err)
		}
		benchmarkConfidences = confidences
	}
}

func BenchmarkDetectorDetectMultiple(b *testing.B) {
	detector, err := language.NewDetector([]language.Language{language.English, language.German}, language.WithLowAccuracyMode())
	if err != nil {
		b.Fatalf("NewDetector failed: %v", err)
	}
	text := "This sentence is English. Dieser Satz ist Deutsch. " + strings.Repeat("This is English. Dieser Satz ist Deutsch. ", 8)
	if _, err := detector.DetectMultiple(text); err != nil {
		b.Fatalf("warm DetectMultiple failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		sections, err := detector.DetectMultiple(text)
		if err != nil {
			b.Fatalf("DetectMultiple failed: %v", err)
		}
		benchmarkLanguageRanges = sections
	}
}

type detectorConstructionCase struct {
	name  string
	build func() (*language.Detector, error)
}

type steadyDetectorCase struct {
	name  string
	text  string
	build func() (*language.Detector, error)
}

func detectorConstructionCases() []detectorConstructionCase {
	subset := benchmarkSubsetLanguages()
	return []detectorConstructionCase{
		{
			name: "subset_lazy_high_accuracy",
			build: func() (*language.Detector, error) {
				return language.NewDetector(subset)
			},
		},
		{
			name: "subset_lazy_low_accuracy",
			build: func() (*language.Detector, error) {
				return language.NewDetector(subset, language.WithLowAccuracyMode())
			},
		},
		{
			name: "subset_preloaded_low_accuracy",
			build: func() (*language.Detector, error) {
				return language.NewDetector(subset, language.WithLowAccuracyMode(), language.WithPreloadedLanguageModels())
			},
		},
		{
			name: "latin_script_lazy_low_accuracy",
			build: func() (*language.Detector, error) {
				return language.NewLatinScriptDetector(language.WithLowAccuracyMode())
			},
		},
		{
			name: "spoken_lazy_low_accuracy",
			build: func() (*language.Detector, error) {
				return language.NewSpokenDetector(language.WithLowAccuracyMode())
			},
		},
		{
			name: "all_lazy_low_accuracy",
			build: func() (*language.Detector, error) {
				return language.NewAllDetector(language.WithLowAccuracyMode())
			},
		},
	}
}

func steadyDetectorCases() []steadyDetectorCase {
	subset := benchmarkSubsetLanguages()
	return []steadyDetectorCase{
		{
			name: "subset_low_accuracy_english_short",
			text: englishBenchmarkText(),
			build: func() (*language.Detector, error) {
				return language.NewDetector(subset, language.WithLowAccuracyMode())
			},
		},
		{
			name: "subset_low_accuracy_japanese_short",
			text: "これは日本語の短い文章です。",
			build: func() (*language.Detector, error) {
				return language.NewDetector(subset, language.WithLowAccuracyMode())
			},
		},
		{
			name: "subset_low_accuracy_english_medium",
			text: strings.Repeat(englishBenchmarkText()+" ", 24),
			build: func() (*language.Detector, error) {
				return language.NewDetector(subset, language.WithLowAccuracyMode())
			},
		},
		{
			name: "latin_script_low_accuracy_english",
			text: englishBenchmarkText(),
			build: func() (*language.Detector, error) {
				return language.NewLatinScriptDetector(language.WithLowAccuracyMode())
			},
		},
		{
			name: "all_low_accuracy_english",
			text: englishBenchmarkText(),
			build: func() (*language.Detector, error) {
				return language.NewAllDetector(language.WithLowAccuracyMode())
			},
		},
	}
}

func benchmarkSubsetLanguages() []language.Language {
	return []language.Language{language.English, language.German, language.Japanese, language.Korean}
}

func englishBenchmarkText() string {
	return "Language detection works well for this short English sentence."
}
