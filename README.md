# WeddingHub

WeddingHub is a connected digital wedding experience: couples manage one central wedding record, send personalized invitations, and unlock a private, mobile-first dashboard for accepted guests.

## Run the connected application

Create a PostgreSQL database and point the API at it, then start it in one terminal:

```sh
createdb weddinghub   # or use any PostgreSQL database you already run
cd backend
export WEDDINGHUB_DATABASE_URL="postgres://localhost:5432/weddinghub?sslmode=disable"
go run ./cmd
```

The API pings the database and applies its embedded schema migrations at startup. It exits with a fatal error when `WEDDINGHUB_DATABASE_URL` is missing or the database is unreachable; there is no in-memory fallback.

Serve the frontend from a loopback origin in another terminal:

```sh
python3 -m http.server 5500
```

Then open `http://localhost:5500`. The Admin Dashboard shows **API connected** when the Go service is available. On its first connection it creates the wedding aggregate and cryptographically secure invitation tokens in PostgreSQL. The API is mandatory: when it is unavailable, the UI blocks with an explicit error instead of falling back to a local browser workspace.

Recommended product journey:

1. Open `pages/dashboard.html` to use the Wedding Admin workspace.
2. Edit **Wedding information** (for example, change the venue) and save.
3. Create a published **Announcement** or add a guest.
4. Select another design from the generated 108-template library.
5. Use **Preview as guest** to open the accepted-guest experience.
6. Confirm that wedding details and published announcements use the same updated API record.
7. To test invitation acceptance, open **Guests & RSVP**, copy a pending guest's generated invitation link, accept it, and enter the newly unlocked guest dashboard.

On a browser that has never completed the guided tour, the admin dashboard opens with a step-by-step walkthrough of every feature: each step highlights a sidebar section, switches to it, and explains what it does. It can be skipped at any point and replayed from **Settings → Getting started**.

`api-client.js` maps the browser view model to the Go API, which is the only source of truth. The browser keeps just the current working copy used to render a page; it is refreshed from the API and is never used as an offline data store.

### Hosted frontend

A static host must be pointed at a deployed API before it can load a wedding. Open **Dashboard → Settings → API connection** and enter the deployed HTTPS API URL, or define `window.WEDDINGHUB_API_URL` before loading `api-client.js`. A static host with no reachable API shows the blocking API-unavailable error. Do not point an HTTPS website at an HTTP API; browsers block that as mixed content.

## Backend API

The Go backend is a standard-library API foundation with:

- Centralized wedding aggregates and wedding-scoped records
- Wedding admin, guest, invitation, event, photo, love story, announcement, RSVP, and message models
- Cryptographically random invitation tokens with only SHA-256 hashes retained
- Published-content projection for guest dashboards
- Thread-safe repository abstraction with the PostgreSQL implementation used in production and an in-memory double used only by tests
- Strict JSON handling, server timeouts, and focused API/domain tests

Run it with:

```sh
cd backend
go run ./cmd
```

Validate it with:

```sh
cd backend
go test ./...
go vet ./...
```

See [`backend/README.md`](backend/README.md) for endpoints, the database schema and migrations, security boundaries, and deployment notes.

## Architecture boundary

```text
Wedding Admin ──writes──▶ Wedding aggregate / repository
                               │
                  ┌────────────┼────────────┐
                  ▼            ▼            ▼
             Invitation   Published view   Admin overview
                  │            │
          secure token      accepted guest
                  └──────▶ private dashboard
```

`wedding_id` scopes all wedding content. Invitation acceptance associates one guest with one wedding; guest projections include only published content and that guest's own RSVP/profile data.

## Production follow-up

Storage is PostgreSQL-backed and the API is mandatory. Before deployment, add real per-user sessions bound to wedding roles, a maintained password-hashing implementation, private object storage, rate limiting, audit logs, email/WhatsApp delivery, and token revocation/expiry. The backend intentionally does not claim production authentication is complete.