# AGENT.md

This file defines practical rules for humans/AI agents working in this repository.

## 1) Project Overview

AssessV2 is an assessment-session-centric system.

- Frontend: Vue 3 + TypeScript + Element Plus
- Backend: Go + Gin + GORM + SQLite
- Desktop shell: Wails 2.x

Primary directories:

- `frontend/`: web UI
- `backend/`: API, business logic, migrations, desktop integration
- `data/`: runtime data files and SQLite DBs
- `docs/`: implementation-aligned documentation

## 2) Critical Domain Invariants (Do Not Break)

1. Each session directory's `assess.db` is the single source of truth for that session's business data and rules.
2. Session business data must be stored under `data/{assessment}/`, not year-based directories.
3. `data/accounts/` is system-level centralized data and may remain shared.
4. Organization tree can be a common source, but session object snapshots are independent after session creation.
5. No runtime auto-migration. Historical data migration must be done via offline commands.
6. Session state model is `preparing / active / completed`.
7. When session state is `completed`, session data/rules are read-only for all roles, including Root.

## 3) Data Layout

```text
data/
  accounts/
    accounts.db
  {assessment}/
    assess.db
    *.json
```

Notes:

- Legacy `business_data.json` / `default_objects.json` may exist but are not runtime truth.

## 4) Local Run Commands

Backend:

```bash
cd backend
go run ./cmd/server
```

- Default address: `127.0.0.1:8080`

Frontend:

```bash
cd frontend
npm install
npm run dev
```

- Default address: `http://127.0.0.1:5173`

Desktop (optional):

```bash
cd backend/desktop
wails dev
```

## 5) Offline Migration Commands

Run from `backend/`:

Session business table migration:

```bash
# dry-run
go run ./cmd/migrate-session-business-db --db ../data/assess.db --data-root ../data

# apply
go run ./cmd/migrate-session-business-db --db ../data/assess.db --data-root ../data --apply
```

Rule file path migration:

```bash
# dry-run
go run ./cmd/migrate-rule-file-paths --db ../data/assess.db --data-root ../data

# apply
go run ./cmd/migrate-rule-file-paths --db ../data/assess.db --data-root ../data --apply
```

Guideline:

- Always run dry-run first.
- If historical DB is elsewhere, adjust `--db` to the real path.

## 6) Script Engine Notes (expr-lang)

Custom script engine: `github.com/expr-lang/expr`.

- Module script (`customScript`) must return number.
- Grade extra condition script (`extraConditionScript`) must return bool.
- Prefer uppercase period codes (for example `Q1`).
- Module script save does not enforce strong validation; runtime failure becomes module score `0`.
- Extra condition script is bool-validated when enabled; runtime failures return business errors.

## 7) Code/Doc Alignment Checklist

Before changing behavior:

1. Verify route entry baseline: `backend/internal/api/router/router.go`
2. Verify migration baseline:
   - business: `backend/migrations/business/0001` to `0010`
   - accounts: `backend/migrations/accounts/1001` to `1002`
3. If behavior changed, update docs under `docs/` to keep code and docs consistent.

## 8) Change Guardrails For Agents

1. Do not introduce runtime auto-migration behavior.
2. Do not move session truth away from per-session `assess.db`.
3. Do not allow business writes when session is `completed`.
4. Prefer minimal, targeted edits; preserve existing architecture decisions unless explicitly requested.
