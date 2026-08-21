package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"commerce-service/internal/model"
)

var ErrNotFound = errors.New("not found")
var ErrInsufficientBalance = errors.New("insufficient coin balance")

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

func (d *DB) GetWallet(ctx context.Context, userID string) (*model.Wallet, error) {
	var w model.Wallet
	err := d.Pool.QueryRow(ctx, `
		SELECT user_id, coin_balance, updated_at FROM wallets WHERE user_id = $1
	`, userID).Scan(&w.UserID, &w.CoinBalance, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &model.Wallet{UserID: userID, CoinBalance: 0}, nil
	}
	return &w, err
}

func (d *DB) GetCreatorBalance(ctx context.Context, userID string) (*model.CreatorBalance, error) {
	var b model.CreatorBalance
	err := d.Pool.QueryRow(ctx, `
		SELECT user_id, earned_coins, updated_at FROM creator_balances WHERE user_id = $1
	`, userID).Scan(&b.UserID, &b.EarnedCoins, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return &model.CreatorBalance{UserID: userID, EarnedCoins: 0}, nil
	}
	return &b, err
}

// BuyCoins credits a wallet and records the purchase in one transaction --
// called only after payment-service has already confirmed the charge
// succeeded, so this is the "the money cleared, now give them the coins"
// step, not the payment step itself.
func (d *DB) BuyCoins(ctx context.Context, userID string, coins int64, amountCents int, chargeID string) (*model.Wallet, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO coin_purchases (user_id, coins, amount_cents, charge_id)
		VALUES ($1, $2, $3, $4)
	`, userID, coins, amountCents, chargeID); err != nil {
		return nil, err
	}

	var w model.Wallet
	err = tx.QueryRow(ctx, `
		INSERT INTO wallets (user_id, coin_balance, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (user_id) DO UPDATE SET coin_balance = wallets.coin_balance + $2, updated_at = now()
		RETURNING user_id, coin_balance, updated_at
	`, userID, coins).Scan(&w.UserID, &w.CoinBalance, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &w, tx.Commit(ctx)
}

// SendGift debits the sender, credits the creator, and records the gift
// in one transaction -- a row lock on the sender's wallet (SELECT ...
// FOR UPDATE) makes two concurrent gifts from the same low-balance
// sender resolve correctly (one succeeds, one sees the post-debit
// balance and correctly fails) instead of racing on a stale read.
func (d *DB) SendGift(ctx context.Context, senderID, recipientID, channelID, giftType string, coinCost int64) (*model.Gift, error) {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO wallets (user_id, coin_balance) VALUES ($1, 0) ON CONFLICT DO NOTHING
	`, senderID); err != nil {
		return nil, err
	}
	var balance int64
	if err := tx.QueryRow(ctx, `
		SELECT coin_balance FROM wallets WHERE user_id = $1 FOR UPDATE
	`, senderID).Scan(&balance); err != nil {
		return nil, err
	}
	if balance < coinCost {
		return nil, ErrInsufficientBalance
	}

	if _, err := tx.Exec(ctx, `
		UPDATE wallets SET coin_balance = coin_balance - $2, updated_at = now() WHERE user_id = $1
	`, senderID, coinCost); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO creator_balances (user_id, earned_coins, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (user_id) DO UPDATE SET earned_coins = creator_balances.earned_coins + $2, updated_at = now()
	`, recipientID, coinCost); err != nil {
		return nil, err
	}

	var g model.Gift
	err = tx.QueryRow(ctx, `
		INSERT INTO gifts (sender_id, recipient_id, channel_id, gift_type, coin_cost)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, sender_id, recipient_id, channel_id, gift_type, coin_cost, created_at
	`, senderID, recipientID, channelID, giftType, coinCost).Scan(
		&g.ID, &g.SenderID, &g.RecipientID, &g.ChannelID, &g.GiftType, &g.CoinCost, &g.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &g, tx.Commit(ctx)
}

func (d *DB) ListGiftsReceived(ctx context.Context, recipientID string) ([]model.Gift, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, sender_id, recipient_id, channel_id, gift_type, coin_cost, created_at
		FROM gifts WHERE recipient_id = $1 ORDER BY created_at DESC
	`, recipientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGifts(rows)
}

func (d *DB) ListGiftsSent(ctx context.Context, senderID string) ([]model.Gift, error) {
	rows, err := d.Pool.Query(ctx, `
		SELECT id, sender_id, recipient_id, channel_id, gift_type, coin_cost, created_at
		FROM gifts WHERE sender_id = $1 ORDER BY created_at DESC
	`, senderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGifts(rows)
}

func scanGifts(rows pgx.Rows) ([]model.Gift, error) {
	gifts := []model.Gift{}
	for rows.Next() {
		var g model.Gift
		if err := rows.Scan(&g.ID, &g.SenderID, &g.RecipientID, &g.ChannelID, &g.GiftType, &g.CoinCost, &g.CreatedAt); err != nil {
			return nil, err
		}
		gifts = append(gifts, g)
	}
	return gifts, rows.Err()
}
