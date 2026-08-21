// Package features is the "Feature Pipeline" + "Redis" steps of the
// spec's diagram (User Events -> Kafka -> Feature Pipeline -> Recommendation
// Model -> Redis -> Personalized Home Feed). The feature itself is
// deliberately simple and interpretable: a per-user, per-category
// affinity score in a Redis sorted set, incremented by weighted events.
// A real recommender would likely add collaborative filtering (users who
// liked X also liked Y) or embeddings on top of this -- this is the
// baseline those would sit on, not a placeholder for something smarter
// that got cut; the pipeline mechanics (events -> features -> scoring ->
// serving, with a cache) are what this task demonstrates.
package features

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Event weights: a gift is a much stronger signal of genuine interest
// than a view, and a subscription (recurring, paid) sits between the two.
const (
	WeightView       = 1.0
	WeightSubscribed = 5.0
	WeightGift       = 10.0
)

type Store struct {
	rdb *redis.Client
}

func New(redisURL string) (*Store, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Store{rdb: redis.NewClient(opt)}, nil
}

func (s *Store) Close() error                   { return s.rdb.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.rdb.Ping(ctx).Err() }

func affinityKey(userID string) string { return fmt.Sprintf("affinity:%s", userID) }
func feedKey(userID string) string     { return fmt.Sprintf("feed:%s", userID) }

// RecordEvent nudges a user's affinity for a category. Called by the
// Kafka consumers in internal/handler for every view/subscribe/gift
// event -- this is the pipeline's only write path, no goroutine ever
// call it directly.
func (s *Store) RecordEvent(ctx context.Context, userID, category string, weight float64) error {
	if category == "" {
		return nil // nothing to attribute the signal to
	}
	return s.rdb.ZIncrBy(ctx, affinityKey(userID), weight, category).Err()
}

type CategoryScore struct {
	Category string
	Score    float64
}

// TopCategories is the "Recommendation Model": ranking is a direct read
// of the user's own accumulated affinity scores, highest first.
func (s *Store) TopCategories(ctx context.Context, userID string, n int) ([]CategoryScore, error) {
	results, err := s.rdb.ZRevRangeWithScores(ctx, affinityKey(userID), 0, int64(n-1)).Result()
	if err != nil {
		return nil, err
	}
	scores := make([]CategoryScore, len(results))
	for i, r := range results {
		scores[i] = CategoryScore{Category: r.Member.(string), Score: r.Score}
	}
	return scores, nil
}

const feedCacheTTL = 60 * time.Second

// CachedFeed / CacheFeed are the diagram's "-> Redis -> Personalized Home
// Feed" step: a short TTL, not because the ranking is expensive to
// recompute (it isn't, at this scale) but so a burst of page loads from
// one user doesn't redo the same ranking work repeatedly.
func (s *Store) CachedFeed(ctx context.Context, userID string, out any) (bool, error) {
	raw, err := s.rdb.Get(ctx, feedKey(userID)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) CacheFeed(ctx context.Context, userID string, feed any) error {
	raw, err := json.Marshal(feed)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, feedKey(userID), raw, feedCacheTTL).Err()
}
