package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"payment-service/internal/model"
)

var ErrNotFound = errors.New("not found")

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

// GetChargeByIdempotencyKey lets a caller check "did this already happen"
// before InsertCharge would hit the UNIQUE constraint -- used to return
// the original charge's result on a retried request instead of an error.
func (d *DB) GetChargeByIdempotencyKey(ctx context.Context, key string) (*model.Charge, error) {
	var c model.Charge
	err := d.Pool.QueryRow(ctx, `
		SELECT id, user_id, amount_cents, currency, description, status, idempotency_key, created_at
		FROM charges WHERE idempotency_key = $1
	`, key).Scan(&c.ID, &c.UserID, &c.AmountCents, &c.Currency, &c.Description, &c.Status, &c.IdempotencyKey, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (d *DB) InsertCharge(ctx context.Context, userID string, amountCents int, currency, description string, status model.ChargeStatus, idempotencyKey string) (*model.Charge, error) {
	var c model.Charge
	err := d.Pool.QueryRow(ctx, `
		INSERT INTO charges (user_id, amount_cents, currency, description, status, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, amount_cents, currency, description, status, idempotency_key, created_at
	`, userID, amountCents, currency, description, status, idempotencyKey).Scan(
		&c.ID, &c.UserID, &c.AmountCents, &c.Currency, &c.Description, &c.Status, &c.IdempotencyKey, &c.CreatedAt,
	)
	return &c, err
}
