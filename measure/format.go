package measure

import (
	"math"
	"strconv"
	"strings"
)

const renderScale = 1_000_000_000

func renderNumber(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	if value == 0 {
		value = 0
	}
	rounded := math.Round(value*renderScale) / renderScale
	text := strconv.FormatFloat(rounded, 'f', 9, 64)
	text = strings.TrimRight(text, "0")
	if strings.HasSuffix(text, ".") {
		text += "0"
	}
	if !strings.Contains(text, ".") {
		text += ".0"
	}
	return text
}

func formatValue[D any](value float64, unit Unit[D]) string {
	return renderNumber(value) + unit.formatSuffix()
}
