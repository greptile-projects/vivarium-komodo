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
  including checkout and updates of the configured default branch. Repositories
  may contain multiple named branches; pushes may create, fast-forward,
  explicitly force-update, or delete any branch. Updates outside `refs/heads/*`
  are rejected in Git's receive quarantine. Git's receive protocol has no force
  flag: the stock client enforces the ordinary non-fast-forward guard and sends
  the update only when the caller explicitly requests force.
  The API runtime therefore requires `git` on
  `PATH`. Repository data is rooted at
  `$REPOSITORY_ROOT`, defaulting to `apps/api/repositories` when started via the
  documented root command.
  `git_compatibility_test.go` is the black-box compatibility suite for complete
  stock-client default-branch and named-branch workflows; after provisioning
  an empty repository, it observes and changes remote state only through Git
  over HTTP.
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
  creating or deleting storage repositories directly. Repositories are private
  by default and may be made public. Public reads are anonymous across JSON and
  Git; private reads, every write, visibility changes, and deletion are limited
  to the owner unless the owner grants a user the repository's contributor
  role. Contributors can read private repository metadata and Git data and may
  publish non-default candidate branches, but cannot update the default branch
  or exercise metadata, visibility, deletion, or access-management powers.
  Authenticated non-participants receive `404` for denied repository access.
  Route Git through the catalog as well as storage so transport access cannot
  bypass ownership, collaborator, and visibility policy.
  Public repository resources also carry an owner-scoped normalized name,
  description, and create/update timestamps; their immutable ID remains the
  API and Git transport identity. Repository and access-grant collections use
  the shared `items`/`page`/`per_page`/`total_count` pagination envelope (30 by
  default, 100 maximum). Public handles resolve through
  `/users/by-handle/{handle}` while canonical user resources remain ID-based.
  Repository proposals and their append-only comments are owned by the
  `proposals.Store` boundary beneath `$PROPOSAL_ROOT` (default
  `apps/api/data/proposals`). Public proposal reads are anonymous; private
  reads and all participation follow repository visibility and membership.
  Any authenticated repository participant with `repository:write` may create
  or discuss a proposal, while
  only its author or the repository owner may edit or close it. Stable user IDs
  preserve authorship and closing attribution as profiles change.
  Pull requests are durable records beneath `$PULL_REQUEST_ROOT` (default
  `apps/api/data/pull-requests`) owned by the `pullrequests.Store` boundary.
  Creation resolves existing source and target branches through storage and
  snapshots both commit IDs; later branch movement must not rewrite that
  represented state. The API derives source-only commits and recursive file
  changes from those snapshots through `GraphStore`; text changes include a
  readable patch while binary changes retain object and mode metadata. Pull
  request discussion is append-only and attributable by stable user ID. Pull
  requests may link a repository proposal, use stable author IDs, and begin in
  the `open` lifecycle status. Apply the same
  repository visibility and participant policy to pull request reads and
  creation rather than reading their files directly.
- **Docs** — `docs/README.md` records decisions once they're made, not before.
  Update it when you change how the apps fit together, not for every change.

## LADDER.md

`/LADDER.md` at the repo root is **not part of this repository**. It is placed
there from outside the checkout — as a symlink or a read-only bind mount — and
is gitignored for that reason. Read it for context on what is being built, but
never edit it, `git add` it, or delete it, and do not treat its presence as
uncommitted work.
