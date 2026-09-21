# WeddingHub backend MVP

A Go standard-library HTTP API backed by PostgreSQL. Run from this directory:

```sh
go run ./cmd
```

The default address is `:8080`; override it with `WEDDINGHUB_ADDR`.

`WEDDINGHUB_DATABASE_URL` is required and must be a PostgreSQL connection string, for example `postgres://weddinghub:secret@localhost:5432/weddinghub?sslmode=disable`. The process pings the database and applies the embedded migrations at startup, and it exits with a fatal error when the database is missing or unreachable.

Development CORS permits browser origins on `localhost`, `127.0.0.1`, and other loopback IPs (with any port) by default. Set `WEDDINGHUB_ALLOWED_ORIGINS` to a comma-separated exact allowlist to replace that default, for example `https://app.example.test,http://localhost:5173`. The legacy single-value `WEDDINGHUB_ALLOWED_ORIGIN` is also supported when the plural setting is unset. CORS never uses a wildcard, and preflight requests are restricted to `GET`, `POST`, `PUT`, and `DELETE` with `Accept`, `Authorization`, and `Content-Type` request headers.

## API

All request and response bodies are JSON unless noted.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Health check |
| `POST` | `/api/auth/signup` | Create an account and return a session token once |
| `POST` | `/api/auth/login` | Verify email and password and return a session token |
| `POST` | `/api/auth/logout` | Invalidate the session token used for the request |
| `GET` | `/api/auth/me` | Read the account behind a session token |
| `POST` | `/api/weddings` | Create a wedding aggregate |
| `GET` | `/api/weddings` | List weddings |
| `GET` | `/api/weddings/{weddingID}` | Read a wedding aggregate |
| `PUT` | `/api/weddings/{weddingID}` | Replace editable wedding/admin/content fields; invitations and guest responses are preserved |
| `DELETE` | `/api/weddings/{weddingID}` | Delete a wedding |
| `POST` | `/api/weddings/{weddingID}/invitations` | Create an invitation and return its raw token once |
| `POST` | `/api/weddings/{weddingID}/invitations/{invitationID}/send` | Deliver an invitation over the configured email/WhatsApp channels |
| `GET` | `/api/invitations/{token}` | Read the invitation and guest-safe wedding projection |
| `POST` | `/api/invitations/{token}/accept` | Accept an invitation |
| `POST` | `/api/invitations/{token}/decline` | Decline an invitation |
| `GET` | `/api/guest/{token}/dashboard` | For accepted invitations, read invitation, RSVP, guest-facing wedding fields, and published content |
| `PUT` | `/api/guest/{token}/rsvp` | Create or update the accepted invitation's RSVP |
| `POST` | `/api/guest/{token}/messages` | For accepted invitations, create a wedding-scoped guest message using `message` or `body` |
| `GET` | `/api/weddings/{weddingID}/admin/overview` | Invitation and attendance totals |

Content statuses are `draft`, `published`, and `hidden`. RSVP statuses are `attending`, `not_attending`, and `maybe`.

### Guest response contracts

`GET /api/invitations/{token}` returns:

```json
{
  "wedding": {
    "id": "...",
    "slug": "...",
    "title": "...",
    "partner_one": "...",
    "partner_two": "...",
    "date": "...",
    "venue": "...",
    "address": "...",
    "city": "...",
    "state": "...",
    "country": "...",
    "message": "...",
    "verse": "...",
    "dress_code": "...",
    "hero_image": "...",
    "template_id": "...",
    "events": [],
    "photos": [],
    "story_sections": [],
    "announcements": []
  },
  "invitation": { "id": "...", "guest_name": "...", "status": "pending", "max_party_size": 2 }
}
```

Optional scalar fields use `omitempty`. The four content arrays are always present and contain only `published` items. The projection does not serialize the wedding status, administrators, guest collection, invitations collection, RSVPs, guest messages, timestamps, or invitation token hashes.

`GET /api/guest/{token}/dashboard` returns the same `wedding` and `invitation` objects plus an optional `rsvp`. Pending or declined invitations receive `403 Forbidden`.

`POST /api/guest/{token}/messages` accepts exactly one non-empty alias, either `{"message":"Best wishes"}` or `{"body":"Best wishes"}`. Messages are trimmed and limited to 2,000 Unicode characters. The response is `201 Created` with `{"id":"...","wedding_id":"...","invitation_id":"...","body":"...","created_at":"..."}`. Pending or declined invitations receive `403 Forbidden`; the repository also rechecks invitation state atomically before storing the message.

### Invitation delivery

`POST /api/weddings/{weddingID}/invitations/{invitationID}/send` delivers an existing invitation. It is admin-authenticated and takes:

```json
{ "token": "<raw invitation token>", "channels": ["email", "whatsapp"] }
```

Only the token hash is stored, so the admin supplies the raw token and the API verifies `SHA-256(token)` against the stored hash before sending anything. `channels` is optional and defaults to every known channel. The response reports one result per requested channel:

```json
{ "wedding_id": "...", "invitation_id": "...", "results": [ { "channel": "email", "to": "guest@example.com", "status": "sent" } ] }
```

`status` is `sent`, `skipped` (channel not configured, or no recipient on file), or `failed`. Delivery is opt-in: with no channel configured the endpoint returns `503`, and when `WEDDINGHUB_PUBLIC_BASE_URL` is unset it cannot build the personal link and also returns `503`. A blank or mismatched token returns `400`/`403` and nothing is sent.

Delivery configuration (every variable is optional; a channel without complete configuration stays inactive):

| Variable | Purpose |
|---|---|
| `WEDDINGHUB_PUBLIC_BASE_URL` | Public origin used to build `.../pages/event.html?token=...` links |
| `WEDDINGHUB_INVITATION_PATH` | Invitation path, default `/pages/event.html` |
| `WEDDINGHUB_SMTP_HOST` | Enables the email channel; requires a from address or username |
| `WEDDINGHUB_SMTP_PORT` | SMTP port, default `587` |
| `WEDDINGHUB_SMTP_USERNAME` / `WEDDINGHUB_SMTP_PASSWORD` | SMTP credentials (PLAIN auth when a username is set) |
| `WEDDINGHUB_EMAIL_FROM` | From address, defaulting to the SMTP username |
| `WEDDINGHUB_WHATSAPP_TOKEN` / `WEDDINGHUB_WHATSAPP_PHONE_NUMBER_ID` | Enables the WhatsApp Cloud API channel |
| `WEDDINGHUB_WHATSAPP_API_VERSION` | Graph API version, default `v21.0` |
| `WEDDINGHUB_WHATSAPP_API_BASE` | Graph API base URL, default `https://graph.facebook.com` |

Recipient email addresses are parsed with `net/mail` before use, which rejects the CR/LF characters a header-injection attempt would need, and phone numbers are reduced to digits for the Cloud API.

### Invitation token handling

Invitation creation uses `crypto/rand` to generate a 256-bit URL-safe opaque token. Only its SHA-256 hash is retained in the repository and the hash is excluded from JSON. The raw token is returned only in the invitation-creation response. Treat that response and guest URLs as secrets; avoid logging them or placing them in analytics/referrer data. TLS is required outside local development.

## Authentication

`POST /api/auth/signup` accepts `{"email":"...","password":"...","display_name":"..."}`. Addresses are normalized (trimmed, lowercased) and must contain one `@` and a dotted domain; passwords must be at least 8 characters; `display_name` is optional and derived from the address when omitted. The response is `201 Created`:

```json
{ "user": { "id": "...", "email": "ada@example.com", "display_name": "Ada Admin", "role": "owner", "created_at": "..." },
  "session_token": "...", "expires_at": "..." }
```

`POST /api/auth/login` takes `{"email":"...","password":"..."}` and returns the same shape with `200 OK`. A wrong password and an unknown address both return `401` with `{"error":"email or password is incorrect"}`; the unknown-address path still performs a password verification so response timing does not reveal which addresses have accounts. A duplicate signup returns `409`.

Sessions last 30 days. `GET /api/auth/me` requires `Authorization: Bearer <session_token>` and returns the account; `POST /api/auth/logout` invalidates that one session. Session tokens are generated with the same 256-bit `crypto/rand` scheme as invitations and only their SHA-256 hash is stored.

Passwords are hashed with PBKDF2-HMAC-SHA256 (210,000 iterations, 16-byte random salt, 32-byte derived key) implemented on the standard library, so the only external dependency is the PostgreSQL driver. The stored record is self-describing — `pbkdf2-sha256$<iterations>$<salt>$<key>` — so `models.PasswordIterations` can be raised later without invalidating existing credentials. Verification is constant time and a malformed record never verifies.

Note that a user session is not yet a wedding role: wedding-scoped endpoints still authorize with the wedding admin capability token (or an accepted committee invitation), and the two credential types are deliberately kept separate. Linking accounts to `wedding.admins` is the next step for real per-user authorization.

## Security and MVP limitations

This is an API foundation, **not complete production authentication or authorization**.

- Wedding endpoints authorize with the wedding admin capability token, and committee routes with an accepted committee invitation. Accounts and sessions exist (`/api/auth/*`), but a session is not yet bound to a wedding role: wedding CRUD, invitation creation, and admin overview still check the wedding token rather than a signed-in user. Bind accounts to `wedding.admins` before relying on login alone. Sessions are stored in PostgreSQL and survive restarts.
- Guest links are bearer capabilities: anyone holding a valid invitation token can read that invitation's public projection and accept or decline it. Dashboard, RSVP, and message access additionally require the invitation to be accepted. Add revocation/rotation and rate limiting in production.
- The API requires PostgreSQL and no longer runs on an in-memory repository. `repository.MemoryRepository` is kept only as a test double for the API and domain tests, and is never used by `cmd`.
- SHA-256 is appropriate for high-entropy random invitation tokens, but not passwords. Password authentication should use a maintained password-hashing implementation such as Argon2id or bcrypt; no password endpoint is provided here.
- Input body size, unknown JSON fields, server timeouts, cache behavior, and basic browser security headers are constrained, but validation, audit logging, abuse protection, observability, CSRF/origin policy, and TLS termination still need production design.
- Photos are URL metadata only. Use private object storage, signed upload/download flows, MIME inspection, size limits, and malware scanning rather than accepting files into the API process.

## PostgreSQL storage

The API requires PostgreSQL and fails fast without it; there is no in-memory fallback. `repository.NewPostgresRepository` opens the connection pool, verifies connectivity with a ping, and applies the embedded migrations in `repository/migrations` before serving traffic. Applied migrations are recorded in `schema_migrations`, so startup is idempotent.

The wedding aggregate is normalized into `users`, `sessions`, `weddings`, `wedding_admins`, `committee_roles`, `guests`, `committee_members`, `invitations`, `events`, `photos`, `story_sections`, `announcements`, `planning_tasks`, `committee_messages`, `rsvps`, and `guest_messages`. Every child table carries a `wedding_id` foreign key with `ON DELETE CASCADE`, so deleting a wedding removes its whole aggregate.

Storage guarantees:

1. Invitation and admin capability hashes live in uniquely indexed text columns and are compared by equality; raw tokens are never stored.
2. Invitation acceptance, RSVP upsert, guest/committee materialization, and guest messages run in transactions. State transitions lock the invitation row with `SELECT ... FOR UPDATE` before reading it, so concurrent acceptance or RSVP writes cannot interleave.
3. Unique constraints enforce one normalized email per user and one slug and one admin hash per wedding. PostgreSQL errors are mapped onto the repository's `ErrNotFound`/`ErrConflict`/`ErrInvalidStatus` sentinels.
4. Each repository call runs with its own context deadline, `timestamptz` is used throughout, and the pool is bounded.
5. `UpdateWedding` replaces the editable content collections and admin roster while preserving invitations, guests, committee state, planning tasks, RSVPs, guest messages, the admin token hash, and the creation time, matching the previous in-memory semantics.

`repository.MemoryRepository` is kept only as a test double for the API and domain tests.

### Testing against PostgreSQL

Unit tests are hermetic. The PostgreSQL repository tests run only when `WEDDINGHUB_TEST_DATABASE_URL` is set, and they truncate the aggregate tables before each test:

```sh
cd backend
WEDDINGHUB_TEST_DATABASE_URL="postgres://localhost:5432/weddinghub_test?sslmode=disable" go test ./repository/ -run Postgres -v
```

## Validation

```sh
go test ./...
go test -race ./...
go vet ./...
```

The PostgreSQL repository tests are skipped unless `WEDDINGHUB_TEST_DATABASE_URL` points at a disposable database (see above).
