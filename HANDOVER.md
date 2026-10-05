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
