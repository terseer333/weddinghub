# Project structure

```text
backend/                 Go HTTP API, domain models, PostgreSQL repository, migrations
frontend/                Static browser application served by the Go process
  assets/                Images, icons, and template artwork
  pages/                 HTML views and page-specific scripts/styles (clean root URLs)
  reset-password/        Password reset page
  templates/             Built-in template catalog and synchronization tool
scripts/                 Build and maintenance commands
docs/                    Architecture notes and project handover material
render.yaml              Render service definition
README.md                Setup, deployment, and product overview
```

The source frontend stays under `frontend/`; `scripts/build_frontend.py` copies its public files
to `dist/` for deployment. The Go server auto-detects `frontend/` during local runs and serves
`dist/` on Render. The HTML files retain their existing relative asset layout, while the backend
maps them to clean root URLs such as `/dashboard` and `/login`.

## Owner dashboard

- `frontend/pages/dashboard.html`: wedding-owner workspace.
- `frontend/pages/dashboard.css`: dashboard styles.
- `frontend/pages/dashboard.js`: dashboard fetch, display, theme, mobile navigation, and empty-state behavior.
- `backend/api/dashboard.go`: authenticated `GET /api/dashboard` response construction.

The dashboard JSON contains `user`, `couple`, `unreadMessages`, `guests`, `views`, `setup`,
`committee`, `activity`, `events`, and `announcements`. See `HANDOVER.md` for field derivation and
unavailable-table placeholders.
