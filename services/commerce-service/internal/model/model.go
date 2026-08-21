package model

import "time"

type Wallet struct {
	UserID      string    `json:"user_id"`
	CoinBalance int64     `json:"coin_balance"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreatorBalance struct {
	UserID      string    `json:"user_id"`
	EarnedCoins int64     `json:"earned_coins"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CoinPurchase struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Coins       int64     `json:"coins"`
	AmountCents int       `json:"amount_cents"`
	ChargeID    string    `json:"charge_id"`
	CreatedAt   time.Time `json:"created_at"`
}

type Gift struct {
	ID          string    `json:"id"`
	SenderID    string    `json:"sender_id"`
	RecipientID string    `json:"recipient_id"`
	ChannelID   string    `json:"channel_id"`
	GiftType    string    `json:"gift_type"`
	CoinCost    int64     `json:"coin_cost"`
	CreatedAt   time.Time `json:"created_at"`
}
