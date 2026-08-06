# Docs

Notes on how this project fits together. Mostly empty for now — things get
written down here as they're decided, not before.

## Entrypoints

- `apps/web` — Next.js frontend. Starts at `src/app/page.tsx`; routes are
  file-based under `src/app`. `bun dev` from the repo root.
- `apps/api` — Go HTTP API. Starts at `main.go`, where routes are registered on
  the mux. `bun run dev:api` from the repo root, serves on `:8080`. Persistent
  repositories live beneath `$REPOSITORY_ROOT`, or `apps/api/repositories` by
  default.

## Git repository storage

`apps/api/storage` is the repository lifecycle boundary. A store owns one root
directory and creates bare Git repositories beneath it using opaque UUID-shaped
IDs. `Create` atomically publishes a repository, `Open` validates and reopens it
by ID, and `Inspect` reports its identity and whether it is bare and empty.

Each repository uses `main` as its unborn default branch and has a standard
bare Git layout at `<store root>/<repository ID>`. `Repository.GitDir` exposes
that location only for integration with stock Git plumbing, such as the future
remote transport; application code performs storage operations through the
interfaces below, and directory creation remains the storage package's concern.

`RepositoryStorage` is the complete read/write contract for an open repository.
It combines the immutable `ObjectStore`, read-only `GraphStore`, mutable
`ReferenceStore`, stable repository identity, and high-level inspection. The
only persistent writes are content-addressed object creation and reference
changes: Git objects are immutable and therefore have no update or delete
operation. `RepositoryStore` owns creation and reopening of repository handles.
The contract covers every platform storage operation:

- lifecycle — `RepositoryStore.Create` and `Open`, then `ID` and `Inspect`;
- objects — `WriteObject`, `ReadObject`, and `ListObjects`;
- graph views — `ReadTree` and `ReadCommit`;
- references — `CreateReference`, `ReadReference`, `UpdateReference`,
  `ListReferences`, and `DeleteReference`;
- default branch — `DefaultBranch` and `SetDefaultBranch`, backed by symbolic
  `HEAD`.

Repository handles also implement `ObjectStore`. `WriteObject` accepts a blob,
tree, commit, or annotated tag's exact content, derives its SHA-1 object ID from
Git's canonical header and content, and atomically stores the zlib-compressed
loose object. `ReadObject` returns the exact type and content after verifying
the canonical size and requested identity. These files are ordinary Git loose
objects, so stock commands such as `git cat-file` can consume them directly.
`ListObjects` discovers all loose objects, including unreachable objects and
objects written by stock Git, and returns them in object-ID order with their
verified identity, type, byte size, and exact content. Its results match
`git cat-file --batch-all-objects` for repositories managed through this
boundary.

Repository handles also implement `ReferenceStore`. Callers can atomically
create and update direct references to verified objects, create symbolic
references, read and deterministically list references (including symbolic
`HEAD`), and delete references. `DefaultBranch` and `SetDefaultBranch` expose
the branch selected by `HEAD`, including an unborn branch. Reference files use
Git's ordinary formats, so changes are immediately visible to stock Git;
direct references packed by Git remain readable, listable, updatable, and
deletable through the same interface.

The same handles implement `GraphStore` for typed traversal of the object
graph. `ReadTree` exposes each directory entry's name, octal Git mode, implied
object type, and object ID in stored order; callers recurse into entries that
name trees. `ReadCommit` exposes the root tree and ordered immediate parents,
while retaining the exact commit content for attribution, messages, and
additional headers. Following those IDs reconstructs both snapshots and merge
ancestry, and the underlying canonical objects and references remain directly
usable by stock `git cat-file` and `git log`.

Compatibility coverage constructs a nested snapshot, branched and merged
history, an annotated tag, a lightweight tag, symbolic and direct references,
and an unreachable object exclusively through `RepositoryStorage`. The entire
repository passes `git fsck --full`, proving both reachable graph integrity and
the validity of enumerated unreachable objects without writing storage files
outside the package boundary.

## Git remote read access

A repository's read-only smart-HTTP remote URL is
`http://<host>:<port>/repositories/<repository ID>`. The API handles the
`info/refs?service=git-upload-pack` discovery request and protocol-v2
`git-upload-pack` exchange by opening the repository through `RepositoryStore`
and invoking stock Git against the handle's `GitDir`. `git` must therefore be
available on the API process's `PATH`.

The server forwards Git's negotiated protocol version and emits the standard
smart-HTTP media types and service preamble. This lets unmodified `git ls-remote`
and `git clone` clients negotiate directly with the server. Discovery
enumerates loose or packed references and, for a populated repository, reports
symbolic `HEAD` as the configured default branch. Clone transfers the complete
reachable object graph, checks out that branch, and preserves snapshot content
and executable modes. Cloning an empty repository also succeeds and selects its
unborn default branch, ready for an initial commit. Write operations are not yet
served.
