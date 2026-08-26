# AGENTS.md

Revision-bound signal contracts live beneath `$SIGNAL_CONTRACT_ROOT` (default
`apps/api/data/signal-contracts`). Repository writers publish optimistic immutable
definitions for metrics, logs, traces, profiles, and events with schemas, units,
dimensions, sampling, aggregation, correlation, retention, volume, quality, owners,
consumers, exact source symbols, service boundaries, collectors, and dependencies.
Reads derive a quantified privacy, security, residency, performance, cardinality,
storage, and cost preview plus alternative comparisons. Repository readers and
read-only agents append assumption challenges with revisioned accessible evidence.
Sensitive unclassified fields, unbounded dimensions, unsupported collectors, changed
dependencies, inaccessible sources, and incomplete impact or quality declarations
remain attributable blocking or incomplete findings. The public API is
`/repositories/{repository}/signal-contracts`. Contracts and review records grant no
repository, telemetry, collector, secret, deployment, environment, spending, or
operational authority.

Accepted current signal contracts create non-authoritative implementation plans and
exact pull-request telemetry proof beneath `$SIGNAL_IMPLEMENTATION_ROOT` (default
`apps/api/data/signal-implementations`). Plans retain permitted human- or agent-owned
tasks, sessions, workspaces, and pull requests across application, library, and
infrastructure repositories. `/pull-requests/{pull}/telemetry-checks` retains bounded
synthetic journey/failure results for emission, schema, units, correlation, sampling,
redaction, access boundaries, overhead, and failure behavior, with sanitized digest
evidence, coverage, cost, authorship, contract differences, and ordinary policy checks.
These records grant no repository, agent, telemetry, preview, review, merge, release,
environment, secret, or operational authority.

Revision-bound observability gaps live beneath `$OBSERVABILITY_GAP_ROOT` (default
`apps/api/data/observability-gaps`). Repository writers open and optimistically revise
a shared operational question from an exact service objective, incident, debugging
workspace, runbook, support thread, deployment, or manual origin. The record retains
the behavior, audience, blocked decision, affected services and journeys, timeliness,
owners, success criteria, and metric, log, trace, profile, or event evidence bound to
exact releases and environments. Reads derive attributable absent coverage, ambiguous
semantics, inaccessible sources, unbound context, and stale instrumentation. The
public API is `/repositories/{repository}/observability-gaps`, and gaps appear in the
Reliability view. Gaps grant no repository, telemetry, secret, deployment,
environment, incident, communication, or operational authority.

Gap-scoped signal evaluations live beneath `$SIGNAL_EVALUATION_ROOT` (default
`apps/api/data/signal-evaluations`) and the observability gap's
`/signal-evaluations` resource. Repository writers freeze exact signal-contract,
rollout, collector, and gap revisions into reproducible queries correlated with
releases, deployments, code, dependencies, and user journeys. Permitted humans and
read-only agents append accessible revisioned citations, findings, uncertainty, and
success-criterion results. Owners connect accepted findings to ordinary service
objective, alert, runbook, investigation, quality-check, or decision revisions, while
misleading or insufficient evidence names ordinary repair work. Retain, revise,
reduce, archive, and remove decisions require independent policy approval, explicit
consumer impacts and acknowledgements, historical semantics, and provenance; archive
or removal remains blocked until cited evidence verifies collection stopped. Prior
queries, findings, conclusions, and lifecycle attempts are never rewritten, and these
records grant no repository, telemetry, collector, data, policy, deployment,
environment, spending, or operational authority.

`executable_runbook_workflow_test.go` is the black-box boundary for the complete
reviewed-procedure-to-proven-recovery loop. It composes public runbook, rehearsal,
and execution APIs while retaining failed preconditions, stale proof, duplicate
alerts, denied approval, unsafe agent performance, interrupted mitigation, revoked
credentials, exact-context shift handoff, failed rollback containment, unmet recovery,
reviewed code and procedure improvements, and fresh revision-matched rehearsal proof.
No runbook record replaces alert, repository, review, agent, deployment, environment,
credential, or operational authority.

Live guided runbook collaboration extends each launch beneath
`$RUNBOOK_EXECUTION_ROOT`. A ready launch freezes the exact published step path and
derives controller, participants, step evidence, pending decisions, bounded
credential references, health, cost, blockers, rollback posture, and predicted next
action. Repository writers use optimistic, idempotent `/controls` to join, discuss,
approve, perform, policy-permitted skip, pause, resume, hand off, abort, or delegate
one step to an approved agent in analyze-only or execute mode. Dependencies,
separation of approval and performance, explicit agent scope, terminal and stale
state, and 15-minute credential bounds precede immutable action receipts. Controls
coordinate existing authority and grant none.

Terminal runbook execution evaluations remain on that frozen launch and test each
declared health, containment, recovery, communication, and rollback criterion against
cited evidence. They retain outcome disposition, deviations, manual work, failed-step
timing, access gaps, agent corrections, total cost, participant feedback, and supported
fitness findings. Exact-version owners may link a supported finding to ordinary
documentation, workflow, policy, infrastructure, or human- or agent-owned code work;
record a reviewed later runbook revision; and attach only current passing rehearsal
proof for that revision. Owner suspension after repeated failure or unsafe drift blocks
new launches and recommendations for the affected revision and names an existing
approved fallback. Historical executions and their procedure version are never
rewritten, and evaluation or learning records grant no connected authority.

Context-bound runbook launches live beneath `$RUNBOOK_EXECUTION_ROOT` (default
`apps/api/data/runbook-executions`). Repository readers can request explained,
non-automatic recommendations for an alert, incident, deployment, failed workflow,
service objective, support thread, or manual observation. Repository writers launch
an exact current runbook version while freezing the origin timeline and audience,
affected resources, signal window, releases, environment state, permitted evidence,
precondition decisions, current access evidence, and rehearsal proof. Stale or missing
proof, current runbook findings, ambiguous matches, failed preconditions, unavailable
dependencies, denied evidence, missing authority, and duplicate executions remain
explicit blockers or choices. Idempotent retries return the original record. Launches
grant no repository, secret, workflow, agent, communication, incident, deployment,
environment, credential, or operational authority. The public API is
`/repositories/{repository}/runbook-executions`.

Revision-bound runbook rehearsals live beneath `$RUNBOOK_REHEARSAL_ROOT` (default
`apps/api/data/runbook-rehearsals`) and beneath each runbook's `/rehearsals` resource.
Repository readers define bounded synthetic or permitted failure scenarios against an
exact runbook version in an isolated or explicitly policy-approved environment.
Human and agent attempts retain inputs, decisions, commands, outputs, timing,
artifacts, cost, permissions, outcomes, and manual gaps. Destructive steps must be
simulated or excluded. Append-only service, dependency, credential, policy, and
runbook-step observations selectively stale affected proof. Rehearsals grant no
repository, secret, workflow, agent, communication, incident, deployment,
environment, credential, or operational authority.

Immutable operational runbooks live beneath `$RUNBOOK_ROOT` (default
`apps/api/data/runbooks`). Repository writers publish versioned procedures for an
exact service, environment, dependency, or signal revision with purpose,
preconditions, diagnostic, action, and human-decision steps, expected evidence,
dependency order, rollback criteria, owners, skills, and escalation paths. Reviewed
commands, workflow components, documentation, and approved agents are pinned as
references; reads preview what each step inspects or changes and which resource
authority and human judgment it requires. Missing step owners, unsafe assumptions,
inaccessible or unreviewed references, secret-bearing input, and inaccessible or
conflicting policy remain attributable findings. The public API is
`/repositories/{repository}/runbooks`, and runbooks appear in the Response repository
view. Runbooks and previews grant no repository, secret, workflow, agent,
communication, incident, deployment, environment, or operational authority.

`on_call_coordination_workflow_test.go` is the black-box boundary for the complete
released-service-signal-to-reviewed-response-learning loop. It composes the public
response policy, rotation, alert workspace, incident, and outcome APIs while retaining
duplicate correlation, a false positive, missed acknowledgement, absent and revoked
responders, failed delivery and retry, noisy dependency evidence, bounded agent access
and budget containment, ordinary mitigation evidence, exact-context handoff, severe
recurrence, consented user recovery, and reviewed signal and runbook work. Response
records coordinate existing authority; they never manufacture paging, repository,
agent, communication, incident, deployment, environment, or operational authority.

Revision-bound response alerts live beneath `$RESPONSE_ALERT_ROOT` (default
`apps/api/data/response-alerts`). Repository writers publish reliability, deployment,
security, privacy, dependency, workflow, and user-impact signals with exact evidence,
affected resources and users, uncertainty, and correlation keys. Alerts freeze the
active response-policy version, derive deadlines and current policy-pinned duty, and
retain deduplication, suppression, maintenance, rate-limit, stale-signal, inaccessible-
evidence, policy-change, and delivery attempts as auditable state. Repository reads and
recipient-filtered reads power the Response view and actionable inbox. Delivery never
constitutes acknowledgement, and alerts grant no repository, secret, communication,
incident, deployment, environment, security, privacy, continuity, governance, or
operational authority. The public API is `/repositories/{repository}/response-alerts`.

An alert's assigned responder acknowledges by opening its durable `/workspace`, which
freezes permitted release, deployment, code, infrastructure, dependency, runbook, and
evidence references beside the signal window. Optimistic workspace actions retain
classification, added correlation keys, invitations, observations, reassignment,
suppression, escalation, and audience. Diagnostics require a distinct approver and may
use only permitted context. A delegated agent receives a 24-hour credential that reads
only selected context and publishes cited findings, questions, or uncertainty; it has
no mitigation or production control endpoint. Qualifying workspaces may create and
link an ordinary incident while preserving the alert timeline and audience controls.

Consent- and audience-bound response outcomes live beneath `$RESPONSE_OUTCOME_ROOT`
(default `apps/api/data/response-outcomes`). Repository writers freeze an exact alert,
policy, rotation, timing, handoff, escalation, missed-target, deduplication, noise,
interruption, responder-load, agent-cost, incident, and user-impact snapshot. User
outcomes require explicit consent and owner-only records do not enter repository or
public projections. Named owners append reviews, signal or routing corrections, and
links to ordinary human- or agent-owned reliability, documentation, automation, or
staffing work. Material authority corrections require a distinct owner's ordinary
approval. Repeated paging or missed coverage, unavailable routing, and unsafe
automation pause only the affected routing scope or activate its declared backup;
they never broaden access. The public API is
`/repositories/{repository}/response-outcomes`, and the Response view reports the
same individual and aggregate evidence without rewriting source alerts or authority.

Durable response duty rotations live beneath `$RESPONSE_ROTATION_ROOT` (default
`apps/api/data/response-rotations`) and are pinned to an exact response-policy version.
Teams publish timezone-aware participants, qualifications, availability, membership
and access observations, handoff windows, layered backups, workload limits, absence
rules, and context-revision-bound shifts. Reads project current and upcoming ownership
and derive accountable escalation for overlaps, gaps, missed handoffs, unavailable or
unqualified responders, workload exhaustion, membership change, and revoked access.
Responders acknowledge inspected context; a swap, delegation, or owner override moves
duty only after the exact recipient accepts the unchanged context revision and
references. The public API is `/repositories/{repository}/response-rotations`, and the
Response repository view exposes the same schedule and decisions. Rotations grant no
repository, team, secret, communication, incident, deployment, environment, security,
privacy, continuity, governance, or operational authority.

Versioned response coverage policies live beneath `$RESPONSE_POLICY_ROOT` (default
`apps/api/data/response-policies`). Repository writers map repository, service,
environment, user-journey, and dependency signal classes and severities to accountable
teams, required skills, response targets, escalation paths, communication audiences,
expected actions, and incident criteria. Immutable versions cite exact organization
membership, service ownership, access, privacy, security, and continuity rules and
retain attributable uncovered resources, conflicting ownership, unavailable teams or
skills, impossible targets, inaccessible rules, and expiring exceptions. The public API
is `/repositories/{repository}/response-policies`, and policies appear in the Response
repository view. They grant no repository, team, secret, communication, incident,
deployment, environment, security, privacy, continuity, or operational authority.

`capacity_planning_workflow_test.go` is the black-box boundary for the complete
accepted-roadmap-outcome-to-verified-capacity loop. It composes stock Git with the
public capacity objective, model, rehearsal, plan, and delivery APIs while retaining
a corrected bad forecast, noisy non-proof, unavailable dependency owner, separately
approved provider quota, stale plan input, ordinary human-agent application and
infrastructure delivery evidence, protected rollout containment, agent budget and
authority limits, scaling regression, unused reservation, and right-sized production
recovery. A containment control remains effective until an authorized operator
resumes from current passing evidence; no capacity record replaces native roadmap,
Git, review, provider, spending, release, or environment authority.

Revision-exact progressive capacity deliveries live beneath `$CAPACITY_DELIVERY_ROOT`
(default `apps/api/data/capacity-deliveries`) and beneath each plan's `/deliveries`
resource. Repository writers stage protected-environment phases pinned to the plan,
objective, model, environment policy, controller, and explicit human operators and
delegated agents. Append-only production observations retain exact release,
infrastructure, schema, and dependency revisions, evidence windows, allocated and
usable capacity, load, forecast, headroom, scaling lag, regional balance, service
levels, dependency health, correctness, reliability, quota, reservation utilization,
and cost. Reads deterministically contain demand shifts, quota denial, imbalance,
lag, regressions, unused reservations, insufficient usable capacity, and budget
breaches and name the connected decision revisit. Operators may stage, pause, resume,
throttle, roll back, or replan; agents may only observe and perform explicitly
delegated stage, pause, or throttle steps. Delivery records grant no spending,
provider, quota, repository, agent, credential, release, environment, deployment, or
operational authority.

Reviewed passing instrumentation revisions roll out beneath `$SIGNAL_ROLLOUT_ROOT`
(default `apps/api/data/signal-rollouts`) and each signal contract's `/rollouts`
resource. A rollout freezes the exact contract, passing telemetry-check run, deployed
and collector revisions, protected environment stages, services, audiences, regions,
traffic percentages, privacy controls, operators, controller, and cardinality,
storage, and query-cost bounds. Append-only production windows retain signal health,
coverage, latency, missingness, sampling bias, cardinality, storage, query cost,
pipeline loss, malformed payloads, sensitive-data detection, collector health, and
service health. Derived privacy exposure and service regressions roll back; malformed
payloads, outages, loss, and budget breaches pause; cardinality and sampling skew
narrow collection. Only named human operators can observe or control a rollout, and
current failing proof blocks resume. Rollout records grant no data, telemetry,
collector, repository, agent, credential, release, deployment, environment, spending,
or operational authority.

Evidence-selected capacity delivery programs live beneath `$CAPACITY_PLAN_ROOT`
(default `apps/api/data/capacity-plans`). Repository writers freeze one objective,
model, rehearsal, and supported candidate into an immutable phased plan with
reservations, procurement/quota/provider dependencies, implementation order, owners,
budgets, decision points, gates, and exit strategy. Append-only owner approvals,
decisions, and connected human- or agent-owned tasks, sessions, workspaces, pull
requests, infrastructure plans, schema changes, dependency negotiations,
observability, operational documentation, releases, and environment changes retain
exact revisions and ordinary gate evidence. Reads keep unapproved reservations,
external dependencies, plan owners, and decisions as explicit gaps. The public API
is `/repositories/{repository}/capacity-plans`, and plans appear in Capacity &
performance. Plan approval grants no spending, procurement, provider, quota,
repository, secret, review, merge, release, environment, deployment, or operational
authority.

Revision-exact capacity rehearsals live beneath `$CAPACITY_REHEARSAL_ROOT`
(default `apps/api/data/capacity-rehearsals`). Repository readers freeze one capacity
objective version, optional model revision, repository-defined scenario revision,
isolated or policy-approved environment, coordinated-load key, explicit duration,
concurrency, rate, and cost bounds, and exact release, infrastructure-plan, schema,
and dependency-configuration candidates. Append-only human or agent attempts retain
synthetic or privacy-preserving workload digests, repetitions, noise, throughput,
latency, errors, saturation, recovery, correctness, resources, declared carbon, cost,
sanitized logs, and artifacts. Reads derive missing proof and classify noisy, unsafe,
incorrect, failed, incomparable, or untestable evidence rather than treating it as
proof. The public API is `/repositories/{repository}/capacity-rehearsals`, and the
repository Capacity & performance surface presents the same comparison. Rehearsals
grant no spending, provider, repository, release, infrastructure, schema, dependency,
environment, credential, deployment, or operational authority.

Revision-exact demand and capacity models live beneath `$CAPACITY_MODEL_ROOT`
(default `apps/api/data/capacity-models`). Repository readers, including read-only
agents, publish immutable forecasts bound to one capacity-objective version, exact
release, forecast window, and sanitized revisioned evidence windows across usage,
performance, reliability, deployment, infrastructure, dependencies, experiments,
and roadmaps. Models retain accountable assumptions, workload segments, explained
saturation points, uncertainty, provenance, alternative demand scenarios, and cost
curves. Reads redact inaccessible evidence bodies and derive instrumentation changes,
anomalous observations, missing citations, and appended forecast disagreements as
explicit gaps. A later model may supersede an earlier ID without rewriting it; human
or read-only-agent challenges are optimistic and append-only. The public API is
`/repositories/{repository}/capacity-models`, and the repository Capacity &
performance surface exposes the same reasoning. Models grant no spending, provider,
repository, release, deployment, environment, credential, or operational authority.

Versioned capacity objectives live beneath `$CAPACITY_OBJECTIVE_ROOT` (default
`apps/api/data/capacity-objectives`). Repository writers define immutable optimistic
contracts for services, APIs, jobs, workspaces, package delivery, or user journeys,
including forecast windows and evidence, traffic shapes, seasonality, service levels,
bottleneck thresholds, dependency limits, regions, owners, budget, lead time, signals,
attributable expiring assumptions, and success and rollback criteria. Links connect
product roadmaps, experiments, performance goals, service objectives, infrastructure,
releases, and funding. Reads derive unsupported forecasts, missing required signals,
conflicting commitments, and expired or soon-expiring assumptions as explicit gaps.
The public API is `/repositories/{repository}/capacity-objectives`, and the repository
Capacity & performance surface shows the same history. Objectives grant no spending,
provider, repository, release, deployment, environment, credential, or operational
authority.

Revision-exact pull-request review plans live beneath `$REVIEW_PLAN_ROOT`
(default `apps/api/data/review-plans`). A pull author or repository owner publishes
immutable optimistic versions whose exact source and target revisions and changed
paths are derived from the pull request and repository objects. Plans retain intent,
risk, policies, affected commitments and context, review areas, expertise, owners,
acceptance questions, evidence, dependencies, and completion rules. Reads preserve
unplanned or overlapping scope, paths outside the change, missing ownership,
inaccessible context, and source or target drift as attributable blockers. The
public resource is `/repositories/{repository}/pull-requests/{pull_request}/review-plans`
and the pull-request web surface has a Review plan section. Plans grant no repository,
review, approval, merge, policy, secret, or operational authority.

Revision-bound review routing lives beneath `$REVIEW_ROUTING_ROOT` (default
`apps/api/data/review-routing`). Repository owners evaluate human and approved-agent
candidates per current review-plan area using permitted code-ownership, project
knowledge, team-responsibility, expertise, availability, capacity, conflict, and
capability evidence. Invitations freeze the exact plan version, pull revision,
paths, questions, deadline, escalation, and reason. Participant acceptance creates
only that bounded review assignment; decline, unavailability, overload, recusal,
maintainer release, and revocation retain attribution and derive reassignment areas.
The public resource is the review plan's sibling `/review-routing` tree and appears
in the pull Review plan section. Routing grants no repository, merge, secret,
policy, governance, or operational authority.

Parallel review work lives beneath `$REVIEW_WORK_ROOT` (default
`apps/api/data/review-work`) and is bound to one current review-plan version,
pull revision, and accepted routing assignment. Its shared queue covers exact
files, diffs, symbols, requirements, checks, previews, accessible context, and
prior decisions; reviewers publish optimistic progress, coverage, cited findings,
uncertainty, discussion, input requests, challenges, and accepted handoffs.
Overlapping coverage, blocked dependencies, and conflicting conclusions remain
visible. Citations must be public or repository-visible and revision-bound, so
private, inaccessible, and embargoed material is rejected rather than propagated.
Approved agents may publish only proposed findings and never satisfy a required
human role. The public sibling resource is `/review-work` and appears in the pull
Review plan section. Work records grant no repository, approval, merge, secret,
policy, governance, or operational authority.

Review-work findings retain append-only classifications, rationale, dissent,
owner-only accepted-risk and bounded-exception decisions, duplicate or superseding
relationships, and exact commit, task, change-session, workspace, or follow-up
links. Repository checks and targeted reproductions record revision-bound outcomes
and accessible citations. An explicit transition between immutable review-plan
versions accounts for every prior finding as addressed, still applicable, or stale
without rewriting its original evidence or discussion; ordinary repository and
agent permissions remain authoritative.

Current review completion lives beneath `$REVIEW_COMPLETION_ROOT` (default
`apps/api/data/review-completion`) and derives a matrix from the exact current
review plan, routing, and shared work. Repository owners select which planned
areas block readiness; each area reports accepted human assignments, inspected
queue evidence, findings and decisions, required acknowledgements, unresolved
gaps, and acknowledgements made stale by changed source, target, risk, ownership,
dependencies, scope, or completion rules. Owner emergency overrides name only
affected areas, expire within seven days, retain attribution and rationale, and
require a follow-up work reference. The sibling `/review-completion` resource and
pull readiness response expose the matrix in the Review plan section. Completion
and overrides grant no review, approval, merge, policy, repository, or operational
authority.

`review_orchestration_workflow_test.go` is the black-box boundary for the complete
cross-cutting pull-request review loop. It composes stock Git, public review plan,
routing, work, and completion APIs, ordinary revision-bound review and checks, the
integration queue, and merge while retaining overload, recusal, inaccessible
evidence, disagreement, handoff, revocation, staleness, changed risk, failed repair,
and recovery. Review completion is passed through pull-request route registration
so the exact current matrix participates in readiness without replacing native
review, check, queue, or merge authority.

Versioned provenance and licensing policies live beneath
`$PROVENANCE_POLICY_ROOT` (default `apps/api/data/provenance-policies`).
Repository and organization owners publish immutable, optimistically
concurrency-checked rules for source, generated code, assets, models, datasets,
packages, and build inputs. Rules retain permitted origins, licenses and uses,
required attribution and contributor attestations, review owners, distribution
contexts, expiring exceptions, and links to contributor pathways, agent
contracts, packages, releases, and private or federated contribution boundaries.
Reads derive unknown licenses, conflicting distribution terms, missing owners,
and expired or soon-expiring exceptions with attribution. The public APIs are
`/repositories/{repository}/provenance-policies` and
`/organizations/{organization}/provenance-policies`; repository policy appears
in `view=governance`, and organization policy appears on `/organizations`.
Policies grant no repository, organization, contribution, agent, package,
release, federation, credential, review, merge, distribution, or operational
authority.

Revision-exact software provenance graphs live beneath
`$PROVENANCE_GRAPH_ROOT` (default `apps/api/data/provenance-graphs`). Repository
writers assemble an immutable graph from an exact visible commit and its
`.komodo/provenance.json` declaration, linking cited files and fragments to
commits, people, agents, tools, assets, dependencies, build inputs and outputs,
upstream projects, licenses, obligations, transformations, and attestations.
File citations bind path and SHA-256 to the exact Git blob; reads retain missing
origins, broken lineage, contradictory claims, stale citations, and rewritten
history as explicit gaps. Anonymous projections replace non-public nodes with
opaque blockers rather than exposing labels, claims, citations, or obligations.
The public API is `/repositories/{repository}/provenance-graphs`, and graphs
appear with policy in `view=governance`. Analysis grants no repository, source,
dependency, agent, build, artifact, distribution, review, merge, release, or
operational authority.

Candidate provenance assessments live beneath `$PROVENANCE_ASSESSMENT_ROOT`
(default `apps/api/data/provenance-assessments`). Repository writers bind an
exact pull request, stack, package, or release candidate revision to one exact
provenance graph, current repository policy version, dependency and tool input
keys, and named distribution targets. Comparisons retain unattributed or
unpermitted origins, incompatible licenses, generated-output uncertainty,
attribution, notice and source-offer duties, contributor attestations, and
owner acknowledgements as individual findings. Repository readers and
read-only agents may append cited challenges or origin evidence; owners record
revision-bound acknowledgement, resolution, or expiring exceptions. Changed
candidate, policy, dependency, tool, or graph inputs stale only decisions bound
to them, and current required assessments participate in pull-request and
release readiness. These records grant no source, Git, policy, exception,
review, merge, release, distribution, credential, or operational authority.

Current provenance findings extend into owner-created repairs beneath the
assessment's `/findings/{finding}/repairs` resource. Each repair freezes the
affected revision, policy version, applicable obligations, acceptance criteria,
permitted finding evidence, human or agent owner, and links to ordinary tasks,
branches, forks, sessions, or workspaces. Replacement, reimplementation,
removal, permission, and isolation strategies remain attributable; clean-room
reimplementation excludes restricted evidence and separates the implementer
from named evidence reviewers. Progress and revision-exact pull/check delivery
remain on the original finding and require an explicit assertion that
authorship was preserved. Repairs grant no source, Git, evidence, agent,
credential, review, merge, disclosure, distribution, release, or operational
authority.

Authorized release provenance bundles live beneath `$PROVENANCE_BUNDLE_ROOT`
(default `apps/api/data/provenance-bundles`). A repository owner publishes one
immutable public or repository-audience claim for an existing release, exact
release revision, current provenance graph and release-candidate assessment,
and exact build artifacts. The Ed25519-signed payload retains artifact digests,
an SBOM with licenses, notices, source and build attestations, dependency
lineage, declared omissions, and independent verification instructions. Public
consumers can download, verify, and compare bundles without repository access,
and public package provenance resolves only a bundle containing that package's
exact artifact digest. Later license changes, revoked attestations, quarantined
packages, provenance drift, or origin gaps append actionable trust notices and
optional propagation-campaign links without changing the signed release claim.
Publication and observation grant no repository, release, artifact, package,
distribution, repair, campaign, credential, or operational authority.

Repository-reviewed collaboration workflow definitions live beneath
`$WORKFLOW_DEFINITION_ROOT` (default `apps/api/data/workflow-definitions`).
Repository writers publish immutable versions with exact source revision and
configuration path, typed event/manual/schedule/webhook triggers, inputs,
conditions, dependency-ordered or parallel steps, outputs, retry and timeout
limits, budgets, owners, policies, and completion criteria. Invocations name an
exact permitted platform action, reusable component, or approved-agent
revision. Reads preview event subscriptions and effective requested authority
without granting it. Dependency cycles and missing steps, emitted-event trigger
loops, inaccessible resources, missing owners, and deny-policy conflicts remain
attributable activation blockers. Only a declared owner can activate the exact
current version; every revision returns the workflow to draft. The public API
is `/repositories/{repository}/workflow-definitions`, and the repository web
surface is `view=agents`. Definitions and activation grant no event, agent,
component, repository, credential, merge, release, deployment, environment, or
operational authority.

Workflow governance is version-bound in the same resource. Definitions may
require exact simulated event cases, named independent review, resource-owner
acknowledgement, and action-class rules with separation of duties, approval
quorum, and bounded expiry. Activation remains blocked until current passing
simulations and decisions exist. Executions retain approval requests and
immutable action receipts; expired approvals cannot dispatch. Owner emergency
disablement and authority-changing revisions pause active effects and revoke
their step credential references, while rollback publishes a prior definition
as a new draft version. Exceptions expire without erasing decisions, receipts,
completed executions, or legitimate outputs.

Active workflow versions may receive durable invocations beneath their
`/executions` resource. Each execution freezes the definition and repository
revision, triggering event revision, attributed human, agent, or system actor,
typed inputs, exact permitted action/component/agent revisions, and current
repository, organization, agent, embargo, environment, and approval decisions.
Duplicate idempotency keys return the original run; declared concurrency, rate,
step, and workflow cost limits prevent unbounded work. Dependency-ready steps
receive only a reference to a credential scoped to that step's exact resource
and capabilities, expiring no later than its timeout or 15 minutes, with no
secret retained. Only declared, accessible, non-secret typed outputs reach
dependent steps. Attempts and events survive process interruption; retry limits,
budget exhaustion, cancellation, access revocation, and stale inputs retain a
deterministic terminal or blocked state and revoke active step credentials.
Execution records still grant no repository, organization, agent, component,
embargo, environment, approval, credential, merge, release, deployment, or
operational authority.

Execution reads form the shared live graph in `view=agents`: frozen provenance,
step dependencies and status, all attempts, typed inputs and accessible outputs,
sanitized logs, redacted artifact metadata, exact agent sessions, costs, timing,
approval/input waits, failures, and derived next actions remain durable after
completion. Repository writers may append attributable pause, resume, cancel,
retry, definition-optional skip, non-secret requested input, named-owner
approval, or declared-manual-step takeover controls. Active credentials are
revoked on pause or invalidation; restricted artifacts retain only a redacted
digest record, and credential-shaped logs, inputs, and session data are rejected.

Reusable collaboration components live beneath `$WORKFLOW_COMPONENT_ROOT`
(default `apps/api/data/workflow-components`). Maintainers publish immutable
versions from their own repository with an exact package version, source
revision and path, artifact digest, attestation, typed input/output contracts,
requested capabilities, engine compatibility, data-use terms, passing
revision-matched tests, support policy, and local or federated publisher
provenance. Consumer installations freeze the full selected component into an
ordinary pull-request revision and require every requested capability to map to
an explicit local permission and resource plus non-secret local configuration.
Each update appends a new installation revision, so retaining or replacing a
pin never rewrites its history or any workflow execution. Reads derive changed
publisher, revoked trust, unavailable peer, vulnerable version, and breaking
compatibility blockers from the latest observation while preserving every
prior source and trust snapshot. The public APIs are
`/repositories/{repository}/workflow-components` and
`/repositories/{repository}/workflow-component-installations`; inspection and
local pins appear in `view=agents`. Publications and installations grant no
package, federation, repository, pull-request, credential, workflow, action,
agent, environment, or operational authority.

`workflow_automation_workflow_test.go` is the black-box boundary for the
complete accepted-issue-to-protected-deployment automation loop. It connects a
reviewed exact workflow version to an attributed accepted issue, bounded agent
repair, stock Git revision, ordinary pull request, human review, required
checks, merge queue, attested release artifact, and separately approved
protected deployment. The retained execution proves duplicate-event
idempotency, stale-revision rejection, pause and credential revocation, failed
and interrupted retry, denied consequential approval, budget containment, and
restart-safe history. Its published repair component is installed in a second
repository with narrower local mappings, while a breaking upgrade observation
preserves the working pin and completed run. None of these links replaces the
native authority checks of the connected resource.

Software adoption workspaces live beneath `$ADOPTION_WORKSPACE_ROOT` (default
`apps/api/data/adoption-workspaces`). Authenticated collaborators open one from
an exact roadmap outcome, support gap, incubator, decision, package, API, or
federated repository and declare required journeys, environments, constraints,
budget, owners, and evaluation criteria before comparing exact candidate
versions. Owners invite consenting provider maintainers, affected users, and
strictly read-only agents with explicit evidence scope. Candidate capability,
provenance, support, security, data-use, compatibility, and gap evidence retains
its revision, availability, visibility, and validity; reads derive missing,
stale, expired, unavailable, and inaccessible evidence rather than presenting it
as proof. Workspaces grant no repository, package, API, agent, credential,
procurement, selection, trial, integration, deployment, or operational authority.
The public API is `/adoption-workspaces`, the web surface is `/adoptions`, and
`adoption_workspace_workflow_test.go` is the black-box boundary for the complete
evaluation-to-upstream-improvement loop, including failed and inaccessible fit,
bounded trials, delivery recovery, upstream acceptance and rejection, and an
exact verified consumer update.
Accepted workspace participants may assemble candidate-revision-bound trials
from attested releases or exact revisions, scoped packages and APIs, synthetic
or explicitly permitted data, declared journeys, policies, sanitized setup,
configuration, and commands, and a budget. Append-only attempts retain
integration changes, checks, previews, measurements, cost, findings,
content-addressed artifacts, reproducibility, and attributable user feedback.
Public projection retains failed and non-reproducible attempts while reducing
provider- or consumer-scoped evidence to an inaccessible blocker; credential-
shaped configuration and commands are rejected. Agents may run bounded trials
but cannot submit user feedback or turn their recommendation into fit proof.
Adopter owners and consented provider maintainers may turn a current passing
trial into an immutable candidate-version integration plan. Plans retain the
architecture, configuration decision ownership, update and support policy,
service and data boundaries, required exceptions, exit strategy, unresolved
fit gaps, recurring cost, compatibility promises, and dependency-ordered human
or agent work across consumer repositories, environments, documentation, and
explicitly permitted provider forks. Previews derive effective access,
accountable owners, cost, and open decisions; plan work grants neither side
repository, secret, deployment, roadmap, or operational authority.
Consumer owners may connect an immutable integration plan to an exact consumer
pull revision containing pinned dependencies and categorized integration work.
Delivery records retain exact provider and consumer revisions, attestations,
ordinary approval, review, policy, rehearsal, and release evidence, support and
user acceptance, and ordered staged-rollout health and cost. Failed criteria,
incompatibility, unhealthy rollout, or revoked access pause adoption; only an
adopter owner may record restoration, and neither providers nor agents gain
merge or environment authority.
Adopter owners may propose redacted trial findings, reproductions, support
questions, compatibility evidence, documentation feedback, and usage outcomes
for an accepted provider maintainer's explicit consent. Each share retains exact
workspace evidence references and a public, participant, provider, or embargoed
audience; inaccessible and embargoed bodies are projected as blockers, and
credential-shaped text is rejected. Consented evidence may accompany ordinary
provider issues or exact-revision local, fork, or federated pull requests.
Contribution links retain human or agent authorship, contributor guidance,
review, checks, security evidence, provider decisions, and a safe local fallback
without granting repository or merge authority. A maintainer-accepted exact
release can replace consumer patches only through an adopter-owner verified
update with revision-matching attestation, approval, review, policy, rehearsal,
release, support, and user evidence. Rejection, embargo, or provider outage does
not erase local resolution or expose consumer evidence.

Responsible public-life records live beneath `$PROJECT_LIFE_ROOT` (default
`apps/api/data/project-life`). A project enters public life only from the exact
current ready launch and retains attested release, documentation, package, API
contract, contributor-opportunity, and governed-environment publications for a
declared audience. Append-only adoption, support, reliability, cost, and success
measure observations cite existing product or operational evidence. Attributable
feedback may revise the roadmap or open connected human- or agent-owned work.
Owners explicitly record whether the incubator graduates to an organization
initiative, remains experimental, merges into an existing project, or is
archived; graduation, merge, and archive require every resource and obligation
to carry a cited resolution. These coordination records grant no publication,
Git, agent, package, environment, release, deployment, or operational authority.
The public API is `/project-life`.
`project_incubation_workflow_test.go` is the black-box boundary for the complete
shared-need-to-continuing-stewardship loop across the incubator, boundary,
delivery, readiness, and public-life resources.

Accepted incubator directions may open activation-gated project boundaries
beneath `$PROJECT_BOUNDARY_ROOT` (default `apps/api/data/project-boundaries`).
The manifest must include or connect organization, repository, team, package,
agent-role, contributor-pathway, documentation, environment, and baseline
review, security, privacy, quality, and release-policy resources. Public
previews derive ownership, effective access, recurring cost, missing kinds,
generated-content provenance, inherited policy, and exact-revision owner
approval blockers. Activation assigns every resource handle atomically;
rollback releases created handles and preserves attempts for retry without
exposing credentials or granting Git, agent, environment, merge, release,
deployment, or operational authority. `project_bootstrap_workflow_test.go` is
the black-box boundary for acceptance, approval, activation, rollback, and
retry.

First-slice delivery records live beneath `$PROJECT_DELIVERY_ROOT` (default
`apps/api/data/project-deliveries`). An active boundary and its exact accepted
incubator alternative may define a dependency-ordered code, test,
documentation, infrastructure, and interface plan plus an expiring human-agent
team. Reproducible workspaces bind boundary repository handles, base revisions,
definition digests, and commands; connected pull requests retain exact
revisions, authorship, ordinary checks, and distinct review. A revision-exact
preview invites named target users to append evidence, while agent actions,
handoffs, costs, and deviations report to the incubator delivery record.
Changed preview or pull revisions reject stale evidence and decisions. These
records grant no Git, agent, workspace, preview, repository, credential,
review, merge, deployment, environment, or operational authority.
`project_delivery_workflow_test.go` is the black-box boundary for the first
running product slice.

Initial public-life readiness records live beneath `$PROJECT_READINESS_ROOT`
(default `apps/api/data/project-readiness`). A record binds an exact accepted
incubator direction, active boundary revision, proven delivery revision, and
declared launch revision to evidence for ownership, support and governance,
licensing and provenance, security and privacy, accessibility, documentation,
package or API adoption, service objectives, continuity, contributor setup,
operating budget, prototype debt, and user validation. Required category owners
accept the current evidence digest or create a 90-day-or-shorter exception with
an explicit narrower scope and connected follow-up work. Reads derive missing
maintainers and evidence, unsafe defaults, unsupported promises, failed user
validation, stale decisions, and expired exceptions as blockers or launch-scope
narrowing. Records grant no repository, release, deployment, credential,
environment, or operational authority. `project_readiness_workflow_test.go` is
the public HTTP regression boundary.

Pre-repository project incubators live beneath `$PROJECT_INCUBATOR_ROOT`
(default `apps/api/data/project-incubators`). Authenticated collaborators open
one from exact product feedback, a support gap, a governed proposal, or a new
idea and record the affected audience, problem, desired outcome, constraints,
success measures, sponsors, decision rights, and participant-only or public
visibility without creating a repository. Source references that the creator
cannot read remain explicit inaccessible gaps and no source body is copied;
matching audience-and-problem records report symmetric possible duplicates.
Accepted human participants may append attributable discussion, public or
participant evidence, assumptions and their dispositions, and complete
before/after scope changes. Invitations and consent remain explicit; an agent
invitation must name an exact currently active repository or organization
onboarding identity. Incubators and participation grant no repository, Git,
agent, credential, governance, approval, merge, deployment, environment, or
operational authority. The pre-repository web surface is `/incubators`.
Accepted incubator participants compare durable candidate project shapes across
product boundaries, architectures, interfaces, dependencies, licenses,
operating costs, security/data risks, and build-versus-adopt choices. Each
alternative may cite exact public or organization-visible decision, prototype,
package, API, and code-intelligence resources. Attributable research,
measurements, dissent, unknowns, supersession, and input-digested experiment
attempts remain attached to the incubator; reproducibility links retain the
original attempt. Experiments declare commands, inputs, success criteria,
budget, and a safety boundary and grant no code, repository, infrastructure,
environment, credential, deployment, or operational authority.

Revision-exact compliance impact assessments live beneath
`$ASSURANCE_ASSESSMENT_ROOT` (default
`apps/api/data/assurance-assessments`). Repository writers bind an exact pull
request, infrastructure plan, schema migration, extension installation, package
update, or release candidate to one assurance-program version and identify its
affected controls, changed evidence, required owner acknowledgements, tests,
notices, retention actions, mitigations, exceptions, residual risk, and exact
input keys. Repository readers and read-only agents may add only public or
repository-visible cited challenges, analysis, alternatives, mitigations, and
residual risk; restricted evidence is rejected. Only named control owners decide
applicability. Changed candidates, policies, dependencies, obligations, or
program versions stale only decisions bound to those inputs, while current
required controls derive merge and release readiness blockers. Assessment
records grant no Git, evidence, exception, approval, merge, release, deployment,
credential, environment, or operational authority. The repository web surface
is `view=assurance`.

Bounded independent reviews live beneath `$INDEPENDENT_ASSESSMENT_ROOT`
(default `apps/api/data/independent-assessments`). Repository owners open one
time-bounded assessment against an exact assurance-program version, named
controls, systems, releases, period, and immutable evidence-package IDs, then
issue an identified internal or external assessor a separately expiring
credential. The credential opens only `/independent-assessor/context` and its
attributable event append: questions, samples, walkthrough requests,
attestation verification, evidence requests, findings, disagreements, and
appeals. Owners answer and resolve through the repository assessment resource;
conflict disclosures, unavailable selected evidence, scope-change history,
contested findings, and appeal decisions remain explicit. A scope change
invalidates every existing invitation so the owner must deliberately re-invite
the assessor. Evidence projection exposes only selected repository-audience
sanitized records and reports inaccessible packages without identifying
restricted resources. Assessment access grants no repository, Git, secret,
credential, production, environment, approval, merge, release, deployment, or
operational authority. Owner management remains in `view=assurance`; the
credential-only external web surface is `/assessments`.

Finding remediation and signed assurance statements live beneath
`$ASSURANCE_DELIVERY_ROOT` (default `apps/api/data/assurance-delivery`). Only
the assessment owner converts an attributable independent-assessment finding
into dependency-ordered human- or agent-owned tasks, sessions, workspaces,
pull requests, policy changes, or operational work. Each item retains the
finding control, permitted evidence-package IDs, affected revision, deadline,
acceptance criteria, resource links, and attributable progress. Closure
requires every item to complete in order, a passing verification on the exact
affected revision and selected evidence digest, and an explicit owner or
currently credentialed assessor disposition. Later revision drift or a reopen
invalidates closure without deleting its prior verification or disposition.
Owners may publish Ed25519-signed public or repository-audience statements for
one exact assurance-program version, release revision, scope, period, controls,
exceptions, evidence digest, and expiry. Reads derive current, changed,
expired, or revoked status while preserving the originally signed payload;
repository-audience statements are rejected from anonymous reads. These
records grant no Git, task, session, workspace, policy, operational, evidence,
approval, merge, release, deployment, credential, environment, certification,
or production authority. The repository web surface is `view=assurance`.
`compliance_assurance_workflow_test.go` is the black-box boundary for the
complete obligation-to-verifiable-assurance loop. It retains missing and
restricted evidence, stale control decisions, rejected exceptions, denied
assessor evidence, contested findings, expiring access, exact human-agent
repair delivery, signed release claims, post-publication drift, and revocation.

Revision-bound design-time threat models live beneath `$THREAT_MODEL_ROOT`
(default `apps/api/data/threat-models`). Repository readers open models from an
exact design proposal, pull request, API or schema evolution, infrastructure
plan, or product experiment and bind code, architecture, dependency, and trust-
boundary inputs. Models retain entry points, privileges, data flows,
dependencies, attacker goals, abuse paths, mitigations, alternatives, residual
risk, and affected owners. Repository readers and read-only agents may add only
public or repository-visible cited findings, challenges, assumptions, and
alternative comparisons; restricted evidence is rejected rather than copied.
Only named owners acknowledge or request changes. Changed bound inputs make
affected analysis and acknowledgements explicitly stale, and pull-request code
drift is derived from the current source revision. These records grant no
repository write, secret, credential, security approval, review, merge,
release, deployment, environment, provider, or operational authority. The
repository web surface is `view=security`.

Named threat-model owners classify cited findings as confirmed, suspected
duplicates, false positives, or accepted risks and select an owners-only,
repository, public, or embargoed audience. Only confirmed current findings may
open ordinary human- or agent-owned proposal tasks, preloaded with the exact
threat revision, abuse paths, audience-permitted citations, and acceptance
criteria. Verification links a distinct repair pull revision, design changes,
commits, review, mitigation coverage, and an approved security scenario whose
attempt history demonstrates failure on the exact base and containment on the
repair. Duplicate, false-positive, accepted-risk, embargoed, and failed-repair
paths remain attributable; none grants implicit Git, secret, environment,
review, or merge authority.

Executable abuse and defense scenarios live beneath `$SECURITY_SCENARIO_ROOT`
(default `apps/api/data/security-scenarios`). Repository readers translate one
exact threat-model abuse path into immutable reviewed versions with attacker
preconditions, explicitly bounded capabilities, synthetic credential-free
fixtures, actions, and observable containment, detection, and recovery
criteria. Attempts bind the current scenario and candidate revision to an
ephemeral networkless workspace or exact pull-request preview and retain only
sanitized commands, logs, traces, content-addressed artifacts, coverage, cost,
provenance, and attributable blockers. Passing evidence requires complete
three-domain coverage and rejects destructive effects, secrets, production
data, hidden test material, inaccessible dependencies, and unsanitized
artifacts; unsafe, blocked, and non-reproducible attempts remain visible rather
than becoming success or disappearing. Scenario collaboration and evidence
grant no repository write, secret, credential, security approval, deployment,
preview, environment, or operational authority. The repository web surface is
`view=security`.

Current security delivery policy lives beneath `$SECURITY_DELIVERY_ROOT`
(default `apps/api/data/security-delivery`). Repository and organization
owners scope immutable requirements by branch, component, asset, and risk
class to exact threat models, complete scenario attempts, named control-owner
acknowledgements, and resolved confirmed findings. The same revision-exact
assessment blocks pull merge, integration-queue publication, release creation,
and deployment promotion while retaining gaps, attempt provenance, residual
risk, and requirement-scoped owner overrides that expire within 30 days.
Sanitized deployment signals bind a release, revision, environment, assumption,
control, and affected input keys; violated assumptions or failed controls can
be connected by an owner to a private incident, advisory, or repair without
copying production evidence. Policies, exceptions, monitoring, and responses
grant no agent merge, disclosure, release, deployment, credential, environment,
or production authority. The repository web surface remains `view=security`.
`security_assurance_workflow_test.go` is the black-box boundary for the complete
expectation-to-sustained-defense loop. It retains revision-bound redesign,
unsafe and inaccessible scenario evidence, failed repairs, ordinary review and
delivery assessments, changed post-release assumptions, and exact connected
repair evidence without allowing stale analysis to become current proof.

Versioned security expectations live beneath `$SECURITY_EXPECTATION_ROOT`
(default `apps/api/data/security-expectations`). Repository writers publish
immutable, optimistically concurrency-checked definitions for repository,
service, interface, package, extension, environment, and user-journey scopes.
Definitions retain protected assets, trust boundaries, actor capabilities,
abuse cases, required controls and guarantees, accountable owners, severity
policy, expiring approved exceptions, and design, privacy, infrastructure, API,
quality, and release commitments. Reads derive missing ownership,
contradictory crossings, unsupported guarantees, and expired or soon-expiring
exceptions with attributable authors and exception owners rather than hiding or
resolving them. Publication grants no repository, secret, credential, review,
release, deployment, environment, or security-approval authority. The
repository web surface is `view=security`.

Versioned infrastructure inventories live beneath `$INFRASTRUCTURE_STATE_ROOT`
(default `apps/api/data/infrastructure-state`). Repository writers publish
immutable definitions pinned to an exact source revision and path, describing
environments and their release mappings plus services, networks, identities,
data stores, compute, and external dependencies. Resources retain accountable
owners, providers, dependency edges, configuration boundaries, and cost,
capacity, security, privacy, reliability, continuity, and regional commitments.
Append-only provider observations bind one definition version, source revision,
environment, optional release, evidence reference, and validity window to only
sanitized resource status, region, capacity, and configuration-state metadata.
Reads explicitly derive missing or conflicting ownership, secret-backed
boundaries, missing or stale observations, inaccessible providers, and unmanaged
resources. No credential or sensitive configuration value is accepted or
exposed, and definitions and observations grant no provider, deployment,
environment, credential, or operational authority. The repository web surface
is `view=infrastructure`.

Pull-request infrastructure change plans live beneath `$INFRASTRUCTURE_PLAN_ROOT`
(default `apps/api/data/infrastructure-plans`). Repository writers create an
immutable plan only for the pull request's exact current source revision and
bind exact infrastructure definition versions and permitted observation IDs.
Plans classify each resource as create, change, replace, or destroy; derive a
dependency order; retain availability, security, privacy, continuity, cost,
and data risks, policy effects, assumptions, affected owners, and rollback
limits. Repository readers and read-only agents may add attributable assumption,
impact, investigation, and concern annotations and request resource-owner
acknowledgements through the pull request. A changed pull revision, definition,
latest observed state, or attributable source/provider/policy/observation
invalidation makes the plan and all acknowledgements stale. Plans and
collaboration grant no provider, credential, deployment, environment, policy,
approval, or execution authority.

Infrastructure plan rehearsals remain beneath the immutable plan. Repository
writers bind an isolated or policy-approved ephemeral environment, expiring
provider credential reference and least-privilege scope, synthetic or expressly
permitted privacy-preserving state, every planned resource's supported,
unsupported, or untestable-destructive classification, and repository-defined
provisioning, connectivity, access-boundary, policy, service-journey,
failure-behavior, cost, teardown, and recovery checks. Attempts retain only
sanitized logs, content-addressed artifacts, timing, resource graphs, cost,
runner and teardown/recovery attestations, and attributable agent actions.
Changed plan inputs make evidence non-current; failed teardown or recovery,
failed checks, unsupported resources, and destructive effects that cannot be
safely tested remain explicit blockers rather than rehearsed success. No secret
credential value or production data is retained, and rehearsal evidence grants
no provider, deployment, environment, approval, or production authority.

Authoritative infrastructure executions remain beneath the exact merged plan.
Only a repository owner may bind a current plan, current passing rehearsal,
every affected owner acknowledgement, satisfying policy effects, and a governed
deployment environment to a provider credential reference that expires within
24 hours, explicit provider scopes, controller, and cost ceiling. Environment
approval requirements gate start. Runs expose the candidate and merge
revisions, dependency-ordered resource states, active controller, sanitized
provider responses, health, cost, blockers, next actions, and an append-only
event history. Owner controls may start, pause, resume, or cancel only at
declared safety points. A delegated agent may update only its named unexpired
step, can never perform a destroy step, and receives no secret value, approval,
unrelated provider, repository, deployment, or broader operational authority.
The public API is beneath each plan's `/executions` resource and the repository
web surface remains `view=infrastructure`.

Completed infrastructure applies do not imply convergence. Append-only
verifications beneath an execution bind permitted post-apply observations,
compare every planned resource, and separately retain service, security,
privacy, cost, and continuity outcomes. Partial and failed attempts remain
visible and move the execution to diverged; only complete matching evidence is
verified convergence. Permission-aware monitoring binds later observations and
derives configuration drift, unmanaged changes, failed cleanup, provider loss,
and credential expiry alongside attributable causes. Owners may link incidents
or exceptions and human- or agent-owned repair, adopt legitimate divergence
through an exact-revision ordinary review, or request restoration with explicit
environment policy. These records never rewrite external changes or grant
provider, credential, incident, exception, review, merge, deployment, repair,
or environment authority.
`infrastructure_workflow_test.go` is the black-box regression boundary for the
complete proposal-to-reconciled-infrastructure loop. It retains application and
infrastructure code in one pull request, exact plans, security and service-owner
decisions, scoped agent analysis and execution, failed teardown, stale evidence,
expired credentials, budget and provider containment, partial-apply recovery,
five-domain convergence evidence, out-of-band drift, and an ordinary reviewed
agent repair without granting hidden provider or environment authority.

Durable state contracts live beneath `$DURABLE_SCHEMA_ROOT` (default
`apps/api/data/durable-schemas`). Repository writers publish immutable,
optimistically concurrency-checked schema versions for databases, queues,
indexes, object stores, caches, event logs, and other persistent stores. Every
version binds an exact reviewed source revision and definition path to typed,
privacy-classified fields, accountable owners, compatibility guarantees,
retention and privacy commitments, and service, environment, policy, or
documentation links. Reads retain missing service and environment links rather
than treating publication as operational authority. Collaborators open schema
migrations from a pull request or decision, freezing exact from/to schema
versions and classified reads, writes, backfills, destructive operations,
affected consumers, rollback limits, dependency-ordered steps, success
measures, and required owners. Owner approvals or rejections are attributable
and append-only. Migration work items bind ordered schema, compatibility,
backfill, verification, or cleanup tasks, agent sessions, and workspaces to an
independently permission-checked repository, exact base revision, owner,
allowlisted paths, bounded context, and acceptance criteria. Linked ordinary
pull requests freeze their exact revision and old/new reader and writer
contracts, rollout flags, idempotency, transformations, owners, and rollback
assumptions. Neither link copies restricted schema context or grants access to
its participant. These records grant no database, queue, deployment,
environment, credential, review, or merge authority. The repository web surface
is `view=state`.

Migration rehearsals remain beneath the migration record in
`$DURABLE_SCHEMA_ROOT`. Repository writers bind synthetic or explicitly
privacy-preserving representative dataset metadata to exact application,
schema, migration-definition, data-shape, and dependency revisions plus bounded
duration and cost. Repository-defined upgrade, dual-read/write, backfill,
validation, rollback, and failure-injection checks retain only sanitized logs,
aggregate row/object counts, invariants, performance, artifact digests, costs,
and runner attestations. Readers and scoped agents may add cited investigation
notes and humans may attest an attempt; these actions grant no execution or
operational authority. Rebinding one input marks only results whose declared
input keys include it stale, preserving unaffected evidence and all prior
attempts.

Live schema migrations remain beneath the migration record. Repository writers
may start one only after every required owner approval and a current passing,
accepted rehearsal, binding an exact active revision and an established governed
release environment to a human controller or an agent's exact delegated schema
work item. Runs advance in order through expand, deploy, backfill, cutover, and
contract with optimistic revisions and retain aggregate progress, lag,
invariants, service health, privacy status, cost, deployment evidence, blockers,
next actions, and attributable controls. Writers can pause, resume, throttle, or
abort only before irreversible cutover; agents cannot infer database,
deployment, destructive, or broader repository authority from the run.
Failures pause at a captured phase, progress, and observation. Attested
aggregate recovery points and append-only actions distinguish idempotent retry,
restore, compatibility-window traffic rollback, and connected human- or
agent-owned repair; agents cannot restore or redirect traffic. Revoked owner
approval also pauses active work. Completed runs retire temporary compatibility
code and obsolete fields through a separately versioned, observation-period and
owner-approved record. Verified completion inventories retained and deleted
data, deletion evidence, irreversible choices, exceptions, cost, and the exact
current schema in every declared environment without rewriting failed attempts.
`schema_migration_workflow_test.go` is the black-box regression boundary for
the complete proposal-to-verified-cleanup loop. It retains reviewed schema and
privacy intent, bounded human-agent work, failed and accepted safe-state
rehearsals, conflicting legacy writes, invariant and interrupted-backfill
safety pauses, forward recovery after cutover, governed deployment evidence,
and owner-approved deletion without granting agents hidden operational
authority.

Revision-exact agent behavior contracts live beneath `$AGENT_PROJECT_ROOT`
(default `apps/api/data/agent-projects`). Repository writers publish immutable,
optimistically concurrency-checked versions linking reviewed prompts,
instructions, tools, models, knowledge sources, dependencies, memory and data-use
policy, tasks, outputs, prohibited actions, budgets, owners, escalation rules,
and deployment boundaries to one exact repository revision and definition path.
Reads derive effective declared tool capabilities plus attributable missing
owners, inaccessible dependencies and knowledge, conflicting prompt/instruction
content, and unsupported model guarantees. Publication is reviewable intent only:
it grants no agent identity, repository, tool, model, knowledge, secret,
credential, evaluation, deployment, environment, merge, release, or operational
authority. The repository web surface is `view=agents`.

Representative agent behavior scenarios live beneath `$AGENT_SCENARIO_ROOT`
(default `apps/api/data/agent-scenarios`). Repository writers publish immutable,
optimistically checked versions from ordinary branches or workspaces, binding
an exact agent-project version and repository revision to cited issues, support
threads, tasks, incidents, decisions, or sanitized prior sessions. Cases retain
inputs, explicitly permitted context, expected outcomes, visible or protected
rubrics, prohibited behavior, budgets, uncertainty, required human judgment,
domain owners, licenses, provenance, and explicit allowed uses. Public reads
redact protected inputs, sources, provenance, context, expectations, review
rationales, and hidden criteria. Personal data,
unsanitized sessions, inaccessible or unlicensed sources, and public embargoed
context are rejected; scenario evaluation permission never implies training or
broader-evaluation permission. Only named owners or the exact scoped agent may
review the current version, and a revision makes prior approval non-current.
Scenario records grant no Git, workspace, agent, training, evaluation, secret,
credential, review, merge, deployment, environment, or operational authority.
The repository web surface remains `view=agents`.

Public agent profiles live beneath `$AGENT_PROFILE_ROOT` (default
`apps/api/data/agent-profiles`). Authenticated operators publish immutable
versions with a server-generated stable agent identity, collision-safe public
handle, ownership, task/tool and model provenance, remote execution and
subprocessor boundaries, data-use/retention terms, pricing/resources, requested
capabilities, availability, support, and change reason. Anonymous reads expose
the complete history and keep operator claims separate from the platform's
narrow authentication and schema-validation evidence. Publishing a profile
grants no repository, secret, credential, review, merge, environment, or
operational authority. The repository web discovery surface is `view=agents`.
Explainable agent comparisons live beneath `$AGENT_DISCOVERY_ROOT` (default
`apps/api/data/agent-discovery`). Repository readers bind a task, proposal,
issue, decision, incident, stewardship mandate, or team role to explicit
workflow, permission, deployment, policy, cost, availability, and comparable-
work constraints. Results are alphabetical rather than scored and retain every
match or conflict, missing or stale evaluation/outcome evidence, and declared
conflicts of interest. Repository writers may attach attributable public or
repository-reader evaluation and outcome observations; anonymous share links
omit the work identifier, creator, private evidence, and private evidence's
indirect effect on gap reporting. Discovery remains non-authoritative.

Bounded project agent evaluations live beneath `$AGENT_EVALUATION_ROOT`
(default `apps/api/data/agent-evaluations`). Repository writers publish
optimistically concurrency-checked immutable suite versions whose sanitized
scenarios freeze exact source revisions, expected outcomes, visible and hidden
correctness or policy checks, budgets, prohibited actions, and human-review
criteria. Candidate trials freeze exact suite, scenario, repository, and agent
profile versions and always declare an isolated authority set with publish,
secret, merge, and environment access disabled. Results retain outputs, tool
actions, artifacts, check summaries, costs, latency, failures, contamination,
budget and policy failures, reproducibility notes, and attributable human
decisions. Reader projections redact hidden expectations and canaries;
server-derived labels distinguish repeated, reproduction, and operator-supplied
trials from first-party evidence. Evaluation grants no durable agent,
repository, credential, review, merge, or operational authority. The repository
web surface remains `view=agents`.
Pull-request agent candidates live in the same store and are assembled only for
the pull's exact current source revision from one exact agent-project behavior
contract plus selected immutable suite scenarios. The candidate digest binds
prompt, instruction, tool, model, knowledge, scenario, and judge revisions as
independently keyed inputs. Isolated attempts declare the exact input subset,
bounded environment, and simulated or permitted services and retain sanitized
traces, tool actions, outputs, artifacts, evaluator decisions, repeated metric
samples, 95% limits, contamination, and nondeterminism. A successor candidate
may reuse an attempt only when every input key that attempt declared is
unchanged; affected evidence remains attributable on the prior candidate.
Pull-request `section=agent-evaluations` compares a current candidate with an
explicit baseline across task success, policy adherence, human corrections,
uncertainty, latency, and cost without converting an aggregate into authority.
Candidate pilots live in the same store beneath repository
`/agent-evaluations/pilots`. An owner publishes one exact pull-request candidate
to selected repositories, roles, named participants, tasks, expected outcomes,
expiry, and a cost ceiling with only read and draft actions. Invited
collaborators explicitly accept or revoke consent, delegate scoped sessions,
guide or stop work, and append candidate-revision-bound feedback and
corrections. Reads retain live session events, drafts, escalations, policy
denials, costs, and expected-outcome comparisons. Expiry, exhausted budget,
revoked consent, unsafe behavior, owner pause, or pull revision drift derives a
paused pilot without deleting prior evidence. A pilot always projects merge,
deploy, disclosure, and authoritative mutation as false and grants no durable
agent identity, Git, review, release, credential, environment, or operational
authority.

Evaluated-agent onboarding records live in the same store beneath repository
`/agent-evaluations/onboardings` and organization `/agent-onboardings`
resources. Owners bind exact clean, accepted trial and profile versions to
roles, named resources and actions, data boundaries, budget, schedule,
approvers, operator agreement, policy exceptions, and a human sponsor for
consequential decisions. Preview derives every activation blocker and explicit
non-authority before activation. Activation creates a distinct scoped
`agent:repository:*` or `agent:organization:*` subject; upgrades require fresh
version-bound decisions and agreement, while denials, old versions, expiry,
activation, and revocation remain visible. Financial, stewardship, governance,
team, or organization standing never contributes implicit technical access.
Activated onboarding trust lives with the same immutable installation record.
Repository/organization readers see only bounded work references and sanitized
summaries for attributable task outcomes, reviewer corrections, verification
failures, reversions, security/policy violations, accepted contributions, cost,
and responsiveness—never task bodies, prompts, logs, or secrets. Owners version
periodic suite requirements and thresholds; overdue or failed reevaluation,
violations, deteriorating results, and anomalous cost produce actionable notices.
Optimistically checked controls narrow, suspend, resume, or revoke effective
authority while retaining commits and evidence. Active-work replacements use
structured, accepted handoffs between independently activated agent identities.
Consent stays pinned to the activated profile version; comparing versions marks
operator, model, data-use/execution, capability, and price/resource changes as
material, requiring the ordinary fresh onboarding version, approvals, agreement,
and activation before that newer profile governs authority.
The profile operator authenticates and accepts onboarding terms through the
public operator-agreement route as itself; repository and organization owners
cannot impersonate that consent and retain every approval and activation
decision. `agent_collaboration_workflow_test.go` is the black-box boundary for
the complete public-profile-to-merged-contribution loop. It retains comparison,
hidden and prohibited evaluation failures, cost containment, scoped team and
session work, ordinary review and merge, profile drift, operator outage, failed
reevaluation suspension, and an independently activated replacement handoff.
Attested collaborator releases live with the same evaluation store beneath
repository `/agent-evaluations/releases`. A release freezes an active onboarding
identity and consented profile, accepted trials and pilot, exact behavior
contract and repository revision, model and tool versions, and operator terms.
Publication requires attributable domain, pilot, data-policy, and resource
decisions. Deployments remain subsets of onboarding roles, resources, and
actions, retain only scoped credential references, budgets, latency bounds, and
an optional attested rollback release, and append sanitized outcome, correction,
cost, latency, policy, and safety signals. Optimistic controls narrow, pause,
resume, or roll back live authority and may link a private finding or named
human/agent repair without copying its evidence. Current onboarding trust and
exact profile consent are re-derived, so a changed profile or suspended trust
cannot become inherited release authority. Releases grant no implicit Git,
secret, review, merge, environment, deployment, or operational authority. The
repository web surface remains `view=agents`.

`agent_development_workflow_test.go` is the black-box boundary for the complete
intent-to-improved-agent loop. It uses public HTTP and stock Git to retain a
human- and agent-authored behavior revision, protected domain scenario,
baseline comparison, bounded pilot, ordinary review, attested release,
production regression, rollback, model-key invalidation, exact reevaluation,
and repaired rollout. Leaked answers, evaluator disagreement, prohibited
actions, budget exhaustion, and revoked participant consent remain contained
evidence and never become agent merge, deployment, credential, environment, or
operational authority.

Project funds live beneath `$PROJECT_FUND_ROOT` (default
`apps/api/data/project-funds`). Repository writers publish governed terms with
named stewards, accepted transfer sources, currency or credit units, spending
limits, approval rules, eligible recipient classes, refund policy, and public
or repository-reader ledger visibility. Readers may record provider-referenced
commitments; only explicitly settled amounts contribute to the server-derived
available balance. Transfer source/reference pairs are idempotent, partial
settlement contributes only its settled portion, and pending, failed, revoked,
refunded, and disputed states remain explicit. Steward reconciliation uses
optimistic concurrency, while funding, stewardship, and eligibility grant no
repository, allocation, credential, review, merge, or operational authority.
The repository web surface is `view=funds`.
Fund stewards bind those settled resources to measurable outcomes beneath the
same store through `/funded-outcomes`. An outcome cites an issue, roadmap
outcome, proposal, stewardship opportunity, incident follow-up, or security
repair and freezes scope, milestone or whole-outcome budgets, acceptance and
evidence requirements, deadline, eligible contributor classes, allocation and
cancellation terms, dependencies, risks, conflicts, overlap keys, and embargo
state. Repository readers may pledge to the whole outcome or a named milestone
and withdraw only their own backing. Immutable term versions and attributable
replanning retain scope changes and withdrawals, while reads derive
underfunding, aggregate fund overcommitment, overfunding, overlapping awards,
declared conflicts, and embargo blockers. Funding remains non-authoritative.
Eligible humans, teams, and approved agent operators submit recipient-accepted
delivery proposals beneath funded outcomes with costed milestones,
availability, dependencies, requested access, conflicts, and attributed work.
Steward selection reserves only settled available value within fund limits and
may link ordinary proposal tasks or delivery teams; selection and compensation
grant no repository, secret, credential, review, merge, environment, or fund-
withdrawal authority.
Selected delivery proposals retain an append-only execution account through
their `/progress`, `/expenses`, and `/controls` resources. Recipient observations
cite ordinary tasks, sessions, workspaces, forks, pull requests, checks,
previews, or delivery teams and report milestone progress, evidence, agent
compute, access and handoff health, and completion forecasts. Stewards approve
expenses against the live reservation and may boundedly change its budget,
pause, resume, replace, or cancel execution. Derived overrun, inactivity,
revoked-access, and failed-handoff blockers stop new expense submission or
approval while retaining prior evidence and spend. Replacement and recorded
work references grant no operational authority.
Each funded milestone is compensated only through its governed settlement
record. Reviewers explicitly named by the outcome (or, when omitted, the
fund's named approvers) inspect bounded commit, authorship, handoff, check,
preview, release, deployment, and outcome-measure references. Their immutable
decisions retain rationale and dissent and may request correction, accept,
partially award, reject, or dispute. Accepted value moves only from the
proposal's original recipient reservation; approved milestone expenses reduce
the remaining award. Recipient withdrawal, deadline timeout, appeal, refund,
and payment failure or retry retain deterministic events and restore or release
value without granting Git, review, merge, release, deployment, credential, or
fund-withdrawal authority.
`project_funding_workflow_test.go` is the black-box regression boundary for the
complete community-backing-to-delivered-outcome loop. It retains roadmap scope
replanning, complementary human and approved-agent selection, ordinary Git,
review, check, preview, merge, release, and measured evidence, pending-cost
overrun containment, replacement, rejection, dispute, attributable settlement
receipts, and refund. Rejected milestone value is released from its reservation,
and pending plus approved expenses over budget block further spending.

## Delivery team operating contracts

Federation identity state lives beneath `$FEDERATION_ROOT` (default
`apps/api/data/federation`). Each instance publishes schema-version `1` discovery
at `/.well-known/komodo-federation`: an Ed25519-signed immutable version names
the canonical HTTPS instance, public endpoints, capabilities, operators, active
and retired keys, the prior document digest, and only actors deliberately made
public. Stable remote subjects are `user:{id}@{instance}`,
`agent:{id}@{instance}`, or `installation:{id}@{instance}`; they remain remote
identifiers and must never resolve as local users, grants, or credentials.
Authenticated operators discover peers and make local trust or revocation
decisions beneath `/federation`; unreachable state, signature failures,
unchained identity changes, rotation, and revoked trust remain explicit without
erasing the last verified document. Discovery grants no authority and private
membership is never part of the published catalog.
Public federated repository projections use stable references of the form
`repository:{id}@{canonical HTTPS instance}`. Instances advertising
`repository.discovery` sign a bounded schema-version `1` snapshot at their
declared repository endpoint containing only public metadata, visible branches,
releases, contributor guidance, public issues, contribution opportunities, and
attributable activity. A home instance resolves only through a trusted peer,
verifies the response against its discovered active or retained retired key,
and stores a followed last-verified observation beneath `$FEDERATION_ROOT`.
The shareable reader surface is `/federation/repositories?ref={reference}`.
Cached snapshots remain read-only remote context: exact source and cache
revisions, signature/key, unsupported capabilities, staleness, outage, and
visibility withdrawal must remain explicit, and inaccessible remote content is
never copied.
Trusted peers may independently advertise `repository.contributions`. A home
instance creates an independently owned local fork by importing a signed,
bounded Git object closure from a current public remote observation. Stock Git
uses only the local repository. Selected upstream branch synchronization is
explicit and fast-forward-only. A cross-instance proposal signs its exact local
source revision, exact observed remote target, bounded objects, remote author
subject, metadata, and public contribution context; the upstream verifies its
trusted peer and content identities before materializing a private immutable
source snapshot for an ordinary pull request. Negotiation, staleness,
signature, transfer, and divergence failures stay explicit. Remote subjects
never become local accounts, grants, credentials, or implicit cross-instance
repository authority.
Accepted federated contributions merge only from the upstream's immutable private source snapshot through ordinary local ownership, review, check, preview, and integration policy. Before target publication the complete Git object set is linked into upstream storage; the pull retains signed proposal provenance, and a signed idempotent merge receipt binds both pull references, source and merge commits, remote author, maintainer, and review/check evidence digests. Receipt failure is retryable and never rolls back local history. Retained receipts distinguish historical verification from current peer trust, so deletion, outage, or revocation cannot erase accepted work or imply continuing trust.
`federation_workflow_test.go` is the black-box regression boundary for the complete two-instance contribution loop. It uses independently persisted TLS applications and stock Git to retain discovery, following, fork synchronization, proposal provenance, maintainer discussion, locally governed agent work and publication, upstream review and merge, receipt acknowledgement and duplicate delivery, chained key rotation, outage recovery, and trust-revocation containment. Agent revision claims remain signed observations and never silently replace the upstream's immutable proposed candidate.
Federated contribution review uses the separate `pull_request.exchange`
capability and signed version-1 pull-request events. Discussion, reviews,
requested changes, exact revision updates, checks, previews, and closure bind
an idempotency key, both stable pull references, remote actor subject, exact
source revision, audience, and bounded evidence. Receiving instances verify a
trusted peer key and enforce repository visibility and embargo policy before
retaining the event. Imported claims remain remote signed observations and
never become local authors, reviews, required checks, preview access, closure,
or merge authority. Reads derive currentness from the local pull revision;
retries reuse the idempotency key, while conflicting payloads and unavailable
peers remain explicit recoverable states.

Federated pull-request agent collaboration remains home-instance execution.
An authorized participant delegates an approved local agent beneath
`/federation/repositories/{source}/agent-sessions`, binding the trusted remote
pull reference, exact local contribution revision and branch, and an allowlist
of paths and evidence. The agent receives only a 24-hour credential for that
local source branch; guidance, controls, credential revocation, and raw runtime
events stay local. Publication derives commits and changed paths from Git and
exports a signed `agent_session` pull-request event containing only the declared
summary, commands, evidence, costs, and residual concerns. It preserves the
agent subject and authorizing user without granting either remote repository,
secret, check, review, merge, or operator authority. State lives beneath
`$FEDERATED_AGENT_SESSION_ROOT` (default
`apps/api/data/federated-agent-sessions`).

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

Repository design systems live beneath `$DESIGN_SYSTEM_ROOT` (default
`apps/api/data/design-systems`). Repository writers publish immutable,
optimistically concurrency-checked versions bound to exact reviewed source and
release revisions. Versions retain design tokens, components, interaction
patterns, content rules, responsive behavior, supported themes, rendered
examples, accountable owners, adoption policy, accessibility and localization
constraints, consumer implementation revisions, rationale, and provenance.
Reads derive missing owners and provenance, stale or absent implementations,
unsupported consumers, and conflicting current token, component, or interaction
definitions across systems rather than selecting a canonical winner. These
records grant no repository, review, merge, release, deployment, or operational
authority. The repository web surface is `view=design`.

Pre-implementation product design proposals live beneath
`$DESIGN_PROPOSAL_ROOT` (default `apps/api/data/design-proposals`). Repository
collaborators open one from exact feedback, issue, roadmap-outcome,
accessibility-finding, or pull-request context and publish immutable optimistic
revisions defining the user goal, journeys, states, content, constraints,
alternatives, success measures, affected components, accessible project
evidence, and declared uncertainty. Designers, developers, invited users, and
evidence-grounded agents attach revision-bound wireframes or interactive
prototypes and attributable comments, support, questions, or dissent. Owner
acknowledgements bind one proposal revision and become stale when it changes.
Private research and inaccessible assets remain counted restricted references
but are excluded from read projections and cannot ground an agent, artifact, or
discussion citation. A fully acknowledged current revision can freeze an
implementation contract at an exact source commit into an ordinary proposal and
dependency-ordered human- or agent-owned tasks. Copied task reasoning retains
exact proposal and artifact versions, journeys, states, content, constraints,
measures, prototype frames/interactions, and exported asset source, author,
license, and transformation history, so existing sessions, shared workspaces,
and pull requests preserve the intent. Revision-bound mappings connect changed
paths and rendered surfaces to requirement IDs; deliberate deviations remain
pending until attributable repository-owner approval. These records grant no
repository, research, asset, fork, workspace, agent, review, merge, release,
deployment, or operational authority. The repository web surface remains
`view=design`, and `design_proposal_workflow_test.go` is the public regression
boundary from review through accountable implementation handoff.

Revision-exact interface checks live beneath `$INTERFACE_CHECK_ROOT` (default
`apps/api/data/interface-checks`). Each pull revision reads schema-version `1`
`.komodo/interface-checks.json`, binding an accepted design proposal or frozen
implementation contract to repository-defined journeys, rendered surfaces,
requirement IDs, and exact code/scenario input blobs. Bounded results retain
viewport, theme, content-length, locale, interaction-state, and assistive-
technology contexts; visual and behavioral differences; recordings and
content-addressed artifacts; coverage, timing, and performance evidence; and
the attributable runner. Pull-request readers see current and historical runs
in `section=interface`. Repository collaborators classify each current
difference as intentional, regression, or false positive with rationale, while
case decisions bind only classified differences. A changed pull revision,
definition blob, or declared case input makes only the dependent evidence,
differences, and decisions stale and preserves unrelated cases. Checks and
review records grant no preview, repository, design, review, merge, release,
deployment, credential, or operational authority.

Design acceptance and system-evolution policy lives beneath
`$DESIGN_GOVERNANCE_ROOT` (default `apps/api/data/design-governance`).
Repository owners and organization owners publish immutable gates targeted by
branch plus component, journey, path, or risk class and requiring current
design-owner, accessibility, content, localization, or invited-user acceptance.
Acceptances bind the exact pull revision and preview. Merge and release
readiness compose inherited organization and repository gates with ordinary
checks and reviews, revision-derived interface evidence, unresolved deviations,
and schema-version `1` `.komodo/design-usage.json` obsolete-component inventory.
Owner exceptions require an accountable owner, governed follow-up, and expiry;
near-term expiry remains visible. Approved design-system changes create
attributable repository and documentation migration work, while feedback and
observed regressions create connected repair work. These records always report
`grants_authority=false` and grant no repository, review, merge, release,
deployment, participant, credential, or operational authority.
`interface_design_workflow_test.go` is the black-box regression boundary for
the complete feedback-to-shipped-interface loop. It retains designer,
invited-user, and evidence-grounded agent comparison; a missing-state prototype
revision; responsive, localization, interaction, and accessibility evidence;
a rejected visual regression and corrected exact-candidate acceptance; design
token history; connected repair work; and downstream consumer migration without
granting hidden delivery or operational authority.

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

Versioned service interface publications live beneath `$API_CONTRACT_ROOT`
(default `apps/api/data/api-contracts`). Repository writers publish immutable
contract versions from reviewed source revisions, retaining definition format
and validation, operations, schemas, errors, authentication, environments,
limits, ownership, stability, support and compatibility promises, and source,
release, documentation, and data-use links. Reads derive invalid-definition,
unreleased-implementation, stale-documentation, unavailable-environment,
missing-provenance, and declared gaps; `/compare?from={number}&to={number}`
reports operation and schema changes alphabetically without assigning a score.
Contracts grant no consumer credential, repository, deployment, or operational
authority. The repository web surface is `view=apis`.

API consumer onboarding lives beneath `$API_CONSUMER_ROOT` (default
`apps/api/data/api-consumers`) and is always bound to one exact valid contract
version. An authenticated repository reader registers an independently owned
project with requested available environments, declared authentication scopes,
contact, and maximum credential lifetime. Repository writers approve only a
subset, set a bounded synthetic-request quota, examples, deterministic failure
rules, and a shorter lifetime, or retain an attributable denial. The consumer
owner must explicitly accept narrowed approved terms; consent and rotation then
return a `vka_` secret once, and only its digest is stored. The application
credential authenticates only `/api-sandbox/{application}/requests`, whose
retained inspections redact authorization and contain synthetic bodies. It is
never accepted as a human, Git, repository, deployment, environment, or
production-data credential. Exposure reports, expiry, revocation, and accepted
ownership transfer invalidate it; denial/revocation/expiry can re-enter the
ordinary approval flow without erasing history. The repository web surface
remains `view=apis`.

Registered API applications connect adoption to reviewable work through
`/integration-work` and `/verifications`. A work brief freezes the exact
contract source, definition, SDK/example references, approved synthetic sandbox
configuration, consumer repository revision, owner kind, and linked ordinary
task, session, or workspace without copying a credential or granting authority.
Verification records bind an ordinary pull request and exact candidate revision
to producer conformance scenarios and consumer tests. They retain only
sanitized request/response fields, logs, content-addressed artifact metadata,
coverage, cost, inaccessible-evidence references, and authorship; agreement is
derived only when both independently defined suites pass the same frozen
contract. Credential-shaped content is rejected from preloads and reusable
evidence.

Incompatible API change is governed beneath
`/repositories/{repository}/api-contract-migrations`. A producer proposal
binds exact old and target contract versions, classified changes, an optional
existing evolution plan, ordered acknowledgement, dual-run, sunset, and
retirement stages, and the registered applications actually pinned to an old
version. Consumer views contain only their own application and may acknowledge
linked evolution tasks, forks, agent sessions, delivery teams, integration
work, or pull requests; retain a dual-version test; request a producer-decided
exception bounded by the final deadline; and attest the exact migrated
revision. Server-derived readiness uses post-proposal tests and shared usage
observations. Requested changes, missing or failed tests, stale or remaining
traffic, unresponsive ownership, revoked access, missing attestation, and an
active final-stage exception block advancement. A final owner action retires
only a fully ready migration; these records grant no Git, agent, team,
credential, deployment, environment, or operational authority.
The collection `GET` applies the same ownership projection as individual
migration reads: repository writers see the producer-wide cohort, each
consumer sees only its own affected application, and unrelated readers receive
an empty collection. The `view=apis` workspace exposes proposal,
acknowledgement, dual-version verification, bounded exception, attestation,
stage advancement, and retirement actions against this projection.

Post-adoption API support remains bound to the same application and exact
contract version. Producers and application owners record sanitized,
window-bounded availability, latency, quota, error, schema-conformance, and
usage observations against an exact release and approved environment. Each
observation is explicitly producer-only, consumer-only, or shared; a side
cannot publish into the other side's private audience, and private payloads,
credentials, and raw usage are represented only by inaccessible-evidence
references. Shared failures open attributable investigations that retain the
evidence set, release, environment, questions, findings, and an explicit
service, contract, client, environment, or unconfirmed classification.
Participants may invite a named agent only for read-only sanitized evidence and
thread participation, and may reproduce approved operations and failure rules
through credential-free synthetic fixtures with authorization redacted. Only a
confirmed classification can link an existing ordinary issue, proposal, task,
or workspace at an exact repository revision; the support record grants no
repository, credential, production-data, review, merge, deployment, or
environment authority.

Repository service objectives live beneath `$SERVICE_OBJECTIVE_ROOT` (default
`apps/api/data/service-objectives`). Repository writers publish immutable,
optimistically concurrency-checked versions across repository, release, and
environment scopes. Each version binds user journeys to named indicators,
measurement windows, targets, error budgets, dependencies, severity responses,
owners, exception policy, and product, performance, accessibility, privacy, and
release commitments. Reads derive missing signals and ownership, unsupported
calculations, overlapping target conflicts, unresolved commitments, and
expiring or expired exceptions. The repository web surface is
`view=reliability`; objectives grant no repository, deployment, telemetry,
credential, or operational authority.
Repository writers attach immutable signal-mapping versions and append-only
attainment observations beneath each objective. Mappings bind an exact
objective version, indicator, window, instrumentation revision, and sanitized
metric, log, trace, health-check, support, delivery, source, package, or
dependency fields. Observations retain exact mapping and objective versions,
bounded release and code provenance, attainment, error-budget consumption,
uncertainty, audience, and comparability. Restricted or unsanitized evidence is
rejected; anonymous public reads omit repository-audience observations, while
instrumentation changes, incomparable windows, and missing mappings or
attainment remain explicit.

Reliability delivery policies live beneath `$RELIABILITY_POLICY_ROOT` (default
`apps/api/data/reliability-policies`). Repository owners bind an exact objective
version to branches, services, environments, journeys, and risk classes and map
budget exhaustion or thresholds, regressions, missing evidence, and dependency
failure to block, slow, pause, or rollback responses. Revision-exact predicted
and observed impacts cover pull requests, integration queues, releases, and
deployments; reads retain active exceptions, required objective-owner
acknowledgements, derived requirements, and available next actions. Policy and
agent evidence grant no merge, queue, release, deployment, credential, or
environment authority.

Revision-bound collaborative diagnosis lives beneath
`$RELIABILITY_INVESTIGATION_ROOT` and accountable repair records live beneath
`$RELIABILITY_IMPROVEMENT_ROOT`. Repository readers and approved read-only
agents compare bounded baseline, operational, dependency, and code evidence;
request affected-owner input; retain uncertainty, challenges, and conclusions;
and expose missing or stale evidence without gaining telemetry or operational
authority. Writers turn a current supported finding or depleted budget into an
ordinary proposal with ordered human- and agent-owned tasks, then link reviewed
pull requests, checks, releases, deployments, decisions, and staged rollout
measurements. `GET /repositories/{repository}/reliability-improvements` is the
repository-readable collection projection used by `view=reliability`; failed
rollouts remain alongside later recovery and restored budget state.
`reliability_workflow_test.go` is the black-box boundary for the complete
released-journey-to-sustained-reliability loop. It retains a noisy signal,
missing dependency evidence, a rejected exception, rollout containment,
revision-exact human-agent investigation, bounded agent authority and cost, a
failed first repair, ordinary review and release, and staged verified recovery.

Repository recovery objectives live beneath `$RECOVERY_OBJECTIVE_ROOT` (default
`apps/api/data/recovery-objectives`). Repository writers publish immutable,
optimistically concurrency-checked continuity commitments covering repository,
package, artifact, configuration, collaboration-record, and deployed-service
state. Every resource binds a user capability to accountable owners,
dependencies, acceptable loss, restoration time, retention, jurisdictions,
validation criteria, and declared feasibility; versions retain exclusions,
time-bounded exceptions, and links to service objectives, environments,
incidents, privacy rules, and governance. Reads derive missing ownership,
impossible or unverified targets, unprotected or ownerless dependencies, and
expiring or expired exceptions. The repository web surface is
`view=continuity`; commitments grant no repository, backup, environment,
credential, restoration, or operational authority.
Protection plans live beneath `$PROTECTION_PLAN_ROOT` (default
`apps/api/data/protection-plans`) and bind an exact recovery-objective version
and its resources to repository or environment scope, snapshot or replica
mode, encryption and key reference, access scope, authorized destinations,
retention, checksum and validation requirements, freshness, and cost limits.
Append-only, idempotent capture manifests retain only bounded content counts,
versions, provenance, dependency versions, checksums, validation digests, cost,
and responsible actor—not protected payloads or credentials. Reads derive
current coverage and recoverability; incomplete or stale captures, changed
plans, missing or deleted source state, corruption, unavailable keys, failed
decryption, unauthorized destinations, missing validation evidence, and cost
overflow remain explicit. Protection evidence grants no repository,
environment, key, snapshot-content, restoration, or operational authority.
Recovery exercises live beneath `$RECOVERY_EXERCISE_ROOT` (default
`apps/api/data/recovery-exercises`). Repository writers launch a bounded
failure scenario against one exact recoverable capture and its protected source
and dependency versions, with an explicit isolated environment, duration and
cost limits, dependency-ordered restore steps, and integrity and user-journey
checks. Exercises reject authoritative-state writes and production-secret
availability. Attributable results retain timing, exact declared commands,
redacted log excerpts and digests, artifact metadata, manual steps, gaps, cost,
and achieved objective resources without retaining protected payloads or
credentials. Reads derive passing or failed status and make historical evidence
non-current when its protection plan, capture recoverability, or dependency
versions change. Exercises grant no environment, secret, restoration, Git, or
operational authority and appear in `view=continuity`.
Revision-bound continuity diagnosis lives beneath `$RECOVERY_INVESTIGATION_ROOT`
(default `apps/api/data/recovery-investigations`). Repository readers and
read-only agents may investigate a completed failed or explicitly risky
exercise using bounded repository- or participant-audience citations to the
exercise, code, dependencies, releases, configuration, ownership, and
protection plan. Findings retain citations, attribution, challenges, verdicts,
and uncertainty; participant-only evidence is not projected to other readers,
and changed or non-current exercise evidence remains explicit. Investigations
grant no protected-state, environment, secret, restoration, Git, or operational
authority.
Accountable continuity repair lives beneath `$RECOVERY_IMPROVEMENT_ROOT`
(default `apps/api/data/recovery-improvements`). A repository writer converts a
current supported finding into an ordinary proposal with ordered human- or
agent-owned tasks and may cite existing sessions or workspaces. Append-only
links retain resulting pull requests, checks, integrations, releases, policy
changes, and approvals without replacing any resource's own access or review
controls. An improvement remains blocked until a distinct, current exercise
passes against a newer version of the same repaired protection plan; failed
verification preserves the original weakness and creates no production-state
authority.

Live recovery responses live beneath `$RECOVERY_RESPONSE_ROOT` (default
`apps/api/data/recovery-responses`). Repository writers activate one only from
an incident or confirmed loss event, binding an exact current protection-plan
version and verified recovery point to one named workspace, environment,
estimated loss, required approvers, communication channels, rollback choices,
and dependency-ordered restoration steps. Named approvals and attributable
pause, resume, cutover, rollback, and cancellation decisions remain immutable.
Each human or agent may execute only a step explicitly delegated to that actor;
destructive steps additionally require an explicit cutover decision. Reads in
`view=continuity` derive progress, next steps, validation, and blockers.
Conflicting writes, unavailable keys, stale replicas, partial restoration,
failed steps, and failed validation pause safely. A response becomes restored
only after all steps and a passing validation, and grants no repository,
snapshot, key, environment, deployment, credential, or operational authority.
Failed response steps retain append-only attempt evidence; an attributable
resume makes only failed steps retryable without erasing the contained result.
`recovery_workflow_test.go` is the black-box boundary for the complete
commitment-to-trusted-return loop through public HTTP and stock Git, while
`recovery_response_workflow_test.go` retains focused live-response containment.

Product feedback lives beneath `$PRODUCT_FEEDBACK_ROOT` (default
`apps/api/data/product-feedback`). Authenticated project readers submit a need
against the project, an exact release, a documentation collection journey, or
an exact preview, with desired outcome, frequency, impact, explicit public,
repository, or accepted-organization audience, separate identity and evidence
visibility, contact preference, and research/update consent. Evidence must be
declared redacted and remains bounded; every read is projected for its viewer,
so restricted identity, contact details, and evidence content never leak into
broader list, detail, discussion, or history responses. Organization feedback
fails closed outside accepted membership. Discussion and validated issue or
product-experiment links remain attributable, while reporter consent withdrawal
disables future contact without deleting the need or its history. The repository
web surface is `view=feedback`; these records grant no repository, research,
experiment, targeting, or follow-up authority beyond the recorded consent.

Developer support questions live beneath `$SUPPORT_QUESTION_ROOT` (default
`apps/api/data/support-questions`). Authenticated repository readers ask against
a repository, package, release, API, documentation journey, or error with their
goal, question, version, environment, attempted steps, urgency, audience, and
contact preference. Missing version, environment, or attempted steps remain
explicit diagnostic gaps rather than invented context. Bounded sanitized logs,
configuration, and sample code have audience or maintainer-only visibility;
contact values and restricted evidence are projected only to the asker and
repository participants. Related support threads and issues are suggested using
only visible titles, goals, questions, and problem summaries, never evidence
content. Discussion, status changes, and authorship are retained in history.
The repository web surface is `view=support`; asking grants no repository,
credential, execution, or operational authority.
Support answers remain in the same thread as immutable revisions. Every answer
declares applicable versions and decomposes its guidance into verified,
inferred, or uncertain claims with citations to an exact source revision and
lines, symbol, documentation collection, package, release, visible prior
support thread, or known issue. The API resolves repository citations and
validates referenced project records before publication; a citation must share
the thread audience, so restricted context cannot silently support broader
guidance. Agent-authored revisions require explicit overall uncertainty, and
non-verified claims require claim-level uncertainty. Authenticated repository
readers can endorse, challenge, comment on, or request clarification of an
exact claim; new guidance supersedes only the current revision and retains all
prior advice and feedback. Guidance publication grants no repository, Git,
credential, execution, review, merge, or operational authority.
Permitted thread participants can verify one exact answer revision in a bounded,
credential-free, networkless workspace at an exact source and stated software
version. Attempts freeze instructions, environment image and resources,
dependency versions, sanitized inputs, commands, redacted logs, outputs,
artifacts, cost, result, attribution, and rerun lineage. Answer, source,
software, environment, dependency, or input changes mark affected evidence
stale. Credential-like inputs and artifacts are rejected, and verification
grants no repository, credential, environment, or operational authority.
The asker or a repository participant resolves a thread only by publishing an
exact answer revision with a successful current verification. Reusable
solutions freeze their tested versions, limitations, audience, verified answer
and attempt, project links, publisher, and attributable participant credit;
repository-scoped threads cannot become public solutions. Current solutions are
searchable from `view=support` and may link validated documentation collections,
packages, releases, and contributor guidance. Maintainers append duplicate
merge, obsolete archive, or newer-version revalidation events and participant
notifications without rewriting the original question, answer revisions,
dissent, verification, authorship, or prior publication. Archived and merged
solutions leave search results but remain in thread history; publication grants
no repository, documentation, package, release, credential, or operational
authority.
When guidance remains insufficient, a repository collaborator may classify the
question as a defect, documentation gap, missing example, compatibility
problem, or product opportunity and open an ordinary issue, documentation
task, proposal, or ordered human/agent proposal work. The immutable improvement
record freezes the question, goal, reproduction, affected version and
environment, acceptance criteria, and only explicitly selected discussion—not
attached support evidence. Pull requests, checks, previews, releases, and
documentation publications append progress to the original thread. This
handoff preserves each downstream workflow's own permissions and grants no
repository write, agent execution, review, merge, release, or publication
authority.
`support_workflow_test.go` is the black-box boundary for the complete package
question-to-tested-guidance-or-improved-product loop. It retains maintainer
refinement, cited agent revisions, failed and clean verification, private
evidence projection, duplicate merging, ordered human-agent improvement work,
ordinary Git/check/review/merge/release delivery, updated guidance, and the
asker's viewer-scoped notification.

Product opportunities live beneath `$PRODUCT_OPPORTUNITY_ROOT` (default
`apps/api/data/product-opportunities`). Repository writers and authorized
read-only agents synthesize permitted feedback, issues, exact-preview findings,
bounded support signals, usage evidence, and experiment outcomes into immutable
versions. Every version declares affected audiences, severity, reach,
confidence, expected value, uncertainty, and source-by-source relevance and
classification as supporting, contradicting, minority, or duplicate evidence;
source revisions are captured and validated where the platform owns the source.
Repository readers inspect both current evidence and complete version history,
append attributable corrections or challenges, and feedback reporters may
detach their own report by creating a new version without rewriting the
historical citation. Opportunities are evidence, not popularity scores, and
grant no research, targeting, repository, experiment, or delivery authority.

Product roadmaps live beneath `$PRODUCT_ROADMAP_ROOT` (default
`apps/api/data/product-roadmaps`). Repository writers publish immutable,
optimistically concurrency-checked versions comparing exact opportunity
versions with goals, capacity, dependencies, risks, governance decisions, and
commitments. Accepted outcomes require available owners, target horizons,
success measures, capacity, sequence, and rationale; rejection reasons remain
visible. Reads derive overcommitment, dependency, owner, commitment-conflict,
and slipped-target replan blockers. Readers may discuss versions and publish
human or agent scenarios, but scenarios have no resource authority and only
repository writers can commit or replan direction.

Accepted roadmap outcomes cross into accountable delivery beneath
`$ROADMAP_DELIVERY_ROOT` (default `apps/api/data/roadmap-deliveries`). Promotion
creates one ordinary proposal and ordered human- or agent-owned tasks whose
acceptance criteria cite the outcome's evidence and collectively cover every
success measure. Delivery links report exact pull request, check, preview,
integration, release, deployment, and experiment resources back to that frozen
roadmap and opportunity version. A release or deployment may make the state
`delivered_not_achieved`; only reported passing measures with no changed
assumptions, unresolved user needs, policy conflicts, or revisit request make it
`achieved`. Failed measures and explicit revisit requests remain attributable
blockers, and the contract grants no Git, review, merge, release, deployment,
experiment, or participant authority.

Post-delivery product learning lives beneath `$PRODUCT_LEARNING_ROOT` (default
`apps/api/data/product-learning`). A roadmap delivery has one reciprocal record
where repository writers publish decision, preview, delivery, rejection, and
measured-outcome updates to explicitly cited feedback contributors and named
stakeholders. Feedback delivery requires active product-update consent; reads
are audience-projected, non-public links are redacted, and a participant may
validate the result with bounded follow-up evidence and dissent or leave future
updates without deleting prior credit and provenance. Maintainers append
optimistically concurrency-checked promised-versus-observed lessons against a
current roadmap revision, retain resulting-work links and dissent, and classify
the opportunity as open, fulfilled, or unsupported. Learning records grant no
roadmap, research, contact, repository, release, or operational authority.

Product experiment plans live beneath `$PRODUCT_EXPERIMENT_ROOT` (default
`apps/api/data/product-experiments`). Repository collaborators register
versioned product signals with their event, unit, permitted properties and
audience consent classes, and explicit instrumentation state. They open a plan
from a proposal, issue, decision, pull request, preview, or release and freeze
its hypothesis, control and variants, eligibility and exclusions, success and
guardrail measures, minimum evidence, duration, owners, participants, stop
conditions, assumptions, and overlap keys. Measures always cite an exact signal
version. Discussion, approvals, changes requested, and assumption changes are
append-only and attributable; every plan revision requires current participant
approval again. Reads explicitly derive missing instrumentation, ineligible
audiences, overlapping plans, changed assumptions, and missing approvals. A
ready plan is only a pre-exposure contract and grants no variant publication,
audience assignment, telemetry, release, deployment, or operational authority.
The repository web surface is `view=experiments`.

Roadmap outcome validations live beneath `$ROADMAP_VALIDATION_ROOT` (default
`apps/api/data/roadmap-validations`). Repository writers open a technical
decision, prototype, documentation concept, or product experiment against an
exact roadmap outcome and opportunity version. Immutable versions bind success
and guardrail measures to cited feedback and a bounded preview or research
revision. Only feedback reporters with active research consent may receive a
purpose-bound participant credential; it grants no repository access. Findings
retain attribution, accessibility needs, dissent, acceptance, and evidence
validity. Collaborator assessments may accept, revise, defer, or reject the
direction without rewriting prior roadmap or validation versions.
Declared variant and instrumentation delivery stays ordinary project work.
Experiment `/work-items` link exact-revision human- or agent-owned tasks,
sessions, and workspaces without creating or starting them. An
`/implementations` record resolves an existing ordinary pull request and
server-captures its exact source commit alongside declared variants, exact
signal event versions and properties, exposure rules, privacy classification,
removal plan, and repository check names for assignment, metric capture,
variant isolation, and fallback. Plan revisions make earlier implementation
records explicitly non-current; neither record grants code, data, agent,
review, merge, release, deployment, or exposure authority.
Repository owners bind an audience policy to one exact release commit and the
complete current variant set. Policies declare structured eligibility and
exclusions, consent class, regions and organizations, deterministic basis-point
allocation, a mutual-exclusion group, exact permitted signal properties,
retention, and named approvers. Reads preview these boundaries and derive stale
plan/release, unapproved, unauthorized-collection, and conflicting-allocation
blockers before assignment. Assignment is stable and idempotent, retains only a
repository-scoped subject digest and decision evidence, and never exposes raw
membership. The contract grants no deployment, exposure, telemetry, or user-data
authority; rollout remains a later independently governed action.
Approved exact-release experiments run only through an existing successful
deployment in a declared release environment. Each retained run attempt binds
the current plan and audience-policy versions, release commit, deployment,
environment, and ordered allocation stages. Participants inspect attributed
exposure, measure values and uncertainty, data and instrumentation quality,
guardrails, consent, operational health, evidence, and cost, and may pause,
resume, or stop. Safety failures deterministically contain only that attempt,
without rebucketing subjects or deleting stages, evidence, costs, or controls;
retry requires a new attempt.
Threshold or stop-condition analyses bind one exact run observation and retain
segments, uncertainty, exclusions, guardrails, bounded agent interpretation,
dissent, and aggregate evidence. Versioned adopt/control/extend/inconclusive
decisions create non-authoritative rollout, rollback, follow-up, or cleanup
tasks. Only tasks completed with pull-request, release, deployment, and evidence
links permit a cleanup receipt; it retires future assignment, launch, targeting,
credentials, and collection while preserving aggregate evidence, provenance,
user protections, and delivery links.
`product_experiment_workflow_test.go` is the black-box regression boundary for
the complete feedback-to-learned-product loop. It retains the feedback source,
human and agent implementation commits, ordinary checks/review/release and
deployment, consent-bounded assignment, deterministic guardrail containment,
progressive retry evidence, agent interpretation, human dissent, acknowledged
choice, delivered outcome, and final retirement of targeting and collection.

`product_discovery_workflow_test.go` is the black-box regression boundary for
the complete released-product-feedback-to-measured-learning loop. Through
public HTTP and stock Git it retains public and maintainer-only feedback,
human-agent synthesis and challenge, an explicit rejected alternative, slipped
target replanning, consent-bound validation and withdrawal, ordinary delivery
evidence, failed adoption, participant dissent, and an open opportunity carried
forward without widening repository or operational authority.

Repository accessibility commitments live beneath
`$ACCESSIBILITY_COMMITMENT_ROOT` (default
`apps/api/data/accessibility-commitments`). Repository writers publish
immutable, optimistic-concurrency versions covering repository, documented
journey, component, and release scopes. Each version names applicable
standards and levels, supported assistive-technology/platform combinations,
target audiences, required scenario-to-scope coverage, severity and review
effects, accountable owners, bounded approved exceptions, and roadmap outcome,
documentation, preview, and release-policy links. Attributable coverage records
bind one current version, scenario, assistive environment, status, revision,
and evidence without rewriting the agreement. Reads derive missing or failed coverage,
unsupported or untested environments, expiring or expired exceptions, and
conflicting requirements across overlapping current scopes. The repository web
surface is `view=accessibility`; commitments and evidence grant no repository,
review, merge, release, credential, or operational authority.

Revision-exact accessibility assessments live beneath
`$ACCESSIBILITY_ASSESSMENT_ROOT` (default
`apps/api/data/accessibility-assessments`). Repositories declare schema-version
`1` checks in `.komodo/accessibility-checks.json`, naming scenarios, code and
scenario inputs, affected audiences, covered dimensions (`semantics`,
`keyboard`, `focus`, `contrast`, `motion`, `captions`, or `journey`), and
dimensions that still require human evaluation. They run as ordinary
credential-free, networkless pull-request checks. Authorized readers,
specialists, and read-only agents append findings only from a permitted exact
preview or a retained revision-exact barrier reproduction, with source
locations, severity, audiences, uncertainty, and attributable judgment.
Maintainers retain confirmations, duplicate links, and false-positive decisions.
Reads compare captured source blob identities with the current pull revision so
unrelated changes preserve evidence while affected automation and findings
become explicitly stale. Both the pull-request Accessibility section and
repository `view=accessibility` show proved coverage, lived experience,
human-evaluation requirements, and unevaluated scenario dimensions. Assessments
grant no preview, workspace, agent, review, merge, or operational authority.

Confirmed current findings extend into governed delivery beneath their
`/repairs` resource. A repository writer selects an existing proposal and
creates a human- or approved-agent-owned task at the assessment revision, with
immutable acceptance criteria, permitted reproduction evidence IDs, an exact
accessibility commitment version, and component guidance. The writer may also
preload an ordinary pull-request change session or shared workspace; those
resources retain their own credential and access boundaries. Attributable
progress remains on the original finding. A delivery link is accepted only
from the repair task's ordinary pull request and an exact-source-revision
preview, and retains separate design and code changes plus interaction and
content tradeoffs. Findings and repairs grant no repository, agent, credential,
review, preview, or merge authority.

Accessibility delivery policies live beneath `$ACCESSIBILITY_POLICY_ROOT`
(default `apps/api/data/accessibility-policies`). Repository owners bind an exact
accessibility commitment version to selected target branches, changed paths,
journeys, or risk classes and require named accessibility checks, scenario
dimensions, and reviewer or participant roles. Pull-request readiness and merge
evaluate only the current candidate: old check runs, assessments, and preview
acknowledgements remain visible but do not satisfy the policy. Invited preview
participants retain attributable confirmations or rejections for their stated
role without representing other access needs. Owner overrides preserve dissent,
require rationale and linked issue, proposal, or task follow-up, and never grant
repository or operational authority.

Accessibility barrier reports live beneath `$ACCESSIBILITY_BARRIER_ROOT`
(default `apps/api/data/accessibility-barriers`). Authenticated repository
readers report release-, page-, documentation-journey-, or preview-scoped
barriers at a server-verified exact revision with access needs, expected
outcome, interaction steps, consent-scoped device context, and explicitly
redacted screenshots, recordings, accessibility trees, speech output, or input
traces. Viewer projections hide reporter identity, sensitive device details,
and evidence bodies outside their declared audience. Repository writers retain
reproduction attempts only against an existing preview or bounded workspace at
the same exact revision and classify the result as reproducible, intermittent,
environment-specific, or unconfirmed. The repository web surface remains
`view=accessibility`; reports and attempts grant no repository, workspace,
preview, credential, review, merge, or operational authority.

`accessibility_workflow_test.go` is the black-box regression boundary for the
complete released-barrier-to-sustained-access loop. Through public HTTP and
stock Git it retains consent-projected lived evidence, bounded reproduction,
specialist and agent assessment and repair, false-positive correction,
revision-exact automation and assistive-technology judgment, stale preview
acceptance, ordinary review, merge, and release provenance, reporter
confirmation, expiring-exception visibility, and release-linked regression
coverage without widening repository or operational authority.

Repository performance contracts live beneath `$PERFORMANCE_GOAL_ROOT` (default
`apps/api/data/performance-goals`). Collaborators publish immutable,
optimistically concurrency-checked versions for a repository, release, user
journey, API, command, or service. Each version retains representative
workloads, unit-bearing metrics and baseline evidence, target ranges,
performance budgets, correctness constraints, supported environment digests,
owners, baseline freshness policy, links to issues, incidents, previews,
releases, and decisions, author, and change reason. Attributable measurements
bind the exact goal version, metric, value, environment, revision, source, and
measurement time. Reads derive missing measurements, incomparable environments,
stale baselines, and conflicting target ranges explicitly; measurements never
silently rewrite the agreed contract. The repository web surface is
`view=performance`.
Authorized collaborators retain reproducible benchmark trials beneath the same
goal. Trials bind its current version to an existing exact commit or matching
attested release and retain definition and sanitized-input digests, environment,
warmups, raw samples, sampling method, derived mean/variance, resource profile,
bounded redacted traces/logs/artifacts, cost, attribution, and rerun lineage.
Repository reads expose the evidence without granting execution or operational
access; credential-like evidence and unverified releases are rejected.
Collaborative performance investigations live beneath
`$PERFORMANCE_INVESTIGATION_ROOT` (default
`apps/api/data/performance-investigations`). A repository participant selects
exact trials from one performance-goal version and invites repository owners or
collaborators to inspect them alongside exact code/symbol, dependency, commit,
release, and runtime-path citations. Participants, including credentialed
read-only agents, publish attributable hypotheses, folded flame graphs,
comparisons, uncertainty, challenges, confirmations, and conclusions without
gaining Git or execution authority. Reads derive stale findings from changed
goal versions and selected trial revision, workload, environment, or
availability. Evidence explicitly scoped to investigation participants cannot
be cited into repository-audience entries, and investigations containing it
remain invisible to uninvolved repository readers.
Supported repository-audience conclusions may create a human- or agent-owned
performance change as an ordinary proposal task, retaining the exact goal
version, workload evidence, baseline trial, diagnosis, constraints, ownership,
and base revision. Candidate comparisons bind an ordinary pull request's exact
source commit to an isolated candidate trial and a valid baseline with the same
definition, workload source, environment, and sampling method. Public goal and
pull-request-linked evidence derives the mean and percentage change, 95%
confidence interval, CPU, memory and cost deltas, correctness checks, affected
scenarios, reproducible commands, attribution, and residual risks. These
records grant no Git, execution, review, check, or merge authority.
Repository owners attach immutable performance delivery policies to a goal's
current version by target branch, path glob, or risk class. Each metric declares
an allowed percentage regression and whether its confidence interval must clear
that threshold. Pull-request readiness and merge independently require an exact-
revision comparison for every applicable policy; revised goals or pull commits
make earlier evidence non-current. Staged observations bind that comparison to
an attested release, deployment, stage, revision, environment, health,
assumptions, and observed metric. Regression, uncertainty, failed health, or an
incomparable environment requires an explicit pause, known-good restore, linked
issue repair, or linked decision revisit. Evidence grants neither agents nor
observers merge or environment authority.
`performance_workflow_test.go` is the black-box regression boundary for the
complete production-concern-to-validated-improvement loop. It retains linked
user impact, a sanitized exact-revision baseline, agent and affected-owner
diagnosis, agent-owned delivery work, statistically comparable candidate
evidence, costs and correctness results, ordinary checks/review/merge/release,
and staged production observations. Noisy confidence, correctness failure,
stale pull revisions, and a missed rollout target must block or pause the
workflow until an attributable retry supplies current evidence; confidence
bounds and delivery thresholds are both percentages.

Project governance charters live beneath `$GOVERNANCE_ROOT` (default
`apps/api/data/governance`). Repository owners and organization owners publish
immutable revisions defining roles and eligibility, decision classes,
participation/quorum/threshold rules, protected resources, terms, removal,
succession, vacancy, and amendment policy. Draft publication derives a live
preview against current ownership, membership, teams, branch/integration,
release, environment, security, and approved-agent boundaries. Unsupported
resources, impossible role minimums, and impossible quorum block activation;
activation also requires an attributable approval and always re-previews live
state. Charters grant no operational authority. Approvals and time-bounded
exceptions remain version-bound, while later revisions preserve completed
active versions. The repository surface is `view=governance`; organizations
present the same charter in their workspace.
Active charters admit time-bounded participant standing from cited contribution,
review, support, ownership, or membership evidence. Invitations, acceptance,
recusal, resumption, suspension, expiry, conflict disclosure, identity
revocation, and federation-trust revocation remain attributable. Public reads
show eligibility, role, responsibilities, term, nominations, appeals, and an
explicitly empty operational-authority boundary; governance voice never grants
code, secret, merge, deployment, membership, or credential access.

Active governance standings also form the live electorate for governed proposals
beneath each repository or organization governance charter. Eligible participants
may open technical decisions, initiatives, policy exceptions, funding or resource
requests, leadership nominations, and charter amendments with frozen charter
rules, alternatives, citations, affected resources, disclosures, discussion
deadline, and implementation effects. Human ballots are single-use and may be
attributable or sealed until tally; agents may add cited analysis but never vote.
Finalization re-evaluates active, unexpired, unrecused standing, separates
abstentions, excludes newly ineligible ballots, applies quorum and threshold
deterministically, and retains a digest, dissent, contests, and resolutions.
Governance outcomes remain evidence and grant no operational authority.

Approved governed proposals create an immutable decision receipt binding the
charter, tally, winner, scope, affected resources, and effects. Their public
implementation record separates the community mandate from required owner
approval and links ordinary policy revisions, initiatives, task plans, role
transitions, or bounded access requests plus blockers back to that receipt.
Those resources retain existing review, integration, release, environment,
extension, and agent controls. Material changes require a new or amended
decision and cannot be appended beneath the prior mandate.

Governance continuity cases extend an active charter beneath `/stewardship`.
Nomination, election, term expiry, recall, succession, deadlock, appeal, and
emergency events retain standings and decision receipts; removal clears derived
governance authority without deleting evidence or prior decisions. Resource
handoffs remain separately owner-approved external actions. Emergency scope is
allowlisted, reviewed, limited to thirty days, and relinquished explicitly or
on the next mutation after expiry. `/health` derives vacancies, expiring terms,
quorum loss, unresolved handoffs, deadlocks, appeals, and emergency powers.
`governance_workflow_test.go` is the black-box regression boundary for the
complete charter-to-delivery-to-renewed-stewardship loop. It retains proven
standing, recusal and failed quorum, evidence-backed approval with dissent,
human-verified agent work, ordinary checks/review/merge/release, successor
election and owner-approved handoff, appeal, and bounded emergency recovery
while proving governance never replaces repository authority.

External integration identities live beneath `$EXTENSION_ROOT` (default
`apps/api/data/extensions`). An authenticated developer registers an extension
with its human owner, operator email, declared capabilities, HTTPS callback and
action endpoints, requested repository permissions, supported events, and
credential-rotation policy. Registration creates a distinct `ext_` principal;
one-time endpoint challenges must verify both declared endpoints before a
repository owner can install it. Authority previews and installations may only
narrow the registered declaration, retain the exact repository, events,
permissions, installer, and extension actor, and explicitly report that no user
or agent impersonation and no credential issuance occurred. Repository owners
alone install and revoke; revocation empties effective authority while retaining
the attributable record. The Access web surface owns extension registration.
Installations are versioned grants with explicit resource types, approve-or-deny
capability decisions, and bounded non-secret settings. Upgrade, suspension,
resume, removal, and publisher transfer retain actor history; suspension and
removal immediately empty effective authority without disturbing other grants
or attributed evidence. Organization owners must explicitly select repositories
owned by that organization, all validated before creation. These contracts
issue no reusable human credential.

Active extension installations consume only matching repository activity and
materialize immutable schema-version `1` deliveries beneath `$EXTENSION_ROOT`.
Each delivery carries a stable source event ID, per-installation ordering ID,
resource identity, bounded redacted changes, and a payload digest. Callback
requests use `application/vnd.komodo.extension-event+json; version=1` and an
HMAC-SHA256 `X-Komodo-Signature-256` over `{unix_timestamp}.{exact_body}` with
the accompanying timestamp, delivery, event, and ordering headers. Repository
readers can inspect retained payloads and attempts; repository owners trigger
delivery or replay. Non-2xx attempts use bounded exponential retry state and
become dead letters after five failures. Reconciliation deduplicates source
events and silently excludes unsubscribed resource types; suspended, removed,
and revoked installations cannot send or replay retained deliveries.
Active installations may also receive a rotatable one-time `vke_` credential
scoped to that installation and repository. Through the public extension
contribution endpoints it can publish idempotent, exact-revision status,
checks, annotations, artifacts, links, and comments, and declare bounded
actions with previewable inputs and effects. Contributions always retain the
`ext_` actor, installation, resource, revision, usage, and an `advisory_only`
policy effect; they never satisfy or bypass ordinary reviews, required checks,
merge, release, deployment, environment, or embargo controls. Collaborator
invocations retain the human actor and create only a requested extension
action; suspension or removal invalidates the credential immediately.
Repository extension operations derive attributed requests, delivery failures,
latency, consumption, permission use, configuration, contribution, invocation,
and contract-test evidence. Installation credentials expire after ninety days;
rotation retains issuance/expiry metadata without stored secrets. Missing or
expiring credentials, dead letters, and anomalous consumption emit actionable
notices. Contract probes grant no authority; quarantine, pause, narrowing, and
removal withdraw future authority while retaining prior evidence. Newly
requested capabilities or permissions always require renewed owner consent.
`extension_workflow_test.go` is the black-box regression boundary for the
complete developer-registration-to-governed-merge loop. It retains repository-
only owner consent, signed pull-request delivery and replay, revision-bound
annotations and artifact evidence, collaborator repair invocation, ordinary
checks and review, renewed capability consent, and post-removal attribution
while proving that the installation credential no longer works.

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
  Regression search boundaries are separate durable resources beneath
  `$REGRESSION_INVESTIGATION_ROOT` (default
  `apps/api/data/regression-investigations`) and share `view=investigations`.
  Repository writers open one from an issue, support thread, failed check,
  release, deployment, or reproduction. Resolve known-good and known-bad
  revisions or releases to visible commits and require the good commit to be
  an ancestor of the bad commit; retain their original references so moved
  branches surface as stale inputs. The current scope records expected and
  regressed behavior, affected environments, comparability, severity, owners,
  and acceptance criteria, while derived blockers expose omissions. Scope
  changes use optimistic versions and preserve full attributed snapshots;
  discussion, hypotheses, status changes, and public or repository-visible
  evidence remain append-only. A ready boundary grants no testing, Git,
  repository, agent, release, deployment, or operational authority.
  Immutable regression scenarios and append-only historical attempts remain
  beneath that investigation. Repository writers and scoped agents derive or
  define bounded commands, inputs, fixture safety, environment requirements,
  timeouts, and cost limits, then record isolated attempts against resolved
  commits, attested releases, or exact dependency combinations. Each attempt
  freezes the environment image and definition digest, OS, architecture,
  toolchain, dependency lock, setup, commands, outputs, sanitized logs,
  artifact metadata, cost, repetitions, runner provenance, and one explicit
  behavior or non-evidence classification. Incompatible setup, missing
  dependencies, flakiness, unsafe fixtures, and untestable revisions never
  become expected or regressed behavior evidence. Scenario and attempt records
  grant no Git, agent, package, release, credential, environment, or execution
  authority.
  Evidence-driven searches are immutable graph snapshots beneath a regression
  investigation. A search binds one scenario, confidence target, good and bad
  graph keys, exact local commits with their complete ordered parent lists, and
  explicitly selected cross-repository or package revisions. Revisions retain
  summaries, relevant diff paths, owners, pull requests, and decisions. The
  derived projection schedules unclassified candidates and shows remaining
  keys, tested classifications, competing transition ranges, confidence, and
  blockers. Working or regressed classifications require cited scenario
  attempts; invalid, flaky, and inconclusive trials never narrow the range.
  Merge transitions remain ambiguous until parent effects are disambiguated,
  and multiple supported ranges never become a single verdict. Human and agent
  causal hypotheses require cited evidence, implicated revisions, confidence,
  and an explicit proposed, supported, disputed, or rejected state. Graph
  snapshots and searches grant no Git, package, agent, runner, pull-request,
  decision, review, merge, release, deployment, or operational authority.
  Owner-governed regression responses remain beneath the exact investigation.
  Each response compares evidence-cited revert, rollout or configuration
  containment, dependency adjustment, and forward-repair options against
  affected releases and current work, with deliberate tradeoffs and backport
  targets. A selection retains the supported culprit range, reproduction,
  constraints, acceptance criteria, original intent, and authorship. Owners may
  link ordinary human- or agent-owned tasks, sessions, or shared workspaces;
  published pull-request links carry that preloaded context but grant no Git,
  rollback, review, merge, release, deployment, environment, or operational
  authority.
  Exact repair and backport candidates continue beneath the response. Each
  freezes its commit or attested release, investigation and scenario versions,
  affected checks and requirements, regression criteria, and the introducing
  change's intent and acceptance criteria. A proof must cite the same historical
  scenario at the candidate commit, both known-working and known-regressed
  baselines, and a complete passing result for every frozen check, requirement,
  and criterion; partial results and stale baselines remain blockers. A
  maintainable scenario may cite its quality-plan entry and required-check name.
  Append-only review, merge, release, deployment, and observed-outcome events
  retain exact revisions and resources. A failed backport, reverted valid
  behavior, or production disagreement reopens both the candidate and its
  investigation instead of allowing a green delivery status to close the
  regression. These proof and delivery records grant no test execution, Git,
  quality-policy, review, merge, release, deployment, environment, telemetry,
  rollback, or operational authority.
  `regression_recovery_workflow_test.go` is the black-box boundary for the
  complete report-to-sustained-recovery loop through the public API and stock
  Git history. It retains attributed user impact, reproducible human-agent
  experiments, flaky and unbuildable midpoints, a rejected dependency culprit,
  merge-introduced ambiguity and supported causal reasoning, containment and
  owner-gated work, failed revert review, exact forward-repair and backport
  proofs, ordinary delivery, revoked reporter access, production disagreement,
  corrected rollout observation, and quality-plan regression coverage without
  treating investigation records as execution or delivery authority.
  Cross-line change propagation campaigns live beneath
  `$PROPAGATION_CAMPAIGN_ROOT` (default `apps/api/data/propagation-campaigns`).
  Repository writers open one only from an exact locally readable commit set
  and a merged pull request, security repair, regression correction, policy
  change, package release, or interface evolution. The immutable campaign
  retains intent, acceptance criteria, evidence references, ordered target
  release lines, repositories, packages, owners, deadlines, and completion
  policy. Every target carries an observed authority basis and an explicit
  pending, unknown, unsupported, inaccessible, or already-equivalent
  disposition; dependency cycles and implicit non-pending states are rejected.
  Reads expose non-equivalent gaps as blockers in `view=propagation` without
  granting target repository, package, Git, review, merge, release, deployment,
  credential, environment, or operational authority.
  Permitted campaign targets retain append-only applicability assessments bound
  to exact source and target revisions. Each assessment compares histories,
  symbols, dependencies, interfaces, schemas, prior fixes, and release
  commitments with citations before classifying the target as directly
  applicable, already satisfied, adaptation required, conflicting, or not
  applicable. An already-satisfied result requires identified behavioral proof;
  similarity alone is only comparison evidence. Humans and read-only agents may
  append cited findings, risks, and uncertainty, while only named target owners
  acknowledge or request changes. A later assessment stales prior assessments
  for that target only and preserves every other target's current analysis.
  A current applicable assessment may become an ordered target-local
  contribution plan with human or agent ownership, scoped task, session,
  workspace, fork, pull-request, or federated-pull references, source authors,
  relevant commits, constraints, acceptance criteria, and citation-only context.
  Direct plans retain proven authorship unchanged; adapted plans must state
  every deviation. Inaccessible or unknown targets cannot receive plans, and an
  independently owned target's pull request must cite an ordinary fork or
  federated contribution path. Plans grant no task, session, workspace, fork,
  repository, Git, review, merge, release, or operational authority.
  Campaign authors may freeze reusable behavioral scenarios derived from the
  exact source outcome, including source evidence, required coverage, ordinary
  check names, bounded environment, timeout, and cost. Append-only target
  attempts bind the current applicability assessment, contribution adaptation,
  source and target revisions, dependencies, and assumptions, and retain
  commands, sanitized logs, content-addressed artifacts, coverage, costs, and
  residual implementation differences. Unsupported scenarios prove equivalence
  only with their predeclared substitute evidence. A newer attempt invalidates
  prior evidence for that target alone; other target revisions remain current.
  Named target owners accept, reject, or request changes without granting test,
  runner, repository, Git, review, merge, release, deployment, environment, or
  operational authority. The resulting matrix remains in `view=propagation`.
  Verified targets accept append-only owner-attributed delivery receipts for
  ordinary review, queue, merge, release, deployment, and observed-outcome
  resources. The campaign derives per-target paused state, supported-user
  exposure, blockers, and next actions without dispatching those operations.
  Failed or rejected receipts pause only their target; bounded 30-day owner
  exceptions, superseded targets, and newly discovered consumers remain visible
  coverage gaps. Completion is derived from required target outcomes and cannot
  be declared while a required delivery or discovered consumer is unresolved.
  `propagation_workflow_test.go` is the black-box boundary for the complete
  verified-repair-to-ecosystem-coverage loop through the public API and stock
  Git histories. It retains direct and divergent human-agent adaptations,
  federation, already-fixed and inaccessible consumers, selective assessment
  and evidence staleness, a failed adaptation, rejected upstream delivery,
  release recovery, bounded owner exceptions, local governance, costs, and
  observed supported-user outcomes without transferring target authority.
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
  Pull request conflict evidence is a caller-aware read at the request's
  `/conflicts` resource. It freezes the represented source and target commits,
  derives their common base, side-only commits and file changes, and reports
  live branch drift without rewriting either ref. Classify stock-Git merge
  failures as textual, overlapping schema or interface definitions as
  structural, and independently overlapping symbols or failed exact-revision
  checks as semantic evidence. Preserve links to each available pull request,
  proposal task, discussion, author, description, and recorded completion or
  contribution criteria; absent intent, checks, inaccessible repositories, and
  unavailable ancestry remain visible gaps rather than inferred resolution.
  A current complete conflict analysis may launch a shared reconciliation
  workspace at `POST .../conflicts/workspace`. The workspace freezes the common
  base, both exact revision identities and side-only histories, audience-safe
  conflict evidence, affected owners, and the target revision's repository-
  defined setup. Its publication repository is the target repository where the
  launcher already has write access; collaboration controls grant only bounded
  workspace access and never copy authority from either repository.
  Reconciliation workspaces retain an append-only semantic resolution ledger at
  `POST .../workspaces/{workspace}/resolutions`. Questions, answers, proposals,
  applied edits, and undo records cite a frozen base, source, target, or
  workspace revision and identify their effect on acceptance criteria, design
  decisions, migrations, and user behavior as preserved, intentionally changed,
  or unknown. Preserve authorship, assumptions, uncertainty, and parent links;
  the ledger explains work but does not apply code or grant merge authority.
  Every reconciliation checkpoint also assembles an immutable verification
  candidate whose digest binds its captured bytes, frozen source and target,
  workspace dependency definition, effective policy, and criteria derived from
  repository commands, declared reproductions, and resolution impacts. Writers
  append typed required-check, reproduction, contract, schema, preview, and
  conflict attempts at its `/verification-attempts` resource; attempts retain
  commands, sanitized logs, content-addressed artifacts, coverage, failures,
  costs, actor, and exact input revisions. Frozen affected owners append approve
  or reject decisions at `/verification-decisions`. Reads derive candidate
  status and stale input keys from each criterion's affected inputs, preserving
  unaffected proof and decisions when only source, target, dependency, or policy
  inputs move. Verification grants no publication, merge, credential, or
  environment authority.
  Publishing reconciliation is a separate accepted-proof transition at the
  checkpoint's ordinary `/publication` resource. Re-read the live source and
  target refs, target workspace definition, effective policy, affected-owner
  approvals, and open origin pull request before writing Git. A target-
  repository writer may compare-and-swap the exact same-repository origin
  source branch or create a distinct connected resolution pull request; fork
  branches never acquire target write authority. Resolution commits retain both
  frozen parents plus workspace, checkpoint, verification, resolution-entry,
  approval, command, publisher, and contributor provenance. Synchronizing the
  origin source makes prior reviews stale, and either route starts ordinary
  commit-bound checks. Queue candidates are rebuilt against the live target and
  must retain the current exact-source owner approval immediately before atomic
  merge. Source movement, withdrawn approval, target drift, or a new merge
  conflict blocks or removes only the queue attempt and preserves both inputs
  for another reconciliation.
  `conflict_resolution_workflow_test.go` is the black-box boundary for the
  complete independently reviewed change-to-integrated-result loop. It retains
  textual and semantic conflicts, competing intent, stale and concurrent
  revisions, repeated queue conflicts, bounded and revoked agent participation,
  rejected suggestions, failed then corrected combined verification, both-owner
  decisions, two-parent attributed publication, exact review, and final queue
  history without treating workspace or agent access as merge authority.
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

Repository data-use commitments live beneath `$DATA_COMMITMENT_ROOT` (default
`apps/api/data/data-commitments`). Repository writers publish immutable,
optimistic-concurrency versions across repository, release, extension,
experiment, and environment scopes. Each version explicitly declares data
categories, purposes, subjects, collection, processing, sharing, retention,
residency, deletion, consent, accountable owners, supported or unsupported
guarantees, applicable policy and user-facing notice links, and attributable
time-bounded exceptions. Repository readers can inspect complete history and
derived missing-ownership, unsupported-guarantee, conflicting-term, and
expiring or expired-exception blockers at `view=privacy`. A declaration or
exception is project evidence only and grants no repository, data, extension,
experiment, environment, credential, or operational authority.

Revision-exact data-flow maps live beneath `$DATA_FLOW_ROOT` (default
`apps/api/data/data-flows`). Repository writers publish declarations from an
exact commit and manifest blob, connecting interaction, interface, package,
store, extension, release, environment, audience, and external-recipient nodes
with `enters`, `moves`, `persists`, and `leaves` edges. Every map cites exact
data-use commitment versions and data-use IDs; server-captured source blob
identities ground code nodes. Repository readers, including read-only agents,
may append bounded cited observations and uncertainty. Reads derive stale
analysis, undeclared or differently observed movement, inaccessible dependency
evidence, and declared paths not observed by current analysis. Restricted
evidence bodies never enter this store or the `view=privacy` surface: retain
only an explicit inaccessible state and bounded reference. Maps and findings
grant no data, repository, extension, release, environment, credential, or
operational authority.

Runtime privacy verification policy lives beneath `$PRIVACY_VERIFICATION_ROOT`
(default `apps/api/data/privacy-verifications`). Schema-version `1`
`.komodo/privacy-checks.json` declarations bind synthetic journeys, exact input
blobs, data-use commitments, and coverage for collection, consent,
minimization, access, retention, export, deletion, telemetry, and recipients.
They run in the ordinary credential-free, networkless exact-revision check
sandbox; privacy logs and artifacts are sanitized before retention, and
unchanged declared inputs may reuse successful evidence with explicit lineage.
Owner policies scope required current checks and coverage by branch and path.
Only named privacy owners can acknowledge evidence from an exact preview;
candidate changes stale both evidence and acknowledgement. Scoped exceptions
must expire within 90 days and cite an issue, proposal, or task. The same
assessment governs pull-request merge and exact-candidate release readiness
without granting data, preview, repository, merge, release, or operational
authority.

Project learning pathways live beneath `$LEARNING_PATHWAY_ROOT` (default
`apps/api/data/learning-pathways`). Repository writers publish immutable,
optimistically concurrency-checked role- or outcome-based versions with
prerequisites, objectives, supported revisions, ordered modules, practical
exercises, expected effort, mentors, learner environments, accessibility and
localization needs, and required completion evidence. Module resources bind
exact repository revisions to documentation, symbols, decisions, issues, API
contracts, packages, and contributor guidance. Reads derive stale or
inaccessible material, departed mentors, missing ownership, and unsupported
environments without rewriting history. The public API is
`/repositories/{repository}/learning-pathways`, and the repository web surface
is `view=learning`. Publishing or reading a curriculum grants no repository,
Git, workspace, agent, assessment, contribution, review, or merge authority.

Pathway module practice attempts live beneath `$LEARNING_EXERCISE_ROOT` (default
`apps/api/data/learning-exercises`). An authenticated repository reader launches
one against an immutable pathway version, module, exercise, and exact supported
Git revision. The detached, unpublished workspace freezes declared tool
versions, synthetic or explicitly permitted dataset digests, setup commands,
instructions, acceptance criteria, command and cost ceilings, disabled network,
and explicit denial of credentials, production data, and authoritative branches.
Append-only learner events retain setup, sanitized commands and outputs,
content-addressed checkpoints, hints, acceptance checks, recovery, cost, and
completion. Reproducibility requires retained setup, command, checkpoint, and
check evidence; credential-shaped content is rejected. Attempts grant no Git,
branch, credential, data, publication, contribution, review, or merge authority.

Each attempt also owns a permission-aware help timeline at `/{attempt}/help`.
The learner selects exact exercise event numbers to disclose and may invite only
a mentor named by the frozen pathway or an agent whose repository onboarding is
currently active and identity-matched. Explanation, hint, demonstration, and
direct action remain distinct; every guidance entry retains its author and exact
module-resource citations. Demonstration and mentor workspace observation or
joining require control recorded in the learner's request. Learners can guide,
pause, or irrevocably revoke an invited agent; agents cannot take direct action
or emit solution, hidden-assessment, secret, or inaccessible-context material.
The timeline exposes no unselected exercise state and grants no workspace,
assessment, repository, agent, credential, branch, contribution, or review
authority.

Practical learning assessments live beneath `$LEARNING_ASSESSMENT_ROOT`
(default `apps/api/data/learning-assessments`). A repository writer publishes
one immutable definition for the exact current pathway version and default-
branch revision, including a public rubric, content-digested protected cases,
required repository checks, accountable reviewers, retry limits,
accommodations, and independent appeal owners. Learner attempts freeze a
reproducible workspace digest, commands, declared assistance, revision, and
append-only evidence. Named reviewers record evidence-linked criterion
decisions, feedback, uncertainty, and copied-solution or agent-overreach
findings. Completion is derived only from a current revision and pathway,
stable passing checks, conclusive human judgment, and every passing criterion;
criteria drift, stale code, flaky or failed checks, copying, and agent overreach
remain blockers. Read projections disclose protected-case titles and digests,
never private material, and scope attempts to learners and accountable owners.
The API is nested at `/repositories/{repository}/learning-pathways/{pathway}/assessments`
and the repository surface remains `view=learning`. Assessment records grant no
repository, Git, agent, contribution, review, merge, or operational authority.

Consent-bounded pathway outcome records live beneath `$LEARNING_OUTCOME_ROOT`
(default `apps/api/data/learning-outcomes`). Repository collaborators append
aggregate module completion, recurring-question, setup-failure, assessment-gap,
mentor-load, contribution, reviewer-correction, and retention observations with
an explicit audience, granted consent, count, exact pathway/project revision,
and evidence references. Human maintainers and scoped agents may connect only
cited observations to supported or uncertain findings, then propose reviewed
documentation, exercise, workspace, pathway, code, or policy improvements
through an ordinary pull request or proposal. Material requirement changes name
affected learner completion records and require an append-only revalidation on
the new pathway version; prior versions, achievements, findings, rejected work,
and evidence remain intact. The nested API is
`/repositories/{repository}/learning-pathways/{pathway}/outcomes`, and
`view=learning` reports the consent-safe trail. These records grant no learner
surveillance, repository, Git, agent, review, merge, or operational authority.

Current supported learning can enter ordinary contribution without becoming an
access grant. `GET /repositories/{repository}/learning-pathways/{pathway}/contribution-matches`
returns ready opportunities at the exact revision of a learner's supported
assessment completion. After the learner claims one normally, its start request
may select that assessment attempt and learner-owned completed, reproducible
exercise attempts. The private fork workspace retains their pathway, module,
revision, authorship, and declared-assistance context; publication copies the
same review-safe references into the pull request alongside mentor and agent
support and acceptance evidence. Claims, forks, checks, review, integration,
readiness outcomes, and any later responsibility grant keep their native owner
and permission checks. Learning completion grants no repository, fork, Git,
agent, secret, review, merge, governance, or operational authority.

`developer_learning_workflow_test.go` is the black-box boundary for the complete
project-native learning-to-trusted-contributor loop. It crosses public learning,
assessment, matching, repository, pull-request, review, check, merge, and outcome
APIs plus stock Git, retaining missing prerequisites, broken setup, inaccessible
material, misleading help, protected-case secrecy, failed assessment, stale
modules, departed mentors, abandoned work, recovery, learner authorship, and the
reviewed pathway improvement that serves the next learner.

Pull-request privacy impact assessments live beneath
`$PRIVACY_ASSESSMENT_ROOT` (default `apps/api/data/privacy-assessments`). Each
assessment binds exact candidate and target commits, cited flow maps,
commitment versions, and server-captured source blobs. It classifies collection,
purpose, recipient, retention, access, and user-control changes and assigns
owner acknowledgements, notices, consent changes, migrations, tests, or
exceptions. Readers and scoped agents may append cited challenges, mitigations,
and residual risk; only named requirement owners acknowledge. A changed pull
revision or cited blob makes evidence and prior acceptance stale. These records
grant no data, repository, merge, release, credential, or operational authority.

Post-release privacy drift lives beneath `$PRIVACY_DRIFT_ROOT` (default
`apps/api/data/privacy-drift`). Repository owners permit aggregate production
monitors only against exact commitment versions, governed data uses, releases,
release revisions, environments, optional extensions, named owners and
participants, drift classes, and bounded evidence retention. Authorized
collaborators retain only sanitized signal references, aggregate counts,
windows, digests, and summaries for undeclared flows, excessive retention,
failed deletion, consent mismatch, or unexpected recipients; raw personal data
and subject identifiers never enter the store. The private repository API
retains attributable containment, bounded participant notification, private-
incident or governed-exception links, resolution, and ordinary human- or
approved-agent repair tasks based at the affected release revision. Signals and
controls grant no data, environment, extension, credential, review, merge,
release, or deployment authority.

`privacy_workflow_test.go` is the black-box regression boundary for the full
commitment-to-corrected-data-use loop. It retains an existing journey, bounded
inaccessible extension evidence, stale impact analysis after human-requested
design revision, a rejected exception, current synthetic consent,
minimization, retention, and deletion proof, privacy-owner approval, ordinary
agent-authored merge and release, sanitized production drift, a permission-
scoped investigation link, an evidence-preloaded repair, and revoked extension
authority. Drift repair tasks copy an immutable reasoning context containing
the affected release revision, expected behavior, verification criteria, and
sanitized signal reference; that context grants no data or operational access.

Repository locale plans live beneath `$LOCALE_PLAN_ROOT` (default
`apps/api/data/locale-plans`). Repository writers publish immutable,
optimistic-concurrency versions for repository, product, documentation, and
release scopes. Plans declare target languages and regions, ordered fallbacks,
terminology, supported formatting requirements, covered journeys, owners,
reviewers, and per-locale release thresholds. Translatable resources bind a
path and format to an exact Git commit; attributable coverage binds the same
resource, locale, journey, plan version, and source revision. Reads derive
missing ownership and coverage, unsupported formats, conflicting terminology,
stale coverage, and unmet release thresholds. The shareable repository surface
is `view=locales`; plans and coverage grant no repository, review, merge,
release, translation-provider, credential, or operational authority.

Pull-request translation extraction lives beneath `$TRANSLATION_UNIT_ROOT`
(default `apps/api/data/translation-units`). Schema-version `1`
`.komodo/localization.json` at the exact pull source revision declares the
source locale, target locales, JSON resources, locale-path templates, message
context, screenshots, plural rules, and stable resource identifiers. Extraction
captures configuration and source blob identities and projects stable
resource/key units as added, changed, removed, or reused, with translated,
untranslated, superseded, or removed state per locale. Repository readers may
submit attributable unit proposals through the pull-request localization
surface; later source edits supersede affected proposals and unchanged
translations without deleting history. Extraction and proposals grant no Git,
repository-write, review, merge, release, translation-provider, credential, or
operational authority. The shareable repository surface remains `view=locales`.

Collaborative translation stays on that exact extraction. Repository readers
claim locale work with optimistic versions, hand it to another permitted
collaborator, and append unit-scoped discussion without receiving source-write
access. An extraction may freeze a current locale-plan version so approved
terminology and required regional reviewers remain visible even if the plan
later changes. Agent suggestions bind the exact source revision and product
context, name the scoped agent and requesting human, and require cited evidence
plus explicit uncertainty. They remain pending until a different human edits,
approves, rejects, or escalates them; accepted text is attributed to that human
and retains its agent origin. Proposal reviews enforce the frozen reviewer set.
Protected or embargoed extraction content is omitted from both list and detail
reads outside its explicit actor audience, and the same boundary covers claims,
discussion, proposals, suggestions, decisions, reviews, and handoffs.

Localized experience verification lives beneath
`$LOCALIZATION_VERIFICATION_ROOT` (default
`apps/api/data/localization-verification`). Schema-version `1`
`.komodo/localization-checks.json` declarations bind variable, pluralization,
formatting, terminology, link, layout-expansion, bidirectional-text, fallback,
and localized-journey checks to an exact pull revision, locale, optional route,
translation units, and interface paths. Retained outcomes separately digest
source, translation, and interface inputs so only evidence affected by a
changed input becomes stale. Ready exact-revision previews can be narrowed to
locale-specific route allowlists and the regional reviewers frozen by the
translation extraction. Translators append route-, revision-, unit-, and
interface-grounded linguistic or functional findings; only invited reviewers
approve or reject affected content. Checks, preview access, findings, and
decisions grant no repository, review, merge, release, credential, or
operational authority. The repository surface remains `view=locales`.

Localization delivery governance lives beneath `$LOCALIZATION_DELIVERY_ROOT`
(default `apps/api/data/localization-delivery`). Repository writers publish
branch-, path-, audience-, risk-, and locale-scoped policies requiring minimum
coverage, named current localization checks, and current regional reviewer
approvals. Each exact pull candidate stages, defers, or withdraws locales with
optimistic concurrency; only staged locales are readiness blockers, so an
explicitly deferred or withdrawn locale cannot be mistaken for supported and
does not block unaffected locale releases. Pull readiness and merge enforce the
same exact-revision assessment. Application releases and documentation
collections retain per-locale publication records with version, source
revision, candidate version, provenance, fallback, and published or withdrawn
state. Repository readers report publication-bound mistranslation, cultural
mismatch, broken formatting, or missing content; writers validate the finding
before linking an existing ordinary human- or agent-owned proposal task with
immutable acceptance criteria. The `view=locales` reader surface exposes the
complete publication and correction trail. These records grant no Git,
translation-provider, review, merge, release, documentation-publication,
credential, or operational authority.

`localization_workflow_test.go` is the black-box regression boundary for the
complete source-change-to-corrected-global-release loop. It retains a
source-superseded human translation, a grounded agent suggestion edited by a
translator, exact-preview regional approval, missing-reviewer containment, a
right-to-left failure and explicit locale withdrawal, ordinary checks, review,
merge, and release, plus a reader finding returned through an agent-owned
proposal task and corrected locale publication. Locale candidates bind the
pull's exact source commit while application publications bind the resulting
merge commit; both identities are verified and retained rather than conflated.

## Reliability investigations

Revision-bound reliability investigations live beneath
`$RELIABILITY_INVESTIGATION_ROOT` (default
`apps/api/data/reliability-investigations`). Repository readers and read-only
agents open one from an objective, pull request, deployment, or
budget-consumption event with frozen objective terms, journeys, repository
revision, and baseline-versus-affected evidence. Participants retain cited
hypotheses, comparisons, challenges, conclusions, uncertainty, and bounded
service or dependency-owner input requests. Reads derive stale evidence,
disputes, inconclusive signals, and unanswered dependency input. Only writers
invite participants or link issues, incidents, decisions, and planned
improvements; investigations grant no telemetry, alert, rollback, repository,
credential, deployment, or operational authority. The reader surface is
`view=reliability`.

Reliability improvements live beneath `$RELIABILITY_IMPROVEMENT_ROOT` (default
`apps/api/data/reliability-improvements`). Repository writers convert a current
supported investigation conclusion or depleted error-budget event into one
ordinary proposal with ordered human- or agent-owned tasks. The retained record
freezes the objective version, affected revisions and journeys, baseline,
dependency context, evidence, acceptance criteria, ownership, and task
dependencies. Attributable delivery links connect exact pull-request, check,
release, deployment, and decision revisions without replacing their existing
permissions or policy. Governed rollout observations compare current indicators
with the recorded baseline: failed evidence derives containment, rollback, or
decision-revisit action while successful evidence restores the derived budget
state. Every attempt and the original impact remain immutable in the record;
improvements grant no Git, review, merge, release, deployment, rollback,
telemetry, credential, or operational authority. The reader surface remains
`view=reliability`.

## LADDER.md

`/LADDER.md` at the repo root is **not part of this repository**. It is placed
there from outside the checkout — as a symlink or a read-only bind mount — and
is gitignored for that reason. Read it for context on what is being built, but
never edit it, `git add` it, or delete it, and do not treat its presence as
uncommitted work.

Durable production-debugging starting context lives beneath
`$DEBUGGING_WORKSPACE_ROOT` (default `apps/api/data/debugging-workspaces`). An
authorized collaborator opens a workspace from an issue, incident, support
thread, deployment, service objective, trace, or manual observation. It binds
the affected immutable release and exact Git commit, environment, time window,
user journey, severity, owners, package, configuration, and infrastructure
revisions, and audience-scoped permitted evidence references. Unavailable
context remains an explicit reasoned gap. Participants add attributable
hypotheses and status/access changes retain attributable history. The workspace
stores no runtime evidence or secrets and grants no repository mutation,
credential, deployment, environment, or operational authority. The repository
web surface is `view=debugging`.

Privacy-safe runtime probes live beneath `$RUNTIME_PROBE_ROOT` (default
`apps/api/data/runtime-probes`) and always bind one production-debugging
workspace and its affected environment. A participant requests a logs, traces,
profile, state-snapshot, or exact-source repository diagnostic probe for at
most 24 hours with bounded scope, purpose, consent actors, and a preview of
data categories, cost, load, sampling, retention, privacy/security policy, and
audience. A named workspace owner separately approves or denies collection.
Append-only captures retain collector provenance, timing, expected and captured
counts, explicit gaps, completeness, transformations, and sanitized bounded
records; secret- and user-data-shaped fields are redacted before persistence.
Expiry, owner revocation, overload, and consent revocation stop collection,
while narrowing remains explicit and partial captures are always labeled
incomplete. Probe requests, approvals, evidence, and controls issue no provider,
environment, deployment, credential, data, repository-mutation, or operational
authority. The repository web surface remains `view=debugging`.
Shared live-behavior investigations live beneath `$RUNTIME_INVESTIGATION_ROOT`
(default `apps/api/data/runtime-investigations`) and remain beneath one exact
production-debugging workspace. Participants select sanitized probe captures
and correlate them with exact-revision symbols, commits, dependencies,
configuration, infrastructure, deployments, and known issues. Every hypothesis,
reproducible query, finding, challenge, support statement, and uncertainty is
attributable and cites retained evidence, a correlation, or another claim;
derived projections expose proposed, supported, disputed, stale, and
inaccessible-evidence-blocked claims. Code paths resolve against the affected
commit, participant-only evidence cannot enter a repository-audience
investigation, and code, service, privacy, and security owner-input requests
remain advisory. SSE events stream the append-only explanation. A delegated
agent receives one shown-once, at-most-24-hour credential limited to selected
investigation evidence and correlations and may only read that context and add
cited claims; guidance, pause, resume, expiry, and revocation remain
attributable. Investigations and agents receive no secret, provider, telemetry,
Git, repository mutation, deployment, environment, or operational authority.
The repository web surface remains `view=debugging`.
Runtime-informed repairs live beneath `$RUNTIME_REPAIR_ROOT` (default
`apps/api/data/runtime-repairs`). Repository writers create ordinary human- or
agent-owned proposal tasks only from one reproduced workspace replay and a
current supported causal claim, freezing the workspace, replay, cause,
affected revision, and acceptance and regression criteria. Exact pull-revision
verification requires a candidate replay attempt that no longer observes the
failure plus every named ordinary required check at that same commit. Staged
production validation binds an actual release and deployment and retains
sanitized signals comparing the original behavior with the delivered result.
Failed signals derive only pause, known-good restoration, or diagnosis reopening;
these records grant no Git, review, merge, release, deployment, environment,
telemetry, credential, rollback, or operational authority. The repository web
surface remains `view=debugging`.
Privacy-safe runtime replays live beneath `$RUNTIME_REPLAY_ROOT` (default
`apps/api/data/runtime-replays`) and remain beneath one exact production-
debugging workspace. Participants derive a minimized synthetic or privacy-
preserving scenario only from permitted sanitized capture or investigation
evidence, retaining transformations rather than protected source values.
Scenarios bind commands and invariants to the affected revision and run only
against a named isolated workspace or preview. Append-only attempts retain the
environment, commands, sanitized traces and outputs, invariant results, cost,
and production differences. Two clean matching attempts are required for the
derived reproduced state; changed revisions, nondeterminism, missing
dependencies, unsafe side effects, and irreducible production conditions remain
explicit blockers. Human and agent participants may append attributable
refinements, but replays grant no provider, production-data, secret, telemetry,
Git, deployment, environment, credential, or operational authority. The
repository web surface remains `view=debugging`.
`production_debugging_workflow_test.go` is the black-box boundary for the
complete released-user-observation-to-confirmed-repair loop. It retains user
consent, a denied probe, redaction, noisy and corrected capture, exact code and
deployment correlations, challenged reasoning, revoked agent access, synthetic
reproduction, failed and passing candidate verification, agent commit
authorship, ordinary review/check/merge/release/deployment, a paused first
stage, and confirmed production outcome. Candidate attempts use the explicit
`repair_verification` replay mode: unlike reproduction attempts, they may bind
the pull's exact changed revision, but only a clean attempt where the original
invariant is absent can satisfy runtime-repair verification.

Versioned repository quality plans live beneath `$QUALITY_PLAN_ROOT` (default
`apps/api/data/quality-plans`). Repository writers publish immutable,
optimistically concurrency-checked agreements covering repository, release,
journey, interface, and supported-environment scopes. Versions retain risks,
expected and explicitly untestable behaviors, test levels, representative-data
descriptions and privacy classes, coverage goals, accountable owners and
judges, schedules, release thresholds, expiring exceptions, and change reasons.
Requirement links distinguish issue, decision, design, accessibility, privacy,
performance, and reliability rationale; automated and manual evidence remains
attributable and revision-aware. Reads derive missing ownership and judges,
contradictory expectations for one subject, untestable claims, absent or
expired evidence, and expired or soon-expiring exceptions rather than treating
a passing suite as complete quality. Plans grant no repository, review, merge,
release, deployment, environment, credential, or operational authority. The
repository web surface is `view=quality`.

Versioned assurance programs live beneath `$ASSURANCE_PROGRAM_ROOT` (default
`apps/api/data/assurance-programs`). Repository writers select exact versions
of regulatory, contractual, and organization requirements and publish immutable,
optimistically checked interpretations, applicability, scope, owners, review
periods, exceptions, evidence criteria, and control objectives. Controls map
claims to exact repositories, policies, data flows, infrastructure,
environments, releases, and operational procedures. Reads retain attributable
conflicting interpretations, missing owners, inherited or unmapped obligations,
unsupported claims, and expired or soon-expiring exceptions rather than
presenting a compliance badge. Programs grant no repository, policy, evidence,
review, security approval, release, deployment, credential, environment, or
operational authority. The repository web surface is `view=assurance`.

Continuous assurance evidence lives beneath `$ASSURANCE_EVIDENCE_ROOT` (default
`apps/api/data/assurance-evidence`). Named control owners define queries for
review, check, access, dependency, build, release, deployment, incident,
continuity, security, privacy, and governance records, binding schedules,
freshness, transformations, audiences, exact control versions, and assessment
periods. Immutable packages retain SHA-256 hashes, collector and source
attestations, revisions, coverage, freshness, gaps, and contradictions.
Credential- or personal-data-bearing records are rejected; embargoed,
inaccessible, and broader-audience records are omitted from least-privilege
projections without hiding their gaps. Evidence grants no repository,
source-system, credential, approval, release, deployment, environment, or
operational authority. The repository web surface remains `view=assurance`.

Versioned capability inventories live beneath `$CAPABILITY_INVENTORY_ROOT`
(default `apps/api/data/capability-inventories`). Repository writers bind a
named product capability and repository definition path to an exact source
revision, then map its interfaces, symbols, flags, packages, schemas,
configuration, documentation, journeys, and releases to their own exact
revisions and accountable owners. Declared repository, service, package,
application, extension, journey, and external consumers retain their permitted
audience, discovery method, status, owners, environments, and covered elements.
Usage observations and compatibility promises remain revision-exact and
attributable. Reads derive missing ownership or current evidence, unverified
consumer status, expired observations or promises, and unknown, inaccessible,
stale, or dynamically discovered use rather than treating it as absence.
Inventories grant no repository write, consumer access, telemetry, secret,
review, removal, release, deployment, environment, credential, or operational
authority. The repository web surface is `view=capabilities`.

Capability retirement plans live beneath `$CAPABILITY_RETIREMENT_ROOT` (default
`apps/api/data/capability-retirements`). Repository writers bind a proposed
removal to one exact capability-inventory version and retain supported
replacements, affected audiences and failure behavior, ordered compatibility
stages, deadlines, success and rollback criteria, communication policy,
commitments, exceptions, assumptions, and required owner acknowledgements.
Repository readers and read-only agents may add only attributable cited impact,
challenge, assumption, and alternative assessments. Only the named owner may
make an approval decision or an expiring bounded policy decision. Changed
inventory usage, incomplete inventory evidence, embargoed dependencies,
conflicting commitments, unresolved exceptions, rejected approvals, and
unresponsive owners remain explicit attributable blockers; a bounded decision
stays attached to the blocker instead of erasing it. Plans grant no repository
write, consumer access, approval impersonation, merge, release, deployment,
credential, environment, or operational authority. The repository web surface
is `view=capabilities`.

Retirement migration work remains beneath the exact plan. Provider writers may
define dependency-ordered human- or agent-owned tasks only for a named
repository and freeze the legacy and replacement contract revisions,
acceptance criteria, documentation changes, and rollout stage. Only a writer
of that task repository may report its sessions, workspaces, forks, pull
requests, revisions, blockers, review, or completion. Repository readers may
report a newly discovered consumer with cited evidence; it becomes an
attributable plan blocker until the inventory and migration scope are revised.
Linked work and progress never grant the retiring provider authority in the
consumer repository.

Migration proof candidates live beneath `$CAPABILITY_PROOF_ROOT` (default
`apps/api/data/capability-proofs`). Repository writers bind one retirement
stage to exact provider, consumer, schema, configuration, and release revisions
and a bounded synthetic environment. Every immutable candidate includes an
input-keyed matrix covering old-only, dual-support, replacement, rollback, and
declared journeys. Attempts retain sanitized artifact digests, duration, cost,
attribution, failures, targeted staleness, and superseded evidence. Usage
observations bind known consumers to exact consumer, configuration, and release
revisions and retain inaccessible, unmeasured, and residual use as removal
blockers. Only named owners acknowledge current proof; missing or failed checks,
unknown use, and requested changes prevent removal readiness. Proof records
grant no repository write, consumer access, release, deployment, credential,
environment, or operational authority. The repository web surface remains
`view=capabilities`.

Controlled capability removals live beneath `$CAPABILITY_REMOVAL_ROOT` (default
`apps/api/data/capability-removals`). A named retirement owner may start one
only from a current ready plan and removal-ready proof, binding the exact
candidate revision to ordered stages and an explicit reversible or irreversible
rollback boundary. Stages link ordinary merge-queue, release, schema-migration,
infrastructure-migration, documentation-publication, and protected-environment
evidence; append-only signals expose remaining use, health, controls, release,
environment, and next action. Failed delivery, regressions, excess use, changed
inputs, or newly discovered consumers pause progress. Owners alone advance,
pause, resume, or restore compatibility, and restoration is rejected after the
irreversible boundary. Completion requires evidence for obsolete code, flags,
data, credentials, telemetry, documentation, and policy exceptions plus exact
outcome measures and retained provenance. Removal records grant no Git, merge,
release, migration, publication, deployment, environment, credential, or
operational authority. The repository web surface remains `view=capabilities`.
`capability_retirement_workflow_test.go` is the black-box regression boundary
for the complete released-capability-to-verified-cleanup loop. It retains a
runtime-discovered independent consumer, corrected inventory and replacement
plan, missed acknowledgement, dependency-ordered human and agent migration
work, failed and corrected coexistence evidence, stale usage, reversible
post-disable restoration, ordinary delivery links, and exact verified cleanup
without granting cross-repository or operational authority.

Reusable test scenarios live beneath `$TEST_SCENARIO_ROOT` (default
`apps/api/data/test-scenarios`). Repository writers publish immutable,
optimistically concurrency-checked versions from ordinary branches, workspaces,
or pull requests. Each version binds its repository definition path and source
revision to accessible exact-revision issue, reproduction, design, API-contract,
documentation, or journey rationale; typed parameters; readable preconditions,
actions, assertions, fixtures; and isolated environment requirements. Generated
cases retain their generator, assumptions, and provenance, while scoped-agent
contributions declare their changed paths and allowlist. Reusable fixtures are
rejected when they contain secrets or production personal data, depend on
inaccessible evidence, or use non-synthetic source material without explicit
transformations. Scenario records grant no Git, workspace, pull-request, secret,
production-data, environment, credential, review, merge, or execution authority.
The repository web surface remains `view=quality`.

Revision-exact exploratory test sessions live beneath
`$EXPLORATORY_SESSION_ROOT` (default `apps/api/data/exploratory-sessions`).
Repository writers open a bounded session from a pull-request preview, release
candidate, issue, or quality plan and bind its exact candidate revision,
isolated environment, permitted route and command prefixes, expiring access,
privacy-classified synthetic or explicitly transformed test data, and time,
cost, and agent-action ceilings. Risk-based charters name behaviors, routes,
techniques, and one human or approved-agent owner. Append-only shared timeline
events retain exercised routes, inputs, observations, commands, coverage,
uncertainty, sanitized content-addressed screenshots and traces, actor, cost,
and candidate revision. Participants may guide or pause work and classify,
reproduce, resolve, or discard evidence-linked findings; only the session lead
may change session state or candidate bindings. A changed candidate marks only
events whose route or behavior intersects its declared affected scope stale and
propagates staleness to dependent findings. Sessions grant no Git, preview,
secret, production-data, review, merge, release, deployment, environment,
credential, or operational authority. The repository web surface remains
`view=quality`.

Confirmed, current exploratory findings extend beneath their session into a
governed delivery link. An authorized collaborator creates a repository issue
and assigned ordinary human- or agent-owned proposal task while freezing the
exact candidate, an explicit permitted subset of timeline events, a minimized
reproduction, and immutable acceptance criteria. Delivery requires one pull
request to retain distinct failure evidence against that exact base and passing
evidence at its repaired revision, attributable review, an exact quality-plan
version, and a versioned reusable regression scenario. Flaky, duplicate,
environment-specific, and non-reproducible findings instead require an
attributable rationale and their applicable duplicate, environment, or
follow-up disposition. Neither path silently excludes evidence or grants Git,
agent, review, merge, environment, credential, or execution authority.
`exploratory_finding_repair_workflow_test.go` is the black-box boundary from a
confirmed finding through assigned repair and durable regression linkage.

Revision-exact quality delivery gates live beneath `$QUALITY_GATE_ROOT`
(default `apps/api/data/quality-gates`). Repository writers publish immutable,
optimistically checked policies binding an exact quality-plan version to
required scenario, exploratory, and test evidence selected by branch, path,
journey, risk class, locale, platform, and release. Pull-request, merge-queue,
and release candidates freeze one policy version and exact revision into a
matrix retaining attempts, failures, flakes, quarantines, gaps, and current
owner acknowledgements. Every attempt declares its behavior, scenario version,
environment, dimensions, code inputs, dependency revisions, evidence, and
actor. Candidate changes stale only intersecting code- or dependency-bound
attempts; acknowledgements and overrides never cross revisions. Overrides are
scoped to named requirements, expiring, attributable, reasoned, and require
linked follow-up work. Post-release sampled signals either verify a requirement
or reopen its quality risk with evidence and follow-up work. These records
expose confidence but grant no Git, tester, agent, merge-queue, release,
environment, credential, deployment, or operational authority. The repository
web surface remains `view=quality`.
`collaborative_test_engineering_workflow_test.go` is the black-box regression
boundary for the complete expectation-to-sustained-quality loop. It retains
product and design intent, reviewed cross-platform scenarios, unsafe-fixture
rejection, bounded human-agent preview exploration, targeted stale evidence, a
minimized finding and agent-authored repair, failed-first and durable regression
proof, missing and flaky platform containment, rejected risk bypass, exact pull,
merge-queue, and release matrices, and attributable post-release verification.

Restricted repository-history remediations live beneath
`$HISTORY_REMEDIATION_ROOT` (default `apps/api/data/history-remediations`). A
repository owner opens one from an exact security finding, privacy incident,
support case, or selected object before further inspection, freezing a
payload-free description and reason, disclosure audience, response owners,
participants, exact object IDs and digests, and affected repositories, refs,
releases, packages, artifacts, and environments. Discovery evidence retains
only exact references, revisions, digests, audience-safe summaries, access
status, and attribution. False matches, inaccessible or expired evidence,
legal holds, conflicting retention or continuity commitments, and pending or
denied named approvals remain derived blockers. Only the creator, response
owners, named participants, and approval owners can discover the record through
`/repositories/{repository}/history-remediations`; the repository web surface
is `view=security`. These starting-point records grant no object inspection,
Git rewrite, repository, release, package, artifact, environment, disclosure,
credential, or operational authority.

Each restricted remediation also retains an append-only reachability map across
branches, tags, pull requests, forks, federated contributions, workspaces,
checkpoints, caches, packages, release artifacts, documentation, deployments,
backups, and active clones. Named participants, including read-only agents, may
append only cited payload-free findings that bind exact object IDs and derived
credential or data references. Reads derive confirmed, suspected, unreachable,
independently controlled, and unverifiable counts and preserve uncertainty;
new findings update the summary without granting inspection or control of a
copy.

Authorized response owners may append immutable payload-free rewrite rules and
assemble private candidates against exact selected refs beneath the same
remediation. Candidates retain replacement digests, unaffected-content proof,
authorship and signature outcomes, a restricted old-to-new commit map, changed
objects, storage effects, broken links, rollback limits, collaborator actions,
and independently controlled or otherwise unrewritable resources without
publishing refs. Append-only bounded rehearsals cover repository integrity,
build, checks, release, dependencies, and representative clone and fetch
behavior in an isolated environment; missing, failed, blocked, or over-budget
domains remain explicit blockers. These planning records grant no Git rewrite,
ref update, mapping disclosure, release, collaborator, credential, environment,
or operational authority.

After publication, append-only containment rounds freeze a completion policy
and continuously recheck repository reachability, ordinary object access, fork
and federation acknowledgements, package and artifact replacement, credential
rotation, deployments, caches, and protected recovery copies. Each check keeps
its exact reference, revision, digest, owner, validity, and payload-free result;
missing, failed, expired, unreachable, independently retained, legally held,
reintroduced, or excepted copies remain attributable blockers rather than an
erasure claim. Response owners may migrate or close superseded pull requests
and workspaces while retaining discussion and attribution against an exact
replacement revision. Push, automation, release, and contribution pauses may
be resumed independently only from named current passing checks in an exact
round, so evidence for one flow never reopens every collaboration surface.
These recovery records grant no copy, repository, fork, federation, package,
artifact, credential, deployment, cache, backup, pull-request, workspace,
release, or operational authority.

After every current required approval and a passing latest rehearsal, an
authorized response maintainer may atomically publish one attested candidate's
exact replacement refs. Publication quarantines named changed objects from
ordinary reachability, retains credential revocation or rotation receipts, and
pauses affected pushes, queues, sessions, workflows, and releases with explicit
recovery guidance. Each local branch, fork, federated copy, open pull request,
or integration receives an audience-bounded full, redacted, or unavailable
mapping and owner-specific instructions. Independently controlled targets must
acknowledge or perform their own rewrite; coordination never grants authority
over them. Active push containment rejects stock Git receive requests with
actionable migration guidance so stale automation cannot silently restore the
removed lineage. `history_remediation_workflow_test.go` is the black-box
boundary for the complete exposure-to-contained-history loop. It retains
payload-free agent analysis, a corrected false match, a signed-commit outcome,
failed rehearsal, atomic partial-ref rejection, credential rotation,
independently rewritten fork, unavailable federated peer, migrated pull
discussion, protected backup, reintroduced object, stale-clone push rejection,
and current evidence-gated restoration of ordinary contribution without
claiming control over every copy.

Reviewable repository restructuring plans live beneath
`$REPOSITORY_RESTRUCTURING_ROOT` (default
`apps/api/data/repository-restructuring`). Repository writers open a plan from
selected exact-revision source repositories before identities change and define
destination names, owners, visibility, default branches, retained identities,
content paths, history treatment, deadlines, success criteria, and rollback
limits. The immutable inventory covers refs, pull requests, issues, tasks,
releases, packages, documentation, policies, workspaces, automation, consumers,
and federated relationships with exact references, revisions, owners, access,
disposition, and destination mappings. Inaccessible, ambiguous, shared, and
unresolved resources remain derived blockers. Repository readers, including
read-only agents, may append only exact-revision cited impact findings; plans
and findings grant no source or destination repository, Git, identity, owner,
visibility, migration, or operational authority. The public API is
`/repositories/{repository}/restructuring-plans` and the repository web surface
is `view=restructuring`.

Repository writers may assemble immutable candidate repository records beneath
an exact restructuring plan without creating destinations or publishing refs.
Each candidate freezes selected mappings, destination object-graph digests,
default refs and commits, object counts and sizes, selected history, authorship,
signature and tag outcomes, license and provenance evidence, cross-repository
links, assembly cost, gaps, and required decisions. Append-only bounded
rehearsals exercise stock Git clone, fetch, and push plus builds, checks, package
and API resolution, documentation, workspaces, and representative consumer
journeys. Missing domains, failures, blocked work, budget overruns, duplicated
history, broken ancestry, path collisions, missing objects, changed signatures,
policy gaps, and resources that cannot move remain attributable blockers.
Candidates and rehearsals grant no repository, Git, package, API, workspace,
redirect, credential, or operational authority.

Open work remains beneath the exact restructuring plan as revisioned mappings
for branches, pull requests, issues, proposals, tasks, decisions, checks,
sessions, workspaces, and queues. Each mapping freezes source revision,
authorship, discussion, reviews, dependencies, acceptance criteria, audience,
and one or more dependency-connected destination contributions. Independently
owned work requires its declared owners' current approval; each destination
owner records whether its contribution continued, blocked, or archived after
cutover. Optimistic revisions reject conflicting actions, while changed source
revisions, embargoed context, removed access, rejected mappings, and work that
cannot migrate remain explicit blockers. These records preserve continuity but
grant the plan owner no authority over source work or destination contributions.

Downstream migration plans beneath an immutable restructuring candidate expose
every affected clone, fork, package, API, dependency, extension, workflow,
documentation link, deployment, and federated follower with its independent
owners, audience, old and authoritative locations, machine-readable mappings,
safe synchronization steps, compatibility deadline, current state, and next
action. Public Git moves require a signed redirect or explicit replacement
remote. Target owners append ordinary pull-request, release, revision, and
evidence outcomes; restructuring ownership cannot report adoption for them.
Redirect loops, namespace collisions, expired credential references,
unavailable peers, rejection, blockage, and unmigrated targets remain explicit
in `view=restructuring`. These records grant no Git, consumer, package, API,
extension, workflow, deployment, federation, credential, merge, release, or
operational authority.

Owner-controlled repository authority cutovers remain beneath the exact plan
and bind one immutable candidate, its current passing rehearsal, downstream
migration plan, and every source revision. Required source and destination
owners approve optimistically before dependency-ordered stages may pause the
declared write boundary, activate destinations, transfer scoped ownership and
policies, atomically publish grouped refs and redirects, verify the topology,
and retire sources as read-only, archived, or removed. Append-only controls,
stage receipts, source and destination health, dependency adoption, Git
traffic, collaboration migration, and completion signals expose live control
and rollback options. Current passing build, release, permission, link,
supported-consumer, and ordinary-contribution evidence is required before
source retirement; late writes, residual use, unmigrated work, failed stages,
or stale prerequisites pause or block cleanup. Cutover coordination grants no
repository, Git, owner, policy, credential, merge, release, deployment,
environment, or operational authority.

`repository_restructuring_workflow_test.go` is the black-box boundary for the
complete monorepo-component-extraction-to-continuing-collaboration loop. It
connects stock Git history, candidate and build evidence, a divided open pull
request, independently owned human and agent consumers, package, documentation,
workflow, release, and federated entry points, staged cutover, an ordinary
change on the new topology, and evidence-gated compatibility retirement. The
retained trail includes corrected path collision and signature evidence,
unmappable review, unavailable federation, failed package release, permission
mismatch with rollback, and a concurrent push without transferring any source,
destination, consumer, package, federation, or operational authority.

Collaborative change stacks live beneath `$STACKED_CHANGE_ROOT` (default
`apps/api/data/stacked-changes`). Repository writers define one shared outcome,
target branch and exact target revision plus an ordered set of existing or new
branches and optional pull requests. Every member freezes its exact revision,
declared parent, authors, and acceptance criteria. Reads derive the individual
scope against that parent and cumulative scope against the target, commit count,
changed paths, base relationships, dependencies, effective read/publish/branch
permissions, and the first reviewable layers. Missing commits or dependencies,
cycles, unrelated histories, duplicate revisions or resulting trees, moved or
inaccessible existing branches remain explicit blockers and no branch is
rewritten. A writer may publish only a blocker-free member's exact frozen
revision for review; publication retains attribution and grants no Git,
pull-request, review, branch-update, merge, credential, or operational
authority. The public API is `/repositories/{repository}/change-stacks` and the
repository web surface is `view=stacks`.

Stack member reads carry complete commit and file-patch scope against both the
declared parent and target, plus exact transitive upstream revisions. Linked
pull requests expose that context at their `/stack-context` resource.
Discussion, review decision, owner acknowledgement, check, preview, and agent
finding references may be bound to a layer or cumulative candidate at one exact
member revision. Reads derive whether the layer is reviewable now, provisional,
or unpublished and enumerate downstream evidence IDs that a change to each
upstream member would stale. Bindings cite existing records without replacing
their native authorship, access, review, approval, check, preview, agent, merge,
or other authority controls.

Change-stack revisions are immutable propagation previews over one exact current
version. Writers may propose a complete reordered member set after creating the
replacement commits with stock Git; reads retain old-to-new commit and base
mappings, authorship continuity, branch ref expectations, review invalidations,
check reruns, and conflict or ownership blockers before apply. Only declared
branch owners may apply, and one optimistic Git ref transaction publishes every
authorized local branch or none. Concurrent pushes, shared branches, revoked
access, failed or unrelated rewrites, and members owned by another repository
remain actionable retained states. Applied revisions preserve prior stack,
publication, evidence, and attempt lineage and grant no Git, repository,
pull-request, review, check, force-push, merge, credential, or operational
authority.

Stack members may also retain branch-owner-created human or approved-agent
assignments. An agent assignment names its existing approval and may authorize
only the member's declared source branch. Assigned participants can open a
session, shared workspace, or conflict-resolution workspace preloaded with the
shared outcome, exact parent and member revisions, acceptance criteria,
permitted evidence, and transitive upstream assumptions. Workspaces grant no
authority of their own. Checkpoints, questions, handoffs, and proposed restacks
form an append-only stack timeline; reads mark entries when an upstream
assumption changes. Repository, participant-only, and embargoed audiences
preserve ordinary disclosure, while cross-repository members and branch updates
continue to require their native owner and repository permissions.

Ready change-stack prefixes may be assembled into immutable landing candidates
against the exact live target. Each candidate freezes its base, source, tree,
resulting revision, generation, and member position; required checks,
reproductions, contracts, previews, policy decisions, and approvals record the
exact three revisions they cover. Maintainers may publish the first current
passing member in order or, where declared policy permits, compare-and-swap the
whole passing stack atomically. Target movement, conflicts, failed evidence,
withdrawn approval, or a changed stack pauses only the unsafe suffix. Rebuilds
retain superseded candidates and evidence while creating a new generation from
the current target, and already merged members remain durable. Landing records
coordinate native branch authority but grant no Git, review, check, approval,
merge, repository, credential, deployment, or operational authority.

`stacked_change_workflow_test.go` is the black-box boundary for the complete
large-change-to-integrated-stack loop. It connects stock Git branches and a
semantic conflict resolution to focused and cumulative review, developer and
approved-agent work, revision propagation, exact candidate evidence, and final
ordered history. The retained run includes revoked agent access, stale review,
a reordered member, an atomically contained concurrent push, a failed middle
check, partial integration, target movement, suffix-only rebuild, and recovery
without granting branch, review, agent, check, merge, or repository authority.

`software_provenance_workflow_test.go` is the black-box boundary for the
complete contribution-to-verifiable-distribution loop. It connects stock Git
human and agent authorship, local and federated provenance, generated code,
transitive and private packages, governed findings and replacement work,
current release gates, an Ed25519 provenance bundle, the public package
registry, and a standard npm consumer. The retained trail includes a corrected
false match, disputed authorship, a rejected exception, generator drift,
restricted dependency projection, revoked upstream attestation, and a
post-release origin discovery without rewriting the signed release claim or
granting repository, evidence, package, release, or distribution authority.
