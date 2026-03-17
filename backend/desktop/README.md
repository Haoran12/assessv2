# AssessV2 Desktop (Wails)

This folder contains the Wails desktop shell for AssessV2.

## Commands

- Development shell (hot-reload from `../../frontend/dist` watcher):

```bash
wails dev
```

- Release build:

```bash
wails build -clean -s
```

The desktop runtime reads:

- Frontend assets from `build/bin/frontend/dist` (release artifact layout), or `../../frontend/dist` during local development.
- Split SQL migrations from `build/bin/migrations/business` and `build/bin/migrations/accounts` (release artifact layout), or `../migrations/business` + `../migrations/accounts` during local development.
