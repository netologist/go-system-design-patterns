package deployment_test

import (
	"net/http/httptest"
	"testing"

	deployment "system-design-patterns/patterns/21_deployment"
)

func TestTrafficSplitter_HeaderOverrideAndWeights(t *testing.T) {
	splitter := deployment.NewTrafficSplitter(0) // 0% weight initially

	// 1. Normal request with 0% weight -> BLUE
	req := httptest.NewRequest("GET", "/", nil)
	if color := splitter.RouteTarget(req); color != deployment.ColorBlue {
		t.Errorf("expected BLUE for 0%% canary weight, got: %s", color)
	}

	// 2. Request with canary header override -> GREEN
	reqHeader := httptest.NewRequest("GET", "/", nil)
	reqHeader.Header.Set("X-Canary-Test", "true")
	if color := splitter.RouteTarget(reqHeader); color != deployment.ColorGreen {
		t.Errorf("expected GREEN for X-Canary-Test override, got: %s", color)
	}

	// 3. Switch weight to 100% -> GREEN
	splitter.SetCanaryWeight(100)
	if color := splitter.RouteTarget(req); color != deployment.ColorGreen {
		t.Errorf("expected GREEN for 100%% canary weight, got: %s", color)
	}

	// 4. Blue/Green full cutover switch
	splitter.SetCanaryWeight(0)
	splitter.SwitchActiveColor(deployment.ColorGreen)
	if color := splitter.RouteTarget(req); color != deployment.ColorGreen {
		t.Errorf("expected GREEN after Blue/Green cutover switch, got: %s", color)
	}
}
