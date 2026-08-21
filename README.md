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

| Task | Service | Status | Details |
|---|---|---|---|
| 1 | `auth-service` — signup/login, JWT, refresh rotation, Google OAuth, role-based access | ✅ done | [docs/task-1-auth-service.md](docs/task-1-auth-service.md) |
| 2 | `stream-service` — channels, scheduling, WebRTC signaling | ✅ done | [docs/task-2-stream-service.md](docs/task-2-stream-service.md) |
| 3 | `chat-service` — WebSocket chat, Redis fan-out, Kafka history | ✅ done | [docs/task-3-chat-service.md](docs/task-3-chat-service.md) |
| — | `frontend` — Next.js web client (channel page, live chat, viewer count) | ✅ done | [docs/task-frontend.md](docs/task-frontend.md) |
| 4 | `subscription-service` + `payment-service` | ✅ done | [docs/task-4-subscriptions-payments.md](docs/task-4-subscriptions-payments.md) |
| 5 | `commerce-service` — wallet, virtual gifts, creator balance | ✅ done | [docs/task-5-commerce-service.md](docs/task-5-commerce-service.md) |
| 6 | `notification-service` — Kafka + WebSocket + email | not started | |
| 7 | search (OpenSearch) | not started | |
| 8 | `recommendation-service` — feature pipeline, model, Redis-served feed | not started | |
| 9 | Kubernetes + AWS deployment | not started | |

Each task's write-up (endpoints, design decisions, what was verified and
how) lives in its own file under [docs/](docs/) rather than in this
README, so this stays a map, not the whole territory.

## Running locally

```bash
docker compose up -d --build
```

- auth-service: http://localhost:8080 · Postgres: localhost:5433
- stream-service: http://localhost:8081 · Postgres: localhost:5434 · Redis: localhost:6381
- chat-service: http://localhost:8082 · Postgres: localhost:5435 · Redis: localhost:6382
- subscription-service: http://localhost:8083 · Postgres: localhost:5436
- payment-service: http://localhost:8084 · Postgres: localhost:5437
- commerce-service: http://localhost:8086 · Postgres: localhost:5438
- Kafka: localhost:9194
- MinIO (S3-compatible, stream recordings): localhost:9002 (API) · localhost:9003 (console, minioadmin/minioadmin)

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

**Frontend:**

```bash
cd frontend
cp .env.example .env.local   # points at the ports above; edit if you changed them
npm install
npm run dev -- -p 3002
```

Open http://localhost:3002 — sign up as a creator, create a channel, go
live, and chat, all through the actual UI.

## Testing

```bash
cd services/auth-service && go build ./... && go vet ./...
cd services/stream-service && go build ./... && go vet ./...
cd services/chat-service && go build ./... && go vet ./...
cd services/subscription-service && go build ./... && go vet ./...
cd services/payment-service && go build ./... && go vet ./...
cd services/commerce-service && go build ./... && go vet ./...
cd frontend && npm run build  # runs its own type check; see docs/task-frontend.md if you also want a standalone `tsc --noEmit`, which needs a prior build to see Next's generated route types
```

Every service so far is covered by manual end-to-end passes against the
running stack, documented in each task's writeup under [docs/](docs/). A
real unit-test suite (`pgxmock` for the Go services' DB layers) is worth
adding before this grows much further — flagged here rather than silently
deferred indefinitely.
