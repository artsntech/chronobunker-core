// internal/telemetry/metrics_test.go
package telemetry_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/artsntech/chronobunker-core/apps/agentops-backend/internal/telemetry"
)

func TestRouterCostCounterIncrements(t *testing.T) {
	initial := testutil.ToFloat64(telemetry.RouterCostUSDTotal.WithLabelValues("tenant-a", "model-x"))
	telemetry.RouterCostUSDTotal.WithLabelValues("tenant-a", "model-x").Add(0.05)
	after := testutil.ToFloat64(telemetry.RouterCostUSDTotal.WithLabelValues("tenant-a", "model-x"))
	if after-initial < 0.049 {
		t.Errorf("expected counter to increase by 0.05, got %f", after-initial)
	}
}