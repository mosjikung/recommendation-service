package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"recommendation-service/internal/cache"
	"recommendation-service/internal/model"
	"recommendation-service/internal/repository"
)

// Sentinel errors — handlers map these to HTTP status codes.
var (
	ErrUserNotFound    = errors.New("user not found")
	ErrModelUnavailable = errors.New("model unavailable")
)

// workerPoolSize controls max concurrent goroutines in batch processing.
const workerPoolSize = 10

// Service orchestrates the recommendation workflow.
type Service struct {
	repo        *repository.Repository
	cache       *cache.Cache
	modelClient *model.Client
}

func New(repo *repository.Repository, cache *cache.Cache, modelClient *model.Client) *Service {
	return &Service{repo: repo, cache: cache, modelClient: modelClient}
}

// GetRecommendations returns personalised recommendations for a single user.
// It checks Redis first; on miss it runs the full pipeline and populates the cache.
func (s *Service) GetRecommendations(ctx context.Context, userID int64, limit int) (*model.RecommendationResponse, error) {

	// 1. Cache check
	cached, err := s.cache.Get(ctx, userID, limit)
	if err == nil && cached != nil {
		return &model.RecommendationResponse{
			UserID:          userID,
			Recommendations: cached,
			Metadata: model.ResponseMeta{
				CacheHit:    true,
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				TotalCount:  len(cached),
			},
		}, nil
	}

	// 2. Fetch user profile
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUserNotFound, err)
	}

	// 3. Fetch watch history (with genres for scoring)
	history, err := s.repo.GetUserWatchHistoryWithGenres(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetch watch history: %w", err)
	}

	// 4. Fetch candidate content (excluding already-watched)
	candidates, err := s.repo.GetUnwatchedContent(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetch candidates: %w", err)
	}

	// 5. Model inference (scoring + simulated latency/failures)
	recs, err := s.modelClient.Score(user, history, candidates, limit)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrModelUnavailable, err)
	}

	// 6. Populate cache (best-effort — don't fail the request on cache error)
	_ = s.cache.Set(ctx, userID, limit, recs)

	return &model.RecommendationResponse{
		UserID:          userID,
		Recommendations: recs,
		Metadata: model.ResponseMeta{
			CacheHit:    false,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			TotalCount:  len(recs),
		},
	}, nil
}

// GetBatchRecommendations processes a page of users concurrently.
// Individual failures are captured per-user and do not abort the batch.
func (s *Service) GetBatchRecommendations(ctx context.Context, page, limit int) (*model.BatchResponse, error) {
	start := time.Now()

	// Fetch paginated user IDs
	userIDs, total, err := s.repo.GetUserIDsPaginated(ctx, page, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch user ids: %w", err)
	}

	results := make([]model.BatchResult, len(userIDs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workerPoolSize) // bounded concurrency

	for i, uid := range userIDs {
		wg.Add(1)
		go func(idx int, userID int64) {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			resp, err := s.GetRecommendations(ctx, userID, 10)
			if err != nil {
				results[idx] = model.BatchResult{
					UserID:  userID,
					Status:  "failed",
					Error:   errorCode(err),
					Message: err.Error(),
				}
				return
			}
			results[idx] = model.BatchResult{
				UserID:          userID,
				Recommendations: resp.Recommendations,
				Status:          "success",
			}
		}(i, uid)
	}
	wg.Wait()

	// Aggregate summary
	var successCount, failedCount int
	for _, r := range results {
		if r.Status == "success" {
			successCount++
		} else {
			failedCount++
		}
	}

	return &model.BatchResponse{
		Page:       page,
		Limit:      limit,
		TotalUsers: total,
		Results:    results,
		Summary: model.BatchSummary{
			SuccessCount:     successCount,
			FailedCount:      failedCount,
			ProcessingTimeMs: time.Since(start).Milliseconds(),
		},
		Metadata: model.ResponseMeta{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

// errorCode converts a service error to the API error code string.
func errorCode(err error) string {
	switch {
	case errors.Is(err, ErrUserNotFound):
		return "user_not_found"
	case errors.Is(err, ErrModelUnavailable):
		return "model_inference_timeout"
	default:
		return "internal_error"
	}
}
