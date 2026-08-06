# AGENTS.md

Guidance for coding agents working in this repository.

## What this is

A Bun workspace monorepo with two apps:

```
apps/web    Next.js frontend (TypeScript, React 19, Tailwind v4)
apps/api    Go HTTP API
docs/       notes on how the pieces fit together
```

`apps/web` has its own `AGENTS.md` with a rule that matters: this is a newer
Next.js than your training data, so read `node_modules/next/dist/docs/` before
writing frontend code rather than reaching for remembered APIs.

## Commands

Run these from the repo root. The runtime is **Bun**, not Node — use `bun`, not
`npm`/`yarn`.

```sh
bun install       # install workspace deps
bun dev           # web  → http://localhost:3000
bun run dev:api   # api  → http://localhost:8080/health
bun run build     # next build
bun run lint      # eslint over apps/web
```

There is no root typecheck script. Typecheck the frontend the way CI does:

```sh
cd apps/web && bunx tsc --noEmit
cd apps/api && go vet ./... && go build ./...
```

## What CI checks

Two workflows gate a pull request, each scoped by path — they only run when
that app changed:

- **web** (`apps/web/**`, `package.json`, `bun.lock`) — `bun install
  --frozen-lockfile`, `bun run lint`, `bunx tsc --noEmit`.
- **api** (`apps/api/**`) — `go vet ./...` and `go build ./...` in `apps/api`.

Run the matching commands locally before pushing; the four above are the whole
gate. `bun install --frozen-lockfile` is what CI uses, so commit `bun.lock`
whenever dependencies change or the web job fails before it starts.

## Conventions

- **Frontend** — App Router, file-based routes under `apps/web/src/app`. Entry
  point is `src/app/page.tsx`, shared shell is `layout.tsx`, global styles are
  `globals.css`. Tailwind v4 is wired through PostCSS
  (`postcss.config.mjs`); there is no `tailwind.config` file.
- **API** — `main.go` registers handlers on a `net/http` mux with Go 1.22+
  method-and-path patterns (`"GET /health"`). The `storage` package owns bare
  Git repository lifecycles; use its `RepositoryStore` boundary rather than
  constructing repository directories elsewhere. `RepositoryStorage` is the
  complete application-facing contract for an open repository. Handles implement
  `ObjectStore` for immutable blob, tree, commit, and tag storage, including
  deterministic enumeration of verified loose objects; do not write loose
  object files outside that boundary. `GraphStore` parses tree entries and
  commit snapshot/parent links so callers can recursively traverse repository
  contents and ancestry without parsing raw object bytes. Repository handles
  also implement
  `ReferenceStore` for direct and symbolic references, including packed direct
  refs, `HEAD`, and the default branch; do not write reference files outside
  that boundary. Smart HTTP is served at `/repositories/{ID}` by invoking stock
  `git upload-pack` and `git receive-pack` against `Repository.GitDir`; stock clients
  can discover, clone, fetch, and pull empty or populated repositories,
  including checkout and fast-forward updates of the configured default branch.
  Pushes may create or fast-forward only that primary branch; deletion,
  non-fast-forward updates, and updates to other refs are rejected in Git's
  receive quarantine so their objects and references are not published.
  The API runtime therefore requires `git` on
  `PATH`. Repository data is rooted at
  `$REPOSITORY_ROOT`, defaulting to `apps/api/repositories` when started via the
  documented root command.
  The package has no third-party
  dependencies and no `go.sum`; adding a dependency means the api workflow's
  `cache: false` line should flip to `cache-dependency-path: apps/api/go.sum`.
  The port comes from `$PORT`, defaulting to `8080`.
- **Docs** — `docs/README.md` records decisions once they're made, not before.
  Update it when you change how the apps fit together, not for every change.

## LADDER.md

`/LADDER.md` at the repo root is **not part of this repository**. It is placed
there from outside the checkout — as a symlink or a read-only bind mount — and
is gitignored for that reason. Read it for context on what is being built, but
never edit it, `git add` it, or delete it, and do not treat its presence as
uncommitted work.
