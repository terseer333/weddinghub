# WeddingHub handover

## Owner dashboard

The refreshed owner dashboard is served at `/pages/dashboard.html`. Its plain HTML,
CSS, and JavaScript live in `pages/dashboard.html`, `pages/dashboard.css`, and
`pages/dashboard.js`. The previous owner dashboard is preserved at
`/pages/dashboard.old.html` and continues using the original root scripts.

`GET /api/dashboard` requires `Authorization: Bearer <session_token>`. It selects the
first wedding linked to the authenticated user through `wedding_admins`; it does not
accept a wedding identifier from the browser. Session validation and owner lookup use
the existing auth and repository methods.

### Sidebar navigation check

The current sidebar routes dashboard sections to the matching anchors on
`/pages/dashboard.html`; the invitation preview opens `/pages/event.html?preview=admin`
and its return button goes back to the dashboard. Existing `rsvps.html` and
`create-event.html` aliases redirect to the new dashboard. Dashboard requests retain
the browser session token and redirect to `login.html` after a `401`.

Standalone owner pages for messages, events, committee, photos/story, announcements,
Card Studio, and Help do not exist yet. Their sidebar entries currently point to the
matching dashboard section as a temporary destination. The Help centre link remains
`#` until a real help page or support address exists.

The response is assembled from the existing wedding aggregate: invitations, RSVP
records, committee members and tasks, events, published announcements, photos, story
sections, guest messages, wedding details, and the owner's profile. Date/time activity
labels are formatted by Go.

## Current data limitations

- Invitation views and weekly views are `0`; no view tracking table exists.
- `unreadMessages` is `0`; guest messages have no read/unread state.
- `shareUrl` is empty. Existing guest links require an individual token, and only its
  one-way hash is stored, so a generic wedding invitation URL cannot be recovered.
- `households` counts guest invitations. `guests.total` sums each invitation's
  `max_party_size`, which is invitation capacity, since the schema has no household
  model and does not store every expected party member before RSVP.

## Local checks

The API module is `backend/`. Set `WEDDINGHUB_DATABASE_URL` to a local PostgreSQL
database before starting the app. The current dashboard endpoint returns `404` when a
valid owner session has no linked wedding, and `401` for missing, expired, or suspended
sessions.
# Short links and link previews

Guest invitations use 12-character base62 codes. PostgreSQL stores only the SHA-256 code hash in `invitations.short_code_hash`; migration `007_invitation_short_codes.sql` adds it. `GET /i/{code}` returns small Go-rendered Open Graph/Twitter HTML and directs browsers into the current invitation page with the short code. Preview lookups use a narrow PostgreSQL join, are rate-limited per client IP, and do not load guest details. No invitation-open timestamp or view counter exists, so crawler requests create no opens.

Wedding banners are 1200×630 PNGs rendered in Go by `backend/preview` using the embedded DM Serif Display font and `golang.org/x/image`. The photo is softened under a cream overlay. Images are stored in PostgreSQL (`wedding_banners`) when owners save wedding details or card design; `/og/{wedding_id}.png` sends cache headers and falls back to the static `/static/og/weddinghub-default.png` when details or a stored render are unavailable. Link preview URLs include a wedding-details version hash.

Guests & RSVP supports Copy link, WhatsApp sharing, and Web Share image sharing when supported. Card Studio displays the banner preview. Invitation SMTP email has a multipart HTML version with the banner and a “View your invitation” button; the existing plain text part remains available. A normal browser's first visit records `opened_at`; known link-preview bots do not. Migration `009_invitation_opened_at.sql` adds the timestamp.

Render's static frontend and Go service are separate. `render.yaml` sets `WEDDINGHUB_PUBLIC_BASE_URL` to the Go service URL, so shared links use the service that handles `/i` and `/og`. To use another public domain for short links, route those paths to the Go service. `render.yaml` does not specify a compute plan: Render Free web services spin down after 15 idle minutes and take about a minute to wake; paid plans do not spin down. See [Render Free web services](https://render.com/docs/free).
