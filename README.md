# WeddingHub

WeddingHub is a connected digital wedding experience: couples manage one central wedding record, send personalized invitations, and unlock a private, mobile-first dashboard for accepted guests.

## Run the connected application

Start the API in one terminal:

```sh
cd backend
go run ./cmd
```

Serve the frontend from a loopback origin in another terminal:

```sh
python3 -m http.server 5500
```

Then open `http://localhost:5500`. The Admin Dashboard shows **API connected** when the Go service is available. On its first connection it bootstraps the demo wedding and creates real cryptographically secure API invitation tokens. If the API is unavailable, the UI explicitly falls back to **Local demo** mode and continues using `localStorage`.

Recommended demo journey:

1. Open `pages/dashboard.html` to use the Wedding Admin workspace.
2. Edit **Wedding information** (for example, change the venue) and save.
3. Create a published **Announcement** or add a guest.
4. Select another design from the generated 108-template library.
5. Use **Preview as guest** to open the accepted-guest experience.
6. Confirm that wedding details and published announcements use the same updated API record.
7. To test invitation acceptance, open **Guests & RSVP**, copy a pending guest's generated invitation link, accept it, and enter the newly unlocked guest dashboard.

`api-client.js` maps the browser view model to the Go API and keeps a local cache for resilient demo behavior. Browser storage is not treated as a production database or security boundary; the API remains authoritative whenever it is connected.

## Backend API

The Go backend is a standard-library API foundation with:

- Centralized wedding aggregates and wedding-scoped records
- Wedding admin, guest, invitation, event, photo, love story, announcement, RSVP, and message models
- Cryptographically random invitation tokens with only SHA-256 hashes retained
- Published-content projection for guest dashboards
- Thread-safe repository abstraction and in-memory implementation
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

See [`backend/README.md`](backend/README.md) for endpoints, security boundaries, and the PostgreSQL migration path.

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

Before deployment, replace the in-memory repository with PostgreSQL and make the API mandatory instead of retaining the local demo fallback. Then add real sessions, password hashing, wedding-scoped role authorization, private object storage, rate limiting, audit logs, email/WhatsApp delivery, and token revocation/expiry. The backend intentionally does not claim production authentication is complete.