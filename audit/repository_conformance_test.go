package audit_test

import (
	"testing"

	"github.com/bluetape4k/bluetape-go/audit"
	"github.com/bluetape4k/bluetape-go/audit/audittest"
)

func TestMemoryRepositoryRunsReusableConformance(t *testing.T) {
	audittest.RunRepositoryConformance(t, func(testing.TB) audit.Repository {
		return audit.NewMemoryRepository()
	})
}
