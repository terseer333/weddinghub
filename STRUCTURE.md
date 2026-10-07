# Project structure

- `backend/api/`: Go HTTP routes, authentication, and JSON handlers.
- `backend/models/`: wedding, invitation, guest, committee, and profile models.
- `backend/repository/`: repository interface, PostgreSQL implementation, and embedded SQL migrations.
- `pages/`: static HTML pages served at `/pages/`.
- `assets/`: local image and icon assets used by the static pages.

## Owner dashboard files

- `pages/dashboard.html`: replacement wedding-owner dashboard design.
- `pages/dashboard.css`: standalone dashboard styles.
- `pages/dashboard.js`: dashboard fetch, display, theme, mobile navigation, and empty-state behavior.
- `pages/dashboard.old.html`: backup of the prior owner dashboard.
- `backend/api/dashboard.go`: authenticated `GET /api/dashboard` response construction.
- Dashboard sidebar targets existing dashboard anchors or the guest-facing invitation preview until the standalone owner pages are built.

The dashboard JSON contains `user`, `couple`, `unreadMessages`, `guests`, `views`,
`setup`, `committee`, `activity`, `events`, and `announcements`. The optional additions
`shareUrl` and `storyImage` support the invitation button and couple photo. See
`HANDOVER.md` for field derivation and unavailable-table placeholders.
# Short links and share banners

- `backend/preview/`: Go PNG generation at 1200×630; embeds the DM Serif Display font and includes its OFL license.
- `backend/preview/fonts/`: embedded font and license.
- `backend/preview/default.png`: embedded fallback banner; `static/og/weddinghub-default.png` is copied to the static frontend build.
- `backend/repository/migrations/007_invitation_short_codes.sql`: hash-only short code storage.
- `backend/repository/migrations/008_wedding_banners.sql`: PostgreSQL banner byte storage.
- `backend/repository/migrations/009_invitation_opened_at.sql`: first normal-browser open timestamp.
- Go API routes: `/i/{code}`, `/og/{wedding_id}.png`, and `/static/og/weddinghub-default.png`.
- `pages/owner-page.js`: guests share controls and Card Studio preview.

The short-link HTML is rendered on the Go service. The static Render site sets its API URL to that service, and `WEDDINGHUB_PUBLIC_BASE_URL` makes shared links use the Go host.
