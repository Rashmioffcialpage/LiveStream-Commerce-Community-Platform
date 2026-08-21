CREATE TYPE subscription_status AS ENUM ('active', 'cancelled');

CREATE TABLE subscriptions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscriber_id       UUID NOT NULL,
    channel_id          UUID NOT NULL,
    status              subscription_status NOT NULL DEFAULT 'active',
    charge_id           UUID NOT NULL, -- payment-service's charge record for this period
    current_period_end  TIMESTAMPTZ NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    cancelled_at        TIMESTAMPTZ
);

-- at most one ACTIVE subscription per (subscriber, channel); a cancelled
-- row doesn't block resubscribing since this index only covers status =
-- 'active' rows -- same partial-unique-index pattern as stream-service's
-- one_live_stream_per_channel.
CREATE UNIQUE INDEX one_active_sub_per_channel ON subscriptions(subscriber_id, channel_id) WHERE status = 'active';

CREATE INDEX idx_subscriptions_channel_id ON subscriptions(channel_id) WHERE status = 'active';
CREATE INDEX idx_subscriptions_subscriber_id ON subscriptions(subscriber_id);
