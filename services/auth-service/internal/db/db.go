package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"auth-service/internal/model"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("already exists")

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

func (d *DB) Close() {
	d.Pool.Close()
}

func (d *DB) CreateUser(ctx context.Context, email string, passwordHash *string, displayName string, role model.Role) (*model.User, error) {
	var u model.User
	err := d.Pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, display_name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, display_name, role, created_at
	`, email, passwordHash, displayName, role).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return &u, nil
}

func (d *DB) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	err := d.Pool.QueryRow(ctx, `
		SELECT id, email, password_hash, display_name, role, created_at
		FROM users WHERE email = $1
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (d *DB) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	err := d.Pool.QueryRow(ctx, `
		SELECT id, email, password_hash, display_name, role, created_at
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.Role, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetOrCreateUserByOAuth links an OAuth identity to an existing user with the
// same verified email if one exists, otherwise creates a new passwordless
// account. Runs as a single transaction so a concurrent signup can't race a
// duplicate oauth_identities row past the UNIQUE(provider, provider_user_id)
// constraint into two different user rows.
func (d *DB) GetOrCreateUserByOAuth(ctx context.Context, provider, providerUserID, email, displayName string) (*model.User, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx, `
		SELECT user_id FROM oauth_identities WHERE provider = $1 AND provider_user_id = $2
	`, provider, providerUserID).Scan(&userID)

	if errors.Is(err, pgx.ErrNoRows) {
		// no existing identity link -- match or create the user, then link it
		err = tx.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&userID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `
				INSERT INTO users (email, password_hash, display_name, role)
				VALUES ($1, NULL, $2, 'viewer')
				RETURNING id
			`, email, displayName).Scan(&userID)
		}
		if err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO oauth_identities (user_id, provider, provider_user_id)
			VALUES ($1, $2, $3)
		`, userID, provider, providerUserID); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	var u model.User
	err = tx.QueryRow(ctx, `
		SELECT id, email, display_name, role, created_at FROM users WHERE id = $1
	`, userID).Scan(&u.ID, &u.Email, &u.DisplayName, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &u, nil
}

func (d *DB) StoreRefreshToken(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	_, err := d.Pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash, expiresAt)
	return err
}

// ValidRefreshToken returns the associated user_id if the token hash exists,
// isn't revoked, and hasn't expired.
func (d *DB) ValidRefreshToken(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := d.Pool.QueryRow(ctx, `
		SELECT user_id FROM refresh_tokens
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
	`, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return userID, err
}

func (d *DB) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := d.Pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1
	`, tokenHash)
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
