# Docs

Notes on how this project fits together. Mostly empty for now — things get
written down here as they're decided, not before.

## Entrypoints

- `apps/web` — Next.js frontend. Starts at `src/app/page.tsx`; routes are
  file-based under `src/app`. `bun dev` from the repo root.
- `apps/api` — Go HTTP API. Starts at `main.go`, where routes are registered on
  the mux. `bun run dev:api` from the repo root, serves on `:8080`.

## Repository storage

`apps/api/storage` is the repository boundary shared by later application and
Git remote work. A `Store` creates bare repositories under one filesystem root,
assigns each an opaque random ID, and reopens them by that ID. `Inspect`
validates the repository before exposing its stable ID, path, bare status, and
default branch.

Repositories use the standard bare Git layout and an unborn `main` branch, so
stock Git can inspect them immediately. Names, ownership, and HTTP concerns sit
above this package; object and reference operations will extend it in later
storage milestones.
