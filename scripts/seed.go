package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Fixed seed — guarantees reproducible data across runs.
const randomSeed = 42

var (
	countries         = []string{"US", "GB", "CA", "AU", "DE", "FR", "JP", "BR", "IN", "SG"}
	subscriptionTypes = []string{"free", "basic", "premium"}
	subWeights        = []float64{0.5, 0.3, 0.2}
	genres            = []string{"action", "drama", "comedy", "thriller", "documentary", "romance", "sci-fi"}
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://user:password@localhost:5432/recommendations?sslmode=disable"
	}

	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	rng := rand.New(rand.NewSource(randomSeed))

	ctx := context.Background()

	// Truncate in reverse FK order so we can re-seed cleanly
	log.Println("truncating tables...")
	_, err = db.Exec(ctx, `TRUNCATE user_watch_history, content, users RESTART IDENTITY CASCADE`)
	if err != nil {
		log.Fatalf("truncate: %v", err)
	}

	// Seed users (min 20)
	log.Println("seeding users...")
	userIDs := seedUsers(ctx, db, rng, 20)

	// Seed content (min 50)
	log.Println("seeding content...")
	contentIDs := seedContent(ctx, db, rng, 50)

	// Seed watch history (min 200)
	log.Println("seeding watch history...")
	seedWatchHistory(ctx, db, rng, userIDs, contentIDs, 200)

	log.Printf("seeding complete: %d users, %d content, 200+ watch records", len(userIDs), len(contentIDs))
}

func seedUsers(ctx context.Context, db *pgxpool.Pool, rng *rand.Rand, n int) []int64 {
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		age := 18 + rng.Intn(48) // 18–65
		country := countries[rng.Intn(len(countries))]
		sub := weightedChoice(rng, subscriptionTypes, subWeights)

		var id int64
		err := db.QueryRow(ctx,
			`INSERT INTO users (age, country, subscription_type) VALUES ($1, $2, $3) RETURNING id`,
			age, country, sub,
		).Scan(&id)
		if err != nil {
			log.Fatalf("insert user: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

func seedContent(ctx context.Context, db *pgxpool.Pool, rng *rand.Rand, n int) []int64 {
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		genre := genres[rng.Intn(len(genres))]
		title := fmt.Sprintf("%s Title %d", capitalize(genre), i+1)

		// Power-law popularity: most content is mid-tier, a few are very popular
		popularity := math.Min(1.0, math.Abs(rng.NormFloat64()*0.3+0.4))

		// Spread creation dates over last 2 years for recency diversity
		daysAgo := rng.Intn(730)
		createdAt := time.Now().AddDate(0, 0, -daysAgo)

		var id int64
		err := db.QueryRow(ctx,
			`INSERT INTO content (title, genre, popularity_score, created_at) VALUES ($1, $2, $3, $4) RETURNING id`,
			title, genre, popularity, createdAt,
		).Scan(&id)
		if err != nil {
			log.Fatalf("insert content: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

func seedWatchHistory(ctx context.Context, db *pgxpool.Pool, rng *rand.Rand, userIDs, contentIDs []int64, minRecords int) {
	inserted := 0
	seen := make(map[string]bool)

	for inserted < minRecords {
		userID := userIDs[rng.Intn(len(userIDs))]
		contentID := contentIDs[rng.Intn(len(contentIDs))]
		key := fmt.Sprintf("%d-%d", userID, contentID)

		if seen[key] {
			continue // avoid duplicate watch entries
		}
		seen[key] = true

		watchedAt := time.Now().AddDate(0, 0, -rng.Intn(180))

		_, err := db.Exec(ctx,
			`INSERT INTO user_watch_history (user_id, content_id, watched_at) VALUES ($1, $2, $3)`,
			userID, contentID, watchedAt,
		)
		if err != nil {
			log.Fatalf("insert watch history: %v", err)
		}
		inserted++
	}
}

// weightedChoice picks an item according to provided weights.
func weightedChoice(rng *rand.Rand, items []string, weights []float64) string {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	r := rng.Float64() * total
	for i, w := range weights {
		r -= w
		if r <= 0 {
			return items[i]
		}
	}
	return items[len(items)-1]
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(s[0]-32) + s[1:]
}
