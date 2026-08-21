package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"subscription-service/internal/model"
)

var ErrNotFound = errors.New("not found")
var ErrAlreadySubscribed = errors.New("already subscribed")

type DB struct {
	Pool *pgxpool.Pool
}

func New(ctx context.Context, url string) (*DB, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Close() { d.Pool.Close() }

func (d *DB) CreateSubscription(ctx context.Context, subscriberID, channelID, chargeID string, periodEnd time.Time) (*model.Subscription, error) {
	var s model.Subscription
	err := d.Pool.QueryRow(ctx, `
		INSERT INTO subscriptions (subscriber_id, channel_id, charge_id, current_period_end)
		VALUES ($1, $2, $3, $4)
		RETURNING id, subscriber_id, channel_id, status, charge_id, current_period_end, created_at, cancelled_at
	`, subscriberID, channelID, chargeID, periodEnd).Scan(
		&s.ID, &s.SubscriberID, &s.ChannelID, &s.Status, &s.ChargeID, &s.CurrentPeriodEnd, &s.CreatedAt, &s.CancelledAt,
	)
	if isUniqueViolation(err) {
		return nil, ErrAlreadySubscribed
	}
	return &s, err
}

func (d *DB) HasActiveSubscription(ctx context.Context, subscriberID, channelID string) (bool, error) {
	var exists bool
	err := d.Pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM subscriptions WHERE subscriber_id = $1 AND channel_id = $2 AND status = 'active')
	`, subscriberID, channelID).Scan(&exists)
	return exists, err
}

func (d *DB) GetSubscription(ctx context.Context, id string) (*model.Subscription, error) {
	var s model.Subscription
	err := d.Pool.QueryRow(ctx, `
		SELECT id, subscriber_id, channel_id, status, charge_id, current_period_end, created_at, cancelled_at
		FROM subscriptions WHERE id = $1
	`, id).Scan(&s.ID, &s.SubscriberID, &s.ChannelID, &s.Status, &s.ChargeID, &s.CurrentPeriodEnd, &s.CreatedAt, &s.CancelledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

func (d *DB) ListActiveSubscribersByChannel(ctx context.Context, channelID string) ([]model.Subscription, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, subscriber_id, channel_id, status, charge_id, current_period_end, created_at, cancelled_at
		FROM subscriptions WHERE channel_id = $1 AND status = 'active' ORDER BY created_at DESC
	`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAll(rows)
}

func (d *DB) ListSubscriptionsBySubscriber(ctx context.Context, subscriberID string) ([]model.Subscription, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, subscriber_id, channel_id, status, charge_id, current_period_end, created_at, cancelled_at
		FROM subscriptions WHERE subscriber_id = $1 ORDER BY created_at DESC
	`, subscriberID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAll(rows)
}

func (d *DB) CancelSubscription(ctx context.Context, id string) (*model.Subscription, error) {
	var s model.Subscription
	err := d.Pool.QueryRow(ctx, `
		UPDATE subscriptions SET status = 'cancelled', cancelled_at = now()
		WHERE id = $1 AND status = 'active'
		RETURNING id, subscriber_id, channel_id, status, charge_id, current_period_end, created_at, cancelled_at
	`, id).Scan(&s.ID, &s.SubscriberID, &s.ChannelID, &s.Status, &s.ChargeID, &s.CurrentPeriodEnd, &s.CreatedAt, &s.CancelledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

func scanAll(rows pgx.Rows) ([]model.Subscription, error) {
	subs := []model.Subscription{}
	for rows.Next() {
		var s model.Subscription
		if err := rows.Scan(&s.ID, &s.SubscriberID, &s.ChannelID, &s.Status, &s.ChargeID, &s.CurrentPeriodEnd, &s.CreatedAt, &s.CancelledAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
