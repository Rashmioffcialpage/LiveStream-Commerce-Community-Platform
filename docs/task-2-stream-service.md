# Task 2 — stream-service

`services/stream-service` — Go, PostgreSQL (own database, `stream`) for
channels/streams, Redis for ephemeral live-viewer presence, `gorilla/
websocket` for the signaling relay. Verifies JWTs auth-service issued
(same `JWT_SECRET`, no shared database, no shared Go package — see the
comment atop `internal/auth/jwt.go`) rather than depending on auth-service
at runtime.

## REST endpoints

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | — | |
| GET | `/demo` | — | browser page that connects as a viewer and renders every signaling event live |
| POST | `/channels` | bearer JWT, `creator` role | `{slug, name, description?, category?}` |
| GET | `/channels/{slug}` | — | |
| GET | `/channels?category=` | — | |
| GET | `/internal/channels/{id}` | — (internal-only by convention) | service-to-service lookup; chat-service uses this to resolve moderation ownership |
| POST | `/channels/{slug}/streams` | bearer JWT, owner | `{title, tags?, scheduled_start_at?}` |
| GET | `/channels/{slug}/streams` | — | includes live `viewer_count` from Redis |
| POST | `/streams/{id}/go-live` | bearer JWT, owner | `scheduled` → `live`; 409 if the channel already has a live stream |
| POST | `/streams/{id}/end` | bearer JWT, owner | `live` → `ended` |
| POST | `/streams/{id}/recording` | bearer JWT, owner | multipart `file`; stream must be `ended`; uploads to S3-compatible storage, see below |
| GET | `/streams/{id}` | — | |
| GET | `/streams/{id}/signal?role=broadcaster\|viewer` | broadcaster: `?token=` query param (WS upgrades can't carry a header) | WebRTC signaling, see below |

## WebRTC signaling (`internal/signaling`)

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
- Scaling past a handful of concurrent viewers per stream needs a real
  SFU (mediasoup, LiveKit) in front of this relay — deliberately out of
  scope, see the package doc comment in `internal/signaling/hub.go`.

## Design notes

- `channels`/`streams` reference `creator_id`/`channel_id` as plain UUIDs,
  not foreign keys into auth-service's database — cross-service
  references are validated against the JWT at write time, not enforced
  by the database, since the two services don't share a database.
- Only one `live` stream per channel is enforced with a partial unique
  index (`WHERE status = 'live'`), not application logic alone, so two
  concurrent `go-live` calls can't both succeed.

## Recordings (S3-compatible storage)

`POST /streams/{id}/recording` (bearer JWT, owner, stream must be
`ended`) — multipart file upload, stored via `internal/storage` and the
real AWS S3 SDK (`aws-sdk-go-v2/service/s3`) pointed at a custom endpoint.
Locally that's MinIO (`docker-compose.yml`'s `minio` service); in
production it's just AWS S3 — leaving `S3_ENDPOINT` unset makes the SDK
fall back to AWS's real endpoints and normal credential resolution, so
going from MinIO to S3 is a config change, not a code change (same
build-locally-first pattern as this project's Terraform for the other
AWS pieces). `EnsureBucket` sets the bucket public-read as a local-dev
shortcut so the frontend can `<video src>` a recording directly; a real
deployment would keep it private and serve through CloudFront with
signed URLs instead.

## Verification

Channel creation + RBAC (creator vs viewer, 403 on the wrong role),
cross-owner protection on stream lifecycle routes (a different creator
can't end your stream), go-live idempotency (409 on retry), Redis
viewer-count visible live through the REST API, and the full signaling
relay via a scripted two-client test: join → offer → answer → ICE both
directions → disconnect → viewer-count update — see the Task 2 commit.
