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

## Web onboarding

The root web route is an authentication-aware entry point rather than a static
dashboard. A visitor can create an account or sign in; successful account
creation immediately establishes a web session and continues into the
workspace. A new user's primary empty state leads directly to repository
creation, while returning users can search their owned repository list and
copy a clone URL. Repository creation supports the API's private-by-default or
public visibility choices.

The access workspace lists the user's active browser and programmatic grants.
Users can issue short-lived Git or API credentials, copy the plaintext secret
from its one-time reveal, and revoke credentials they no longer use. Signing
out revokes the current browser session.

The workspace repository collection includes projects a user owns and projects
they have been invited to contribute to. Shared projects are marked as
contributing work, so membership remains discoverable after signing out or
switching devices instead of depending on an out-of-band repository URL.

Public projects are discoverable independently of membership through the
anonymous `GET /repositories/public` collection. Its optional `q` parameter
searches normalized repository names and descriptions, and it uses the shared
pagination envelope. The authenticated workspace shows matching public
projects the actor has not already joined; opening one exposes the existing
fork action, so first contact does not depend on an owner invitation.

Client code calls the same-origin `/api/*` boundary implemented by the Next.js
catch-all route handler. It forwards requests and the API's `komodo_session`
cookie to the Go service, avoiding cross-origin exposure of the HttpOnly
credential. The server-only `$API_ORIGIN` selects that service and defaults to
`http://localhost:8080` for the documented two-process development setup.
`$NEXT_PUBLIC_GIT_ORIGIN` selects the externally reachable origin used in clone
URLs and has the same local default.

## Repository browser

Repository cards lead to a browser route at `/repositories/{id}`. The page
presents repository name, description, visibility, and clone URL alongside a
branch-aware file tree and reachable commit history. Directories and text files
can be traversed in place; binary files retain their object identity and size
without attempting a text preview. Empty repositories instead show the first
push workflow. Branch, directory/file path, history view, and exact commit
selection are encoded with the `ref`, `path`, and `view` query parameters, so a
link continues to identify the revision it was opened against even as someone
navigates deeper into the project.

The same-origin web proxy exposes these read-only Go API resources:

- `GET /repositories/{id}/branches` lists named branches and identifies the
  default branch;
- `GET /repositories/{id}/commits?ref={branch-or-commit}` lists reachable
  history with the shared bounded pagination envelope and parsed authorship;
- `GET /repositories/{id}/commits/{object-id}` inspects one commit;
- `GET /repositories/{id}/tree?ref={branch-or-commit}&path={directory}` lists
  one directory snapshot;
- `GET /repositories/{id}/blob?ref={branch-or-commit}&path={file}` previews a
  verified UTF-8 blob, with a 1 MiB display limit.

All revision resolution, graph traversal, and object reads remain behind the
repository catalog and storage boundaries. Public repository browsing is
anonymous. Private browsing requires owner or contributor membership and a
`repository:read` grant, matching metadata and Git transport policy.

## Web proposal workflow

The repository Proposals tab brings decision context into the same browser as
code and history. It leads with title and description search plus open, closed,
and all-state filters so collaborators can find overlapping ideas before
starting work. A proposal detail presents its durable description and complete
conversation; authenticated collaborators can create and discuss proposals,
while authors and repository owners can edit or close them. Closed proposals
remain readable as historical context.

Proposal navigation uses the repository route with `view=proposals`. The
`state`, `q`, and `proposal` query parameters preserve the current discovery
filter and exact inspected record in a shareable URL. Browser mutations call
the existing repository-scoped proposal endpoints through the same-origin web
proxy, so the API remains the authority for visibility and mutation policy.

## Web pull request workflow

The repository Pull requests tab turns published candidate branches into
reviewable application records. Authenticated repository participants can
select distinct source and target branches, describe the change and requested
feedback, and optionally connect the work to an open proposal. The resulting
page brings the purpose, related proposal, captured branch commits, current
branch tips, source-only commits, recursive file changes, text patches or
binary metadata, attributable discussion, and the full maintainer workflow
into one inspection surface. Participants can approve or request changes and
replace or withdraw their current decision. Decisions visibly retain the
commit they evaluated and become stale after follow-up work. The author can
explicitly synchronize a moved source branch, while the readiness panel
explains branch, approval, change-request, conflict, and permission blockers.
Only an authorized owner sees the final merge control, and it remains disabled
until the API reports the request ready.

Pull request navigation uses the repository route with `view=pulls`. The
`pull` query parameter identifies the durable request and `section` preserves
the overview, commits, files, or discussion view in a shareable URL. Browser
reads and mutations use the existing repository-scoped pull request endpoints
through the same-origin proxy. Public repository changes and readiness remain
anonymously readable; creation, discussion, review, synchronization, and merge
controls appear according to the authenticated actor's API-backed permissions.

A fork owner can turn independently published work into an upstream pull
request from the fork header's **Contribute upstream** action. Creation is sent
to the upstream repository's pull-request collection with the fork's
`source_repository_id`, source branch, and selected upstream target branch.
The durable request remains discoverable under the upstream target while
retaining both repository IDs and both exact opening revisions. Its commits and
file changes read the source snapshot from the fork and the base snapshot from
upstream, so fork-only objects never need to be copied into the target merely
for review. The author can explicitly synchronize the request after an
authorized push advances the fork branch; the source repository and lineage
are revalidated before the captured source revision changes.

Ownership remains split across that boundary. The fork owner may explicitly
enable maintainer modification for an open request. An upstream owner or a
collaborator who has already reviewed or discussed that request can then issue
a 24-hour Git credential scoped to only the fork repository and contribution
branch; this neither creates fork membership nor permits another branch update.
Turning the option off or closing the request revokes all such credentials.
The author or upstream owner may close an open request. Readiness reports a
missing source branch or unavailable source repository as a safe blocker.
Merge remains upstream-owner-only and links the fork's immutable objects into
upstream storage before creating the two-parent merge, without sharing refs.
Repository-defined checks remain governed and discoverable beneath the upstream
pull request while retaining the fork repository as the immutable snapshot
source used for manifest discovery, isolated execution, and reruns. Required
checks therefore evaluate the same exact proposed revision reviewed by the
maintainer. The merge commit records `Source-Repository`, `Source-Branch`, and
`Source-Commit` trailers alongside contributor and maintainer IDs; the pull
request record and linked commit parent keep that attribution and provenance
readable after the fork or contribution branch is removed.

The complete open-contribution regression begins with anonymous public search
and proves a previously unknown user can fork, explicitly synchronize a newer
upstream revision, publish through stock Git, open a cross-repository pull
request, accept maintainer guidance through a fork-scoped agent run, satisfy a
required exact-revision check, and be merged. It also asserts that no step
silently creates upstream membership and that the durable pull request and
merge trailers retain contributor, maintainer, fork, branch, and commit
provenance.

### Agent change sessions

The pull request's Agent sessions section is the durable entry point for agent
collaboration. An authenticated repository participant can start a session on
an open pull request. Creation captures the initiating user's stable ID and the
exact source commit represented by the pull request, enters the
`awaiting_instructions` state, and appends a `session.started` event. The web
lists prior sessions newest first and renders their public ordered timeline, so
a collaborator can leave and reconnect without access to worker processes or
their internal logs.

Session data lives beneath `$CHANGE_SESSION_ROOT`, or
`apps/api/data/change-sessions` by default, behind `changesessions.Store`. The
repository-scoped public API is:

- `POST /repositories/{id}/pull-requests/{pull}/change-sessions` starts a
  session and returns its canonical `Location`;
- `GET /repositories/{id}/pull-requests/{pull}/change-sessions` lists durable
  sessions with the shared pagination envelope;
- `GET /repositories/{id}/pull-requests/{pull}/change-sessions/{session}`
  returns its state and captured context;
- `GET /repositories/{id}/pull-requests/{pull}/change-sessions/{session}/events`
  returns its ordered, paginated timeline.

Reads use the same public/private visibility and participant rules as the pull
request. Starting a session requires authenticated `repository:write` access
and an open pull request. `POST .../change-sessions/{session}/runs` accepts a
mandate, the explicitly selected captured source revision, optional relevant
repository paths, an agent identity (default `codex`), and a working branch. It
creates a durable queued run and returns its worker Git secret exactly once.
The grant expires after 24 hours and Git transport enforces the pull request's
source repository ID and exact `refs/heads/...` source branch, even when the
session itself is stored beneath a different upstream target repository.
Cross-repository delegation by an upstream maintainer or participant requires
the fork owner's active maintainer-modification opt-in; turning that policy off
or closing the pull request revokes agent and human delegated-write grants for
the contribution branch. The pull request author may delegate directly against
their own fork. `DELETE .../runs/{run}/credential` lets that initiator revoke it
immediately. Workers publish progress with their one-time run credential at
`POST .../change-sessions/{session}/runs/{run}/events`. Accepted ordered record
types cover run status, agent messages, tool actions, artifacts, failures, and
branch updates; bounded metadata carries public summaries, paths, tool names,
errors, and commit IDs without exposing private execution logs. The API derives
and persists the run ID, initiating user, agent identity, and captured revision
on every event, verifies the exact grant, repository, and working branch, and
advances runs through queued, running, paused, succeeded, failed, or canceled
states. Terminal runs reject later records, and reporting a failed run revokes
its branch credential immediately. The web timeline refreshes while work is
active.

Repository participants can post `guidance`, `answer`, `pause`, `resume`, and
`cancel` interventions at `POST .../runs/{run}/interventions`. Every accepted
action is appended as an attributable `run.intervention` timeline event in the
same storage transaction as its state transition. Guidance and answers require
a message; pause is valid only for queued or running work, resume only for
paused work, and terminal states reject every later intervention. A paused
worker cannot publish progress. Cancellation is terminal and revokes the worker
Git grant, including when a peer collaborator stops the run. Before
cancellation, workers use their exact-run credential at
`GET .../runs/{run}/control` to poll the authoritative state and ordered
intervention sequence. The session page exposes follow-up, answer, pause,
resume, and cancel controls and preserves each action in the shared timeline.

Delegated runs write directly to the pull request source branch in its source
repository; the run Git grant cannot publish another branch or write to the
upstream target. After committing and pushing a descendant of the session's
captured revision, a running worker completes the review
handoff with `POST .../runs/{run}/publication`. The request supplies an outcome,
checks performed, and unresolved concerns. The API verifies the branch tip,
derives exact new commits and changed paths from repository storage, persists a
structured publication, synchronizes the pull request snapshot, and revokes the
worker grant. The session links those commits and files into ordinary pull
request inspection; commit-bound reviews become stale and readiness evaluates
the new revision without a separate author synchronization step.

The connected workflow is covered by a black-box developer-agent collaboration
test. It delegates both failed and successful attempts, redirects active work,
reopens all durable stores during the successful run, publishes through stock
Git and the credential-bound worker API, then reviews and merges through the
ordinary pull request contract. This keeps reconnection, failure containment,
attribution, publication, review, and merge behavior from drifting into
independent endpoint assumptions.

## Repository-defined checks

A candidate revision opts into automatic verification with
`.komodo/checks.json` in that revision. Schema version `1` contains a `checks`
array; every check has a unique `name`, a shell `command`, and may select a
repository-relative `working_directory`, a `timeout_seconds` value from 1 to
1800 (600 by default), bounded string `environment` entries, and up to 20
repository-relative regular-file `artifacts` to retain (25 MiB each). At most
20 checks are accepted. Because the manifest is read through `GraphStore` from the
pull request's exact source commit, both the commands and their execution
context change through ordinary code review.

Opening a pull request automatically queues every check declared by its source
commit. The same trigger runs after an author synchronizes a later source tip
or an agent publishes a new revision. A run permanently records its repository,
pull request, commit, copied definition, queued/running/terminal state, timing,
and exit status beneath `$CHECK_RUN_ROOT` (`apps/api/data/check-runs` in the
documented root development setup). `GET
/repositories/{repository}/pull-requests/{pull}/check-runs` returns the durable
newest-first collection through the ordinary repository read policy and shared
pagination envelope.

Every run is also an inspectable durable resource at `GET
/repositories/{repository}/pull-requests/{pull}/check-runs/{run}`. Its ordered,
one-based evidence sequence includes queued/running/terminal status records,
timestamped stdout and stderr chunks (bounded to 10 MiB per attempt), the exact
exit code and timeout outcome, and artifact metadata with size, media type, and
SHA-256 digest. `GET .../{run}/events?after={sequence}` returns only later
records plus the current state and latest sequence, allowing polling clients to
disconnect and resume active output without guessing which records they saw.
Declared artifacts are copied into check-run storage before the isolated
workspace is removed and downloaded at `GET
.../{run}/artifacts/{artifact}`. Collection, detail, evidence, and artifact
reads all follow the pull request repository's existing visibility policy;
separate attempts remain tied to their verified commit after success or
failure.

The pull request's Checks section keeps verification beside the code and
discussion. It lists every automatic and rerun attempt, polls active attempts
for live stdout and stderr, and exposes retained artifacts as authenticated
same-origin downloads. Any authenticated repository participant can cancel a
queued or running attempt or rerun a terminal one. Cancellation terminates the
active sandbox process and appends a terminal attributed event. A rerun copies
the original exact commit and check definition into a distinct attempt with
the requesting user's stable ID and a link to the prior attempt, so neither
history nor authorship is overwritten.

The control resources are `POST .../check-runs/{run}/cancel` and `POST
.../check-runs/{run}/rerun`. They require ordinary authenticated repository
write participation; public and private read behavior for attempts, evidence,
and artifacts remains unchanged.

A failed attempt for the pull request's current source revision can become an
agent repair workspace with `POST .../check-runs/{run}/change-session`. The
new session captures that exact failed revision, the complete copied check
definition, ordered stdout/stderr, command outcome, and retained artifact
identities. Its artifacts continue to download from the originating run, so
the check-run store and repository read policy remain authoritative. The web
opens the resulting session beside its mandate form, letting a collaborator
delegate without reconstructing the failure. A published descendant revision
uses the existing agent publication path, which synchronizes the pull request
and automatically queues the new revision's manifest-defined checks.

Repository owners declare the quality bar for a target branch with `PUT
/repositories/{repository}/required-checks`, supplying the branch and up to 20
unique check names; `GET .../required-checks?branch={branch}` reads the policy
through ordinary repository visibility rules. The pull request Checks view
lets maintainers select names observed in its repository-defined manifest.

Readiness evaluates each required name against the pull request's exact source
commit and reports the target branch, evaluated commit, requirement name,
selected run and run commit, and one of `missing`, `pending`, `failed`,
`canceled`, `stale`, or `succeeded`. Only `succeeded` satisfies a requirement;
a successful run from a different revision is explicitly stale. The merge
endpoint repeats this commit-bound evaluation independently, so bypassing the
readiness UI cannot publish a revision that did not pass every target-branch
requirement.

Each command receives a newly materialized copy of its exact Git snapshot with
no `.git` directory or credentials. Bubblewrap gives it a private process,
mount, IPC, user, and network namespace; only the writable snapshot, disposable
`/tmp`, and read-only runtime paths are visible. Symlinks and submodules are not
materialized, the environment is cleared before declared values are installed,
and every process is killed at its configured timeout. The API runtime therefore
requires `bwrap` on `PATH` in addition to Git.

The connected verification workflow is covered by a black-box test that opens
a pull request through the public API, executes its repository-defined check in
the real sandbox, inspects the failed logs and artifact, and starts an
evidence-backed agent repair session. The worker publishes its descendant
revision through stock Git and the credential-bound publication API; the test
then observes the new check pass, verifies required-check readiness selects
that exact commit and attempt, and reviews and merges through the ordinary pull
request contract. Both the original failed evidence and repaired successful
attempt remain readable after merge, so the safety decision does not depend on
ephemeral runner state.

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

Readable repositories can also seed independently owned forks. `POST
/repositories/{upstream}/forks` accepts optional `name`, `description`, and
`visibility` fields, defaults the name and description from upstream and the
visibility to private, and returns the ordinary repository resource with
`upstream_repository_id` and `upstream_api_url` lineage fields. The caller
needs `repository:write` and must be able to read the upstream repository; no
upstream write or contributor role is granted. The fork has its own Git remote,
owner policy, references, branches, and lifecycle.

Fork creation and later synchronization share immutable object files through
filesystem hard links managed by `storage.Store`. This preserves one physical
copy of identical Git objects while making either repository safe to delete;
reference files are never shared. A fork owner synchronizes a named branch with
`POST /repositories/{fork}/sync` and `{ "branch": "main" }`. The API links any
new upstream objects before fast-forwarding the identically named fork branch,
returns the before/after commits and whether it changed, and rejects missing
upstream branches or diverged fork history without rewriting independent work.
The web repository header exposes fork creation, lineage inspection, and the
same selected-branch synchronization action.

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

Owners manage membership from the repository People tab by entering a user's
public handle. The invited contributor then sees the repository in their normal
workspace and can copy the same Git remote used by the browser. Proposal,
discussion, pull-request, and review surfaces resolve stable actor IDs to
current display names and handles; durable records and merge trailers continue
to retain immutable IDs.

`GET /repositories?affiliation=all` supplies the signed-in web workspace with
owned and contributed repositories. Omitting `affiliation` preserves the
owner-only collection contract, and public repositories are not implicitly
included in either user's workspace.

These endpoints require repository API scopes in addition to ownership (`read`
for listing and `write` for changes). A contributor with `repository:read` may
inspect a private repository by ID. The default repository collection remains
owner-specific, while `affiliation=all` includes contributed work. Contributors
cannot change repository metadata or visibility,
delete the repository, inspect or manage its collaborator list, or grant access
to anyone else. Denied authenticated users receive the same `404` response as
an unknown repository.

## Repository activity

Meaningful collaboration changes are recorded in an append-only ledger beneath
`$ACTIVITY_ROOT`, or `apps/api/data/activities` by default. Events retain a
stable actor ID, repository ID, typed affected resource, timestamp, and compact
event-specific metadata. Access changes also retain the affected user's stable
ID. Resolvable `@handle` references in proposal and pull-request titles,
descriptions, and comments produce distinct `mention.created` events associated
with the same resource and linked to the source event; the mentioned user's ID
remains valid if their handle later changes.

`GET /repositories/{id}/activity` returns newest-first activity with the shared
bounded pagination envelope. Public repository activity is anonymously readable;
private activity requires owner or contributor membership and
`repository:read`, matching the underlying resources. Recorded types cover
proposal creation, edits, closure, and discussion; pull-request creation,
synchronization, discussion, and merge; review submission, replacement, and
withdrawal; contributor access grants and revocations; and mentions. This
ledger is the durable source for later attention and inbox views rather than a
replacement for the affected proposal, pull request, review, or repository.

## Actionable inbox

The signed-in workspace derives one global inbox from repository activity and
the actor's current repository affiliations. It includes only work with a
useful next step: owner review of an open pull request, responses to active
review or discussion, direct mentions, and awareness of access or a completed
outcome. Items are classified as `review`, `response`, or `awareness`, include
current actor and repository display context, and link directly to the affected
proposal, pull request, or repository. Activity from inaccessible repositories
and the actor's own actions is excluded; completed resources do not continue to
present stale response or review work.

`GET /inbox` requires authenticated `repository:read` access, uses the shared
pagination envelope, and accepts an optional `classification` filter.
`DELETE /inbox/{eventID}` clears one item for the current actor. Clearance is
durable beneath `$INBOX_ROOT`, or `apps/api/data/inbox` in the documented root
development setup, while the underlying append-only activity remains intact.
The web workspace exposes the count in primary navigation, matching filters,
empty states, direct collaboration links, and per-item Clear actions.

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
