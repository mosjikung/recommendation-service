package model

import (
	"fmt"
	"math/rand"
	"sort"
	"time"
)

// Client simulates an ML recommendation model.
// In production this would call an external model-serving endpoint.
type Client struct{}

func NewClient() *Client {
	return &Client{}
}

// Score takes a user, their watch history, and candidate content items,
// then returns the top-N scored recommendations.
//
// Scoring formula (weights sum to ~1.0):
//   popularity  * 0.40
//   genre_boost * 0.35
//   recency     * 0.15
//   noise       * 0.10
func (c *Client) Score(
	user *User,
	history []WatchHistoryItem,
	candidates []Content,
	limit int,
) ([]ScoredContent, error) {

	// 7.3 — Simulate realistic model latency (30-50 ms)
	latency := time.Duration(30+rand.Intn(21)) * time.Millisecond
	time.Sleep(latency)

	// 7.3 — Simulate 1.5% random failure rate
	if rand.Float64() < 0.015 {
		return nil, fmt.Errorf("model inference failed")
	}

	// Step 1: Build normalised genre preference map
	genrePrefs := buildGenrePreferences(history)

	// Step 2 & 3: Score every candidate
	scored := make([]ScoredContent, 0, len(candidates))
	now := time.Now()

	for _, content := range candidates {
		popularityComponent := content.PopularityScore * 0.40

		genreWeight := genrePrefs[content.Genre]
		if genreWeight == 0 {
			genreWeight = 0.1 // small default for unseen genres
		}
		genreBoost := genreWeight * 0.35

		daysSince := now.Sub(content.CreatedAt).Hours() / 24
		recencyFactor := 1.0 / (1.0 + daysSince/365.0)
		recencyComponent := recencyFactor * 0.15

		noise := (rand.Float64()*0.1 - 0.05) * 0.1 // ±0.005

		finalScore := popularityComponent + genreBoost + recencyComponent + noise

		scored = append(scored, ScoredContent{
			ContentID:       content.ID,
			Title:           content.Title,
			Genre:           content.Genre,
			PopularityScore: content.PopularityScore,
			Score:           finalScore,
		})
	}

	// Sort descending by score
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// Return top N
	if limit > len(scored) {
		limit = len(scored)
	}
	return scored[:limit], nil
}

// buildGenrePreferences normalises watch-history genre counts to [0, 1].
func buildGenrePreferences(history []WatchHistoryItem) map[string]float64 {
	counts := make(map[string]int)
	for _, item := range history {
		counts[item.Genre]++
	}

	total := 0
	for _, c := range counts {
		total += c
	}

	prefs := make(map[string]float64, len(counts))
	if total == 0 {
		return prefs
	}
	for genre, count := range counts {
		prefs[genre] = float64(count) / float64(total)
	}
	return prefs
}
