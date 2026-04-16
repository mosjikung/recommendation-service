package model

import "time"

// --- Domain Entities ---

// User represents a record from the users table.
type User struct {
	ID               int64     `json:"id"`
	Age              int       `json:"age"`
	Country          string    `json:"country"`
	SubscriptionType string    `json:"subscription_type"`
	CreatedAt        time.Time `json:"created_at"`
}

// Content represents a record from the content table.
type Content struct {
	ID              int64     `json:"id"`
	Title           string    `json:"title"`
	Genre           string    `json:"genre"`
	PopularityScore float64   `json:"popularity_score"`
	CreatedAt       time.Time `json:"created_at"`
}

// WatchHistoryItem is the result of the JOIN between
// user_watch_history and content — used by the model for scoring.
type WatchHistoryItem struct {
	ContentID  int64     `json:"content_id"`
	Genre      string    `json:"genre"`
	WatchedAt  time.Time `json:"watched_at"`
}

// ScoredContent is a content item enriched with a recommendation score.
type ScoredContent struct {
	ContentID       int64   `json:"content_id"`
	Title           string  `json:"title"`
	Genre           string  `json:"genre"`
	PopularityScore float64 `json:"popularity_score"`
	Score           float64 `json:"score"`
}

// --- API Request / Response types ---

// RecommendationResponse is the JSON response for GET /users/{id}/recommendations.
type RecommendationResponse struct {
	UserID          int64           `json:"user_id"`
	Recommendations []ScoredContent `json:"recommendations"`
	Metadata        ResponseMeta    `json:"metadata"`
}

// ResponseMeta contains cache and timing information.
type ResponseMeta struct {
	CacheHit    bool   `json:"cache_hit"`
	GeneratedAt string `json:"generated_at"`
	TotalCount  int    `json:"total_count"`
}

// BatchResult is the per-user result inside the batch response.
type BatchResult struct {
	UserID          int64           `json:"user_id"`
	Recommendations []ScoredContent `json:"recommendations,omitempty"`
	Status          string          `json:"status"` // "success" | "failed"
	Error           string          `json:"error,omitempty"`
	Message         string          `json:"message,omitempty"`
}

// BatchResponse is the JSON response for GET /recommendations/batch.
type BatchResponse struct {
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	TotalUsers int           `json:"total_users"`
	Results    []BatchResult `json:"results"`
	Summary    BatchSummary  `json:"summary"`
	Metadata   ResponseMeta  `json:"metadata"`
}

// BatchSummary aggregates success/failure counts and timing.
type BatchSummary struct {
	SuccessCount     int   `json:"success_count"`
	FailedCount      int   `json:"failed_count"`
	ProcessingTimeMs int64 `json:"processing_time_ms"`
}

// ErrorResponse is the standard error body.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
