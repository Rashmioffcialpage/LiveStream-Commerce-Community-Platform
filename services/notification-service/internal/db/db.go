package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"notification-service/internal/model"
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

func (d *DB) Insert(ctx context.Context, userID string, typ model.NotificationType, title, body string) (*model.Notification, error) {
	var n model.Notification
	err := d.Pool.QueryRow(ctx, `
		INSERT INTO notifications (user_id, type, title, body)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, type, title, body, read_at, created_at
	`, userID, typ, title, body).Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.ReadAt, &n.CreatedAt)
	return &n, err
}

func (d *DB) ListByUser(ctx context.Context, userID string, limit int) ([]model.Notification, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, user_id, type, title, body, read_at, created_at
		FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notifications := []model.Notification{}
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	return notifications, rows.Err()
}

func (d *DB) MarkRead(ctx context.Context, id, userID string) (*model.Notification, error) {
	var n model.Notification
	err := d.Pool.QueryRow(ctx, `
		UPDATE notifications SET read_at = now()
		WHERE id = $1 AND user_id = $2 AND read_at IS NULL
		RETURNING id, user_id, type, title, body, read_at, created_at
	`, id, userID).Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.ReadAt, &n.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &n, err
}
