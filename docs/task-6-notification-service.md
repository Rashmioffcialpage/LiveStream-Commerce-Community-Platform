# Task 6 — notification-service

Implements the spec's real-time notifications (new subscriber, gift
received, stream started) over **Kafka + WebSocket + email**, per the
spec's list. `mention` and `follow` are deliberately not implemented --
see "Deliberately out of scope" below.

This is also where `subscription-events` and `gift-events` (produced
since Tasks 4 and 5, consumed by nothing until now) actually get used,
plus a new `stream-events` topic from `stream-service`.

## Endpoints

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | — | |
| GET | `/notifications` | bearer JWT | caller's own inbox, last 50, newest first |
| POST | `/notifications/{id}/read` | bearer JWT | marks one of the caller's own notifications read |
| GET | `/notifications/ws` | bearer JWT via `?token=` | live push; first frames replay the 10 most recent (oldest first) so a reconnect doesn't need a separate REST call to catch up |

## What produces the three events it consumes

| Kafka topic | Producer | Consumer group | Fires on |
|---|---|---|---|
| `subscription-events` | subscription-service (Task 4) | `notification-service-subscriptions` | `type: "subscribed"` (not `"cancelled"`) |
| `gift-events` | commerce-service (Task 5) | `notification-service-gifts` | every gift |
| `stream-events` | stream-service (**new this task**) | `notification-service-streams` | `go-live` succeeding |

`stream-service` previously had no Kafka producer at all -- this task
added one (`internal/kafka`), used for exactly this one event, not a
general event-sourcing layer for the service.

## Design notes

- **Three separate consumer groups, not one.** The first version of this
  used a single group ID (`"notification-service"`) for all three
  topics from three separate readers in the same process, and silently
  consumed nothing -- Kafka's group coordinator expects members of one
  group to have consistent topic subscriptions, and three readers with
  the same group ID but different single-topic subscriptions confused
  it. Caught by the end-to-end test (no notification ever arrived, no
  error either), fixed by giving each topic its own group ID
  (`notification-service-subscriptions` / `-gifts` / `-streams`) -- the
  actually-correct shape for "one process, several independent topic
  subscriptions" regardless of what caused the bug.
- **Durable row before ephemeral push.** `notify()` always inserts the
  Postgres row first, then attempts the Redis Pub/Sub push and the email
  send -- a crash between steps loses at most the live notification (the
  client sees it on next `GET /notifications` or WebSocket reconnect),
  never the notification itself.
- **Two new internal (unauthenticated-by-convention) endpoints** this
  task needed and added to already-shipped services: `auth-service`'s
  `GET /internal/users/{id}` (display_name + email, for a readable
  notification body and an actual send-to address) and
  `subscription-service`'s `GET /internal/channels/{id}/subscribers`
  (fanning "stream started" out to every active subscriber). Same
  internal-lookup convention as `stream-service`'s existing
  `GET /internal/channels/{id}`.
- **Email is a `Sender` interface with one implementation:**
  `ConsoleSender`, which logs what would have been sent. No SMTP
  credentials exist in this deployment -- same "build locally first"
  pattern as `payment-service`'s simulated charges and `stream-service`'s
  MinIO-standing-in-for-S3. A real provider (SES, SendGrid) is a second
  `Sender` implementation, not a change to any caller.

## Deliberately out of scope

- **`mention`** would need chat-service to detect `@name` tokens in
  messages and resolve them to a real `user_id` (today it only has a
  display name typed in a chat box, which isn't a reliable target for a
  notification -- two users can share a display name).
- **`follow`** (a free, non-monetary relationship, distinct from paid
  subscriptions) doesn't exist anywhere in this platform yet -- adding it
  would mean a new relationship model in `subscription-service` or a new
  service, not something notification-service should decide unilaterally
  while consuming events.

Both are flagged here rather than silently dropped.

## Verification

Full chain tested end to end against the running stack: viewer
subscribes -> creator's open WebSocket receives a `new_subscriber` push
within seconds, with the subscriber's actual display name resolved (not
a raw UUID); viewer sends a gift -> creator receives `gift_received` with
gift type and coin cost in the body; creator goes live -> the (now)
subscribed viewer receives `stream_started`. `GET /notifications`
confirmed both durable rows exist after the live pushes; `POST
.../read` confirmed marking one read. Email sends were confirmed via the
`ConsoleSender`'s log line for all three event types, addressed to the
correct resolved recipient email. Frontend: a notification bell with an
unread-count badge, driven through a headless-browser pass that
subscribed as one user and confirmed the notification appeared live in
the creator's dropdown in a second, independent browser session.
