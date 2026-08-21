# Frontend — Next.js web client

`frontend/` — Next.js 16 (App Router, Turbopack), TypeScript, Tailwind v4.
The first client that exercises all three backend services together
through a real UI instead of curl/WebSocket test scripts.

## What it does

- **Auth** (`app/login`) — signup/login against `auth-service`, session
  (JWT + user) held in a React context and persisted to `localStorage`.
- **Channels** (`app/page.tsx`, `app/create-channel`) — list channels from
  `stream-service`; creators can create one.
- **Channel page** (`app/channel/[slug]`) — the combined view: channel
  info, a creator-only "schedule + go live" / "end stream" control, a
  live viewer count (via `stream-service`'s signaling WebSocket, consumed
  read-only for the count broadcast — see `lib/use-viewer-count.ts`), and
  live chat.
- **Chat** (`components/ChatPanel.tsx`) — connects to `chat-service`'s
  WebSocket when logged in (live send/receive, reactions, moderation
  events); falls back to read-only REST history when logged out, since
  chat's WebSocket requires a sender identity.
- **Recordings** (`components/PastStreamRow.tsx`) — a channel owner can
  upload a recording file for any of their ended streams; anyone can play
  it back via a plain `<video>` tag pointing at the S3/MinIO URL
  `stream-service` returns (see `docs/task-2-stream-service.md`'s
  Recordings section).

## Cross-cutting fix this required

All three Go services needed CORS headers added (`withCORS` in each
`cmd/server/main.go`) — a browser calling `stream-service`/`chat-service`/
`auth-service` from `localhost:3002` is a cross-origin request, and none
of the services had ever needed to answer one before (curl and Python
WebSocket clients don't enforce CORS). Wide open (`*`) for local dev,
documented as needing to be scoped to a real frontend origin in production.

## Verification

Full lifecycle driven through a headless-browser script (Playwright), not
just typecheck/build: sign up as a creator → create a channel → schedule
and go live → send a chat message → send a reaction → end the stream, all
through the real rendered UI, with browser console errors captured
(catches things `tsc`/`next build` can't, like the DOM-only regex bug
below).

**Bug found and fixed by that process:** the create-channel slug input's
HTML `pattern` attribute (`[a-z0-9][a-z0-9-]{1,48}[a-z0-9]`) failed to
compile under Chrome's newer strict regex parsing (`v`-flag character-class
rules treat a trailing `-` inside `[...]` as ambiguous) — logged a console
error on every page load and silently skipped that field's validation.
Fixed by escaping the hyphen (`[a-z0-9\-]`); confirmed clean on rerun.

## Known gaps (by design, for now)

- No access-token refresh — the 15-minute JWT TTL just expires and the
  user has to sign in again. `auth-service`'s `/refresh` endpoint exists;
  wiring it into the frontend's fetch layer is straightforward follow-up,
  not done yet.
- No actual `RTCPeerConnection`/video element — the channel page shows a
  placeholder where the video would render. `stream-service`'s signaling
  relay is real and tested (see Task 2); driving an actual WebRTC peer
  connection off it from the browser is separate frontend work.
- Channel/stream state is polled (5s interval) rather than pushed, unlike
  chat and the viewer count. Acceptable for now; a natural upgrade once
  `stream-service` gets its own state-change broadcast the way
  `fraud-decision` and `chat-service` already do.
- The live viewer count includes the creator's own dashboard view (it
  connects to the same signaling socket as any viewer to read the count)
  — cosmetically odd for a creator watching their own stream, not
  incorrect: it's an accurate count of open signaling connections, which
  is exactly what it's built to report.
