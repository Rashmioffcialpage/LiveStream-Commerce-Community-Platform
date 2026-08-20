# LiveStream Commerce & Community Platform

A mini Twitch + creator-commerce platform: live streaming, real-time chat,
subscriptions, virtual gifting, notifications, search, and ML-driven
recommendations — built as a set of independent Go microservices behind an
API gateway, the same distributed-systems shape as a production streaming
platform.

Built incrementally, one working vertical slice at a time — each task is
implemented, run, and verified end to end before the next one starts.

## Target architecture

```
                    Next.js React Frontend
                               |
                         API Gateway
                               |
        +----------------------+----------------------+
        |                      |                       |
   Auth Service          Stream Service            Commerce
        |                      |                       |
   PostgreSQL               Redis                  PostgreSQL
                               |
                             Kafka
                               |
              +----------------+----------------+
              |                |                |
         Chat Service     Notification      Analytics
              |                |                |
          WebSocket           Email          ClickHouse
```

Frontend: Next.js + TypeScript + Tailwind + React Query + WebSockets.
Backend: Go microservices. Data: PostgreSQL, Redis, Kafka, OpenSearch,
ClickHouse. Infra: Docker, Kubernetes, AWS (EKS/RDS/ElastiCache/MSK/S3/
CloudFront) — local first, cloud once the local system is solid.

## Progress

| Task | Service | Status |
|---|---|---|
| 1 | `auth-service` — signup/login, JWT, refresh rotation, Google OAuth, role-based access | ✅ done |
| 2 | `stream-service` — channels, scheduling, WebRTC signaling | ✅ done |
| 3 | `chat-service` — WebSocket chat, Redis fan-out, Kafka history | ✅ done |
| 4 | `subscription-service` + `payment-service` | not started |
| 5 | `commerce-service` — wallet, virtual gifts, creator balance | not started |
| 6 | `notification-service` — Kafka + WebSocket + email | not started |
| 7 | search (OpenSearch) | not started |
| 8 | `recommendation-service` — feature pipeline, model, Redis-served feed | not started |
| 9 | Kubernetes + AWS deployment | not started |

## Task 1 — auth-service

`services/auth-service` — Go, `net/http` (stdlib router), PostgreSQL via
`pgx`, JWT via `golang-jwt/jwt`, bcrypt password hashing, Google OAuth via
`golang.org/x/oauth2`.

**Endpoints:**

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | — | |
| POST | `/signup` | — | `{email, password, display_name, role?}` — `role` is `viewer` (default) or `creator` |
| POST | `/login` | — | `{email, password}` |
| POST | `/refresh` | — | `{refresh_token}` — rotates: old token is revoked, a new pair is issued |
| GET | `/me` | bearer JWT | |
| GET | `/oauth/google/login` | — | redirects to Google; 501 if `GOOGLE_CLIENT_ID` isn't set |
| GET | `/oauth/google/callback` | — | exchanges code, upserts user, issues tokens |
| GET | `/creator/ping` | bearer JWT, `creator` role | proves `RequireRole` middleware end to end; real creator routes land in `stream-service` |

**Design notes:**

- Access tokens are short-lived (15min) stateless JWTs; refresh tokens are
  random opaque strings, stored **hashed** (SHA-256) in Postgres so a DB
  read can't be replayed as a live token, and **rotated** on every use —
  the token in a `/refresh` response is single-use.
- OAuth identities live in their own `oauth_identities` table
  (`provider`, `provider_user_id`) so a user can later link multiple
  providers to one account, and `GetOrCreateUserByOAuth` runs as a single
  transaction to avoid a race creating two user rows for the same login.
- Passwordless accounts (pure OAuth signup) are supported —
  `password_hash` is nullable.

## Task 2 — stream-service

`services/stream-service` — Go, PostgreSQL (own database, `stream`) for
channels/streams, Redis for ephemeral live-viewer presence, `gorilla/
websocket` for the signaling relay. Verifies JWTs auth-service issued
(same `JWT_SECRET`, no shared database, no shared Go package — see the
comment atop `internal/auth/jwt.go`) rather than depending on auth-service
at runtime.

**REST endpoints:**

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | — | |
| POST | `/channels` | bearer JWT, `creator` role | `{slug, name, description?, category?}` |
| GET | `/channels/{slug}` | — | |
| GET | `/channels?category=` | — | |
| POST | `/channels/{slug}/streams` | bearer JWT, owner | `{title, tags?, scheduled_start_at?}` |
| GET | `/channels/{slug}/streams` | — | includes live `viewer_count` from Redis |
| POST | `/streams/{id}/go-live` | bearer JWT, owner | `scheduled` → `live`; 409 if the channel already has a live stream |
| POST | `/streams/{id}/end` | bearer JWT, owner | `live` → `ended` |
| GET | `/streams/{id}` | — | |
| GET | `/streams/{id}/signal?role=broadcaster\|viewer` | broadcaster: `?token=` query param (WS upgrades can't carry a header) | WebRTC signaling, see below |

**WebRTC signaling (`internal/signaling`):**

This app never touches media — browsers negotiate their own
`RTCPeerConnection` and exchange audio/video directly. The relay's only
job is delivering SDP offers/answers and ICE candidates between the one
broadcaster and each viewer for a stream, since two browsers otherwise
have no way to find each other. Star topology, one room per live stream:

```
Broadcaster ──offer, ICE──▶ Hub ──(tagged "from"/"to")──▶ Viewer N
Broadcaster ◀──answer, ICE── Hub ◀────────────────────── Viewer N
```

- A viewer connecting fires `viewer-joined` (with its connection ID) to
  the broadcaster, so the broadcaster's client can create a new
  `RTCPeerConnection` and send that viewer a targeted offer.
- Every relayed message is re-tagged server-side (`from`/`to` set by the
  hub, not trusted from the sender) — a viewer physically cannot address
  a message to another viewer or spoof its own connection ID.
- `viewer-count` is broadcast to everyone in the room on join/leave,
  backed by a Redis set per stream (`internal/realtime`) so the count
  survives independent of any single connection's state.
- Verified end to end with a scripted two-client test exercising
  join → offer → answer → ICE both directions → disconnect → count
  update — see the Task 2 commit.
- Scaling past a handful of concurrent viewers per stream needs a real
  SFU (mediasoup, LiveKit) in front of this relay — deliberately out of
  scope, see the package doc comment in `internal/signaling/hub.go`.

**Design notes:**

- `channels`/`streams` reference `creator_id`/`channel_id` as plain UUIDs,
  not foreign keys into auth-service's database — cross-service
  references are validated against the JWT at write time, not enforced
  by the database, since the two services don't share a database.
- Only one `live` stream per channel is enforced with a partial unique
  index (`WHERE status = 'live'`), not application logic alone, so two
  concurrent `go-live` calls can't both succeed.

## Task 3 — chat-service

`services/chat-service` — Go, own Postgres (`chat`), own Redis, Kafka
(introduced to the platform here). Verifies auth-service's JWTs the same
way stream-service does; calls stream-service over HTTP (`internal/
streamclient`) to check a stream is live and to resolve stream → channel →
`creator_id` for moderation authorization.

**Endpoints:**

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | — | |
| GET | `/streams/{id}/chat/history` | — | last 50 messages, chronological |
| GET | `/streams/{id}/chat` | bearer JWT via `?token=` (see below) | live chat WebSocket |
| POST | `/streams/{id}/chat/mute` | bearer JWT, channel owner | `{user_id, duration_seconds?}` — default 10min, Redis-TTL'd (a timeout, not a permanent ban) |
| POST | `/streams/{id}/chat/unmute` | bearer JWT, channel owner | `{user_id}` |
| DELETE | `/messages/{id}` | bearer JWT, channel owner | soft-delete; broadcasts `message-deleted` live |

The chat WebSocket takes its token as `?token=`, not an `Authorization`
header — a native browser `WebSocket` can't set custom headers on the
upgrade request, same constraint stream-service's broadcaster signaling
socket hit first (see `internal/auth/jwt.go`'s `RequireAuthWS`).

**Design notes:**

- **Two paths per message, split by concern.** An accepted message is
  broadcast immediately over Redis Pub/Sub (any chat-service replica
  holding a viewer's WebSocket needs to hear it, not just the replica
  that received it) and produced to Kafka for durable history — latency
  path and durability path, same split as fraud-detection's decision
  cache vs. Postgres. A slow Kafka broker delays when a message shows up
  in `/history`, never when it shows up on screen.
- **Idempotent by design, not by luck.** The Kafka consumer's Postgres
  insert is `id`-keyed with `ON CONFLICT DO NOTHING`, and that `id` is
  generated by chat-service at WebSocket-ingress time — a redelivered
  Kafka message (kafka-go's consumer group is at-least-once) can't
  double-insert.
- **Moderation is rejected before it exists anywhere.** Banned-word /
  invalid-reaction / muted / rate-limited messages are never broadcast
  and never produced to Kafka — the sender gets a targeted `error` frame
  back, nobody else sees the message happen.
- Rate limiting is a fixed 10-second window (5 messages), not a true
  sliding window — documented trade-off in `internal/realtime/
  ratelimit.go`: it can admit up to 2x the limit across a window
  boundary, which is fine for an abuse guard, not fine for a precise quota.

## Running locally

```bash
docker compose up -d --build
```

- auth-service: http://localhost:8080 · Postgres: localhost:5433
- stream-service: http://localhost:8081 · Postgres: localhost:5434 · Redis: localhost:6381
- chat-service: http://localhost:8082 · Postgres: localhost:5435 · Redis: localhost:6382
- Kafka: localhost:9194

(host ports remapped throughout to avoid clashing with other local
projects; every container-internal port is the standard one)

```bash
# signup as a creator, then use the returned access_token below
curl -s -X POST localhost:8080/signup -H 'content-type: application/json' \
  -d '{"email":"you@example.com","password":"password123","display_name":"You","role":"creator"}'

curl -s -X POST localhost:8081/channels -H "Authorization: Bearer <token>" -H 'content-type: application/json' \
  -d '{"slug":"your-channel","name":"Your Channel"}'
```

Google OAuth is optional locally — set `GOOGLE_CLIENT_ID` /
`GOOGLE_CLIENT_SECRET` env vars before `docker compose up` if you want to
exercise it; without them `/oauth/google/login` returns `501`.

## Testing

```bash
cd services/auth-service && go build ./... && go vet ./...
cd services/stream-service && go build ./... && go vet ./...
cd services/chat-service && go build ./... && go vet ./...
```

Both services so far are covered by manual end-to-end passes against the
running stack, documented in each task's commit (`auth-service`: signup/
login/refresh-rotation/RBAC; `stream-service`: channel + stream lifecycle,
ownership boundaries, and the full WebRTC signaling relay via a scripted
two-client test). A real unit-test suite (`pgxmock` for the DB layer) is
worth adding before this grows much further — flagged here rather than
silently deferred indefinitely.
