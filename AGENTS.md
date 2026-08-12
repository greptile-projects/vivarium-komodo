# AGENTS.md

## Delivery team operating contracts

Repository documentation collections live beneath `$DOCUMENTATION_ROOT`
(default `apps/api/data/documentation`). Owner-published immutable versions bind
ordered pages beneath one repository path to exact source commits, supported
version labels and optional releases, participant owners, audiences, navigation,
rendering, publication policy, links, author, and change reason. Reads resolve
page blobs and commit authorship only from that reviewed revision and explicitly
report missing ownership, broken sources, rendering mismatches, and stale release
mappings. The shareable web surface is `view=documentation&ref={revision}`.
Documentation tasks extend a collection through repository-scoped
`/documentation-tasks` resources. Repository writers open one from a proposal,
issue, pull request, release, code investigation, or stewardship opportunity at
an exact visible commit, choosing an explicit scoped branch or an ordinary
shared workspace. Tasks retain the collection version, origin, revision,
preloaded evidence, page path, creator, and attributable append-only drafts,
discussion, and suggestions. Code references are server-resolved to exact blobs
and bounded line excerpts; suggestions require citations and retain declared
uncertainty. Workspace edits continue through the workspace's own access and
agent-control boundary, and task publication grants no additional authority.
Pull-request documentation previews snapshot a collection's rendered page blobs,
navigation differences, verified examples, affected versions, and gaps at the
exact source revision. Technical and audience invitations, bounded comments,
and area decisions retain their exact content subject; synchronization derives
staleness only from changed previewed page blobs and reports affected paths.
Repository documentation verification is declared at the exact pull-request
source revision in version-1 `.komodo/documentation-checks.json`. Each named
check identifies its collection, kind (`links`, `symbols`, `build`, `sample`,
`command`, or `tutorial`), bounded command, declared documentation/code inputs,
pages, exact source/package/release version matrix, optional link and symbol
coverage, expected output, and retained artifacts. It executes through the
ordinary credential-free, networkless check sandbox and appears beneath public
pull-request check-run resources with logs, build products, and output diffs.
Successful evidence is content-addressed by only the declared input blobs:
unrelated commits may reuse it with explicit prior-run provenance, while any
affected input queues a fresh attempt. Branch protection can require these
ordinary named checks, so documentation evidence participates in existing
readiness and stale-commit rules without a separate merge authority.
Merged exact-revision previews publish immutable reader editions beneath the
collection store. Editions retain pull request, preview, source and merge
revisions, rendered blobs, version/release mappings, audiences, redirects, and
publisher. Readers get stable links, search, redirects, release selection,
visibility, and explicit archives; their bounded redacted feedback is triaged
once into a linked issue, proposal, or documentation task.
`documentation_workflow_test.go` is the black-box regression boundary for the
complete code-to-trusted-guidance loop. It retains proposal and grounded draft
authorship, documentation check matrices, rendered review, ordinary merge and
release provenance, archived reader editions, redacted older-version feedback,
and an agent-authored version-specific repair published through the same policy.

Delivery teams are pre-execution contracts beneath `$DELIVERY_TEAM_ROOT`
(default `apps/api/data/delivery-teams`). A repository collaborator forms one
around a proposal, initiative, decision, incident follow-up, or other planned
outcome. Immutable charter versions retain measures, principles, total budget,
deadline, escalation, author, and change reason. Human and organization-approved
agent invitations retain complementary roles, rationale, responsibilities,
individual budgets, deadlines, escalation, acceptance, replacement, and removal.
Effective-access previews derive only from existing repository roles or current
organization agent grants and are intersected with the organizer's own authority;
teams never issue credentials or grant authority. Team plans decompose the
outcome into immutable revision-bound work-stream versions with accepted owners,
inputs, expected artifacts, dependencies, criteria, repository paths, budgets,
assumptions, and integration order. Derived blockers expose overlap, missing
access, budget overflow, incompatible dependency order, and stale upstream
assumptions; every materially revised plan requires its affected owners to
accept again. Shared repository scopes at incompatible commits are blockers as
well. Mutations use optimistic concurrency and append attributable
events. The shareable repository surface is `view=teams&team={id}`; plans remain
pre-execution contracts and do not start work or issue credentials.
Accepted work-stream owners may attach an existing change session,
investigation, decision experiment, or shared workspace only at a repository
revision already accepted in that stream's scope. Resolve the named resource
through its established store and reject missing or revision-mismatched
contexts. Their team timeline accepts
cited findings, questions, checkpoints, artifacts, decisions, and residual
uncertainty; every citation must stay within the stream's exact repository,
commit, and path scope and within the team repository's read-policy boundary.
Structured handoffs retain immutable timeline input
IDs, their exact revisions, context, stream revision, author, recipient,
acceptance criteria, declared uncertainty, and the recipient's attributable
verification. Never infer or copy private terminal input, credentials, hidden
prompts, raw execution logs, or evidence outside the accepted scope.
Accepted stream owners publish safe live checkpoints with captured revision,
status, active action, question, predicted next action, cumulative resource use,
and coarse access/output health. Derive the team runtime view from those
checkpoints and the accepted plan, including dependencies, stale revisions,
disconnection, conflicts, exhausted budgets, bounded failure recovery, and
explicit escalation. Team controls are immutable and attributable. Guidance,
pause, resume, and cancellation may target a stream or the effort; operational
reassignment must use an accepted participant with existing effective actions,
and narrowing must remain within accepted paths. Controls preserve accepted
work, never rewrite plan ownership, never expand authority, and do not bypass
the attached resource's own control or credential contract. Team integration
reconciliations bind accepted streams, exact Git tips, conflict results, cited
evidence, handoffs, criteria, and risks in integration order. Only ready
reconciliations publish ordinary pull requests and checks; review, queues,
release policy, repository permissions, and owner-only merge authority remain
unchanged. `delivery_team_workflow_test.go` is the black-box regression boundary
for the accepted-decision-to-release loop. It retains challenged and resolved
evidence, redirected agent work, a verified agent-to-human handoff, costs,
controls, review links, and release inclusion, and proves removal blocks further
team publication without erasing accepted progress.

Guidance for coding agents working in this repository.

Opportunity onboarding composes existing resources rather than granting new
authority. An active claimant starts through
`/repositories/{repository}/contribution-opportunities/{opportunity}/start`;
the API creates a private contributor-owned fork and an exact-revision shared
workspace whose source context retains the upstream opportunity, current
pathway prerequisites/setup guidance, issue or proposal evidence, acceptance
criteria, and validated repository sample-data paths. Repository-defined setup
commands remain the reproducibility boundary. Grounded questions use the
workspace context at that revision. Contributors can append typed
`missing_access`, `obsolete_instructions`, or
`non_reproducible_prerequisite` reports to the opportunity; reports must never
contain credentials or raw private execution state. Neither a claim nor a
start grants upstream write authority.
Each started opportunity also owns a collaboration ledger at its
`/collaboration` resource. The claimant, designated mentors, repository owner,
and organization-approved agents use renewable presence, typed questions,
checkpoint requests, advice, handoffs, interventions, availability, and a
declared response deadline without making mentors members of the private fork.
Every entry retains actor role and decision owner. Contributor- or owner-issued
agent controls are explicit `explain`, `diagnose_setup`, or exact-path `edit`
modes; pause or revocation immediately blocks later actions, and edits flow
through the existing workspace mutation boundary with agent authorship. Scope
change, inactivity, or lost availability ends in an attributable
`reassignment_required` or `exited` transition that revokes controls while
preserving the thread and legitimate workspace progress.
An opportunity claimant publishes a safe workspace checkpoint through the
upstream opportunity's `/publication` action. Preflight requires the exact
current pathway acknowledgement, the opportunity revision and workspace,
reproducible checkpoint evidence, and satisfied evidence for every acceptance
criterion; its response separates blocking project requirements from
non-blocking coaching needs. Publication writes only the verified checkpoint to
the claimant's fork and opens an ordinary cross-repository pull request against
the upstream repository. The pull request retains the opportunity, pathway
version, setup commands and dependencies, mentor guidance, agent assistance,
criteria evidence, workspace/checkpoint, and exact contributors. From that
point ordinary discussion, review, checks, reproduction, acknowledgements,
queue, and permission policy remain authoritative.
After that exact guided pull request is merged and included in a release, the
repository owner records one immutable opportunity outcome containing explicit
contributor credit, maintainer feedback, measured support hours, readiness for
later work, and an optional next opportunity. Release inclusion and the guided
pull-request context are server-verified; the assessment grants no membership
or authority and remains visible on the Contribute surface.

Contributor participation guidance lives beneath `$CONTRIBUTOR_PATHWAY_ROOT`
(default `apps/api/data/contributor-pathways`). A repository owner publishes
immutable, concurrency-checked pathway versions containing goals,
prerequisites, conduct and security guidance, supported setup, communication
and review expectations, and outside work categories explicitly suitable for
humans, agents, or both. Typed references connect documentation, current
ownership, releases, issues, proposals, and workspace definitions; reads
derive current, stale, or inaccessible health from repository state rather
than trusting the publication snapshot. Public repository pathways are
anonymous-readable, acknowledgements are attributable and version-bound, and
the shareable web surface is `view=contribute&ref={revision}`. A pathway grants
no repository, Git, agent, review, or merge authority.

Ready newcomer work lives beneath `$CONTRIBUTION_OPPORTUNITY_ROOT` (default
`apps/api/data/contribution-opportunities`). Repository owners expose an
opportunity only by naming an existing triaged issue, open proposal, ready
unassigned proposal task, or current stewardship finding; the API derives its
title, readiness state, and exact revision from that source. The owner adds
required skills, interests, expected outcome, bounded scope, dependencies,
risk, mentors, and available human or agent assistance. Authenticated readers
save interests, skills, time/risk constraints, and assistance preferences, then
inspect deterministic match scores with both reasons and gaps. Claims are
exclusive, attributable, and expire after at most fourteen days; only the
claimant may release one early. Profiles, matches, and claims require read
access only and explicitly grant no repository, Git, agent, review, or merge
authority. The shareable discovery surface remains
`view=contribute&ref={revision}` alongside the contributor pathway.

Ordinary unexpected-behavior reports live beneath `$ISSUE_ROOT` (default
`apps/api/data/issues`) and are owned by the `issues.Store` boundary. An
authenticated repository reader may open one against the repository or an
existing release, with required expected and observed behavior, severity,
environment, and ordered reproduction steps. Evidence is limited to typed
logs, screenshots, traces, and sample inputs: at most ten attachments, one MiB
each and five MiB total. Reporters choose public or repository-participant
visibility; list summaries omit evidence bodies and duplicate suggestions are
computed only from issues already visible to the caller. Discussion and status
changes append attributable history, and only the reporter or repository owner
may close or reopen a report. The shareable web surface is
`view=issues&issue={id}`; later reproduction and resolution workflows should
extend these issue resources rather than copying evidence into another store.
Issue reproduction attempts extend that same resource beneath
`/issues/{issue}/reproductions`. Reports may pin an exact visible source commit
or an attested release commit. The reporter or repository owner launches one
repository-declared `.komodo/reproductions.json` command with explicit sanitized
inputs; execution materializes only that commit and runs in credential-free,
networkless Bubblewrap with bounded CPU, memory, disk, time, logs, and artifacts.
Attempts immutably retain the environment definition and digest, revision,
release, initiator, inputs and checksums, command outcomes, observed result,
failure reason, and artifacts. Repository participants may inspect failed
attempts and create attributable reruns from the exact retained definition and
inputs. Reject credential-like inputs and artifacts and redact credential-like
log output rather than retaining secrets or inaccessible machine state.
Repository owners triage that same issue through its concurrency-checked
`/triage` resource with an explicit classification, priority, participant
assignees, labels, and visible duplicate target. Typed `/relationships` retain
attribution and exact code revisions while connecting dependencies, releases,
deployments, incidents, proposals, pull requests, decisions, and existing
investigations. Participants open an `/investigations` record only from an
existing reproduction attempt. Hypotheses, findings, evidence requests,
conclusions, and challenges are append-only and cite exact attempt events or
artifacts, code revisions, or issue relationships; challenges visibly dispute
their target and a newer-revision investigation marks older entries stale.
Owners may issue a 24-hour agent credential for only the selected issue and
reproduction. Its global context and entry endpoints grant no repository read,
Git, or write authority and accept only cited public investigation entries.

Confirmed repairs extend the same issue at `/repairs`. Creation requires owner
triage, a completed reproduction that observed the failure, a current
undisputed conclusion at that exact revision, explicit acceptance criteria, and
a human repository participant or the `codex` agent. The API creates an
ordinary proposal task and assignment whose immutable reasoning context retains
the issue, reproduction, diagnosis, revision, and criteria; assignment issues
no credential. Work continues through established task change sessions or
shared workspaces. Only a pull request published from that exact task may be
linked at `/repairs/{repair}/pull-request`; the issue grants no Git,
repository-write, review, or merge authority.

Candidate repair verification extends the linked repair beneath
`/repairs/{repair}/verifications`. Bind each attempt to the pull request's exact
source commit, replay the retained sanitized reproduction in a clean
credential-free environment, and join only required checks from that revision.
Retain definition, input, environment, acceptance-criteria, check, and outcome
fingerprints; changed manifests, inputs, check evidence, or pull request
revisions invalidate prior confirmation. Reporter confirmation or rejection is
append-only. Owner overrides require a reason and preserve reporter dissent.
Safe previews remain retained reproduction evidence under the issue read policy
and grant no execution or repository access.

After a confirmed candidate is merged, a reporter or owner may run the same
named reproduction and sanitized inputs against a newer release by supplying
its `release_id`. The release is eligible only when its server-derived pull
request inclusions contain the linked repair and the exact candidate has a
reporter confirmation or reasoned owner override. Retain this delivered-release
attempt separately; it never rewrites the affected-release failure or candidate
verification. Link the resulting release and deployment through ordinary issue
relationships before reporter-or-owner closure. `issue_resolution_workflow_test.go`
is the black-box regression boundary for the complete released-report,
request-and-retry, agent diagnosis/repair, review, merge, release, deployment,
fixed-release retest, and reporter-resolution loop.

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
	Pull request previews begin in `section=previews`. A writable repository
	participant launches the exact snapshotted source commit through the
	version-1 `.komodo/previews.json` definition. Retain the definition and
	configuration digests, declared CPU/memory/disk/build-time/lifetime limits,
	creator, logs, URL, and terminal failure on each immutable attempt beneath
	`$PREVIEW_ROOT` (default `apps/api/data/previews`). Configuration names must
	be declared by that revision; inject their launch values into only the
	isolate and never persist or return the values. Serve ready applications
	through the authenticated pull-request preview proxy. Pull request source
	synchronization makes older attempts stale on read and must never rewrite or
	restart the environment participants already evaluated.
	Preview owners may invite a named user or a validated issue, decision, or
	proposal participant to one exact attempt with an expiring `view`, `test`, or
	`feedback` role. Invitations grant only preview entry, never repository, Git,
	workspace, deployment, environment, or production authority. The manifest's
	audience policy declares network, data, identity, and action restrictions;
	the gateway strips browser credentials and identity headers, intersects role
	and action policy, and audits invitations, entry, and revocation. Keep the
	effective audience and restrictions visible without exposing configuration
	values or private services.
	Feedback-role invitees and repository participants attach findings to the
	exact preview attempt rather than detached pull-request comments. Findings
	server-capture the candidate revision and submitted route, ordered reproduction
	steps, classification, lifecycle, duplicate and related-finding links, and
	attributable discussion/history. Bounded screenshot, recording, console, trace,
	or annotation evidence remains in an `exact_preview` audience behind a dedicated
	access check; ordinary preview and pull-request JSON exposes metadata only.
	Redact credential-like textual fields and sensitive route parameters before
	persistence, and never widen evidence merely because its finding is discussed.
	Repository participants convert one exact-preview finding into immutable
	acceptance criteria and an assigned human/Codex proposal task at the observed
	revision, optionally preloading a pull-request change session or shared
	workspace with selected evidence IDs. Evidence bodies retain exact-preview
	access. Repair publication accepts only the pull request's already-current
	source revision, validates cited commits, retains commands, checks, sessions,
	workspaces, and authors, and launches a new ordinary preview attempt; it grants
	no credential and does not synchronize or otherwise mutate the source branch.
	Repository owners define preview acceptance requirements through
	`/repositories/{repository}/preview-acceptance-requirements`, selecting target
	branches and optional path globs, labeling risk classes, and naming scenarios
	with required owner, repository-participant, test, or feedback roles. Scenario
	acknowledgements and rejections bind to the exact preview revision; later pull
	request revisions expose them as stale. Owner overrides require a retained
	reason. Pull-request readiness and merge enforce applicable current scenarios
	and unresolved current blocking findings alongside ordinary reviews and checks.
	Finding evidence accepts base64 content only on creation; redact and store it
	behind exact-preview access, and clear content before metadata is persisted or
	returned. `collaborative_preview_workflow_test.go` is the black-box regression
	boundary for failed-build recovery, expiring outsider access, feedback, agent
	repair, stale/current acceptance, checks, review, merge, and release.
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
  Revision-aware code intelligence is derived on demand at
  `/repositories/{ID}/code-intelligence` from one visible immutable commit.
  The response joins supported-language definitions, references, callers,
  tests, first-parent last-touch commit ownership, and permission-filtered
  declared relationships, with source path/line evidence. Preserve the exact
  `commit_id`, analysis completeness reasons, scan bounds, embargo visibility,
  and provider repository read checks; never combine results from different
  revisions or disclose a dependency on an unreadable repository. The web
  surface is the shareable `view=intelligence&ref={revision}` repository tab.
  Grounded questions are durable resources beneath `$QUESTION_ROOT` (default
  `apps/api/data/questions`). Authenticated collaborators ask from repository,
  file, proposal, task, pull-request, incident, or workspace context at
  `/repositories/{ID}/questions`. Resolve the revision once; retain actor,
  context, exact commit, structured evidence/inference/uncertainty claims, and
  exact citations. Replay ordered results through `/events?after={sequence}`
  SSE; never regenerate old conversations at a moved branch or cite unreadable
  dependency evidence. A grounded conversation may seed a shared investigation
  only at that same exact commit; retain its `conversation_id` through the
  investigation and every reasoning-connected task, session, and pull request
  so the eventual review record links back to the original attributed answer.
  Shared code investigations live beneath `$INVESTIGATION_ROOT` (default
  `apps/api/data/investigations`) and are opened from the repository's
  `view=investigations&investigation={id}` surface. Only explicitly invited
  repository participants may read or append to a canvas; an invitation never
  grants repository access. Entries are ordered, actor-attributed code
  references, reproducible queries, bounded-workspace observations,
  hypotheses, agent findings, challenges, or conclusions. Resolve code
  citations to the canvas commit through `RepositoryStorage`, and accept a
  runtime observation only when it cites an existing event in a same-commit
  bounded workspace. Challenges and supersessions retain their target entry.
  Reruns append an exact-revision run and mark older commit-bound entries stale
  rather than rewriting them; never persist workspace output, credentials, or
  hidden agent context in the canvas.
  Prospective impact assessments live beneath `$IMPACT_ASSESSMENT_ROOT`
  (default `apps/api/data/impact-assessments`) and begin from selected code,
  symbols, an exact current investigation conclusion, or a proposed diff. Keep
  every assessment pinned to one visible commit and retain references, tests,
  last-touch ownership, packages, published interfaces, readable consumers,
  releases, and deployment environments as explicit evidence. Participants may
  refine impacts and record mitigations, accepted risks, and unknowns; affected
  owners acknowledge or raise concerns without receiving repository authority.
  Assessment agents receive only credential-bound read-only context and may
  cite retained evidence in attributable findings; they never receive Git or
  repository-write access. The shareable surface is
  `view=impact&assessment={id}&ref={commit}`.
  Consequential technical choices live beneath `$DECISION_ROOT` (default
  `apps/api/data/decisions`). Repository participants may open a pending
  decision from repository, proposal, investigation, incident, evolution-plan,
  or stewardship-opportunity context with an accountable participant owner,
  constraints, success measures, deadline, and affected resources. Scope
  changes append complete actor-attributed versions and discussion is
  append-only. Participants propose alternatives as append-only assumption,
  tradeoff, risk, compatibility, cost, outcome, and dissent claims; newer claims
  explicitly supersede older ones without deleting them. Citations retain exact
  code revisions or dependency, release, incident, and usage resource IDs plus
  observation times. The derived comparison reports missing common criteria,
  evidence-kind coverage, dissent, and evidence made stale by a newer scope.
  Decision owners request participant acknowledgements or named policy approvals;
  only the named actor responds, and pending or rejected requirements remain
  public blockers. Publication appends an immutable commitment version containing
  the selected and rejected alternatives, scope version, rationale, accepted
  tradeoffs, dissent, conditions, review date, exact retained evidence, and
  approval snapshot. Material scope or alternative evidence changes reopen a
  published decision without rewriting it. Owner-authorized exceptions remain
  linked to an exact commitment version with scope, conditions, attribution,
  expiry, and revocation state; they grant no additional authority.
  A 24-hour decision-research credential reads only its selected alternative
  and publishes findings citing retained evidence; it grants neither Git nor
  repository-write access. Use `context_kind` and `context_id` collection filters to expose
  advisory pending context on related work; a pending decision does not block
  unrelated contributions. The shareable surface is
  `view=decisions&decision={id}`.
  Decision alternative experiments are exact-revision, policy-bounded shared
  workspaces launched only with named commands from `.komodo/workspaces.json`.
  Their attributed checkpoints cite retained workspace logs, diffs, artifacts,
  measurements, and derived resource use; validity compares code, dependency,
  and environment fingerprints. Experiments carry no implicit publication,
  pull-request, or merge authority, and reproductions remain separate records.
  A published commitment may be promoted once into an ordinary proposal and
  ordered human/Codex tasks at an exact base commit. The tasks must collectively
  cover every committed constraint, success measure, and condition verbatim;
  their immutable reasoning context retains the decision and commitment version
  through sessions, workspaces, pull requests, checks, queues, releases, and
  deployments. Decision-governed pull requests require a structured delivery
  account with every assigned criterion met. Agent-authored tasks additionally
  require their scoped change session; human-authored tasks retain assignment,
  Git, pull-request, and review attribution without an agent session. Changed
  assumptions, failed measures, incompatible work, or constraint deviations
  instead append an evidence-linked revisit request and reopen the decision; do
  not silently waive them or rewrite the commitment. `decision_workflow_test.go` is the
  black-box regression boundary from genuine uncertainty through reproduced
  alternatives, approval and dissent, human/agent delivery, release evidence,
  and a post-release revisit.

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
  Standing agent stewardship is an organization-owned, immutable versioned
  mandate beneath `/organizations/{organization}/stewardship-mandates`. Each
  version names observable outcomes, repository-and-branch scope, trusted
  signals, exclusions, bounded hours/runs, cadence and expiry, one approved
  agent, allowed actions, and decisions reserved for humans. A new or revised
  version remains pending until one of that agent's current operators accepts
  it. Owners may pause, resume, or irrevocably revoke a version; its schedule
  makes expiry effective without deleting evidence. Preview joins current
  organization policy and independently granted agent roles for every scoped
  repository, but the mandate itself never issues a credential or supplies
  write or merge authority.
  Active stewardship mandates publish their shared backlog beneath
  `/organizations/{organization}/stewardship-opportunities`. Evaluations are
  accepted only from the approved agent's current operator, for a repository
  and exact signal named by that active mandate. Findings retain severity,
  expected value, confidence, affected owners and revisions, typed citations,
  and the explicit mandate-scope rationale. A stable mandate/repository/finding
  key deduplicates reevaluations while preserving collaborator discussion,
  ranking, dismissal, snooze, incorrectness, and stale-evidence decisions.
  Never move citations along with a branch: report stale evidence explicitly
  and refresh the same finding only after evaluating a new exact revision.
  Opportunity admission is controlled by immutable mandate-version work-policy
  versions. Approval and bounded auto-start use the policy version and decision
  count as concurrency tokens, and must expose stale evidence, policy/mandate
  changes, risk or budget excess, conflicting work, active incidents, and
  embargoed evidence as blockers. An accepted opportunity promotes only into a
  linked ordinary proposal and ordered tasks retaining owners, completion
  criteria, risk, verification plan, and evaluated base revision. Promotion
  creates no credential, branch, session, or implicit repository authority.
  Promotion also snapshots the opportunity, mandate version, citations, and
  affected-owner acknowledgement requirements into each task's immutable
  reasoning context. A stewarded task may publish only from its scoped change
  session and must attach review-facing implementation reasoning, commands,
  residual risks, and an exact status for every recorded completion criterion.
  The resulting pull request retains this server-attributed delivery evidence;
  ordinary checks, reviews, acknowledgements, fork policy, integration queues,
  and owner-only merge authority remain unchanged.
  Long-running stewardship history is exposed at each exact mandate version's
  `/report`: preserve recommendation disposition, implementation, verification,
  release, actual resource use, false-positive feedback, goal results, retained
  evidence citations, and automation notices. Owners may version class priority,
  minimum confidence, and required evidence kinds from that history, but these
  work-policy settings must never change mandate scope, agent identity, allowed
  actions, budgets, or authority. Those changes require a new mandate version
  and fresh operator acceptance. Repeated failures, inactivity, revoked access,
  and anomalous consumption reported by an owner or accepted agent operator
  atomically pause only the affected version and append an actionable notice;
  resumption remains an explicit lifecycle decision.
  `stewardship_workflow_test.go` is the black-box regression boundary for the
  complete proactive signal-to-release loop. It proves bounded operator-accepted
  authority, evidence-backed prioritization and dismissal, policy admission,
  guided branch-only agent work, structured delivery evidence, ordinary checks,
  review and merge, verified release attestation, outcome/cost reporting, and
  final mandate revocation through public HTTP and stock Git surfaces.
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
  Repository owners configure the workspace resource, network, idle, retention,
  expiry-notice, sharing, and approved-agent envelope at `/workspace-policy`;
  organization owners have the corresponding organization policy resource.
  Each workspace snapshots its effective envelope, caps repository-declared
  resources, and attributes command runtime consumption. Policy tightening
  marks retained environments for an explicit rebuild and immediately revokes
  newly forbidden sharing or agent controls. Idle work suspends automatically;
  retention expiry is announced before terminal expiry. Owners may stop or
  expire an environment without deleting checkpoints or publication evidence,
  and creators may download a secret-filtered archive of authored unpublished
  paths during an announced-expiry window. A changed or unavailable exact
  environment definition is a visible rebuild requirement, never a silent
  resume.
  Workspace checkpoints capture only explicitly declared repository paths as
  content-addressed changes against the immutable workspace revision. They
  retain creator, parent lineage, environment definition, dependency and
  reproduction declarations, readable diffs, and safety status. Reject
  credential-like paths/content, setup output, terminal streams, symlinks, and
  unrelated runtime files. Restore must preflight the base, definition, and
  every changed path and reject divergence instead of overwriting it.
  Checkpoint publication reconstructs Git content only from the immutable base
  and verified checkpoint blobs, then creates or base-checked advances a named
  branch. Optional pull-request creation retains workspace/checkpoint,
  proposal/task/session or originating-request context and exact contributors,
  summarizes declared commands, and starts ordinary commit-bound checks. Keep
  mutable workspace files, private command output, credentials, and activity
  outside the commit; later review, required-check, queue, and stale-revision
  behavior belongs to the established pull-request workflow. A proposal-task
  publication must also register the pull request as that task's ordinary
  contribution so queue reconciliation can mark the task merged. Terminal stop
  or expiry removes the mutable environment while retaining workspace records,
  checkpoint blobs, publications, Git commits, and review evidence.
  `workspace_collaboration_workflow_test.go` is the black-box regression for
  the complete planned-task-to-expired-environment loop across public HTTP,
  stock Git, checks, review, and protected integration.
  Validated investigation conclusions and impact items cross into delivery at
  `POST /repositories/{ID}/connected-work`. The endpoint creates one proposal
  and ordered, optionally human- or Codex-owned plan tasks while snapshotting
  the selected claim or risk, exact commit, evidence, verification paths, and
  owner acknowledgements in each task's `reasoning_context`. Carry that same
  immutable context into task change sessions and task/workspace pull requests;
  never replace it with the current branch or a later analysis run. A later
  investigation rerun marks its original entries stale while existing work
  continues to show the revision and reasoning that actually initiated it.
- **Docs** — `docs/README.md` records decisions once they're made, not before.
  Update it when you change how the apps fit together, not for every change.

## LADDER.md

`/LADDER.md` at the repo root is **not part of this repository**. It is placed
there from outside the checkout — as a symlink or a read-only bind mount — and
is gitignored for that reason. Read it for context on what is being built, but
never edit it, `git add` it, or delete it, and do not treat its presence as
uncommitted work.
