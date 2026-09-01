package reliability

import (
	"context"
	"errors"
	"time"
)

// Recommendation represents a product recommendation.
type Recommendation struct {
	ProductID string
	Title     string
	IsDegraded bool // Indicates whether this came from fallback
}

// PersonalizedRecommendationEngine primary dynamic downstream service.
type PersonalizedRecommendationEngine interface {
	GetRecommendations(ctx context.Context, userID string) ([]Recommendation, error)
}

// FallbackProvider static or cached fallback source.
type FallbackProvider interface {
	GetStaticTopPicks(ctx context.Context) []Recommendation
}

// ResilientRecommendationService serves personalized recommendations, falling back to static top picks if downstream fails or times out.
type ResilientRecommendationService struct {
	primary  PersonalizedRecommendationEngine
	fallback FallbackProvider
	timeout  time.Duration
}

func NewResilientRecommendationService(
	primary PersonalizedRecommendationEngine,
	fallback FallbackProvider,
	timeout time.Duration,
) *ResilientRecommendationService {
	if timeout <= 0 {
		timeout = 100 * time.Millisecond
	}
	return &ResilientRecommendationService{
		primary:  primary,
		fallback: fallback,
		timeout:  timeout,
	}
}

// GetRecommendations executes primary engine with timeout; if it errors or times out, falls back to static defaults.
func (s *ResilientRecommendationService) GetRecommendations(ctx context.Context, userID string) ([]Recommendation, error) {
	childCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	recs, err := s.primary.GetRecommendations(childCtx, userID)
	if err == nil && len(recs) > 0 {
		return recs, nil
	}

	// Primary failed or timed out -> Graceful Degradation to fallback
	fallbackRecs := s.fallback.GetStaticTopPicks(ctx)
	if len(fallbackRecs) == 0 {
		return nil, errors.New("both primary and fallback recommendation sources failed")
	}

	// Mark as degraded mode for observability / UI badge
	degraded := make([]Recommendation, len(fallbackRecs))
	for i, r := range fallbackRecs {
		degraded[i] = Recommendation{
			ProductID:  r.ProductID,
			Title:      r.Title,
			IsDegraded: true,
		}
	}

	return degraded, nil
}
