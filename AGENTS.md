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
  including checkout and updates of the configured default branch. Pushes may
  create, fast-forward, explicitly force-update, or delete only that primary
  branch. Updates to other refs are rejected in Git's receive quarantine so
  their objects and references are not published. Git's receive protocol has
  no force flag: the stock client enforces the ordinary non-fast-forward guard
  and sends the update only when the caller explicitly requests force.
  The API runtime therefore requires `git` on
  `PATH`. Repository data is rooted at
  `$REPOSITORY_ROOT`, defaulting to `apps/api/repositories` when started via the
  documented root command.
  `git_compatibility_test.go` is the black-box compatibility suite for the
  complete stock-client single-branch workflow; after provisioning its empty
  repository, it observes and changes remote state only through Git over HTTP.
  Passwords and access grants are owned by the `auth.Store` boundary beneath
  `$AUTH_ROOT` (default `apps/api/data/auth`). Passwords are bcrypt hashes and
  issued bearer secrets are persisted only as SHA-256 digests. Browser sessions,
  API tokens, and Git credentials have separate scope and lifetime policies;
  authenticate HTTP requests through that boundary rather than reading its files.
  The port comes from `$PORT`, defaulting to `8080`.
  Human identities are durable JSON resources beneath `$USER_ROOT`, defaulting
  to `apps/api/data/users` when started via the documented root command. Use the
  `users.Store` boundary for account creation, inspection, and profile updates;
  user IDs are immutable while unique handles and display names are mutable.
  Account creation establishes the user's password; profile mutation requires
  authenticated `profile:write` access belonging to that same user.
  Owned repository resources are managed through the `repositories.Store`
  catalog beneath `$REPOSITORY_CATALOG_ROOT` (default
  `apps/api/data/repositories`). Their ID is the storage ID and therefore also
  identifies the Git remote at `/repositories/{ID}`. Repository API reads and
  writes require `repository:read` and `repository:write`, respectively; use
  the catalog boundary for ownership-aware lifecycle operations rather than
  creating or deleting storage repositories directly.
- **Docs** — `docs/README.md` records decisions once they're made, not before.
  Update it when you change how the apps fit together, not for every change.

## LADDER.md

`/LADDER.md` at the repo root is **not part of this repository**. It is placed
there from outside the checkout — as a symlink or a read-only bind mount — and
is gitignored for that reason. Read it for context on what is being built, but
never edit it, `git add` it, or delete it, and do not treat its presence as
uncommitted work.
