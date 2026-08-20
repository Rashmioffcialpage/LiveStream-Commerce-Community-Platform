package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"stream-service/internal/model"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("already exists")
var ErrAlreadyLive = errors.New("channel already has a live stream")

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

func (d *DB) CreateChannel(ctx context.Context, creatorID, slug, name, description, category string) (*model.Channel, error) {
	var c model.Channel
	err := d.Pool.QueryRow(ctx, `
		INSERT INTO channels (creator_id, slug, name, description, category)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, creator_id, slug, name, description, category, created_at
	`, creatorID, slug, name, description, category).Scan(
		&c.ID, &c.CreatorID, &c.Slug, &c.Name, &c.Description, &c.Category, &c.CreatedAt,
	)
	if isUniqueViolation(err) {
		return nil, ErrConflict
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (d *DB) GetChannelBySlug(ctx context.Context, slug string) (*model.Channel, error) {
	var c model.Channel
	err := d.Pool.QueryRow(ctx, `
		SELECT id, creator_id, slug, name, description, category, created_at
		FROM channels WHERE slug = $1
	`, slug).Scan(&c.ID, &c.CreatorID, &c.Slug, &c.Name, &c.Description, &c.Category, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (d *DB) GetChannelByID(ctx context.Context, id string) (*model.Channel, error) {
	var c model.Channel
	err := d.Pool.QueryRow(ctx, `
		SELECT id, creator_id, slug, name, description, category, created_at
		FROM channels WHERE id = $1
	`, id).Scan(&c.ID, &c.CreatorID, &c.Slug, &c.Name, &c.Description, &c.Category, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

func (d *DB) ListChannels(ctx context.Context, category string, limit int) ([]model.Channel, error) {
	var rows pgx.Rows
	var err error
	if category != "" {
		rows, err = d.Pool.Query(ctx, `
			SELECT id, creator_id, slug, name, description, category, created_at
			FROM channels WHERE category = $1 ORDER BY created_at DESC LIMIT $2
		`, category, limit)
	} else {
		rows, err = d.Pool.Query(ctx, `
			SELECT id, creator_id, slug, name, description, category, created_at
			FROM channels ORDER BY created_at DESC LIMIT $1
		`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	channels := []model.Channel{}
	for rows.Next() {
		var c model.Channel
		if err := rows.Scan(&c.ID, &c.CreatorID, &c.Slug, &c.Name, &c.Description, &c.Category, &c.CreatedAt); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, rows.Err()
}

func (d *DB) CreateStream(ctx context.Context, channelID, title string, tags []string, scheduledStartAt any) (*model.Stream, error) {
	var s model.Stream
	err := d.Pool.QueryRow(ctx, `
		INSERT INTO streams (channel_id, title, tags, scheduled_start_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, channel_id, title, tags, status, scheduled_start_at, started_at, ended_at, created_at
	`, channelID, title, tags, scheduledStartAt).Scan(
		&s.ID, &s.ChannelID, &s.Title, &s.Tags, &s.Status, &s.ScheduledStartAt, &s.StartedAt, &s.EndedAt, &s.CreatedAt,
	)
	return &s, err
}

func (d *DB) GetStream(ctx context.Context, id string) (*model.Stream, error) {
	var s model.Stream
	err := d.Pool.QueryRow(ctx, `
		SELECT id, channel_id, title, tags, status, scheduled_start_at, started_at, ended_at, created_at
		FROM streams WHERE id = $1
	`, id).Scan(&s.ID, &s.ChannelID, &s.Title, &s.Tags, &s.Status, &s.ScheduledStartAt, &s.StartedAt, &s.EndedAt, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

func (d *DB) ListStreamsByChannel(ctx context.Context, channelID string) ([]model.Stream, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, channel_id, title, tags, status, scheduled_start_at, started_at, ended_at, created_at
		FROM streams WHERE channel_id = $1 ORDER BY scheduled_start_at DESC
	`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	streams := []model.Stream{}
	for rows.Next() {
		var s model.Stream
		if err := rows.Scan(&s.ID, &s.ChannelID, &s.Title, &s.Tags, &s.Status, &s.ScheduledStartAt, &s.StartedAt, &s.EndedAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		streams = append(streams, s)
	}
	return streams, rows.Err()
}

// GoLive transitions scheduled -> live. Relies on one_live_stream_per_channel
// (a partial unique index on streams(channel_id) WHERE status='live') to
// reject a second concurrent "go live" for the same channel -- caught here
// as ErrAlreadyLive rather than a raw constraint-violation error.
func (d *DB) GoLive(ctx context.Context, id string) (*model.Stream, error) {
	var s model.Stream
	err := d.Pool.QueryRow(ctx, `
		UPDATE streams SET status = 'live', started_at = now()
		WHERE id = $1 AND status = 'scheduled'
		RETURNING id, channel_id, title, tags, status, scheduled_start_at, started_at, ended_at, created_at
	`, id).Scan(&s.ID, &s.ChannelID, &s.Title, &s.Tags, &s.Status, &s.ScheduledStartAt, &s.StartedAt, &s.EndedAt, &s.CreatedAt)
	if isUniqueViolation(err) {
		return nil, ErrAlreadyLive
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

func (d *DB) EndStream(ctx context.Context, id string) (*model.Stream, error) {
	var s model.Stream
	err := d.Pool.QueryRow(ctx, `
		UPDATE streams SET status = 'ended', ended_at = now()
		WHERE id = $1 AND status = 'live'
		RETURNING id, channel_id, title, tags, status, scheduled_start_at, started_at, ended_at, created_at
	`, id).Scan(&s.ID, &s.ChannelID, &s.Title, &s.Tags, &s.Status, &s.ScheduledStartAt, &s.StartedAt, &s.EndedAt, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &s, err
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
