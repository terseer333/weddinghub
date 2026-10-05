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
