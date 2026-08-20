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
| 2 | `stream-service` — channels, scheduling, WebRTC signaling | not started |
| 3 | `chat-service` — WebSocket chat, Redis fan-out, Kafka history | not started |
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

## Running locally

```bash
docker compose up -d --build
```

- auth-service: http://localhost:8080
- Postgres: localhost:5433 (host port remapped to avoid clashing with
  other local projects; container-internal port is the standard 5432)

```bash
curl -s -X POST localhost:8080/signup -H 'content-type: application/json' \
  -d '{"email":"you@example.com","password":"password123","display_name":"You","role":"creator"}'
```

Google OAuth is optional locally — set `GOOGLE_CLIENT_ID` /
`GOOGLE_CLIENT_SECRET` env vars before `docker compose up` if you want to
exercise it; without them `/oauth/google/login` returns `501`.

## Testing

```bash
cd services/auth-service
go build ./... && go vet ./...
```

(Unit tests land alongside the next service that has real business logic
to isolate — `auth-service`'s behavior so far is covered by the manual
end-to-end pass documented in the Task 1 commit; a `pgxmock`-based test
suite is a natural Task 1.5 if this is followed up on rather than moving
straight to Task 2.)
