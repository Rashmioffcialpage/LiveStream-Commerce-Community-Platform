CREATE TYPE charge_status AS ENUM ('succeeded', 'failed');

CREATE TABLE charges (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    amount_cents    INTEGER NOT NULL,
    currency        TEXT NOT NULL DEFAULT 'usd',
    description     TEXT NOT NULL DEFAULT '',
    status          charge_status NOT NULL,
    -- caller-supplied, so a retried "subscribe" request that already
    -- charged the card can't charge it twice; UNIQUE enforces it, not
    -- just application logic.
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_charges_user_id ON charges(user_id);
