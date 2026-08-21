# Task 1 — auth-service

`services/auth-service` — Go, `net/http` (stdlib router), PostgreSQL via
`pgx`, JWT via `golang-jwt/jwt`, bcrypt password hashing, Google OAuth via
`golang.org/x/oauth2`.

## Endpoints

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

## Design notes

- Access tokens are short-lived (15min) stateless JWTs; refresh tokens are
  random opaque strings, stored **hashed** (SHA-256) in Postgres so a DB
  read can't be replayed as a live token, and **rotated** on every use —
  the token in a `/refresh` response is single-use.
- JWT claims carry `sub`, `email`, `display_name`, and `role` — `display_name`
  was added in Task 3 so downstream services (chat) have a name to show
  without a round trip back to auth-service.
- OAuth identities live in their own `oauth_identities` table
  (`provider`, `provider_user_id`) so a user can later link multiple
  providers to one account, and `GetOrCreateUserByOAuth` runs as a single
  transaction to avoid a race creating two user rows for the same login.
- Passwordless accounts (pure OAuth signup) are supported —
  `password_hash` is nullable.

## Verification

Signup (both roles), login, JWT-protected `/me`, refresh-token rotation
with single-use enforcement, duplicate-email conflict (409), wrong-password
rejection (401), and `RequireRole` blocking a `viewer` token from the
`creator`-only `/creator/ping` route (403) — see the Task 1 commit.
