# Docs

Notes on how this project fits together. Mostly empty for now — things get
written down here as they're decided, not before.

## Entrypoints

- `apps/web` — Next.js frontend. Starts at `src/app/page.tsx`; routes are
  file-based under `src/app`. `bun dev` from the repo root.
- `apps/api` — Go HTTP API. Starts at `main.go`, where routes are registered on
  the mux. `bun run dev:api` from the repo root, serves on `:8080`.

## Git repository storage

`apps/api/storage` is the repository lifecycle boundary. A store owns one root
directory and creates bare Git repositories beneath it using opaque UUID-shaped
IDs. `Create` atomically publishes a repository, `Open` validates and reopens it
by ID, and `Inspect` reports its identity and whether it is bare and empty.

Each repository uses `main` as its unborn default branch and has a standard
bare Git layout at `<store root>/<repository ID>`. `Repository.GitDir` exists
for later object, reference, and remote operations that must interoperate with
stock Git; directory creation itself remains the storage package's concern.
