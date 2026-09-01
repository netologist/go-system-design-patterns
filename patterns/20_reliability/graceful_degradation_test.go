package reliability_test

import (
	"context"
	"errors"
	"testing"
	"time"

	reliability "system-design-patterns/patterns/20_reliability"
)

type mockPrimaryRecs struct {
	recs []reliability.Recommendation
	err  error
}

func (m *mockPrimaryRecs) GetRecommendations(ctx context.Context, userID string) ([]reliability.Recommendation, error) {
	return m.recs, m.err
}

type mockFallbackRecs struct {
	topPicks []reliability.Recommendation
}

func (m *mockFallbackRecs) GetStaticTopPicks(ctx context.Context) []reliability.Recommendation {
	return m.topPicks
}

func TestResilientRecommendationService_GracefulDegradation(t *testing.T) {
	fallback := &mockFallbackRecs{
		topPicks: []reliability.Recommendation{
			{ProductID: "top-1", Title: "Popular Best Seller"},
		},
	}

	// 1. Primary succeeds
	primaryGood := &mockPrimaryRecs{
		recs: []reliability.Recommendation{
			{ProductID: "pers-10", Title: "Personalized Pick for You", IsDegraded: false},
		},
	}
	svc := reliability.NewResilientRecommendationService(primaryGood, fallback, 100*time.Millisecond)

	recs, err := svc.GetRecommendations(context.Background(), "user-123")
	if err != nil || len(recs) != 1 || recs[0].IsDegraded {
		t.Errorf("expected personalized non-degraded recommendations: %+v", recs)
	}

	// 2. Primary fails -> Falls back gracefully to static top picks with IsDegraded=true
	primaryFailing := &mockPrimaryRecs{
		err: errors.New("ML recommendation cluster unreachable"),
	}
	svcDegraded := reliability.NewResilientRecommendationService(primaryFailing, fallback, 100*time.Millisecond)

	degradedRecs, err := svcDegraded.GetRecommendations(context.Background(), "user-123")
	if err != nil {
		t.Fatalf("expected graceful degradation to succeed, got error: %v", err)
	}

	if len(degradedRecs) != 1 || !degradedRecs[0].IsDegraded || degradedRecs[0].ProductID != "top-1" {
		t.Errorf("expected degraded static fallback recommendation: %+v", degradedRecs)
	}
}
