package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chat-service/internal/model"
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

// InsertMessage is the Kafka consumer's write path -- idempotent on id, so
// a redelivered Kafka message (at-least-once delivery) can't double-insert.
func (d *DB) InsertMessage(ctx context.Context, m model.ChatMessage) error {
	_, err := d.Pool.Exec(ctx, `
		INSERT INTO chat_messages (id, stream_id, user_id, display_name, type, body, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO NOTHING
	`, m.ID, m.StreamID, m.UserID, m.DisplayName, m.Type, m.Body, m.CreatedAt)
	return err
}

func (d *DB) History(ctx context.Context, streamID string, limit int) ([]model.ChatMessage, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, stream_id, user_id, display_name, type, body, deleted_at, created_at
		FROM chat_messages
		WHERE stream_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, streamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []model.ChatMessage{}
	for rows.Next() {
		var m model.ChatMessage
		if err := rows.Scan(&m.ID, &m.StreamID, &m.UserID, &m.DisplayName, &m.Type, &m.Body, &m.DeletedAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (d *DB) GetMessageByID(ctx context.Context, id string) (*model.ChatMessage, error) {
	var m model.ChatMessage
	err := d.Pool.QueryRow(ctx, `
		SELECT id, stream_id, user_id, display_name, type, body, deleted_at, created_at
		FROM chat_messages WHERE id = $1
	`, id).Scan(&m.ID, &m.StreamID, &m.UserID, &m.DisplayName, &m.Type, &m.Body, &m.DeletedAt, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &m, err
}

// SoftDelete marks a message deleted without erasing it -- moderation
// history (what was said, who removed it) stays auditable.
func (d *DB) SoftDelete(ctx context.Context, id string) (*model.ChatMessage, error) {
	var m model.ChatMessage
	err := d.Pool.QueryRow(ctx, `
		UPDATE chat_messages SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, stream_id, user_id, display_name, type, body, deleted_at, created_at
	`, id).Scan(&m.ID, &m.StreamID, &m.UserID, &m.DisplayName, &m.Type, &m.Body, &m.DeletedAt, &m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &m, err
}
