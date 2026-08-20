CREATE TYPE stream_status AS ENUM ('scheduled', 'live', 'ended');

CREATE TABLE channels (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- owned by a user in auth-service's database; no FK across service
    -- boundaries -- creator_id is validated against the JWT at write time,
    -- not against a shared users table.
    creator_id      UUID NOT NULL,
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    category        TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_channels_creator_id ON channels(creator_id);
CREATE INDEX idx_channels_category ON channels(category);

CREATE TABLE streams (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id          UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    title               TEXT NOT NULL,
    tags                TEXT[] NOT NULL DEFAULT '{}',
    status              stream_status NOT NULL DEFAULT 'scheduled',
    scheduled_start_at  TIMESTAMPTZ NOT NULL,
    started_at          TIMESTAMPTZ,
    ended_at            TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_streams_channel_id ON streams(channel_id);
CREATE INDEX idx_streams_status ON streams(status);

-- a channel can only have one stream that's actually live at a time;
-- enforced with a partial unique index rather than application logic
-- alone, so a race between two concurrent "go live" calls can't both succeed
CREATE UNIQUE INDEX one_live_stream_per_channel ON streams(channel_id) WHERE status = 'live';
