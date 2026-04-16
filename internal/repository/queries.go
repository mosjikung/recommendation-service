package repository

import (
	"context"
	"fmt"

	"recommendation-service/internal/model"
)

// GetUserByID fetches a single user by primary key.
func (r *Repository) GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	row := r.db.QueryRow(ctx,
		`SELECT id, age, country, subscription_type, created_at
		   FROM users
		  WHERE id = $1`, userID)

	u := &model.User{}
	err := row.Scan(&u.ID, &u.Age, &u.Country, &u.SubscriptionType, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get user %d: %w", userID, err)
	}
	return u, nil
}

// GetUserWatchHistoryWithGenres returns the 50 most-recent watch events
// for a user, joined with genre data from the content table.
func (r *Repository) GetUserWatchHistoryWithGenres(ctx context.Context, userID int64) ([]model.WatchHistoryItem, error) {
	rows, err := r.db.Query(ctx,
		`SELECT c.id, c.genre, uwh.watched_at
		   FROM user_watch_history uwh
		   JOIN content c ON uwh.content_id = c.id
		  WHERE uwh.user_id = $1
		  ORDER BY uwh.watched_at DESC
		  LIMIT 50`, userID)
	if err != nil {
		return nil, fmt.Errorf("get watch history for user %d: %w", userID, err)
	}
	defer rows.Close()

	var items []model.WatchHistoryItem
	for rows.Next() {
		var item model.WatchHistoryItem
		if err := rows.Scan(&item.ContentID, &item.Genre, &item.WatchedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// GetUnwatchedContent returns up to 100 candidate content items
// that the user has NOT yet watched, ordered by popularity descending.
func (r *Repository) GetUnwatchedContent(ctx context.Context, userID int64) ([]model.Content, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, title, genre, popularity_score, created_at
		   FROM content
		  WHERE id NOT IN (
		        SELECT content_id
		          FROM user_watch_history
		         WHERE user_id = $1
		        )
		  ORDER BY popularity_score DESC
		  LIMIT 100`, userID)
	if err != nil {
		return nil, fmt.Errorf("get unwatched content for user %d: %w", userID, err)
	}
	defer rows.Close()

	var items []model.Content
	for rows.Next() {
		var c model.Content
		if err := rows.Scan(&c.ID, &c.Title, &c.Genre, &c.PopularityScore, &c.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// GetUserIDsPaginated returns a page of user IDs for batch processing.
// page and limit follow 1-based pagination.
func (r *Repository) GetUserIDsPaginated(ctx context.Context, page, limit int) ([]int64, int, error) {
	// Total count
	var total int
	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	offset := (page - 1) * limit
	rows, err := r.db.Query(ctx,
		`SELECT id FROM users ORDER BY id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("paginate users: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	return ids, total, rows.Err()
}
