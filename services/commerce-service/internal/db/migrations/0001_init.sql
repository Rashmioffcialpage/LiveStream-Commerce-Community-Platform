-- Spendable coin balance for a user who has bought coins. Lazily created
-- (upserted) on first purchase, not provisioned at signup -- most users
-- never buy coins, so there's no reason every signup should write a row
-- here.
CREATE TABLE wallets (
    user_id         UUID PRIMARY KEY,
    coin_balance    BIGINT NOT NULL DEFAULT 0 CHECK (coin_balance >= 0),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Earned coin balance for a creator, from gifts received. Deliberately a
-- separate table/pool from wallets: a viewer's spendable balance (bought
-- with real money) and a creator's earned balance (accumulated from
-- gifts, would need a payout/cash-out flow to become real money) are not
-- the same currency even though both are denominated in "coins".
CREATE TABLE creator_balances (
    user_id         UUID PRIMARY KEY,
    earned_coins    BIGINT NOT NULL DEFAULT 0 CHECK (earned_coins >= 0),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE coin_purchases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    coins           BIGINT NOT NULL,
    amount_cents    INTEGER NOT NULL,
    charge_id       UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_coin_purchases_user_id ON coin_purchases(user_id);

CREATE TABLE gifts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id       UUID NOT NULL,
    recipient_id    UUID NOT NULL, -- the creator
    channel_id      UUID NOT NULL,
    gift_type       TEXT NOT NULL,
    coin_cost       BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_gifts_sender_id ON gifts(sender_id);
CREATE INDEX idx_gifts_recipient_id ON gifts(recipient_id);
