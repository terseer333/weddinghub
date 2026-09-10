# WeddingHub backend MVP

A Go standard-library HTTP API backed by a thread-safe, in-memory Wedding aggregate repository. Run from this directory:

```sh
go run ./cmd
```

The default address is `:8080`; override it with `WEDDINGHUB_ADDR`.

Development CORS permits browser origins on `localhost`, `127.0.0.1`, and other loopback IPs (with any port) by default. Set `WEDDINGHUB_ALLOWED_ORIGINS` to a comma-separated exact allowlist to replace that default, for example `https://app.example.test,http://localhost:5173`. The legacy single-value `WEDDINGHUB_ALLOWED_ORIGIN` is also supported when the plural setting is unset. CORS never uses a wildcard, and preflight requests are restricted to `GET`, `POST`, `PUT`, and `DELETE` with `Accept`, `Authorization`, and `Content-Type` request headers.

## API

All request and response bodies are JSON unless noted.

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/healthz` | Health check |
| `POST` | `/api/weddings` | Create a wedding aggregate |
| `GET` | `/api/weddings` | List weddings |
| `GET` | `/api/weddings/{weddingID}` | Read a wedding aggregate |
| `PUT` | `/api/weddings/{weddingID}` | Replace editable wedding/admin/content fields; invitations and guest responses are preserved |
| `DELETE` | `/api/weddings/{weddingID}` | Delete a wedding |
| `POST` | `/api/weddings/{weddingID}/invitations` | Create an invitation and return its raw token once |
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

### Invitation token handling

Invitation creation uses `crypto/rand` to generate a 256-bit URL-safe opaque token. Only its SHA-256 hash is retained in the repository and the hash is excluded from JSON. The raw token is returned only in the invitation-creation response. Treat that response and guest URLs as secrets; avoid logging them or placing them in analytics/referrer data. TLS is required outside local development.

## Security and MVP limitations

This is an API foundation, **not complete production authentication or authorization**.

- Admin endpoints currently have an authentication/authorization placeholder: they are unauthenticated and must not be exposed publicly. Put real authenticated user identity and wedding-scoped role checks in front of wedding CRUD, invitation creation, and admin overview before deployment.
- Guest links are bearer capabilities: anyone holding a valid invitation token can read that invitation's public projection and accept or decline it. Dashboard, RSVP, and message access additionally require the invitation to be accepted. Add revocation/rotation and rate limiting in production.
- Data is process-local, is lost on restart, and cannot support multiple API instances.
- SHA-256 is appropriate for high-entropy random invitation tokens, but not passwords. Password authentication should use a maintained password-hashing implementation such as Argon2id or bcrypt; no password endpoint is provided here.
- Input body size, unknown JSON fields, server timeouts, cache behavior, and basic browser security headers are constrained, but validation, audit logging, abuse protection, observability, CSRF/origin policy, and TLS termination still need production design.
- Photos are URL metadata only. Use private object storage, signed upload/download flows, MIME inspection, size limits, and malware scanning rather than accepting files into the API process.

## Recommended PostgreSQL migration

Keep `repository.Repository` as the application boundary and add a PostgreSQL implementation. Normalize the aggregate into `users`, `weddings`, `wedding_admins`, `guests`, `invitations`, `events`, `photos`, `story_sections`, `announcements`, `rsvps`, and `guest_messages` tables.

Recommended constraints and transaction behavior:

1. Use UUID/opaque primary keys, foreign keys with deliberate delete behavior, UTC `timestamptz`, and database checks/enums for statuses and party sizes.
2. Store invitation hashes in a unique indexed `bytea` column (decode the current hexadecimal representation when migrating); never store raw tokens. Include expiry and optional revocation timestamps.
3. Put invitation response, guest creation, and RSVP upsert operations in transactions. Lock the invitation row (`SELECT ... FOR UPDATE`) for state transitions.
4. Enforce unique normalized user email and wedding slug constraints. Store only a password hash produced by a dedicated password-hashing library.
5. Add migration tooling, connection pooling, context deadlines, backups, encryption/key management, audit events, and tests against a real PostgreSQL instance.
6. Return purpose-built admin and public projections rather than loading every child row for all requests as data volume grows.

## Validation

```sh
go test ./...
go test -race ./...
go vet ./...
```
