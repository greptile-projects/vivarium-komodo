# Docs

Notes on how this project fits together. Mostly empty for now — things get
written down here as they're decided, not before.

## Live workspace collaboration

Every ready development workspace is also a permission-aware shared room.
Repository participants renew presence through `POST
/repositories/{repository}/workspaces/{workspace}/presence`; the lease expires
after 45 seconds, so a lost connection disappears without manufacturing a
durable leave event. Presence reports the active files, terminal, command,
preview, or discussion surface. The workspace record retains an ordered
activity stream whose `kind` distinguishes observation, instruction,
authorship, and execution.

Participants discuss work through `/messages` and create explicit `/controls`
grants. A grant names a repository-participant human or an agent approved by
the repository-owning organization, an `observe`, `edit`, or `execute` mode,
and a nonempty subset of `files`, `terminal`, and `preview`. Guide, pause,
resume, take-over, and revoke interventions require the grant's current version;
stale concurrent decisions return a conflict. Revocation is terminal.

File reads return a SHA-256 digest. Sending it as `base_digest` on a file write
turns concurrent modification into a visible conflict instead of silently
overwriting another collaborator. File and command mutations are serialized so
overlapping actions have one durable order. Shared command activity intentionally stores
only actor, timing, surface, and exit status. The raw command, stdin, stdout,
and stderr remain private to the invoking response because terminal input often
contains credentials or other context that was never offered to the room.

## Attested packages

A package version is a reusable view of bytes that already passed the release
pipeline, not a separate upload that can claim its own provenance. Repository
owners publish through `POST /repositories/{repository}/packages`, selecting an
exact succeeded build attempt and artifact from an immutable release. The API
requires the newest attempt for every release build definition to have passed
at the release commit, derives the owner-scoped identity, and records the source
commit, release, build command and attempt, SHA-256 checksum, platform,
dependencies, publisher, visibility, and lifecycle.

Artifact bytes are copied to `$PACKAGE_ROOT` and checked against the build
evidence before the version metadata is renamed into the visible catalog. A
conflicting identity/version, authorization failure, checksum mismatch, or I/O
failure therefore leaves no partially available package. Public package
versions can only originate in public repositories; private versions remain
available to repository participants. The repository Releases tab publishes
and displays this evidence. Publication may also attach Markdown documentation;
its SHA-256 digest becomes part of the immutable version record.

Anonymous discovery uses the paginated `GET /packages?q={query}` collection and
`GET /packages/{version-id}` inspection resource. They expose public versions
only and search identity, semantic version, and documentation. The public
`/packages` web catalog keeps documentation, platform/runtime compatibility,
declared dependencies, lifecycle warnings, source commit, release, build
attempt, artifact checksum, and size together rather than asking a collaborator
to trust an opaque download.

Standard npm clients resolve metadata at
`GET /package-registry/{encoded-package-identity}` and immutable tarballs at the
returned `dist.tarball` URL. Public versions require no credential. For private
resolution, a repository participant calls `POST
/repositories/{consumer}/package-credentials` with an explicit nonempty list of
package version IDs they can already read and a lifetime of at most 24 hours.
The returned secret is shown once and configured as the registry Bearer token.
It has only `package:read`, retains the consuming repository and immutable
version allowlist, and is revalidated against consumer and publisher access
whenever used. It cannot read Git, mutate either repository, publish packages, reuse a
publisher credential, discover unlisted private packages, or download their
bytes. Isolated builds can therefore receive the same narrow token without a
developer's or publisher's general credential.
Registry compatibility translates attested `amd64` and `386` architectures to
npm's `x64` and `ia32` CPU names; the embedded Komodo attestation continues to
expose the original platform value.

```sh
npm config set @publisher:registry https://kanso.example/api/package-registry
npm config set //kanso.example/api/package-registry/:_authToken "$PACKAGE_TOKEN"
npm install @publisher/sdk@1.2.3
```

### Commit-derived dependency inventories

A repository records what code and environments actually use by committing two
schema-version `1` files. `.komodo/packages.json` contains a
`direct_dependencies` array of owner-scoped package identities.
`.komodo/packages.lock.json` contains the complete resolved `packages` graph;
each entry names an immutable `package_version_id`, its matching identity and
version, and dependency package-version IDs. This keeps human intent distinct
from exact direct and transitive resolution.

`POST /repositories/{repository}/dependency-inventories` reads both files from
the supplied exact commit through Git storage. Callers cannot upload replacement
manifest contents or claim resolutions. The durable snapshot retains both file
digests, creator, direct/transitive classification, and explicit gaps for absent
direct locks, unavailable package IDs, mismatched identity/version data, and
missing transitive nodes. Repeating the same code and evidence tuple is a
conflict rather than silently rewriting history.

An inventory may link a release, successful release-build attempt, and
successful deployment. The API verifies every supplied resource belongs to the
consumer repository and names the same source commit. This gives owners a path
from reviewed source to a build artifact and running environment while keeping
source-only inventory useful before delivery. Data is rooted at
`$DEPENDENCY_INVENTORY_ROOT`, defaulting to
`apps/api/data/dependency-inventories` under the documented root command.

Repository-policy readers use `GET /repositories/{repository}/dependency-inventories`.
Public package inspection
uses `GET /packages/{version}/consumers`; it includes only public consuming
repositories and reports exact commits, direct or transitive use, linked build,
release and deployment evidence, and remaining provenance gaps. Private
repository identities and usage are never revealed by this anonymous reverse
index. Package publication also accepts immutable `license` and `support_url`
metadata, displayed alongside reverse consumer evidence in the package catalog.

### Verified dependency updates

Repository owners define direct-package update policy with `PUT
/repositories/{repository}/dependency-update-policies/{identity}`, choosing a
target branch and the largest accepted semantic-version change (`patch`,
`minor`, or `major`). Policy is consumer state beneath
`$DEPENDENCY_UPDATE_ROOT`; publishing a package does not run consumer code or
grant its publisher access to consumer repositories.

An authenticated consumer writer explicitly evaluates an immutable inventory
with `POST /repositories/{repository}/dependency-updates`. The service selects
the newest active, readable release allowed by policy and opens a proposal with
a ready delivery task. The retained update evidence includes the old and new
immutable package IDs, proposed `.komodo/packages.json` and
`.komodo/packages.lock.json` documents, release notes, publisher source and
build identities, artifact checksum, compatibility caveats, and every affected
direct-to-transitive dependency path. The same base commit, dependency, and
candidate cannot generate duplicate adoption work.

The proposal task is deliberately the handoff into the existing collaboration
contract. A repository participant can assign a human or Codex at an exact
consumer commit, investigate compatibility or check failures through its scoped
change session, and publish the result as an ordinary pull request. Required
checks, owner review, protected integration queues, releases, and deployments
therefore apply unchanged; neither the package publisher nor update evidence
can bypass consumer permissions. Policy and retained adoption work are readable
at the corresponding `GET /dependency-update-policies` and
`GET /dependency-updates` collections under repository visibility rules.

### Unsafe package containment and repair

Publisher owners append safety policy with `PUT
/repositories/{repository}/packages/{version}/safety`. A notice marks the
version `deprecated` or `quarantined`, records an attributable reason and an
optional active replacement of the same identity, and preserves prior notices.
Publication evidence and artifact bytes are not rewritten: repository evidence
downloads, dependency inventories, and existing deployments retain the exact
unsafe version for audit.

Quarantine denies every new registry metadata and artifact request, including
credentials issued before the transition. Deprecation is denied by default; a
consumer owner may change its repository policy or record a reasoned,
time-bounded exception for an inventoried exposure. Promotion checks an exact
release-commit inventory against the same policy before creating a deployment.

`GET /packages/{version}/exposure` exposes public consumers with exact commits,
direct/transitive use, deployments, and open remediation. Affected repository
owners receive targeted activity. Consumer writers use `POST
/repositories/{repository}/package-repairs` to open urgent human- or
Codex-owned proposal work at the captured revision. Ordinary checks, review,
integration, release, and deployment remain consumer-controlled, so publisher
coordination never grants authority over downstream code or environments.
Repair collection and exposure reads project the linked proposal task as
`open`, `in_progress`, `in_review`, `remediated`, or `closed`, and retain the
contribution pull request once one exists. The immutable exposure and repair
records therefore remain connected to current review and integration status
without duplicating delivery state.

## Embargoed security repairs

Security repairs are advisory-owned workspaces, not private-flavored ordinary
pull requests. A repair captures an affected repository and version line, exact
base commit, intended outcome, and optional dependencies on other repairs in
the same advisory. Its randomized `refs/heads/embargo/*` ref is excluded from
normal Git advertisements and JSON repository browsing; direct commit browsing
also requires reachability from a non-embargo branch. This keeps branch names,
new commits, agent activity, discussion, and review out of normal contribution
surfaces before disclosure.

Each human or agent session receives a separately revocable, 24-hour Git grant
for exactly one repository and repair branch. Human assignees must be both on
the advisory response team and an existing repository owner or collaborator,
so the security workflow does not expand repository authority. Session records
retain attributable discussion, branch revisions, and approve or
request-changes review decisions inside the advisory. Revoking a session—or
removing its human assignee from the response team—revokes its Git grant.

Verification stays advisory-owned as well. `POST
/security-reports/{report}/repairs/{repair}/verification` opens a ledger at the
current embargo branch tip and records required-check and private-security-
reproduction outcomes by safe name, opaque attempt identity, and definition
digest. The ledger never accepts commands, fixtures, or logs. A branch-tip
change requires a new ledger, preventing evidence from silently following a
different candidate.

Protected integration is rejected until at least one required check, one
security reproduction, and one current approval all pass and no remaining gap
is recorded. A repository maintainer can then connect the protected queue entry
and integration commit, followed by checksum-addressed release artifacts whose
version line must match the repair. The advisory workspace summarizes every
line's coverage, failures, approvals, integration, artifacts, and explicit
gaps without exposing the confidential reproduction itself.

Disclosure is a separately prepared, maintainer-owned commit point. A plan
freezes a public advisory ID, redacted summary, upgrade guidance, explicit
credits, optional publication time, and one ordinary branch ref for every
attested repair line. Preparation is rejected unless every repair has passed
private verification, protected integration, and release artifact attestation.

At or after the scheduled time, publication creates every repaired branch at
its exact integrated commit before lifting the embargo. If any ref cannot be
published, refs created by that attempt are rolled back and the private plan
pauses with its remaining actions; no advisory becomes anonymously readable.
On success, `GET /security-advisories/{advisory}` exposes affected versions,
fixed branches, release and artifact checksums, upgrade guidance, and credits,
while omitting reporter identity, contact, evidence, private refs, findings,
sessions, impact rationale and citations, internal report and repair IDs, and
audit history. Affected repository owners and collaborators
receive targeted upgrade activity containing only public guidance.

The complete report-to-disclosure boundary is covered by
`security_remediation_workflow_test.go`. An external researcher reports a
critical issue, a read-only agent assesses two supported lines, and human- and
agent-authored fixes travel through distinct scoped embargo Git grants. The
regression requires exact-candidate reviews, normal checks, private security
reproductions, protected-integration identities, and artifact attestations for
both lines before disclosure. It also proves that embargo refs and commits,
the advisory, and security activity remain absent from ordinary public Git,
repository-browser, advisory, and activity surfaces until the atomic publish;
afterward both fixed refs, redacted advisory evidence, credit, and actionable
upgrade activity become available together.

## Versioned interface relationships

Repositories publish named interface versions with `POST
/repositories/{repository}/interfaces`. A publication must reference an
existing release in that repository, so the platform—not the caller—binds the
interface to its exact source commit. Versions use three-part semantic versions;
an optional schema path points collaborators to the contract inside that
snapshot. Name/version pairs are immutable and unique within a repository.

Consumers declare their evidence with `POST
/repositories/{repository}/dependencies`, identifying a readable provider,
interface name, compatibility constraint, and either an immutable consumer
release or an exact unreleased commit. Constraints support exact versions,
caret and tilde ranges, minimum versions, and `*`. Declarations are append-only,
actor-attributed snapshots rather than a mutable inventory.

`GET /repositories/{repository}/relationships` returns the readable graph
touching that repository. Repository nodes retain owner identity; every edge
retains consumer and provider commits and releases, the chosen compatible
publication, and the newest deployment evidence per environment. A relationship
is `unresolved` when no readable compatible publication exists and `stale` when
its release evidence is missing or the consumer has released a newer revision
without a new declaration. Private linked repositories are omitted unless the
caller can independently read them. The repository `view=relationships` web tab
exposes the same graph, status reasons, owners, exact revisions, and environment
state. Storage defaults to `$RELATIONSHIP_ROOT` at
`apps/api/data/relationships`.

### Interface evolution decisions

An authorized provider collaborator can turn either a proposal or pull request
into an evolution plan before the candidate merges. The plan resolves the pull
request's immutable source revision (or validates the proposal's supplied exact
revision), reads the candidate schema and newest released predecessor through
the Git storage boundary, and retains their SHA-256 digests. It snapshots only
affected consumer declarations whose repositories the creator can read, with
their exact revisions, constraints, and owner identities.

Provider and consumer owners record the migration contract in the plan:
classified breaking, compatible, behavioral, or unknown changes; an overall
strategy; ordered owner-attributed steps; explicit exceptions; and replaceable
acknowledge or request-changes decisions. The repository `view=relationships`
workspace presents the comparison, blast radius, contract, and agreement beside
the existing relationship graph.

A provider collaborator may delegate analysis over an explicit subset of the
provider and affected consumer repositories they can read. The one-time
credential expires after 24 hours and is accepted only by the evolution-analysis
context, selected-repository exact-blob read, and finding publication resources.
It has no Git or application mutation authority. Findings retain `agent:<name>`
attribution, selected repository IDs, and explicit uncertainty, and return to
the shared plan without exposing the persisted credential digest.

Evolution plans also retain ordered migration tasks for the provider and every
affected consumer. Each task names its target version, observable outcome,
completion criteria, dependencies, discussion, work repository, and merge
target. Provider collaborators define that shared contract, but only a writer
of the task's work repository can assign a participating human or the `codex`
agent at a verified base commit. Starting work creates a task-scoped change
session and candidate branch in that repository (or its independently owned
fork); publication creates an ordinary commit-bound pull request against the
declared target. The plan tracks branch head and review state, and marks work
complete—and unblocks dependent tasks—only after the linked pull request is
merged. Plan authors receive no credential or implicit consumer write access.

### Cross-repository compatibility verification

Once provider and consumer migration tasks have linked open pull requests,
participants can start verification from the evolution plan. The provider task
is first, each repository appears once, and the platform stores every exact
repository, source commit, task, pull request, and dependency identity before
execution begins.

The provider revision defines `.komodo/evolution-checks.json` with schema
version `1` and an `evolution_checks` array. Checks are classified as `contract`
or `integration` and use the bounded check fields for command, working
directory, timeout, environment, and artifact paths. Bubblewrap materializes
the matrix beneath `/workspace/repositories/{repository_id}`, runs from the
provider snapshot, and exposes no Git metadata, credentials, host filesystem,
or network.

Plan surfaces retain matrices, actors, ordered logs and lifecycle events,
artifact checksums/downloads, and failed or superseded attempts. Evidence is
attested only when every run succeeds and every captured task still names the
same head. A later task commit therefore supersedes only matrices containing
that revision while unrelated retained evidence remains current.

### Governed evolution rollout

A provider owner can turn a current, fully passing evolution verification into
ordered rollout phases with named compatibility gates. Each phase identifies
the repositories that participate and the established integration-queue,
release, deployment, rollback, or repair evidence expected from each one.
Phases run sequentially; a later phase cannot become ready until every step in
the prior phase has reached a safe outcome.

Participation remains repository-owned. Only the owner of a repository named
in a phase may approve or reject that repository's window, and linking an
outcome requires ordinary write authority in that repository. The rollout API
accepts only real resources owned by the named repository and derives their
state from the integration queue, release build attempts, or governed
deployment record. It never executes those workflows with plan-level authority
or accepts a caller's claim that they passed.

A failed queue, build, or deployment pauses its phase without changing already
successful repositories or allowing later phases to advance. The same retained
timeline can link a governed rollback or agent-assisted repair deployment; a
safe rollback completes the affected step while repair remains in progress
until it returns through normal review, integration, release, and deployment.
The relationship workspace shows the attested matrix, phase gates, owners,
approvals, current compatibility state, next action, linked resources, and all
prior outcomes together.

The complete proposal-to-ecosystem boundary is covered by
`evolution_workflow_test.go`. A released provider and independently owned
consumer become an explicit dependency; a provider maintainer proposes a
breaking candidate, a read-only Codex analysis records risk and uncertainty,
and the consumer owner acknowledges the migration window. Provider agent work
and human consumer work in a contributor-owned fork retain separate assignment,
session, branch, pull-request, and actor identities. Verification freezes both
heads plus the dependency declaration before rollout. A failed consumer
deployment then proves that completed integration remains safe, provider
cutover stays blocked, rollback evidence is retained, and only the respective
repository owners can advance the recovered consumer and provider phases to a
completed ecosystem migration.

## Release builds and attestations

A release candidate is an immutable source and collaboration snapshot. Its
exact commit must contain `.komodo/releases.json` with schema version `1` and
an ordered `builds` array. Each build names its shell command and may declare a
working directory, timeout, environment, artifact paths, and dependencies on
steps declared earlier in the array. Dependencies make execution order and
blocked follow-on work explicit.

Release builds use the same Bubblewrap boundary as commit checks: only the
materialized source snapshot and bounded system runtime are visible, with no
Git metadata, host credentials, host filesystem, or network namespace. Public
release attestation resources retain every attempt's exact source commit,
definition, initiating actor, status/outcome events, stdout/stderr, and
checksum-addressed artifacts. Artifact downloads remain governed by repository
read policy. A terminal attempt can be rerun from its captured commit and
command; the prior logs, outcome, and artifacts remain available, and the
attestation derives verification from the newest attempt for every named build.

## Governed release delivery

Repository owners define an ordered environment chain through the environment
API. Each environment captures its deployment command, scoped non-secret
configuration, required independent approval count, and concurrency limit.
Credential values are encrypted separately beneath `$DEPLOYMENT_ROOT`, are
represented publicly only by their names, and are injected only into the
deployment process. Output is redacted against all injected values.

Repository participants may promote one retained artifact from a currently
verified release. The promotion snapshots the release and source commit,
successful build attempt, artifact identity, path, and SHA-256 digest. It stays
pending until enough collaborators other than the initiator approve, then runs
with `KOMODO_ARTIFACT_PATH`, `KOMODO_ARTIFACT_SHA256`, `KOMODO_RELEASE_ID`, and
`KOMODO_SOURCE_COMMIT` available to the configured command. Public deployment
records retain ordered initiation, approval, state, and redacted log events;
the Releases web view polls that contract for live status. `$DEPLOYMENT_ROOT`
defaults to `apps/api/data/deployments` in the root development command.

The deployed release also owns its rollout policy. Its exact source commit must
contain `.komodo/deployments.json` with schema version `1` and an
`environments` array keyed by configured environment name. Every environment
declares ordered `stages`; each stage may run a rollout command and must declare
one or more health signals with a name, shell command, and optional timeout (60
seconds by default, 600 maximum). Commands receive the scoped deployment
variables and credentials plus `KOMODO_ROLLOUT_STAGE`; health probes also
receive `KOMODO_HEALTH_SIGNAL`.

Promotion validates this policy against the immutable release revision before
reserving environment concurrency. The deployment timeline retains stage and
signal outcomes alongside redacted output, artifact checksum, and affected
commit. Repository participants can pause or resume between rollout operations,
cancel active or waiting delivery, or explicitly mark a running rollout
unsuccessful through the deployment `/control` resource. Decisions record the
actor, reason, current stage, resulting state, and time; unhealthy outcomes and
interventions become deployment-linked inbox activity. Paused rollouts retain
their environment concurrency slot.

An unhealthy deployment has two explicit recovery paths at `POST
/repositories/{repository}/deployments/{deployment}/recovery`. `rollback`
finds the newest earlier successful deployment in the same environment and
starts a new governed deployment using that retained release, build attempt,
artifact identity, checksum, and source commit. It does not bypass the
environment's approval or concurrency policy, and both records retain their
recovery relationship.

`repair` creates a draft pull request on a unique `codex/recovery-*` branch at
the failed release commit and immediately queues a change-session run. The
session snapshots release, deployment, artifact, redacted logs, rollout stage,
health-signal outcomes, and source context. Its one-time credential lasts at
most 24 hours and writes only that candidate branch; it carries neither
environment secrets nor API deployment authority. Publication therefore
returns through pull-request synchronization, required checks, review,
integration, a new release, and governed promotion.

Agent publication leaves the recovery pull request as a draft until its author
explicitly calls the pull request's `POST .../request-review` resource. The
transition does not change the represented source commit, so the published
revision, check evidence, and deployment-failure context remain continuous as
ordinary review begins. `release_delivery_workflow_test.go` exercises the
complete public merge-to-release path, a health-signal failure, known-good
rollback, evidence-backed agent repair, maintainer review, and corrected
promotion.

## Incident coordination

Repository participants coordinate service risk through repository-scoped
incidents. Declaration captures severity, current impact, affected repository
and environment pairs, and an incident commander with optional operations and
communications leads. It may be manual or reference an exact failed health
signal in a retained deployment timeline; the API validates every affected
environment, role holder, and signal through the existing repository and
deployment boundaries.

Every accepted status, scope, role, follow, acknowledgement, and update action
is appended to the incident timeline with its stable actor and time. Updates
declare either a participant or public audience so responders can maintain one
operating picture without publishing internal coordination accidentally.
Followers and per-update acknowledgements make receipt explicit. Resolved
incidents retain their full record and reject later coordination updates.
Storage is rooted at `$INCIDENT_ROOT`, defaulting to
`apps/api/data/incidents`; reads follow the anchor repository's visibility and
writes require participant repository access. The web workspace uses the
shareable `view=incidents&incident={id}` repository context.

Diagnosis in that workspace is a durable notebook rather than free-form status
text. Responders attach source pointers for deployment logs, health signals,
deployments, releases, exact commits, pull requests, and prior incidents. Log
sources require an explicit start and end time, health signals retain their
event sequence, and every attachment records who captured it and when. The
linked source remains live and inspectable while the pointer preserves the
historical window used during response.

Investigation entries are typed as observations, hypotheses, reproducible
queries, or conclusions. Each entry may cite exact attachment IDs and retains
its author, timestamp, query or command text, and audience. Both attachments
and findings use `participants` or `public` access; public incident reads omit
participant-only diagnosis and timeline updates as well as follower and
acknowledgement state. Source creation is accepted only when the responder can
read the source repository and the referenced record exists through its owning
storage boundary.

The declaring responder or a current incident role holder can delegate a
bounded agent investigation. A session captures its mandate, selected incident
evidence IDs, exact repository commits, and an allowlist of affected deployment
log or health-signal resources. Its one-time 24-hour worker credential is
recognized only by the investigation context, operational-read, and record
publication endpoints; it is not an API, Git, deployment-control, environment,
or secret-management credential. Operational reads return retained redacted
deployment evidence and have no mutation counterpart.

Agents append attributable findings, tool actions, questions, and explicit
uncertainty to the durable session and participant incident timeline. Incident
responders can add guidance and pause, resume, or cancel a session; paused
sessions reject worker publication, while cancellation immediately invalidates
the credential. The public incident representation omits delegated sessions,
and persisted credential digests are never returned through incident or worker
responses.

Mitigation is an incident-owned decision record layered over delivery
authority, not a new production authority. A responder proposal cites exact
incident evidence, the affected environment and deployment, the intended
pause, attested restore, or emergency repair, and concrete recovery criteria
identified by deployment health-event sequence. Participants can discuss the
proposal; a responder other than the proposer approves or rejects it, while an
incident commander may make an explicitly recorded override. Execution
attempts retain actor, outcome, explanation, and the governed deployment or
pull-request resource they changed. Recovery is complete only when a responder
submits existing deployment health-signal events matching every declared
criterion; unhealthy results and failed attempts remain visible. Deployment
pause still uses deployment control permissions, restores remain ordinary
governed rollback deployments, and emergency repairs remain draft pull
requests that pass checks, review, integration, release, and promotion.

Resolution is a reviewed handoff, not a bare status change. A responder must
record an attributed impact summary, condensed chronology, contributing
factors, and citations to incident conclusions, then create at least one
corrective proposal task with an owner, exact starting revision, mandate, and
due time. Those tasks use the ordinary proposal assignment and contribution
workflow. Reconciliation projects linked pull requests and their latest checks,
release candidates, and deployments back into the incident record. A missing,
closed, canceled, or superseded task invalidates the commitment; unfinished
work past its due time becomes overdue. Both transitions create actionable
owner inbox activity while the complete resolution and delivery provenance
remain reviewable from the incident workspace.

The complete response loop is covered as one black-box public-surface workflow.
A real failed release health signal becomes an incident with audience-aware
coordination and read-only agent diagnosis, then an independently approved
known-good rollback whose declared signal must pass before recovery is accepted.
Resolution creates owned corrective work that publishes through stock Git,
checks, review, merge, release build, and governed deployment; reconciliation
projects every resulting identity and outcome back into the original incident.
The regression also proves that an incident worker cannot control deployment
state and that anonymous readers receive only explicitly public investigation
context.

## Private vulnerability intake

Security reports are global private collaboration records rather than
repository resources. An authenticated researcher creates one with `POST
/security-reports`, naming one or more repositories they can already read,
affected version expressions, structured evidence, and a safe contact channel.
The reporter and every affected repository owner can inspect the exact report.
The caller-specific `GET /security-reports` collection exposes only title,
scope identities, severity, embargo state, and timestamps; it omits summary,
versions, contact coordinates, evidence, messages, and audit detail.
Unauthorized exact reads return `404`.

Affected-repository owners triage through `PATCH
/security-reports/{id}/triage` and may grant or revoke at most 20 registered
responders through the report's `/team` resources. The reporter, owners, and
current responders communicate through private `/messages`; revocation takes
effect on the next request. Every exact-report view, triage change, invitation,
removal, and message is appended to the report-private access audit with stable
actor identity and time.

Security intake has no integration with repository activity, public project
search, ordinary inbox, or incident timelines, preventing embargoed details
and responder identities from leaking through discovery or notifications.
Storage is rooted at `$SECURITY_REPORT_ROOT`, defaulting to
`apps/api/data/security-reports`. The workspace Security reports view provides
submission, triage, invitation, conversation, evidence, and audit controls
through the same-origin API proxy.

Responders build the advisory's private provenance graph through `/resources`:
typed links connect exact commits and dependencies to builds, release artifacts,
deployments, and supported version lines. Attributable `/findings` are either
hypotheses or conclusions and must cite submitted or linked evidence IDs. The
upserted `/impact` resource keys each row by repository, version line, and
environment, retaining its confirmed, suspected, unaffected, or fixed state,
rationale, citations, actor, and update time.

A report participant may delegate selected evidence through
`/security-reports/{id}/investigations`. The returned credential is shown once,
expires after 24 hours, and is accepted only by the read-only
`/security-investigations/context` and append-only `/records` resources. Context
contains a safe advisory header plus selected evidence—not reporter contact,
conversation, response-team, audit, matrix, or other evidence. Agent findings,
tool notes, questions, and explicit uncertainty remain private and attributed;
participant pause/resume/guidance is retained, while cancellation revokes the
credential. This authority cannot read or mutate repositories, Git, builds,
releases, artifacts, deployments, or any other platform resource.

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

Each proposal also owns a shared delivery plan at `GET
/repositories/{id}/proposals/{proposal}/plan`. The response combines ordered
tasks with their append-only change history. Tasks name an observable outcome,
carry a `planned`, `in_progress`, `completed`, or `canceled` status, reference
same-plan dependencies, and may link to exact proposal comment IDs that
motivated the work. `ready` is derived for planned tasks only when every
dependency is completed; canceled prerequisites continue to block dependents
until collaborators revise the plan. Authenticated repository participants add
tasks through `POST .../plan/tasks` and replace task details, order, status,
dependencies, or discussion links through `PATCH .../plan/tasks/{task}`. The
store rejects unknown, self, and cyclic dependencies and retains a full task
snapshot, stable actor ID, action, and timestamp for every accepted change.

The proposal detail page renders this contract as an executable plan above the
conversation: immediately startable work is called out, expected outcomes and
prerequisites stay visible, collaborators can reorder or update work, and a
history disclosure explains who changed each task and when.

A ready task can have exactly one pre-work assignment. `PUT
.../plan/tasks/{task}/assignment` claims or assigns it to a repository human or
the available `codex` agent with an explicit mandate, repository, and exact
base commit. The response previews only `contents:read` and
`candidate_branch:write`; it does not create a credential or begin execution.
Reassignment supplies the current `expected_assignment_id`, while `DELETE` on
the same resource uses that ID to revoke it. Missing or stale tokens and
simultaneous claims return conflicts, and accepted assignment, reassignment,
and revocation transitions retain stable actor attribution in plan history.

Readiness is continuously derived from task state and linked contribution
outcomes: only a completed task or merged contribution satisfies a dependency,
and each response includes `blocked_by` task IDs when it does not. Plan edits
and pull-request merge or closure reconcile every dependent assignment and add
targeted ready, blocked, changed, or obsolete activity to its human assignee's
inbox. An authorized participant can move an unstarted assignment to a verified
commit with `PATCH .../plan/tasks/{task}/assignment/base`, supplying the current
`expected_assignment_id`. The transition is retained as `task.base_rebased` in
plan history. Once a session or contribution exists, changing its outcome,
dependencies, or assigned base returns `409 task_has_active_work`; the captured
session and pull-request context therefore remains explicit instead of being
silently rewritten by a later plan revision.

Integration-queue publication reconciles a linked proposal-task contribution
as `merged` at the same time it records the pull request outcome. Dependents
therefore become ready from the durable queued result just as they do after a
direct merge; retained queue evidence and the plan history identify the owner
who authorized publication. `orchestration_workflow_test.go` exercises this
complete idea-to-integrated-change contract across proposal discussion,
dependent human and agent assignments, guided execution, connected pull
requests, checks, reviews, the queue, and stock Git.

An agent assignment starts without manufacturing a pull request through `POST
.../plan/tasks/{task}/change-sessions`, using `expected_assignment_id` as the
start concurrency token. The API creates a unique `codex/task-*` branch that
points exactly at the assignment's base revision, captures the proposal title
and description, task outcome, mandate, completed dependency snapshots, and
repository metadata in a durable session, and immediately queues the Codex
run. Its one-time credential lasts at most 24 hours and can write only the
assigned repository and exact working branch. The task becomes `in_progress`
and retains the session and branch identities; a repeated or stale start is a
conflict.

The task's `/change-sessions` collection, session detail, and `/events`
resources provide the same reconnectable public timeline available during
pull-request work. Repository participants guide, answer, pause, resume, and
cancel through `/runs/{run}/interventions`; workers read `/control` and append
typed progress through `/events` using their branch credential. Cancel and
failure revoke that credential immediately. These sessions deliberately do not
create a pull request: connecting their eventual candidate commits to review
is the subsequent publication transition.

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

Owners can protect an individual target branch with an integration queue from
the same readiness surface. `GET` and `PUT
/repositories/{id}/integration-queue?branch=...` expose and configure whether
the queue is required, its bounded concurrency, its pause-or-remove failure
behavior, and the branch's existing required checks and owner-approval rule.
Once protected, direct merge returns `integration_queue_required`. An owner can
admit a currently ready request with `POST
/repositories/{id}/pull-requests/{pull}/queue`; the durable entry captures its
exact reviewed source revision, current target revision, actor, and ordered
position. Admission also creates an immutable two-parent candidate commit in
the target repository: the current eligible target is its first parent, the
reviewed source revision is its second parent, and its tree is the prospective
recursive merge result. Cross-repository source objects are hard-linked into
the target first, while no branch reference is advanced.

The target branch's required check names are executed anew against that exact
candidate commit. Queue entry reads expose the source, base, candidate commit,
candidate tree, required check attempts, and a derived `verifying`, `blocked`,
or `passed` lifecycle. Attempt IDs lead through the ordinary public pull-request
check resources to retained status events, stdout/stderr, outcomes, and
artifacts; the candidate commit itself is inspectable through the repository
commit browser. Thus source-revision success remains admission evidence while
queue success describes the repository state that would actually be merged.
`GET /repositories/{id}/integration-queue/entries?branch=...` is
repository-policy readable and returns those ordered candidates plus their
governing policy. Queue data lives beneath `$INTEGRATION_QUEUE_ROOT`, defaulting
to `apps/api/data/integration-queue`.

The oldest eligible entry advances automatically when every required check for
its current candidate succeeds. Publication uses an atomic compare-and-swap:
the target must still name the candidate's exact first parent, so neither an
HTTP push nor another coordinator can make passing but stale evidence land.
After each merge, later entries are rebuilt in order against the new branch tip
and receive a new candidate generation and new checks. Attempts for superseded
generations remain available as evidence but are never considered current.
This same recovery loop handles target updates made outside the queue and
resumes durable work after process restarts.

A queued source-branch update removes only the obsolete queue entry; the pull
request remains open for synchronization, review, and readmission. A merge
conflict blocks the head under `pause` policy or removes it under `remove`
policy. Failed and canceled candidate checks follow the same configured
behavior with distinct durable reasons. Removing a queue entry never closes or
rewrites its pull request, while a successful candidate records that pull
request as merged at the exact candidate commit.

Integration coordination is visible in the repository's **Integration queue**
tab (`view=queue&ref={target-branch}`) as well as on each pull request. The
branch view combines current policy, ordered active entries, candidate checks,
plain-language blockers and next actions, every retained candidate generation,
and completed outcomes. Repository owners can pause or resume an entry, retry
its exact candidate checks, move it within the active order, or remove it;
each intervention is appended to the durable entry with stable actor
attribution. `PATCH /repositories/{id}/integration-queue/entries/{entry}`
accepts `pause`, `resume`, `retry`, `reprioritize` (with a one-based
`position`), or `remove`. The branch collection retains completed entries and
their candidate attempt history rather than treating removal or publication as
erasure. Automatic blocked, removed, and merged outcomes also enter the
activity ledger and produce pull-request links in the affected contributor's
inbox, so required intervention remains connected to the shared change.

`integration_queue_workflow_test.go` is the black-box regression boundary for
this complete capability. It uses stock Git, public repository policy, pull
request review, agent delegation/publication, queue, and check-evidence
surfaces to admit three independently valid changes concurrently. The fixture
proves that the first human change lands in policy order, a second candidate
is rebuilt and isolated when it fails only against that evolved target, and a
later agent-published change is rebuilt, freshly verified, and merged while
the failed pull request stays open. It also asserts that enqueue, rebuild,
failure, and merge attribution plus every superseded and current candidate
attempt remain readable after the queue drains.

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

Proposal-plan work enters this review contract through the task's
`/contributions` resource. A human assignee supplies a candidate branch; an
agent may use the exact branch and branch-only credential captured by its task
session. Publication resolves immutable source and target commits, rejects an
unchanged base, creates the ordinary pull request, and starts its commit-bound
checks. The pull request links back through `proposal_id`, `task_id`, and
`change_session_id`, while the task contribution and pre-review session link
forward to the pull request. Reviewers can therefore move from agreed intent
through execution events and check evidence to the exact review snapshot.

Publication does not complete planned work. A contribution begins in `draft`
or `review`; a later attempt marks the prior active contribution `superseded`,
closing records `closed`, and merge records `merged`. Only a merged task
contribution satisfies dependent-task readiness. Draft requests remain
readable but are blocked by pull-request readiness. Merge commits retain
`Proposal-Task` and `Change-Session` trailers with the existing collaboration
and source provenance.

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

## Release definitions

Repository participants define immutable delivery candidates through `POST
/repositories/{id}/releases`; public or participant readers inspect the
paginated collection and individual records through the corresponding `GET`
resources. A definition captures a repository-unique version, release notes,
an exact verified commit, stable creator and creation time, and the `candidate`
lifecycle state. Data is owned by `releases.Store` beneath `$RELEASE_ROOT`,
defaulting to `apps/api/data/releases`.

An optional `prior_release_id` establishes the comparison boundary. The API
requires its captured commit to be an ancestor of the new source commit, then
walks repository history itself. It snapshots every merged pull request whose
merge commit is reachable from the candidate but not the prior release, along
with linked proposal and task IDs and deduplicated pull-request author IDs.
Clients cannot submit inclusion or contributor claims, so later build,
attestation, and promotion work can depend on one durable account of exactly
what is being delivered and why. The repository Releases tab exposes this
contract at `view=releases`; `release={id}` keeps the inspected candidate
shareable.

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
# Organization portfolio

Organizations are durable group identities stored beneath `$ORGANIZATION_ROOT`.
An invitation is only a pending membership record: the invited user explicitly
accepts before receiving repository collaborator access, and removal revokes
those portfolio grants. Organization owners may create repositories directly in
the group or request an ownership transfer. A transfer changes the repository
catalog owner only after the receiving controller accepts, preserving the same
storage ID, Git history and references, forks, pull requests, proposals, checks,
releases, packages, deployments, incidents, and attribution.

`GET /organizations/{id}/portfolio` is a joined view, not a second source of
truth. It reads the organization's repositories and their existing package,
open-pull-request, release, and unresolved-incident records. Repository JSON
uses the organization ID as `owner_id` and exposes the acting human owner as
`administrator_id`; internal repository policy continues through that explicit
administrator plus accepted members' collaborator grants until later team and
portfolio-role milestones replace the coarse membership mapping.

## Organization teams and approved agents

Organizations keep nested teams, accepted team membership, maintainers,
repository-area responsibility, and organization-approved agent identities in
the same durable organization record and audit sequence. Owners create teams at
`POST /organizations/{id}/teams`; a maintainer can invite or remove people and
declare responsibility for that team or any descendant. Invitations become
effective only through the invited user's team acceptance endpoint. Every team
mutation includes `expected_version`, and stale concurrent edits return `409`
without overwriting the intervening decision.

Responsibilities name an organization-owned repository and an area such as a
path glob, subsystem, service, or operational domain. Approved agents retain
their visible capability declarations and accepted human operators; registering
an agent grants no repository authority. Removing an organization member also
removes their direct team memberships and operator links.

`GET /organizations/{id}/directory` is anonymously readable. Anonymous callers
see public teams, public responsibilities, public approved agents, and accepted
effective members. `via_team_ids` records the nested path explaining why each
person is effective in a team. Authenticated organization members additionally
see internal teams, responsibilities, direct membership records, and pending
invitations.

The organization access ledger turns that responsibility into reviewable,
temporary authority. An owner assigns a `viewer`, `contributor`, `maintainer`,
or `operator` role to a team or approved agent, selecting individual
repositories, packages, environments, and collaboration resources. Nested
resources retain their repository boundary. Every grant has a reason, an
expiry no more than 30 days away, and optional explicit denied-action
exceptions such as `deploy:production`; it does not silently add collaborators
or grant the rest of the portfolio.

Members request the same contract at `POST
/organizations/{id}/access-requests`; they may request only for a team they
belong to or an approved agent they operate. Organization owners approve or
deny once, and approval creates the role grant linked from the request. `GET
/organizations/{id}/access/effective` returns the active, unexpired grants
derived for the caller through accepted nested-team membership or agent
operation, including exact resources, exceptions, reason, and expiry. The
Organizations web workspace presents this explanation and pending decisions.

A contributor-or-higher grant over one selected repository can derive a
one-time-visible, branch-only Git credential for at most 24 hours through the
grant's `/credentials` resource. The credential cannot outlive its role and an
exception named `candidate_branch:write` prevents issuance. Revoking a grant
immediately revokes every credential derived from it. Removing a team or
organization member revokes only that user's credentials from grants they no
longer inherit, leaving unaffected teammates intact. The organization event
sequence audits requests, decisions, grants, credential issuance, and
revocation without storing credential secrets.

## Organization policy baselines and exceptions

Organization owners define portfolio governance as immutable policy versions
in the organization record. Rules cover `repository_visibility`, `reviews`,
`required_checks`, `integration`, `release_provenance`, `dependency_use`,
`environment_promotion`, and `agent_authority`; each is required or advisory
and carries domain-specific JSON configuration. A policy targets the whole
organization, one organization-owned repository, or a team. Team targeting
follows declared repository responsibilities and grants no repository access.

`POST /organizations/{id}/policies` starts a draft lineage and `POST
/organizations/{id}/policies/{policy}/versions` appends its next immutable
version. Drafts have no effect. Members submit repository IDs to a draft
version's `/preview` resource to see active-plus-proposed rules and their target
explanations before activation. Activation supersedes only the prior active
version in that lineage; other policy lineages remain additive, and existing
pull requests, runs, releases, deployments, and captured work are not rewritten.

Maintainers inspect inherited rules at `GET
/organizations/{id}/repositories/{repository}/effective-policy`. Required rules
remain visible even when an exception applies. An owner or holder of an active
organization `maintainer` or `operator` grant for the repository may request a
reasoned exception at `POST /organizations/{id}/policy-exceptions`. It binds one
exact policy version, rule, and repository, expires within 30 days, and is
ineffective until an owner approves it. Pending, denied, expired, and approved
decisions retain requester/resolver attribution; a newer policy version needs a
new explicit exception.

## Cross-repository portfolio initiatives

An organization member creates an initiative at `POST
/organizations/{id}/initiatives` from one or more existing proposals, interface
evolution plans, incidents, or private security reports they are authorized to
see. The API verifies every source and its organization-owned repository. The
initiative records the shared outcome and source identities without copying
their discussion, status, or evidence.

Ordered items connect proposal tasks, evolution tasks, incident actions, or
security repairs across organization repositories. Each item names an
observable outcome, dependencies on other initiative items, and one accountable
accepted human, team, or approved agent. It may point to ordinary contribution
records, upcoming immutable releases, applicable policy exceptions, and the
specific decision that moves the work forward. Dependency cycles and missing
organization resources are rejected.

`GET /organizations/{id}/initiatives` is a live coordination view. It retains
the historical assignee but derives incomplete dependencies and reassignment
blockers from current organization membership, approved-agent operation, and
repository ownership. Membership removal or repository transfer therefore
turns affected work into an explicit `needs_reassignment` decision instead of
silently deleting or orphaning it. The Organizations web workspace presents
these blockers and links beside the portfolio's authoritative delivery state.

Initiative accountability follows the same least-privilege boundary as work.
An organization owner has portfolio controller authority; another human is
actionable only while they inherit a current repository-scoped role through an
accepted team, and an approved agent or team needs its own current scoped role.
Team responsibility remains directory and routing evidence, not repository
authority. Losing membership, an operator, or a grant changes the live item to
`needs_reassignment` without removing its assignee, contribution, release,
policy-exception, decision, or update attribution.

`organizations.TestOrganizationGovernanceCollaborationLoop` composes the
organization governance contract as one regression. It retains multi-team
responsibility, developer and approved-agent grants, shared review/check/
promotion/agent policy, an expiring exception, ordered cross-repository human
and agent work, delivery resource identities, membership-change blockers, and
the complete persisted evidence sequence.

## Reproducible development workspaces

Development environments are repository collaboration resources rather than
undocumented local-machine recipes. A repository opts in with the exact
revision's `.komodo/workspaces.json` (schema version `1`). The definition names
the expected tools and dependencies, provides ordered setup commands, and sets
bounded CPU seconds, memory, disk, and total setup time. A launch fails closed
when the definition is absent or invalid.

Authenticated repository writers launch with `POST
/repositories/{repository}/workspaces`, naming a full commit ID and one durable
source context: the repository itself, an assigned or contributed proposal task,
a same-repository pull request snapshot, or an incident emergency-repair pull
request. The API verifies that contextual resource already names the requested
commit; a branch name or caller-asserted association is never accepted as the
foundation. Collection and detail reads follow ordinary repository read policy.

Setup materializes only the captured Git tree beneath `$WORKSPACE_ROOT`
(default `apps/api/data/workspaces`) and executes each command in Bubblewrap
without network, credentials, repository metadata, or host filesystem access.
The durable record retains creator, exact revision, source IDs, effective access
at launch, the complete normalized definition and SHA-256 digest, lifecycle,
commands, bounded stdout/stderr, outcomes, and timestamps. `POST .../{id}/suspend`
retains that materialized state. `POST .../{id}/resume` uses the retained files
and re-reads the definition from the same immutable commit; missing files or a
digest mismatch produce a conflict rather than a silent rebuild. The repository
web tab at `view=workspaces&workspace={id}` launches, reconnects to setup
evidence, and controls suspension and resume. A ready environment also exposes
path-safe file reads, writer-attributed edits and deletions, bounded text search,
and credential-free command execution. Commands run in a fresh Bubblewrap
namespace over the retained environment with the definition's resource bounds;
the durable record retains actor, command, bounded output, exit code, and exact
revision. File evidence retains actor, path, deletion state, and SHA-256 digest
rather than a second content snapshot.

Schema-version `1` definitions may declare preview `ports`, each mapping a
familiar port number and label to a relative static-output directory. The
workspace preview route serves only declared text assets through repository read
policy with restrictive content security, no cache, and the exact revision
header. Generated documentation and static browser applications are therefore
inspectable without forwarding a sandbox or host network socket. Setup, later
commands, and previews receive no repository credentials or secrets. The web
workbench combines explorer, editor, search, command terminal, retained evidence,
declared ports, and a sandboxed preview in the shareable workspace URL. Live
pairing and publication build on this lifecycle.

### Safe workspace checkpoints

Ready workspace participants create durable checkpoints at `POST
/repositories/{repository}/workspaces/{workspace}/checkpoints`. A checkpoint
names the repository paths that constitute meaningful unfinished work and
captures only their add, modify, or delete delta from the workspace's immutable
base commit. It retains the creator, optional parent checkpoint, exact base,
complete environment definition and digest, readable text patches or binary
metadata, and declared dependencies and reproduction commands. File bytes are
stored by SHA-256 beneath `$WORKSPACE_ROOT`; internal blob addresses and bytes
are not returned by public workspace resources.

The explicit path boundary prevents setup products and unrelated mutable
runtime state from becoming evidence accidentally. `.env`, SSH/AWS/Git stores,
private-key files, dependency directories, symlinks, oversized files, and
credential-like content are rejected. Private commands, terminal streams,
credentials, and undeclared files are never inferred into a checkpoint.

Checkpoint reads expose parent lineage, changes, patches, missing declared
dependencies, and non-reproducibility reasons. Restore rechecks the workspace
base and environment definition and compares each target path with both its
base and checkpoint digest. Divergence returns `checkpoint_conflict` with the
conflicting paths before any file is changed; a clean restore verifies every
content-addressed blob before applying it. Collaborators can create a child
checkpoint after restoring or extending an earlier one, preserving a branch of
unfinished work without depending on the original machine.

### Governed workspace publication

Repository writers publish an immutable checkpoint through `POST
/repositories/{repository}/workspaces/{workspace}/checkpoints/{checkpoint}/publication`.
The request names a working branch and commit message and may create an ordinary
pull request against an existing target branch. Publication reconstructs a Git
tree from the checkpoint base plus only its explicitly captured, digest-verified
changes; it never walks the mutable environment. The branch is created or
atomically advanced only from that base, so moved branches reject stale work.

The commit, checkpoint, workspace, and pull request retain bidirectional IDs,
the publisher and exact checkpoint/file contributors, proposal-task and change-
session context when present, and an originating pull request when applicable.
Publishing from a proposal task registers the request as that task's review
contribution, so protected-queue integration reconciles the plan to `merged`
rather than leaving completed workspace work detached from its original intent.
Commit trailers preserve the workspace and checkpoint link in Git history. Pull
request text summarizes the declared change and verification commands without
copying private command output or activity. New requests enter existing checks,
reviews, required-check, integration-queue, and stale-revision governance.
Credentials, unselected files, setup products, terminal data, and runtime state
are never inferred into repository content.

### Workspace governance and expiry

Repository owners manage the default envelope at `GET|PUT
/repositories/{repository}/workspace-policy`; organization owners can maintain
their organization envelope at `GET|PUT
/organizations/{organization}/workspace-policy`. Policies bound CPU, memory,
disk, network mode, idle suspension, retention, advance expiry notice, sharing,
and approved-agent execution. A launch snapshots its effective policy and caps
the revision's declared resource limits. Workspace reads expose active presence,
the captured envelope, and actor-attributed command-runtime consumption.

Tightened repository policy marks existing environments as requiring rebuild
and revokes controls that now violate private-sharing or agent-execution rules.
Resume also records a rebuild requirement when the exact base definition is
missing or changed. Idle environments suspend automatically. Retention produces
an `expiry_announced` lifecycle event before terminal expiry, giving the creator
time to checkpoint or download a ZIP containing only safe collaborator-authored
paths from `GET .../{workspace}/export`. Credential-like paths and content use
the same fail-closed exclusions as checkpoints.

Repository owners can announce an explicit expiry with `POST .../{workspace}/expiry`
or stop/expire immediately with `POST .../{workspace}/stop`. Terminal lifecycle
transitions revoke live controls and presence, delete the mutable materialized
environment, and prevent further compute, but
the workspace record, activity, checkpoints, checkpoint blobs, published Git
commits, and pull-request links remain inspectable under ordinary repository
read policy.

`workspace_collaboration_workflow_test.go` composes the complete boundary. It
launches from an assigned plan task, joins a peer and organization-approved
agent, records edits, execution and human intervention, reconnects after
suspension, conflict-preflights and restores a checkpoint, publishes its linked
task contribution, passes repository checks and protected integration, then
expires the compute while proving the merged intent and collaboration trail
remain readable through public resources and stock Git.
