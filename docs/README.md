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

## Web interface foundation

The web app uses a persistent responsive shell for global search, primary
navigation, pinned repositories, notifications, account access, and page
content. Pages extend that shell through the root layout rather than rendering
their own global navigation. Shared interface primitives and line icons live in
`apps/web/src/components`; global color, typography, spacing, focus, motion,
and responsive rules live in `apps/web/src/app/globals.css`.

The baseline accessibility contract includes semantic landmarks and headings,
a keyboard skip link, visible `:focus-visible` treatment, labelled icon-only
controls, and reduced-motion support. Interactive components expose hover,
focus, and pressed states, and navigation collapses to a native disclosure on
small screens.

## Human identity

The API exposes durable human accounts as JSON resources. Accounts live beneath
`$USER_ROOT`, or `apps/api/data/users` by default, and are managed through the
`apps/api/users` storage boundary. A UUID-shaped `id` is the stable actor key
for attribution. The public `handle` is case-normalized and unique, while
`display_name` provides the minimal human-readable profile; both profile fields
may change without changing the actor ID. Creation and updates are written
atomically so a successfully returned identity survives process restarts.

The account contract is:

- `POST /users` creates an account from `handle`, `display_name`, and a password
  of 12 to 72 bytes, returning
  `201 Created` and its canonical `Location`;
- `GET /users/{id}` inspects the stable identity and current profile;
- `GET /users/by-handle/{handle}` resolves a case-insensitive public handle and
  identifies its canonical `/users/{id}` resource through `Content-Location`;
- `PUT /users/{id}` replaces the mutable profile while retaining `id` and
  `created_at`; it requires that user's `profile:write` access.

Responses include `created_at` and `updated_at` UTC timestamps. Invalid
profiles return `422`, duplicate handles return `409`, and unknown or malformed
IDs return `404`.

## Authentication and access grants

Authentication state lives beneath `$AUTH_ROOT`, or `apps/api/data/auth` by
default. Passwords are bcrypt hashes and never appear in user resources. Access
tokens contain 256 random bits with a recognizable `vkm_` prefix; only their
SHA-256 digests are stored. The plaintext token is returned exactly once when a
programmatic grant is created. Grant metadata includes its client kind, scopes,
creation and expiration times, last use, and optional revocation time, so access
can be inspected without recovering a credential.

`POST /sessions` accepts a handle and password and creates a 12-hour `web`
grant. Its credential is carried in an HttpOnly, SameSite=Strict cookie and is
not exposed in the JSON response. `GET /session` returns the current actor and
grant metadata; `DELETE /session` revokes it and clears the cookie. Bearer API
tokens may also use `GET /session` to inspect their actor and effective access.

An authenticated grant with `access:manage` may create, list, and revoke
programmatic credentials through `POST /access-grants`, `GET /access-grants`,
and `DELETE /access-grants/{id}`. API grants last at most 90 days and may request
only `profile:read`, `profile:write`, `access:manage`, `repository:read`, and
`repository:write`. Git grants last at
most 30 days and may request only `git:read` and/or `git:write`. Web grants are
created only by password sign-in, last at most 12 hours, and cannot be minted
through the programmatic endpoint. Expired and revoked credentials fail
immediately. A failed scope check returns the same `401` outcome as an invalid
credential, avoiding disclosure of broader grant details.

API clients send `Authorization: Bearer <token>`. Stock Git clients use any
Basic-auth username and a Git token as the password, commonly by configuring a
credential helper rather than embedding it in a remote URL.

## Owned repository lifecycle

Application repository metadata lives beneath `$REPOSITORY_CATALOG_ROOT`, or
`apps/api/data/repositories` by default, and is managed through the
`apps/api/repositories` boundary. Each resource records its immutable opaque
repository ID, immutable owner user ID, owner-scoped normalized name, optional
description of at most 280 characters, `private` or `public` visibility,
creation and update times, and current empty state. Names may contain lowercase
letters, numbers, `.`, `_`, and `-`. Repositories are private by default;
catalog records written before visibility existed are also interpreted as
private.
The catalog delegates bare repository creation, inspection, and deletion to
`apps/api/storage`, so application callers do not construct or remove Git
directories themselves.

The authenticated JSON contract is:

- `POST /repositories` creates a named empty repository from `name`, optional
  `description`, and optional `visibility` (private by default), and requires
  `repository:write`;
- `GET /repositories` lists only the current actor's repositories and requires
  `repository:read`;
- `GET /repositories/{id}` inspects a repository; public resources are
  anonymous while private resources require their owner's `repository:read`;
- `PATCH /repositories/{id}` partially changes `name`, `description`, or
  `visibility` and requires the owner's
  `repository:write`;
- `DELETE /repositories/{id}` removes an owned repository and all of its Git
  data and requires `repository:write`.

Creation returns `201 Created` with a canonical `Location`; deletion returns
`204 No Content`. Repository lists remain owner-specific. Authenticated actors
denied by ownership and unknown IDs both receive `404`, so ownership is not
disclosed; anonymous private reads receive `401`. The `id`, canonical API path, and
`git_url` all use the same storage identity. The returned relative Git URL can
be resolved against the API origin and used immediately with a Git access
token.

Collection endpoints (`GET /repositories` and `GET /access-grants`) return an
envelope containing `items`, `page`, `per_page`, and `total_count`. They accept
positive `page` and `per_page` query parameters, default to 1 and 30, and cap
`per_page` at 100. Invalid pagination returns `invalid_pagination` with status
`422`. Validation and conflict responses consistently expose stable
machine-readable `error` codes.

## Repository collaborators

A repository owner can grant an existing user the single `contributor` role.
Membership is durable repository metadata and is managed only by the owner:

- `PUT /repositories/{id}/collaborators/{user_id}` grants contributor access
  idempotently and returns the user's current public profile plus role;
- `GET /repositories/{id}/collaborators` lists contributors with the shared
  bounded pagination envelope;
- `DELETE /repositories/{id}/collaborators/{user_id}` revokes access
  immediately.

These endpoints require repository API scopes in addition to ownership (`read`
for listing and `write` for changes). A contributor with `repository:read` may
inspect a private repository by ID, but repository collections remain
owner-specific. Contributors cannot change repository metadata or visibility,
delete the repository, inspect or manage its collaborator list, or grant access
to anyone else. Denied authenticated users receive the same `404` response as
an unknown repository.

## Repository proposals

Proposals give collaborators a durable, repository-scoped place to establish
context before or alongside code. They live beneath `$PROPOSAL_ROOT`, or
`apps/api/data/proposals` by default, behind the `apps/api/proposals` boundary.
Each proposal has an immutable ID, repository and author IDs, mutable title and
body, open or closed state, creation and update times, and closing actor and
time. Discussion comments are append-only records with immutable IDs, author
IDs, bodies, and creation times. Stable actor IDs keep the full conversation
attributable even when a user's public profile changes.

The repository-scoped JSON contract is:

- `POST /repositories/{id}/proposals` creates a proposal from `title` and
  optional `body`;
- `GET /repositories/{id}/proposals` lists proposals with shared pagination
  and an optional `state=open|closed` filter;
- `GET /repositories/{id}/proposals/{proposal_id}` inspects one proposal;
- `PATCH /repositories/{id}/proposals/{proposal_id}` changes `title` or `body`,
  and closes it by setting `state` to `closed`;
- `POST /repositories/{id}/proposals/{proposal_id}/comments` adds an
  attributable comment;
- `GET /repositories/{id}/proposals/{proposal_id}/comments` returns the
  append-only conversation with shared pagination.

Public repository proposals and discussions are anonymously readable. Private
reads require owner or contributor membership. Creation and discussion require
authenticated `repository:write` access; editing and closing
are limited to the proposal author or repository owner. A closed proposal and
its conversation remain readable and cannot be reopened, preserving the
recorded outcome. Denied authenticated users receive `404` just as they do for
the containing repository.

## Pull requests

Pull requests connect published candidate branches to the maintained branch
they propose changing. Durable records live beneath `$PULL_REQUEST_ROOT`, or
`apps/api/data/pull-requests` by default, behind the `apps/api/pullrequests`
boundary. Each record carries an immutable repository and author ID, optional
link to a proposal in that repository, title and body describing the purpose,
source and target branch names, the exact commit ID at each branch tip when the
request was opened, an `open` lifecycle status, and creation and update times.
Moving either branch later does not silently change the state represented by
the request. The author may explicitly synchronize an open request after
publishing follow-up commits; this advances only its represented source commit,
while existing reviews retain the commit they actually evaluated.

The repository-scoped JSON contract is:

- `POST /repositories/{id}/pull-requests` opens a request from `title`, optional
  `body` and `proposal_id`, plus distinct `source_branch` and `target_branch`;
- `GET /repositories/{id}/pull-requests` lists requests with the shared bounded
  pagination envelope;
- `GET /repositories/{id}/pull-requests/{pull_request_id}` inspects the durable
  request and its commit snapshot.
- `POST /repositories/{id}/pull-requests/{pull_request_id}/synchronize` lets
  the author advance an open request to the current source-branch commit after
  publishing review follow-up work;
- `GET /repositories/{id}/pull-requests/{pull_request_id}/commits` lists the
  source-only commits represented by the snapshot in oldest-first order;
- `GET /repositories/{id}/pull-requests/{pull_request_id}/files` lists the
  recursive source-versus-target file changes in path order;
- `POST /repositories/{id}/pull-requests/{pull_request_id}/comments` appends an
  attributable discussion comment;
- `GET /repositories/{id}/pull-requests/{pull_request_id}/comments` returns the
  append-only discussion;
- `PUT /repositories/{id}/pull-requests/{pull_request_id}/reviews/me` creates
  or replaces the authenticated actor's decision with `approve` or
  `request_changes`;
- `DELETE /repositories/{id}/pull-requests/{pull_request_id}/reviews/me`
  withdraws the authenticated actor's current decision;
- `GET /repositories/{id}/pull-requests/{pull_request_id}/reviews` lists each
  participant's current decision with the shared bounded pagination envelope;
- `GET /repositories/{id}/pull-requests/{pull_request_id}/readiness` reports
  every currently known merge blocker without changing repository state.
- `POST /repositories/{id}/pull-requests/{pull_request_id}/merge` applies a
  ready request to its target branch as a two-parent merge commit; only the
  repository owner may perform it.

Both named branches must exist and point directly to commits at creation. A
linked proposal must belong to the same repository. Commit inspection walks
the source ancestry while excluding every commit reachable from the target.
File inspection recursively compares the two snapshotted trees and reports
added, modified, and deleted paths with old/new object IDs and modes. UTF-8
text files include additions, deletions, and a full-file unified patch; binary
files are identified without embedding their bytes. Comments are immutable
records with stable author IDs, bodies, and creation times. A review records
the reviewer's stable user ID and the exact source commit evaluated. Each
reviewer has at most one current decision: another `PUT` replaces it and a
`DELETE` withdraws it. Review responses derive `stale` by comparing the
evaluated commit with the live source-branch tip, so later source commits do not
silently inherit an earlier decision; a missing source branch also makes every
remaining review stale. Explicit synchronization similarly makes reviews of
the previous source commit stale until reviewers evaluate the updated work.

Readiness is caller-aware and reports `ready`, the caller's `can_merge`
permission, live source and target branch tips alongside their snapshotted
commit IDs, review counts, conflict state, and an ordered list of stable blocker
codes with explanations. A ready pull request is open, still has its exact
snapshotted source commit at the source branch, has both branches available,
has one current approval from the repository owner, has no current
request-changes reviews, merges cleanly into the live target, and is being
inspected by the owner. Target movement is visible but does not itself block a
clean merge. Source movement blocks the represented request and makes its prior
reviews stale. Conflict inspection uses stock Git with a disposable object
directory and the repository object database mounted read-only as an alternate,
so the readiness request writes neither objects nor references. When missing or
changed branches prevent a meaningful merge probe, `has_conflicts` is `null`
and the corresponding branch blockers explain why.

Creation, discussion, and review mutation require authenticated
`repository:write` access;
public reads are anonymous, private reads require owner or contributor
membership and `repository:read`, and denied authenticated users receive
`404`. Commit, file, comment, and review lists use the shared pagination
envelope. A successful merge records `status=merged`, `merge_commit_id`,
`merged_by_id`, and `merged_at`, appends an attributable outcome comment, and
closes a linked proposal with the maintainer as its closing actor. The target
branch is rechecked immediately before publication. The merge commit has the
live target and snapshotted source as its parents, uses the merged tree, and
retains the pull request title/body plus pull request, proposal, author, and
maintainer stable IDs in its message. Thus stock Git history and application
resources both preserve what changed and the collaboration that accepted it.

Black-box workflow coverage provisions two accounts and then uses only the
public JSON and smart-HTTP Git contracts to admit a newcomer, discuss a
proposal, publish a candidate branch, open a linked pull request, receive and
address a change request, synchronize the follow-up commit, earn approval, and
merge. A fresh anonymous clone and the closed proposal verify the resulting
code and collaboration state without inspecting application storage.

## Git repository storage

`apps/api/storage` is the repository lifecycle boundary. A store owns one root
directory and creates bare Git repositories beneath it using opaque UUID-shaped
IDs. `Create` atomically publishes a repository, `Open` validates and reopens it
by ID, `Delete` removes it, and `Inspect` reports its identity and whether it is
bare and empty.

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

- lifecycle — `RepositoryStore.Create`, `Open`, and `Delete`, then `ID` and
  `Inspect`;
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

## Git remote access

A repository's authenticated smart-HTTP remote URL is
`http://<host>:<port>/repositories/<repository ID>`. The API handles the
`info/refs?service=git-upload-pack` discovery request and protocol-v2
`git-upload-pack` exchange by opening the repository through `RepositoryStore`
and invoking stock Git against the handle's `GitDir`. `git` must therefore be
available on the API process's `PATH`.

Upload-pack discovery and exchange are anonymous for public repositories.
Private repository reads require an owner or contributor Git access token with
`git:read`. Receive-pack discovery and exchange require an owner or contributor
token with `git:write`, regardless of visibility. Contributors may create,
advance, force-update, and delete candidate branches, but a pre-receive policy
rejects every contributor update to the repository's default branch. Only the
owner controls that maintained branch. A valid credential belonging to neither
an owner nor contributor receives the same non-disclosing `404` as the JSON
interface; missing credentials receive `401` when authentication is required.
This allows a public clone to need no credential and a private clone or pull
credential to omit write authority, while publishing access may be short-lived,
independently revoked, and limited by repository membership.

The server forwards Git's negotiated protocol version and emits the standard
smart-HTTP media types and service preamble. This lets unmodified `git ls-remote`
and `git clone` clients negotiate directly with the server. Discovery
enumerates loose or packed references and, for a populated repository, reports
symbolic `HEAD` as the configured default branch. Clone transfers the complete
reachable object graph, checks out that branch, and preserves snapshot content
and executable modes. Cloning an empty repository also succeeds and selects its
unborn default branch, ready for an initial commit. Existing clones can fetch
newly reachable objects and updated remote-tracking state, then fast-forward the
checked-out branch with `git pull` without recloning. Upload-pack's negotiation
limits that transfer to objects the client is missing. All named branches are
advertised, so clients can discover them and create corresponding remote-tracking
references during fetch.

Write discovery and requests use the corresponding `git-receive-pack` smart
HTTP endpoints. A stock client may create, fast-forward, explicitly force-update,
and delete any named branch. Git's receive protocol does not carry a force bit:
an ordinary stock client refuses to send a non-fast-forward update, while
`--force` explicitly bypasses that client-side check. Deleting the branch named
by symbolic `HEAD` keeps `HEAD` intact, letting later clones select the same
unborn default branch and a later push recreate it. Receive policy rejects
updates outside `refs/heads/*` while incoming objects remain quarantined.

The API compatibility suite proves the complete workflow as one black-box
session. After provisioning an empty repository, it uses only an unmodified Git
client and the smart-HTTP URL to exercise both the complete default-branch
lifecycle and an independent candidate branch. Coverage includes branch
discovery, fetch, creation, advancement, pull, force-update, deletion, and
default-branch recovery. Remote observations use `git ls-remote` or a fresh
working copy rather than direct access to server-side references or objects.
