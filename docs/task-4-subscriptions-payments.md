# Task 4 — subscription-service + payment-service

Implements the spec's chain: **Viewer → Subscribe → Payment Service →
Subscription DB → Kafka Event → (Creator Dashboard)**. Two services,
each owning its own Postgres database, talking to each other and to
`stream-service` over plain HTTP with the caller's own JWT forwarded --
no shared database, no shared Go package, same pattern as every other
service pair in this project.

## payment-service

A **simulated** payment processor -- no real card network, no real money
moves. What's real is the contract: idempotent charges, a durable
`charges` record, and a success/decline outcome, so swapping this for an
actual Stripe integration later changes this service's internals, not
how anything integrates against it.

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | — | |
| POST | `/charges` | bearer JWT | `{amount_cents, currency, description, idempotency_key}` — `user_id` comes from the JWT, never the body |

- `idempotency_key` is `UNIQUE` in Postgres; a repeated key returns the
  **original** charge's result instead of charging again.
- `amount_cents == 66600` always declines (returns `402`) -- a fixed
  "this card gets declined" sentinel, the same convention Stripe's own
  test-card numbers use, so callers can exercise the failure path
  deterministically without a real gateway's test mode.

## subscription-service

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | — | |
| POST | `/channels/{slug}/subscribe` | bearer JWT | charges `SubscriptionPriceCents` (flat $4.99, no tiers -- see config.go) via payment-service, then creates the subscription |
| GET | `/channels/{slug}/subscribers` | bearer JWT, channel owner | creator dashboard's subscriber list |
| GET | `/me/subscriptions` | bearer JWT | the caller's own subscriptions, any status |
| POST | `/subscriptions/{id}/cancel` | bearer JWT, subscriber | |

**Design notes:**

- **Charge before write, write before event.** Each step in `Subscribe`
  only runs if the previous one succeeded: no subscription row for a
  declined charge, no Kafka event for a subscription that doesn't exist.
  A pre-check (`HasActiveSubscription`) also runs *before* charging, so a
  duplicate-subscribe attempt doesn't waste a charge on a request that
  was always going to be rejected.
- **A cancelled subscription doesn't block resubscribing.** At most one
  `active` subscription per `(subscriber_id, channel_id)` is enforced by
  a partial unique index (`WHERE status = 'active'`) -- the same pattern
  as `stream-service`'s one-live-stream-per-channel guard. Cancelling
  then resubscribing creates a new row rather than fighting over the old
  one, preserving history.
- **The subscriber's own token pays for it.** `subscription-service`
  forwards the caller's bearer token to `payment-service` unchanged
  rather than asserting a `user_id` on their behalf -- it can't charge
  anyone but the person who's actually authenticated.
- Self-subscribing (a creator subscribing to their own channel) is
  rejected before any charge is attempted.
- `subscription-events` on Kafka (`{type: "subscribed"|"cancelled",
  subscription_id, subscriber_id, channel_id}`) is what Task 6's
  `notification-service` will consume -- not built yet, so today the
  event is produced and durably sits on the topic with no consumer.

## Verification

Full chain tested against the running stack: subscribe charges and
creates an active subscription; subscribing again is rejected (`409`)
*without* a second charge; a non-owner can't list subscribers (`403`); a
creator can't subscribe to their own channel (`400`); only the subscriber
can cancel their own subscription (`403` for anyone else); cancelling
then resubscribing succeeds with a new row; a direct `POST /charges` with
the decline sentinel returns `402` with a `status: "failed"` charge
record. `subscription-events` messages were read directly off the Kafka
topic (`kafka-console-consumer`) and matched the subscribe/cancel actions
taken. The frontend's Subscribe button and creator subscriber-count were
also driven through a headless-browser pass against the real UI, not just
the API.
