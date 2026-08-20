CREATE TYPE user_role AS ENUM ('viewer', 'creator');

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT NOT NULL UNIQUE,
    -- NULL for accounts created purely via OAuth (no local password set)
    password_hash   TEXT,
    display_name    TEXT NOT NULL,
    role            user_role NOT NULL DEFAULT 'viewer',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per (provider, provider_user_id): lets a user link multiple
-- OAuth identities to one account without duplicating the users row.
CREATE TABLE oauth_identities (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider            TEXT NOT NULL,
    provider_user_id    TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id)
);

-- Refresh tokens are stored hashed and revocable (e.g. on logout / password
-- change), unlike short-lived access tokens which are stateless JWTs.
CREATE TABLE refresh_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      TEXT NOT NULL UNIQUE,
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_oauth_identities_user_id ON oauth_identities(user_id);
