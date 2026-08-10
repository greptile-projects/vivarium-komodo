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
  Persistent application chrome lives in `src/components/app-shell.tsx`;
  extend it instead of recreating navigation within pages. Reusable interface
  primitives live in `src/components/ui.tsx`, shared line icons in
  `src/components/icons.tsx`, and design tokens plus global interaction states
  in `globals.css`. Preserve visible focus treatment, the skip link, reduced-
  motion behavior, and responsive mobile navigation when adding workflows.
  Browser workflows reach the Go service through the same-origin
  `src/app/api/[...path]/route.ts` proxy so HttpOnly web sessions remain
  first-party. The proxy targets `$API_ORIGIN`, defaulting to
  `http://localhost:8080`; keep that server-only setting aligned with the API
  deployment rather than calling the Go origin directly from client code.
  `$NEXT_PUBLIC_GIT_ORIGIN` supplies the browser-visible Git clone origin and
  has the same local default.
  Repository browser routes live at `src/app/repositories/[id]`; branch, path,
  and exact-commit context belongs in the URL (`ref`, `path`, and `view`) so
  file and history navigation remains shareable. Read repository graph data
  through the JSON browser endpoints rather than interpreting Git objects in
  the frontend. Proposal discovery is the `view=proposals` repository tab;
  state, search, and inspected proposal context remain shareable through the
  `state`, `q`, and `proposal` query parameters. Keep proposal mutations behind
  the same-origin proxy and reflect API author/owner permissions in controls.
  Pull request discovery and creation is the `view=pulls` repository tab;
  inspected request and section context remain shareable through the `pull`
  and `section` query parameters. Pull request pages present their immutable
  branch snapshot alongside live branch tips, proposal context, source-only
  commits, file patches, discussion, current and stale review decisions, and a
  live readiness report. Keep review, author synchronization, and owner merge
  actions separate and permission-aware; synchronization deliberately makes
  prior commit-bound reviews stale.
  Fork owners open upstream pull requests from the fork repository surface. A
  cross-repository request is stored beneath the upstream target repository and
  retains both `repository_id` (target) and `source_repository_id` (fork), plus
  both exact branch revisions. Resolve live source state, synchronization,
  commits, and the new side of file comparisons through the source repository;
  never assume the source branch or its pushed objects exist in the target.
  Cross-repository source writes remain under fork-owner control while review,
  checks, closure, and merge remain target policy. Authors may opt in to
  maintainer modification; an upstream owner or existing pull-request
  participant then receives a 24-hour Git credential limited to the fork and
  exact contribution branch. Disabling the policy or closing the request
  revokes those credentials. Merge links immutable source objects into the
  target without sharing refs or granting general fork access. Cross-repository
  check runs are cataloged beneath the target pull request but retain and read
  their exact snapshot from `source_repository_id`; reruns must preserve that
  source identity. Merge commits retain source repository, branch, and commit
  trailers so provenance survives later fork or branch deletion.
  Agent collaboration begins in the pull request's `section=sessions` view.
  Change sessions retain their initiating user, captured pull request source
  commit, current state, and ordered public timeline; keep later worker
  execution details behind this application-facing contract.
  Delegated runs retain their mandate, exact session revision, selected context
  paths, agent identity, and explicit working branch. Their one-time worker Git
  credential is limited to the pull request's source repository and exact source
  branch, expires after 24 hours, and may be revoked by the initiating
  collaborator; never substitute a general user credential or expose its secret
  after creation. On cross-repository requests, the author may delegate against
  their fork directly; other upstream participants require the author's active
  maintainer-modification opt-in, and disabling it or closing the request revokes
  agent grants alongside human delegated-write grants.
  Workers use that exact-run credential to append typed public progress records
  for status, messages, tool actions, artifacts, failures, and branch updates.
  Derive initiator, agent, run, and revision attribution from session storage
  rather than trusting worker-supplied identity fields, and keep raw execution
  logs and secrets out of event metadata.
  Any authenticated repository participant may guide or answer an active run,
  pause it, resume a paused run, or cancel it through the session. These
  interventions are ordered timeline records. Workers poll the credential-bound
  control resource for the current run state and full intervention sequence;
  paused runs reject progress publication; failed and canceled runs are
  terminal and immediately revoke the worker Git credential.
  Delegated publication is limited to the pull request source branch. After a
  worker pushes a descendant revision, its credential-bound publication action
  derives exact commits and changed paths from Git, records its summary, checks,
  and unresolved concerns, synchronizes the pull request snapshot, and revokes
  the run credential. Prior commit-bound reviews then become stale and readiness
  is recalculated from the synchronized revision.
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
  A user may fork any repository they can read without gaining authority over
  it. Fork catalog records retain their immediate upstream repository ID while
  ownership, visibility, collaborators, references, and Git write policy remain
  independent. Storage creates and synchronizes forks by hard-linking immutable
  object files, never by sharing mutable references or copying object bytes;
  deletion of either repository must leave the other valid. Fork branch sync is
  owner-only and fast-forward-only from the identically named upstream branch,
  so diverged independent work is never overwritten.
  The default repository collection remains owner-only; the
  `affiliation=all` collection returns the actor's owned and contributed
  repositories for durable workspace discovery, not public search.
  Anonymous public project discovery uses the separately paginated
  `/repositories/public` collection and its `q` name/description search; the
  web workspace excludes projects the actor has already joined and leads
  unknown contributors from those results into the repository fork workflow.
  Route Git through the catalog as well as storage so transport access cannot
  bypass ownership, collaborator, and visibility policy.
  Read-only browser endpoints for branches, commits, trees, and blobs are
  registered in `repository_browser_http.go`. They resolve data only through
  the catalog and `RepositoryStorage`, and apply the same anonymous-public or
  authenticated-participant policy as repository metadata and Git reads.
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
  Proposal delivery plans are repository-policy readable and participant
  editable beneath each proposal's `/plan` resource. Ordered tasks require an
  observable outcome, may depend only on tasks in the same plan, and may link
  back to proposal comment IDs. The API rejects dependency cycles and derives
  `ready` only for planned tasks whose dependencies are completed. Task creates,
  edits, reordering, and status decisions retain immutable actor-attributed
  snapshots in the plan history.
  Ready plan tasks may carry one durable assignment contract naming a human
  repository participant or the available `codex` agent, an exact commit in
  that repository, a mandate, and the future `contents:read` plus
  `candidate_branch:write` scope. Assignment does not issue a credential or
  start work. Use the assignment ID as the concurrency token for explicit
  reassignment and revocation; stale or simultaneous claims must return a
  conflict and every accepted transition remains actor-attributed in plan
  history.
  Starting a ready `codex` assignment uses that same assignment ID as a
  concurrency token and creates a proposal-task-scoped change session plus a
  unique `codex/task-*` branch at the assigned commit. The session immutably
  captures proposal, task, dependency, mandate, and repository context and
  immediately queues a run with a 24-hour Git credential limited to that exact
  repository and branch. These pre-pull-request sessions use the task's
  `/change-sessions` public read, event, control, and intervention resources;
  publication into ordinary pull-request review is a separate transition.
  Publish completed proposal-task work through the task's `/contributions`
  resource. It creates an ordinary commit-bound pull request and links its
  proposal, task, optional pre-review session, checks, and exact source/target
  revisions in both directions. Human assignees publish a candidate branch
  with repository-write authority; agents publish their captured task-session
  branch with the run's branch-only credential. Contribution state remains
  `draft` or `review` until the linked request is merged, closed, or superseded;
  only `merged` satisfies dependencies. Preserve these links and the task and
  session merge trailers rather than treating a successful run as completed
  work.
  Recompute every dependent task after contribution and plan transitions;
  expose the exact blocking task IDs and notify assigned humans when work
  becomes ready, blocked, changed, or obsolete. Unstarted assignments may move
  to a verified commit through the concurrency-checked assignment base resource.
  Once a task has a session or contribution, reject outcome, dependency, and
  base-revision changes so its captured execution and review context cannot be
  silently invalidated.
  `orchestration_workflow_test.go` is the black-box regression boundary for the
  complete proposal-to-integration lifecycle. It proves that a queued task
  contribution reconciles the proposal plan before dependent human or agent
  work can start, and retains discussion, assignment, session, review, check,
  queue, and Git attribution through both merges.
  Pull requests are durable records beneath `$PULL_REQUEST_ROOT` (default
  `apps/api/data/pull-requests`) owned by the `pullrequests.Store` boundary.
  Creation resolves existing source and target branches through storage and
  snapshots both commit IDs; later branch movement must not rewrite that
  represented state implicitly. After publishing review follow-up commits, the
  pull request author may explicitly synchronize the represented source commit;
  prior reviews remain tied to their evaluated commits and become stale. The API
  derives source-only commits and recursive file
  changes from those snapshots through `GraphStore`; text changes include a
  readable patch while binary changes retain object and mode metadata. Pull
  request discussion is append-only and attributable by stable user ID. Pull
  requests may link a repository proposal, use stable author IDs, and begin in
  the `open` lifecycle status. Each repository participant may hold one current
  `approve` or `request_changes` review per pull request, replace or withdraw
  it, and reviews retain the exact source commit evaluated so the API can mark
  them stale when the live source branch moves or disappears. Apply the same
  repository visibility and participant policy to pull request reads and
  mutations rather than reading their files directly. Pull request readiness
  is a caller-aware, read-only report: it requires a current owner approval,
  rejects current change requests, verifies that both branches exist and that
  the source still names the snapshotted commit, checks the live target for Git
  merge conflicts without writing repository objects, and identifies that only
  the owner has merge permission. Only the repository owner may merge a ready
  pull request. A merge creates a two-parent commit through the storage object
  boundary, advances the target branch, records the merge commit, actor, and
  time, appends an attributable outcome comment, and closes any linked proposal.
  Merge messages retain pull request and proposal IDs plus stable author and
  maintainer IDs so repository history preserves collaboration context.
  Candidate verification is defined by the exact revision's
  `.komodo/checks.json` manifest (schema version `1`). Each named check declares
  a shell command plus optional working directory, timeout, and environment.
  Opening a pull request, explicitly synchronizing its source, or publishing an
  agent revision automatically creates commit-bound runs beneath
  `$CHECK_RUN_ROOT` (default `apps/api/data/check-runs`). Checks execute in a
  Bubblewrap namespace containing only a writable materialization of that Git
  snapshot, read-only system runtime files, and disposable `/tmp`; it has no
  repository metadata, credentials, host filesystem, or network namespace.
  Preserve this exact-commit and bounded-access contract when adding reruns or
  merge requirements. Run state is repository-policy readable at the pull
  request's `/check-runs` collection. Each run retains an ordered evidence
  stream of status transitions, stdout/stderr chunks, command outcome, and
  declared artifacts. Consumers reconnect through the run's
  `/events?after={sequence}` resource and download artifacts through their
  run-scoped artifact resource; both use ordinary repository read policy.
  Check attempts are investigated in the pull request's `section=checks` web
  view. Authenticated repository participants may cancel queued or running
  attempts and rerun terminal attempts. A rerun copies the original exact
  commit and check definition into a new durable attempt, while requester and
  canceler stable user IDs remain visible on the run and its evidence stream.
  Repository owners select required check names per target branch through the
  repository required-checks resource. Readiness and merge both evaluate the
  newest attempt for each required name against the pull request's exact source
  commit; a result for any other commit is stale, and missing, queued/running,
  failed, canceled, or stale requirements block publication. Preserve the
  readiness response's target branch, evaluated commit, requirement names,
  attempt identities, and attempt commits so clients can explain the decision.
  Repository owners may additionally protect a target branch with its durable
  integration-queue policy. The policy declares bounded concurrency and whether
  a future failed candidate pauses the queue or is removed. Protected branches
  reject direct merge; only pull requests that satisfy the ordinary current
  readiness report may be admitted, in order, with exact source and target
  revisions captured beneath `$INTEGRATION_QUEUE_ROOT` (default
  `apps/api/data/integration-queue`). Queue admission does not weaken or replace
  review, permission, conflict, or required-check policy.
  Admission materializes an immutable two-parent integration candidate in the
  target repository without advancing a reference. Its first parent is the
  latest eligible target commit and its second parent is the pull request's
  exact reviewed source commit; cross-repository source objects are linked into
  the target before construction. Required checks run against that candidate
  commit, not the source snapshot. Queue entry reads derive `verifying`,
  `blocked`, or `passed` lifecycle state from candidate-bound attempts and
  expose the candidate/base/source IDs plus the same run IDs used by public
  check logs and artifacts. The FIFO head advances the target with an atomic
  compare-and-swap only after its current candidate passes. Remaining
  candidates whose base is displaced are rebuilt and rechecked; superseded
  attempts stay readable but cannot authorize publication. Live source
  movement removes only that queued snapshot. Target movement rebuilds clean
  candidates, while conflicts and failed or canceled checks block or remove
  entries according to branch policy. Reconciliation is completion-driven and
  periodically recovered after restarts.
  Integration coordination is a shared repository workflow at `view=queue`,
  with the target branch in `ref`. Queue reads retain completed outcomes,
  candidate generations, check-attempt evidence, operator events, blockers,
  and the next scheduled action. Owners may reprioritize, pause/resume, retry,
  or remove active entries through the entry resource; keep those operations
  attributable and preserve prior candidates and check runs. Automated blocked,
  removed, and merged outcomes create pull-request-linked inbox activity for
  the contributor.
  `integration_queue_workflow_test.go` is the black-box regression boundary for
  parallel human and agent publication through the protected-branch queue. It
  proves policy-order landing, evolved-target rechecks, failed-change isolation,
  continued integration, and retained candidate evidence and attribution using
  public HTTP surfaces and stock Git.
  Pull-request change sessions live beneath `$CHANGE_SESSION_ROOT` (default
  `apps/api/data/change-sessions`) behind the `changesessions.Store` boundary.
  Any authenticated repository participant may start one on an open pull
  request; reads follow ordinary repository visibility. Sessions snapshot the
  represented source commit, initiator, state, and ordered events so public API
  clients can reconnect without reading worker storage or execution logs.
  A failed check on the pull request's current source revision may seed a
  change session directly. That session immutably snapshots the failed commit,
  copied check definition, ordered logs, outcome, and artifact identities;
  preserve this evidence link when evolving agent delegation. Artifact bytes
  remain owned by the originating check run and its repository read policy.
  `verification_workflow_test.go` is the black-box regression boundary for the
  complete verify-repair-merge loop. It uses stock Git, public check evidence,
  evidence-backed delegation, agent publication, required-check readiness,
  review, and merge, and proves that failed and successful attempt evidence
  remains available after publication.
  Release candidates are durable immutable source definitions beneath
  `$RELEASE_ROOT` (default `apps/api/data/releases`) owned by the
  `releases.Store` boundary. Authenticated repository participants create one
  from an exact verified commit, a repository-unique version, release notes,
  and an optional prior release that must be its ancestor. The API derives and
  snapshots merged pull requests in that history delta plus their linked
  proposals, tasks, and stable contributor IDs; clients must not supply this
  attribution. Reads follow repository visibility, while later build and
  promotion workflows should evolve the candidate lifecycle without replacing
  its captured source or inclusion definition. A candidate requires the exact
  revision's `.komodo/releases.json` manifest (schema version `1`). Its ordered
  build definitions declare a name, command, optional working directory,
  timeout, environment, artifact paths, and dependencies on earlier steps.
  Builds execute sequentially in the same Bubblewrap isolation contract as
  checks but retain release-scoped attempts, logs, outcomes, SHA-256 artifact
  metadata, source revision, initiating actor, and commands as a public
  attestation. Failed or successful steps may be rerun against their original
  immutable definition and commit without replacing earlier evidence. The web surface is the
  shareable `view=releases&release={id}` repository tab.
  Verified release artifacts may be published as immutable package versions
  beneath `$PACKAGE_ROOT` (default `apps/api/data/packages`). Only the repository
  owner may publish. The server derives the `@owner/package` identity, requires
  every latest release build to have succeeded at the exact release commit, and
  copies plus verifies the selected artifact before atomically exposing its
  version record. Package provenance retains the release, source commit, exact
  build attempt and command, artifact checksum, platform and dependencies,
  publisher, visibility, and active lifecycle. Public packages require a public
  source repository; private package reads and downloads follow repository
  participant policy. Immutable publisher-authored documentation is checksumed
  with the version. Anonymous `/packages` search and inspection expose only
  public versions; the `/packages` web catalog presents documentation,
  compatibility, dependencies, lifecycle, checksums, source, release, and build
  evidence. A participant may issue a 24-hour install-only credential at a
  consuming repository's `/package-credentials` resource for an explicit set of
  version IDs they can already read. The credential is bound to that consumer,
  carries only `package:read`, and the npm-compatible `/package-registry`
  metadata and artifact endpoints must never resolve unlisted private versions
  or confer publisher/repository authority. The release web view remains the
  publication surface. Package attestations retain Go-style platform names,
  while npm registry metadata translates architecture compatibility (`amd64`
  to `x64`, `386` to `ia32`) for standard-client enforcement.
  Dependency inventories are immutable, actor-attributed snapshots beneath
  `$DEPENDENCY_INVENTORY_ROOT` (default `apps/api/data/dependency-inventories`).
  Derive them only from an exact visible commit's schema-version `1`
  `.komodo/packages.json` direct manifest and `.komodo/packages.lock.json`
  resolved graph. Optional release, successful release-build, and successful
  deployment links must name that same commit. Preserve direct/transitive
  classification, manifest and lock digests, unavailable or mismatched package
  resolutions, and explicit provenance gaps. Repository inventory reads follow
  repository policy; anonymous package consumer reads expose only public
  consuming repositories. Published versions may retain immutable license and
  support metadata beside their provenance.
  Package safety is mutable policy layered over immutable publication evidence.
  Publisher owners append deprecation or quarantine notices with a reason and
  active same-package replacement. Quarantine blocks every new registry fetch;
  deprecation follows the consuming repository's owner policy and time-bounded
  exceptions. Preserve exact inventory/deployment exposure, targeted owner
  activity, and consumer-controlled human or Codex proposal repair tasks.
  Promotions from inventoried releases enforce the same policy.
  Consumer-owned dependency update policy and evaluation live beneath
  `$DEPENDENCY_UPDATE_ROOT` (default `apps/api/data/dependency-updates`). Owners
  bound each direct package to patch, minor, or major updates on a target branch;
  an authenticated repository writer explicitly evaluates an immutable inventory.
  Eligible active readable releases create an attributable proposal and ready
  delivery task containing proposed manifest/lock documents, publisher release
  and build provenance, checksum, compatibility caveats, and affected dependency
  paths. Repeated evaluation of the same base/package/candidate is idempotent.
  Assignment, scoped agent execution, contribution, checks, review, queueing,
  release, and deployment then use the ordinary proposal workflow; package
  publication and update discovery never confer consumer repository authority.
  Package repair and exposure reads derive live remediation status from the
  linked proposal task (`open`, `in_progress`, `in_review`, `remediated`, or
  `closed`) and retain its contribution pull request; do not add a second
  mutable recovery status that can diverge from ordinary delivery state.
  Organization identities and accepted membership live beneath
  `$ORGANIZATION_ROOT` (default `apps/api/data/organizations`). Organization
  invitations grant no repository access until accepted; removal revokes the
  member's portfolio collaborator grants. Repositories created in or transferred
  to an organization retain their storage ID, Git objects, refs, timestamps,
  upstream lineage, and linked collaboration/delivery evidence. Ownership
  transfers are durable pending records and the receiving user or organization
  owner must explicitly accept before the catalog ownership pointer changes.
  Organization portfolio reads join repositories, packages, open pull requests,
  releases, and unresolved incidents from their authoritative stores rather than
  copying lifecycle state into the organization record. The organization ID is
  the public repository owner identity; `administrator_id` identifies the human
  owner currently exercising repository-owner policy on the group's behalf.
  Organization teams are nested durable resources on the organization record.
  Owners create teams and approve agent identities; team maintainers manage
  their team and descendants. Team invitations require acceptance, every
  mutation appends actor-attributed organization evidence, and team versions are
  optimistic-concurrency tokens. Responsibilities bind a team to one
  organization repository plus a free-form area. The public organization
  directory exposes only public teams, responsibilities, approved agents,
  accepted effective members, and the nested path explaining membership;
  authenticated members may also inspect internal entries and pending
  invitations. Removing an organization member removes their team memberships
  and approved-agent operator links. Owners assign viewer, contributor,
  maintainer, or operator roles to a team or approved agent across explicit
  repository, package, environment, and collaboration resource IDs for at most
  30 days. Grants retain explicit denied-action exceptions and a reason;
  members request elevation through an owner-approved, audited request. Effective
  access is derived through accepted nested-team membership or approved-agent
  operation. Contributor-or-higher repository grants may mint at most 24-hour
  branch-only Git credentials; grant or membership revocation invalidates only
  the credentials derived by the affected principal. These scoped roles do not
  silently add repository collaborators or replace existing owner policy.
  Organization governance baselines are immutable policy versions in the
  organization record. Rules cover repository visibility, reviews, required
  checks, integration, release provenance, dependency use, environment
  promotion, and agent authority, and target the organization, a repository, or
  a team's declared repository responsibilities. Draft preview is read-only;
  activation supersedes only the prior version in the same lineage and does not
  rewrite captured work. Effective-policy reads retain every inherited required
  rule and annotate an approved exception instead of removing the rule.
  Repository-scoped maintainers and operators may request owner-approved,
  actor-attributed exceptions bound to an exact policy version and rule for at
  most 30 days; pending, denied, and expired requests never weaken the baseline.
  Portfolio initiatives are organization-owned coordination records built from
  verified existing proposals, evolution plans, incidents, or security work.
  Their ordered items retain repository-scoped source and contribution links,
  dependencies, accountable teams, accepted humans, or approved agents,
  upcoming releases, policy exceptions, and the next decision. Initiative
  reads derive dependency blockers and reassignment needs from current
  membership, repository ownership, and active repository-scoped role grants.
  Organization membership or team responsibility alone is not repository
  authority; non-owner humans inherit authority through accepted teams, while
  teams and approved agents need their own current grant. Never erase an
  invalid assignment or copy the lifecycle state of its authoritative source.
  `organizations.TestOrganizationGovernanceCollaborationLoop` is the regression
  boundary for composing teams, approved agents, grants, policies, exceptions,
  initiatives, delivery links, and membership-loss reassignment without losing
  evidence or attribution.
  Versioned cross-repository relationships live beneath `$RELATIONSHIP_ROOT`
  (default `apps/api/data/relationships`). A repository participant publishes
  an interface only from an existing immutable release, which binds its name,
  semantic version, optional schema path, release, exact commit, owner, and
  actor. Consumer declarations bind an exact release or commit to a readable
  provider repository, interface name, and an exact, caret, tilde, minimum, or
  wildcard compatibility constraint. Read the caller-filtered graph through
  `/repositories/{repository}/relationships`; it joins existing release and
  deployment evidence and reports resolved, stale, and unresolved edges
  explicitly. The shareable web surface is `view=relationships`.
  Potentially breaking provider proposals and pull requests become durable
  evolution plans beneath the same relationship boundary. A plan snapshots the
  candidate and released predecessor schema digests, readable affected consumer
  declarations and owners, classified changes, migration strategy, ordered
  owner work, exceptions, and owner acknowledgements. Its delegated analysis
  credential is shown once, expires after 24 hours, and permits only exact blob
  reads from explicitly selected readable repositories plus attributable
  findings and uncertainty; never accept it on repository mutation, Git,
  proposal, pull-request, release, or deployment resources.
  Evolution migration tasks connect provider and affected-consumer work to that
  plan with target versions, completion criteria, dependencies, discussion,
  assignment, exact base/head revisions, change sessions, and pull requests.
  Provider collaborators define tasks, but assignment and branch creation
  require write authority in the named work repository or independently owned
  fork. A linked pull request must be merged before the task completes and
  dependent work becomes ready; never infer consumer write authority from plan
  authorship.
  Cross-repository verification is defined by the provider candidate's
  `.komodo/evolution-checks.json` manifest (schema version `1`). Contract and
  integration checks run against an immutable matrix of exact linked pull-
  request source revisions beneath `repositories/{repository_id}` with no Git
  metadata, credentials, host filesystem, or network. Plans retain every
  attempt, matrix, log, artifact checksum, and actor; a moved task head
  supersedes only evidence containing that revision, and passing evidence is
  attested only while every captured task head remains current.
  A provider owner may govern an attested evolution through ordered rollout
  phases. Each participating repository owner approves only their repository,
  and phase outcomes link existing queue, release-build, deployment, rollback,
  or repair resources whose state is derived server-side. Failures pause the
  affected phase and retain safe prior outcomes; later phases stay blocked.
  Never use evolution-plan authority to execute repository or environment work,
  and never trust caller-supplied compatibility or rollout success state.
  `evolution_workflow_test.go` is the complete cross-repository regression
  boundary. It retains consumer discovery, read-only agent findings,
  independently authorized human and agent migration work, fork provenance,
  exact revision matrices, owner approvals, progressive rollout, failure
  containment, rollback, and final cutover as one attributable workflow.
  Governed delivery is stored beneath `$DEPLOYMENT_ROOT` (default
  `apps/api/data/deployments`). Owners define ordered environments with
  commands, scoped configuration, encrypted write-only credentials, required
  independent approvals, and concurrency limits. Participants promote one
  artifact from a currently verified release build; deployments retain the
  exact release, source commit, build attempt, artifact checksum, initiator,
  approvers, state transitions, and redacted logs. Never return environment
  secret values or replace this evidence when adding health and rollback.
  Exact release commits define rollout policy in `.komodo/deployments.json`
  (schema version `1`): named environments contain ordered stages, and every
  stage has repository-defined shell health signals. Promotion validates the
  matching environment policy before reserving concurrency. Deployment events
  retain stage and signal outcomes, redacted logs, affected revision, and
  attributed pause, resume, cancel, or manual-failure decisions. Paused
  deployments retain their concurrency slot; participant controls use the
  deployment `/control` resource and create deployment-linked inbox activity.
  Failed deployments recover through their `/recovery` resource. A rollback
  selects the newest earlier successful deployment in the same environment and
  creates a new governed deployment of that exact artifact, preserving
  approvals, concurrency, health evidence, and attribution. An agent repair
  creates a draft pull request and branch at the failed release commit,
  snapshots only redacted deployment evidence into its change session, and
  issues an ordinary 24-hour branch credential. It receives no environment
  credentials or deployment authority; publication must proceed through
  checks, review, integration, and a separately defined release. After agent
  publication synchronizes that draft to the pushed revision, its author must
  explicitly request review through the pull request's `/request-review`
  action; this preserves the exact snapshot while removing the draft blocker.
  `release_delivery_workflow_test.go` is the black-box regression boundary for
  the complete merge-to-recovery loop, including governed rollback and a
  reviewed agent repair delivered as a corrected release.
  Incidents are durable coordination records beneath `$INCIDENT_ROOT` (default
  `apps/api/data/incidents`) owned by the `incidents.Store` boundary. An
  authenticated repository participant may declare one manually or from an
  exact failed deployment health-signal event. Incidents retain severity,
  current lifecycle status, affected repository/environment pairs, commander,
  operations, and communications roles, followers, acknowledgements, and an
  immutable actor-attributed timeline. Role holders and the declaring actor
  must be participants in the affected scope; affected environments and source
  signals are resolved through repository and deployment stores. Updates are
  explicitly audience-labelled `participants` or `public`, while incident
  reads continue to follow the anchor repository's ordinary visibility policy.
  The shareable web surface is `view=incidents&incident={id}`.
  Incident investigations attach durable pointers to time-bounded deployment
  logs, health signals, deployments, releases, commits, pull requests, and
  prior incidents. Observations, hypotheses, queries, and conclusions cite
  those evidence IDs and retain author, creation time, reproducible query text,
  and `participants` or `public` audience. Validate every source through its
  owning store and repository policy; anonymous public reads must redact all
  participant-only evidence, findings, timeline entries, follower state, and
  acknowledgements rather than exposing responder context.
  The declaring actor or an assigned incident role holder may start a bounded
  delegated investigation with selected incident evidence IDs, exact verified
  repository commits, a mandate, and allowlisted affected deployment-log or
  health-signal reads. Its 24-hour credential is an incident-worker credential,
  not a general API or Git grant: accept it only on the investigation context,
  operational-read, and progress-record resources, never on deployment
  controls, environment or secret management, repository writes, or Git.
  Agent findings, tool actions, questions, and uncertainty append to the
  participant timeline with agent attribution. Responders may guide, pause,
  resume, or cancel; paused publication is rejected and cancellation revokes
  the credential. Never expose the persisted credential digest, and keep
  delegated sessions out of anonymous public incident views.
  Incident mitigations turn evidence into an explicit responder decision.
  Proposals name an affected environment, exact evidence IDs, an intervention
  type (`pause_rollout`, `restore_release`, or `emergency_repair`), and exact
  deployment health events that define recovery. Keep discussion, independent
  approval or commander-attributed overrides, rejection, every execution
  attempt, governed deployment or pull-request links, and verification actors
  durable in the incident. Incident execution must reuse deployment controls,
  governed rollback, and ordinary draft repair/review workflows; it never
  grants broader environment, secret, repository, or Git authority.
  Incident resolution requires an attributed impact summary, condensed
  timeline, contributing factors, cited conclusion findings, and at least one
  linked corrective proposal task with an explicit owner, mandate, exact base
  revision, and due time. Do not permit a bare transition to `resolved`.
  Reconcile each commitment through its ordinary task contribution, pull
  request, latest check, release, and deployment provenance; retain invalidated
  and overdue state in the incident and emit one actionable inbox transition
  to the owner when either first occurs.
  `incident_response_workflow_test.go` is the black-box regression boundary for
  the complete degraded-signal-to-corrected-deployment lifecycle. It proves
  public and participant communication, bounded agent diagnosis, independent
  governed recovery, health verification, resolution ownership, and ordinary
  corrective task, Git, check, review, release, and deployment provenance while
  asserting that incident-worker and anonymous access remain least-privileged.
  Meaningful collaboration transitions are appended through the
  `activities.Store` boundary beneath `$ACTIVITY_ROOT` (default
  `apps/api/data/activities`). Repository activity retains stable actor and
  target-user IDs, typed affected resources, and event-specific metadata;
  resolved `@handle` mentions become separate stable-ID events. Read activity
  through the repository-scoped endpoint so visibility and membership policy
  remains aligned with the affected repository.
  The authenticated global inbox is derived from that ledger at `GET /inbox`:
  only events implying review, response, or awareness for the current actor are
  included, resolved collaboration is not retained as stale action, and every
  item links to its repository resource. Per-user clearance state lives beneath
  `$INBOX_ROOT` (default `apps/api/data/inbox`) and is changed through
  `DELETE /inbox/{eventID}` without mutating the append-only activity ledger.
  Private vulnerability reports are global security resources beneath
  `$SECURITY_REPORT_ROOT` (default `apps/api/data/security-reports`), not
  repository activity. Authenticated reporters may name public repositories or
  private repositories they can read, affected versions, bounded evidence, and
  a safe contact channel. The reporter and affected-repository owners receive
  access automatically; those owners alone set severity and embargo state and
  grant or revoke up to 20 responder seats. Every view, access change, triage
  decision, and message appends to the report-private audit ledger. Keep only
  titles, scope identities, and status in the caller-specific collection;
  exact authorized reads own contact, summary, versions, evidence,
  conversation, and audit detail. Never mirror pre-disclosure report data into
  repository activity, search, inbox, incidents, or ordinary notifications.
  Advisory diagnosis stays inside that embargoed resource. Responders connect
  typed commit, dependency, build, release-artifact, deployment, and supported-
  version evidence; hypotheses and conclusions cite stable evidence IDs, and
  the version-by-environment matrix records confirmed, suspected, unaffected,
  or fixed state with actor and rationale. A responder may delegate selected
  evidence to a 24-hour read-only investigation. Its credential reads only the
  safe advisory header and selected evidence and appends findings, tool notes,
  questions, and uncertainty; it has no repository, Git, build, release,
  deployment, conversation, team, or general advisory access. Cancellation
  immediately revokes it, and responses never return its stored digest.
  Embargoed vulnerability repairs remain owned by the security report rather
  than ordinary proposals, pull requests, activity, or inbox. Each repair
  targets one declared affected repository/version and exact base commit, may
  depend on another report repair across repositories, and uses an opaque
  `refs/heads/embargo/*` branch. Normal Git advertisement and repository browser
  reads hide those refs and commits, including from repository owners. Human or
  agent repair sessions receive a one-time 24-hour Git credential limited to
  the exact repository and embargo branch; human assignees must independently
  retain both response-team and repository-participant access. Private session
  records retain messages, status, published commit IDs, and commit-bound
  approve/request-changes decisions with stable authorship. Session revocation
  and response-team removal revoke matching Git grants immediately. Keep all
  repair collaboration inside the advisory until the later disclosure workflow
  deliberately publishes it.
  Exact-candidate repair verification is also report-private. Its ledger binds
  required-check and security-reproduction attempt summaries, definition
  digests, approvals, gaps, protected-integration identity, and release
  artifact checksums to the current embargo branch tip. Never store or return
  private reproduction commands, fixtures, or logs in the advisory. Integration
  requires all recorded gates to pass, both gate classes to be represented, a
  current approval, and no gaps; release attestations must cover the repair's
  affected version line.
  Security disclosure is prepared only after every repair is integrated and
  release-attested. The plan freezes redacted public text, guidance, credits,
  schedule, and ordinary refs. Publication creates every exact repaired ref
  before lifting the embargo; a failed attempt rolls back its new refs and
  retains private remaining-work state. Anonymous security-advisory reads must
  never expose reporter identity, contact, evidence, embargo refs, findings,
  sessions, impact rationale or citations, internal report or repair IDs, or
  audit records. `security_remediation_workflow_test.go` is the black-box
  regression boundary for the complete confidential-report-to-public-upgrade
  lifecycle. It uses public HTTP and stock Git to prove supported-line human
  and agent repairs, exact-candidate verification and attestations, atomic
  disclosure, and negative pre-disclosure checks across Git advertisement,
  exact-commit browsing, public advisory, and ordinary activity surfaces.
  Reproducible development workspaces are durable records beneath
  `$WORKSPACE_ROOT` (default `apps/api/data/workspaces`). Exact commits define
  `.komodo/workspaces.json` schema version `1`, including declared tool
  versions, dependencies, ordered setup commands, and CPU, memory, disk, and
  setup-time bounds. Authenticated repository writers may launch from a
  repository, proposal task, same-repository pull request, or incident emergency
  repair only when the named source context resolves to that exact revision.
  Setup materializes the immutable snapshot and runs without network,
  credentials, Git metadata, or host filesystem access in Bubblewrap while
  retaining command, output, outcome, creator, access, context, definition
  digest, and lifecycle evidence. Suspension retains the materialized
  foundation; resume revalidates the captured definition at the captured commit
  and must never resolve a moving branch or rebuild silently. The shareable web
  surface is `view=workspaces&workspace={id}`.
  Ready workspaces expose path-safe file browse/edit/delete, bounded text search,
  and credential-free Bubblewrap commands through repository-scoped APIs.
  Mutations require repository write access; retained file evidence records
  actor, path, deletion, and digest, while command evidence records actor,
  revision, and outcome without terminal input or output. Optional schema-version `1` `ports`
  entries map a port number and label to a relative static-output directory;
  previews serve declared text assets through repository read policy with
  restrictive CSP and the workspace revision header. Never turn previews into
  host-network forwarding or inject general repository credentials into setup,
  commands, snapshots, logs, or preview responses.
  Ready workspaces are shared collaboration records: participant presence is a
  renewable 45-second lease, while discussion, file authorship, command
  execution metadata, control grants, and interventions are durable ordered
  activity. Human control targets must remain repository participants and agent
  targets must be approved by the repository-owning organization. Grants use
  explicit observe/edit/execute modes, files/terminal/preview scopes, and a
  version concurrency token for guide, pause, resume, take-over, and revoke.
  File saves may carry the digest returned by the file read and conflict if the
  live file changed. Never place raw terminal commands, stdin, stdout, or stderr
  in shared workspace activity; only the invoking response may contain them.
  Workspace checkpoints capture only explicitly declared repository paths as
  content-addressed changes against the immutable workspace revision. They
  retain creator, parent lineage, environment definition, dependency and
  reproduction declarations, readable diffs, and safety status. Reject
  credential-like paths/content, setup output, terminal streams, symlinks, and
  unrelated runtime files. Restore must preflight the base, definition, and
  every changed path and reject divergence instead of overwriting it.
- **Docs** — `docs/README.md` records decisions once they're made, not before.
  Update it when you change how the apps fit together, not for every change.

## LADDER.md

`/LADDER.md` at the repo root is **not part of this repository**. It is placed
there from outside the checkout — as a symlink or a read-only bind mount — and
is gitignored for that reason. Read it for context on what is being built, but
never edit it, `git add` it, or delete it, and do not treat its presence as
uncommitted work.
