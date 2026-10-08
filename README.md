# WeddingHub

WeddingHub is a connected digital wedding experience: couples manage one central wedding record, send personalized invitations, and unlock a private, mobile-first dashboard for accepted guests.

## Run the connected application

Create a PostgreSQL database and point the API at it, then start it in one terminal. The API reads
its configuration from the environment, so put the connection string in a `.env` file copied from
the tracked template:

```sh
createdb weddinghub   # or use any PostgreSQL database you already run
cp .env.example .env  # then set WEDDINGHUB_DATABASE_URL in .env
cd backend
go run ./cmd
```

`.env` is optional and never overrides a real environment variable, so a host such as Render can be
configured with the same names (`WEDDINGHUB_DATABASE_URL`, and `PORT` is used automatically).

The API pings the database and applies its embedded schema migrations at startup. It exits with a fatal error when `WEDDINGHUB_DATABASE_URL` is missing or the database is unreachable; there is no in-memory fallback.

The Go server also serves the frontend. At startup it registers a Go handler for every HTML page
in the configured frontend directory, using root extensionless URLs (for example, `/login` and
`/dashboard`). Requests to legacy `.html` or `/pages/...` URLs redirect to the clean URL; `/` and
directory index pages have clean routes too.
When started from `backend/`, it detects the repository's frontend files automatically, so open
`http://localhost:8080` to use the pages and API on one origin. To work on the frontend through a
separate static development server instead, use:

```sh
cd frontend
python3 -m http.server 5500
```

Then open `http://localhost:5500`. The Admin Dashboard shows **API connected** when the Go service is available. On its first connection it creates the wedding aggregate and cryptographically secure invitation tokens in PostgreSQL. The API is mandatory: when it is unavailable, the UI blocks with an explicit error instead of falling back to a local browser workspace.

### Render deployment

The Render Blueprint builds the frontend into `dist` and serves it from the Go `weddinghub` web service. Page requests such as `/`, `/login`, `/dashboard`, `/reset-password`, and `/api/...` therefore share one origin; the browser uses that origin for API calls, so no frontend API URL or cross-origin allowlist is needed. If you attach a custom domain, point it at this web service.

Recommended product journey:

1. Open `frontend/pages/dashboard.html` to use the Wedding Admin workspace.
2. Edit **Wedding information** (for example, change the venue) and save.
3. Create a published **Announcement** or add a guest.
4. Select another design from the generated 108-template library.
5. Use **Preview as guest** to open the accepted-guest experience.
6. Confirm that wedding details and published announcements use the same updated API record.
7. To test invitation acceptance, open **Guests & RSVP**, copy a pending guest's generated invitation link, accept it, and enter the newly unlocked guest dashboard.

On a browser that has never completed the guided tour, the admin dashboard opens with a step-by-step walkthrough of every feature: each step highlights a sidebar section, switches to it, and explains what it does. It can be skipped at any point and replayed from **Settings → Getting started**.

`frontend/api-client.js` maps the browser view model to the Go API, which is the only source of truth. The browser keeps just the current working copy used to render a page; it is refreshed from the API and is never used as an offline data store.

### Separate static frontend hosting

A separately hosted frontend can still be pointed at a deployed API. Open **Dashboard → Settings → API connection** and enter the deployed HTTPS API URL, or define `window.WEDDINGHUB_API_URL` before loading `frontend/api-client.js`. A static host with no reachable API shows the blocking API-unavailable error. Do not point an HTTPS website at an HTTP API; browsers block that as mixed content.

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

## Password recovery email

The login page links to a password recovery form. Reset links are random, single-use, expire after 30 minutes, and are stored as SHA-256 hashes. Password recovery uses Brevo's HTTP API; it does not replace the existing SMTP invitation sender. Add these Render environment values to enable recovery mail:

- `BREVO_API_KEY`: Brevo API key.
- `EMAIL_FROM`: sender address verified in Brevo.
- `EMAIL_FROM_NAME`: optional sender display name (defaults to `WeddingHub`).
- `APP_BASE_URL`: public app origin, for example `https://your-app.onrender.com`.

For Render setup: create a Brevo account, verify a sender under **Senders, Domains & Dedicated IPs → Senders**, then create an API key under **SMTP & API → API Keys**. In Render, open the service’s **Environment** settings, add the values above, and redeploy. The service applies the password-reset-token migration automatically at startup. A 401 from Brevo usually means the API key is wrong; a 400 often means the sender is not verified. If an email does not arrive, check spam and Brevo’s email logs. Brevo’s free tier has a daily sending limit (about 300 emails/day) and may add a footer. Without the settings, the API logs a startup warning and still returns the same generic recovery response without sending mail.

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
