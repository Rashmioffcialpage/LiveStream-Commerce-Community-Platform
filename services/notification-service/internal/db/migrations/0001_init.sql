CREATE TYPE notification_type AS ENUM ('new_subscriber', 'gift_received', 'stream_started');

CREATE TABLE notifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL, -- the recipient
    type            notification_type NOT NULL,
    title           TEXT NOT NULL,
    body            TEXT NOT NULL,
    read_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user_id_created_at ON notifications(user_id, created_at DESC);
