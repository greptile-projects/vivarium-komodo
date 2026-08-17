# Docs

Notes on how this project fits together. Mostly empty for now — things get
written down here as they're decided, not before.

## API contracts

Repository API producers publish immutable service interface versions at
`/repositories/{repository}/api-contracts`. A version freezes its reviewed
source revision and definition path alongside operations, schemas, errors,
authentication modes, environments, limits, owners, stability, support and
compatibility promises, and links to releases, documentation, and data-use
commitments. Readers can inspect retained history or compare two version
numbers at `/{contract}/compare?from=1&to=2`. The API and `view=apis` workspace
keep invalid definitions, unreleased implementations, stale documentation,
availability limitations, missing provenance, and producer-declared gaps
visible; publication does not issue credentials or operational authority.

## Evaluated agent onboarding

Repository owners turn accepted, uncontaminated agent trials into explicit
participation through `/repositories/{repository}/agent-evaluations/onboardings`;
organization owners use `/organizations/{organization}/agent-onboardings`.
Each immutable version names its exact evidence and agent profile, roles,
resources, actions, data boundaries, budget, schedule, approvers, exceptions,
operator agreement requirement, and human sponsor boundary. Reads include an
authority preview with activation blockers and exclusions. Activation issues a
distinct project-agent subject, not an operator account or financial/governance
role. Version upgrades invalidate old approvals and agreements; denials,
expiry, activation, and revocation remain attributable and visible.
The operator-agreement endpoint is the one onboarding mutation authenticated
as the exact public profile operator rather than as the project owner. Owners
still exclusively define, approve, activate, narrow, and revoke authority, but
cannot manufacture third-party consent on the operator's behalf.

### Current agent trust and replacement

Each onboarding detail now projects a repository-owned `trust` record. Owners
set its periodic suite, failure containment, verification-rate, and average-cost
policy at `/trust-policy`; append privacy-safe delivered observations at
`/outcomes`; and bind exact completed trials through `/reevaluations`. Outcome
records accept only a bounded summary, typed work/evidence identifiers, cost,
responsiveness, occurrence time, and attribution. They deliberately do not
accept task content, prompts, terminal logs, artifacts, credentials, or private
evidence payloads.

Security/policy violations, deteriorating verification, anomalous cost, overdue
evaluation, and reevaluation failure appear as retained notices with explicit
next actions. A failed required evaluation can suspend effective authority.
Owners use optimistic `/authority-controls` to narrow the original resource and
action sets, suspend/resume them, or revoke the installation promptly without
deleting legitimate outcomes or commits. `/handoffs` transfers one named active
work item to another independently activated onboarding identity with completed
evidence references, remaining work, verification criteria, risks, and explicit
recipient acceptance.

The activated consent remains pinned to its exact profile version. The
`/profile-comparison` projection classifies operator, model, data-use/execution,
capability, and price/resource changes as material. Such a newer profile is not
silently adopted: maintainers must use the existing immutable onboarding-version
flow with fresh evidence, approvals, operator agreement, and activation. The
Agents web surface shows effective authority, notices, attributable outcomes,
reevaluations, and handoffs alongside the trials that support current trust.

`agent_collaboration_workflow_test.go` composes these contracts with public
profile publication and discovery, stock Git, branch-scoped change sessions,
and ordinary pull-request review and merge. Its retained path includes a hidden
canary failure, a prohibited action, a cost overrun, a material profile upgrade,
operator outage and failed-reevaluation suspension, and a verified handoff to
an independently evaluated and activated replacement. The merged commit stays
attributed to both the authorizing human and consented agent identity without
giving either agent review or merge authority.

## Continuity commitments

Authorized repository collaborators define immutable recovery-objective
versions through `/repositories/{repository}/recovery-objectives`. Each
commitment names the repository, package, artifact, configuration,
collaboration-record, and deployed-service state behind a user-facing
capability, together with owners, dependencies, acceptable loss, restoration
time, retention, jurisdictions, validation criteria, feasibility, exclusions,
and time-bounded exceptions. Links retain the relevant service objectives,
environments, incidents, privacy rules, and governance decisions.

Repository readers inspect the complete attributable history in the
`view=continuity` workspace. Derived blockers keep missing ownership,
impossible or unverified targets, unprotected dependencies, and exception
expiry visible. The contract does not create backups, credentials, environment
access, or restoration authority. Durable state lives beneath
`$RECOVERY_OBJECTIVE_ROOT`; later protection and rehearsal work should extend
this versioned contract rather than infer recovery promises from job status.

Exact objective versions become executable protection contracts through
`/repositories/{repository}/protection-plans`. Immutable plan versions bind
named resources to repository or environment scope, encrypted snapshot or
replica mode, access classes, authorized destination metadata, schedule,
maximum age, retention, checksums, validation criteria, and cost limits.
Writers append provider-idempotent capture manifests; manifests retain source
versions and provenance, dependency versions, bounded object and byte counts,
checksums, validation digests, cost, and actor attribution, but never accept or
return protected content, encryption material, or credentials.

The continuity workspace exposes coverage, freshness, validation, cost, and
failure metadata. Recoverability is server-derived only when the exact current
plan has every required committed resource and the manifest, checksum,
decryption, key availability, destination authorization, validation evidence,
freshness, and cost checks all pass. Deleted or uncommitted source state,
partial captures, corruption, key loss, stale evidence, unauthorized storage,
and provider-reported failure therefore remain visibly non-recoverable even if
the capture job itself completed. Durable state lives beneath
`$PROTECTION_PLAN_ROOT`; the contract does not grant routine access to keys,
contents, environments, or restoration controls.

Repository writers rehearse those inputs through
`/repositories/{repository}/recovery-exercises`. A launch freezes a named
failure scenario, an exact recoverable capture and resource/dependency versions,
an isolated non-authoritative environment, duration and cost bounds,
dependency-ordered restore commands, and repository-defined integrity and
user-journey checks. The service rejects exercises that claim production
secrets are available or authoritative state is writable.

The result records attributable start/finish timing, the exact frozen command
for every step and check, explicitly redacted log excerpts and content digests,
artifact metadata, manual steps, gaps, achieved recovery-objective resources,
and cost. It never accepts snapshot contents, credentials, or unredacted logs.
The Continuity workspace retains passing and failed history while deriving an
exercise as non-current whenever its protection-plan version changes, its exact
capture stops being recoverable, or the latest captured dependency versions no
longer match. Durable state lives beneath `$RECOVERY_EXERCISE_ROOT`; exercises
record evidence and do not provision environments or grant restoration or
operational authority.

Completed exercises feed accountable diagnosis through
`/repositories/{repository}/recovery-investigations`. Collaborators and
authenticated read-only agents correlate the frozen exercise with bounded
code, dependency, release, configuration, ownership, and protection-plan
references. Every observation, hypothesis, challenge, and conclusion cites the
permitted evidence it uses and retains attribution, uncertainty, and a
supported, disputed, or inconclusive verdict. Participant-audience evidence
keeps the whole investigation participant-only; a changed or non-current
exercise makes its diagnosis explicitly non-current. State lives beneath
`$RECOVERY_INVESTIGATION_ROOT`, and no evidence record contains protected
payloads or confers access to production state.

Repository writers convert a current supported conclusion through
`/repositories/{repository}/recovery-improvements`. The conversion creates an
ordinary proposal and dependency-ordered human or agent tasks at an exact base
revision; tasks may cite an already-governed session or workspace, but do not
create credentials or start either resource. The repair record then retains
attributable links to sessions, workspaces, pull requests, checks, integration,
releases, policy changes, and approvals. These links are evidence only, so the
linked resources keep their ordinary ownership, review, check, merge, release,
and approval rules. A repair stays blocked until a distinct current exercise
passes against a newer version of the same protection plan. Failed re-exercise
evidence remains attached as `verification_failed` rather than hiding the
original gap. Durable repair state lives beneath `$RECOVERY_IMPROVEMENT_ROOT`.

## Live recovery response

When normal operation is unsafe, repository writers activate a shared control
record through `/repositories/{repository}/recovery-responses`. Activation must
cite an incident or confirmed loss event and freezes the exact current
protection-plan version, verified recovery point, production environment,
estimated loss, one coordination workspace, required approvers, communication
channels, rollback choices, and dependency-ordered steps. This is a control and
evidence plane: it does not open protected payloads, issue keys or credentials,
or acquire environment authority.

Named approvers retain their rationale before execution can begin. Every step
names one human or agent executor, exact command, expected result, dependencies,
and whether it is destructive; agents cannot act outside that delegation, and
destructive cutover requires a separate attributable decision. Optimistic
concurrency keeps approvals, decisions, progress, communications, and
validation coherent. Conflicting writes, an unavailable key, stale replica,
partial restore, failed step, or failed validation becomes an explicit pause
blocker. Each execution attempt remains append-only; an attributable resume
returns failed steps to pending for a bounded retry while preserving the failed
evidence. Completion alone enters validation; only passing evidence marks the
response restored and safe for return. The `view=continuity` surface shows the
active control, selected state and loss, progress, blockers, messages,
validation, and rollback options. Durable state lives beneath
`$RECOVERY_RESPONSE_ROOT`.

`recovery_workflow_test.go` exercises the complete public-API and stock-Git
path from continuity terms and corrupted capture containment through a failed
regional rehearsal, agent diagnosis, ordinary reviewed repair, failed and
passing re-verification, and a validated simulated destructive response.

## Shared service objectives

Authorized repository collaborators define reliability contracts through
`/repositories/{repository}/service-objectives`. Immutable versions describe
repository, release, and environment scope; user-visible journeys; available
or missing signals and supported calculations; rolling or calendar measurement
windows; explicit targets and error budgets; dependencies; escalating response
thresholds and owners; and time-bounded exceptions. Links retain the relevant
product, performance, accessibility, privacy, and release commitments rather
than treating reliability as a separate operations-only judgment.

Repository readers inspect current terms and complete attributable history in
the `view=reliability` workspace. Derived blockers keep missing signals and
owners, unsupported calculations, conflicting overlapping targets, unresolved
commitments, and expiring or expired exceptions visible. Defining an objective
does not grant telemetry access, deployment control, repository authority, or
credentials. Durable state lives beneath `$SERVICE_OBJECTIVE_ROOT`.

Each objective also owns repository-defined signal mappings at
`/{objective}/signal-mappings` and append-only observations at
`/{objective}/attainment`. A mapping version freezes the exact objective
version, indicator, measurement window, instrumentation revision, permitted
sanitized fields, and sources across metrics, logs, traces, health checks,
support reports, deployments, releases, commits, pull requests, packages, and
dependent services. An observation must cite that exact mapping version and
retains its window, attainment, error-budget consumption, uncertainty,
comparability, audience, and revision-exact evidence references. Unsanitized or
restricted-user evidence is rejected. Anonymous readers of public repositories
see only public observations; authenticated repository readers may also inspect
repository-audience evidence. Historical observations are never recomputed when
terms or instrumentation change, and derived gaps call out missing mappings,
missing attainment, changed instrumentation, and incomparable windows.

### Reliability delivery policy

Repository owners bind an exact service-objective version to delivery through
`/repositories/{repository}/reliability-delivery-policies`. A policy selects
branches, services, environments, journeys, and risk classes, names required
objective owners, and maps exhausted or threshold-crossing error budgets,
regressions, missing evidence, and dependency failure to `block`, `slow`,
`pause`, or `rollback` responses. Policies are immutable and live beneath
`$RELIABILITY_POLICY_ROOT`.

Maintainers append revision-exact predicted or observed `/impacts` for a pull
request, integration queue candidate, release, or staged deployment. The
`/assessment` projection returns impact history, owner acknowledgements, active
exceptions, requirements, and available next actions for enforcement by the
existing delivery controller. These records never merge a change or operate an
environment, and agent evidence cannot silently become either authority.

### Reliability investigations

Authenticated repository readers open shared diagnosis at
`/repositories/{repository}/reliability-investigations`. Each investigation
binds an exact service-objective version, affected journeys, repository
revision, and an objective, pull-request, deployment, or budget-consumption
trigger. Creation requires a baseline and affected sample; operational and code
evidence retains its resource, revision, audience, window, summary, and
uncertainty. Participant evidence keeps the investigation participant-only.

Readers, including approved read-only agents, append cited observations,
comparisons, hypotheses, challenges, conclusions, and responses and request
bounded input from service or dependency owners. Conclusions distinguish
supported, disputed, and inconclusive judgments. Objective owners participate;
only repository writers invite others or link a cited conclusion to an issue,
incident, decision, or planned improvement. Changed objective terms make cited
reasoning stale without rewriting it. Uncertainty, dissent, inconclusive
signals, and unanswered dependency input remain explicit blockers. Records
grant no telemetry, alert, rollback, Git, deployment, or operational authority
and live beneath `$RELIABILITY_INVESTIGATION_ROOT`.

### Accountable reliability improvements

Repository writers use `/repositories/{repository}/reliability-improvements`
to convert either a current supported investigation conclusion or a depleted
error-budget event into an ordinary proposal and ordered human- or agent-owned
tasks. The improvement freezes its exact service-objective version, affected
revisions and journeys, baseline signal window, dependency context, evidence,
acceptance criteria, assignees, and task dependencies. This preloads the reason
for the work while the proposal and task APIs retain their normal authorship,
access, session, pull-request, review, and check boundaries.

Writers append exact `/delivery-links` for pull requests, checks, releases,
deployments, and decisions. A `/rollouts` observation binds a release,
deployment, revision, environment, stage, and current measurement to the
recorded baseline. Failed measures keep the budget depleted and derive a
containment, rollback, or decision-revisit requirement; later passing evidence
may restore the derived budget state without deleting the failed attempt or
original impact. The durable store is `$RELIABILITY_IMPROVEMENT_ROOT`.
Nothing in the record grants Git, review, merge, release, deployment, rollback,
telemetry, credential, or operational authority.

Repository readers can discover the complete retained repair collection at
`GET /repositories/{repository}/reliability-improvements`. The Reliability web
workspace combines that projection with current objectives, delivery policies,
investigations, affected-owner requests, delivery links, failed rollout stages,
and later recovery so a restored budget does not hide the original user impact
or correction trail.

`reliability_workflow_test.go` proves the complete released-journey-to-sustained-
reliability loop through public HTTP and stock Git. The regression includes
noisy burn evidence, missing dependency input, a rejected exception, policy
containment, agent diagnosis and bounded task authority, retained compute cost,
a failed first repair, ordinary checks and owner review, release, staged failure,
and verified recovery with the earlier impact preserved.

## Product feedback

## Accessibility barrier reproduction

Authenticated repository readers submit lived barriers through `POST
/repositories/{repository}/accessibility-barriers`. A report targets an
existing release, documentation journey, preview, or repository page and the
server captures or verifies its exact revision. It retains access needs and
expected interaction rather than medical context, plus ordered steps, declared
browser/device/assistive-technology settings, and only evidence explicitly
marked redacted. Screenshots, recordings, accessibility trees, speech output,
and input traces each carry audience or maintainer-only visibility.

All reads are projected for the viewer: restricted reporter identity,
sensitive device details, and evidence content are omitted without erasing the
report. Repository writers add immutable `/attempts` only from an existing
bounded workspace or preview whose server-held revision exactly matches the
attempt. Reproducible, intermittent, environment-specific, and unconfirmed
results remain distinct and attributable. The `view=accessibility` workspace
supports both safe reporting and reproduction; neither action grants access to
the cited runtime or any repository or delivery authority.

## Accountable accessibility assessment

Repository owners can turn the evidence into an exact-candidate delivery gate
through `/repositories/{repository}/accessibility-delivery-policies`. A policy
binds a commitment version to branches and optional paths, journeys, or risk
classes, then names required checks, scenario evaluation dimensions, and
reviewer or participant roles. Pull readiness reports individual
`accessibility_*` blockers and merge enforces the same current-revision result.
Release tooling obtains the same projection from
`/repositories/{repository}/releases/accessibility-readiness` for an exact
pull-request candidate before publication.

Invited preview users confirm or reject a scenario through
`/pull-requests/{pull}/accessibility-acknowledgements`; the invitation role must
match and the decision is revision-bound. Rejection remains immutable. An owner
may override it only with retained rationale and linked issue, proposal, or task
follow-up. This does not generalize one participant's experience, rewrite
dissent, or grant delivery authority. Policy state lives beneath
`$ACCESSIBILITY_POLICY_ROOT`.

An open pull request freezes declared journeys and their source paths at its
exact source revision through `POST
/repositories/{repository}/pull-requests/{pull}/accessibility-assessments`.
Each scenario names affected audiences and the required semantics, keyboard,
focus, contrast, motion, caption, and whole-journey evaluation dimensions. The
server captures source blob identities rather than trusting caller-supplied
digests.

Schema-version `1` `.komodo/accessibility-checks.json` definitions execute as
ordinary isolated pull-request checks. A definition declares `scenario_ids`,
`evaluations`, `inputs`, `affected_audiences`, and optional
`requires_human_evaluation` beside its bounded command and artifacts. Successful
evidence may be reused at a later revision only when all declared input blobs
are unchanged. Attaching its run under an assessment explains exactly what the
machine proved and preserves the explicit human-only remainder.

Repository readers, accessibility specialists, and credentialed read-only
agents add `/findings` from a permitted preview or an existing barrier report
with a revision-exact reproduction. Findings retain result, severity, affected
audiences, source locations, uncertainty, attribution, and whether a person
must evaluate the behavior. Repository writers append immutable `confirmed`,
`duplicate`, or `false_positive` decisions with rationale. A changed pull
revision compares current source blobs with each finding and check input: only
affected evidence is stale, its judgment remains visible, and unrelated
coverage remains current. Derived gaps enumerate every declared scenario and
evaluation dimension without current evidence. Repository-wide reads are at
`/repositories/{repository}/accessibility-assessments`; the same projection is
shown in the pull Accessibility section and `view=accessibility` workspace.

`accessibility_workflow_test.go` composes these contracts into the durable
released-barrier-to-sustained-access boundary. It proves a consent-projected
report and clean bounded reproduction, specialist and agent collaboration,
false-positive correction, an unavailable restricted attachment, stale
reporter acceptance, exact-candidate automated and assistive-technology
evidence, ordinary review/merge/release provenance, an expiring exception, and
release-linked scenario coverage. The reporter's preview confirmation remains
their bounded observation; it neither generalizes their access needs nor grants
repository authority.

An assigned contributor opens the ordinary repair pull request with both
`proposal_id` and `task_id`. The API accepts that task link only when the task
belongs to the proposal and its current assignee is the pull-request author,
preserving the repair's reasoning and authorship chain without granting access.

## Governed project funds

Repository writers create resource contracts through `POST
/repositories/{repository}/funds`. A fund declares its stewards, accepted
funding sources, currency or credit unit, per-allocation, per-recipient, and
total limits, approval threshold and approvers, eligible recipient classes,
refund policy, and either public or repository-reader ledger visibility. The
same contract is inspectable in the repository `view=funds` workspace before
collaborators promise paid work.

Repository readers commit resources through a provider source and stable
transfer reference. Duplicate source/reference pairs conflict rather than
double-crediting the ledger. Pending, failed, and revoked transfers contribute
no available value; a partial transfer contributes only its settled portion,
while refunds and disputes remain separate explicit balances. Named stewards
may reconcile a non-terminal transfer with optimistic concurrency. All balances
are derived from retained transfers, and the fund exposes an empty operational
authority boundary: money, stewardship, approval eligibility, or recipient
eligibility grants no code, repository, credential, allocation, review, merge,
release, or deployment authority. Durable state is beneath
`$PROJECT_FUND_ROOT`.

### Measurable funded outcomes

Named fund stewards use `POST /repositories/{repository}/funded-outcomes` to
connect one governed fund to an issue, roadmap outcome, proposal, stewardship
opportunity, incident follow-up, or security repair. The immutable initial
version spells out scope, acceptance criteria, evidence requirements, budget,
deadline, eligible contributor classes, allocation method, cancellation terms,
dependencies, risks, conflicts, overlap keys, embargo state, and optional
milestones whose budgets exactly cover the outcome. Eligibility must be within
the fund's published recipient classes.

Authenticated repository readers can pledge to `outcome` or
`milestone:{id}`. They may later withdraw only their own pledge with an
optimistic-concurrency version and reason. Stewards replan changed terms the
same way, preserving every prior version, author, reason, and a specific scope
change or backing-withdrawal event. Reads derive underfunding, overfunding,
milestone overfunding, aggregate settled-fund insufficiency, overlapping award
keys, declared conflicts, and embargo boundaries. These records expose whether
the work and its required proof are worth pursuing; they reserve no resources,
select no recipient, issue no credential, and grant no repository or
operational authority.

### Accountable delivery selection

Eligible repository participants submit comparable offers at
`/repositories/{repository}/funded-outcomes/{outcome}/delivery-proposals`.
Each offer identifies a human, team, or approved agent operator and retains its
approach, costed milestones, dependencies, availability, requested access, and
attributed prior work. The named recipient must optimistically accept before
selection, and participants can append attributable conflict disclosures.

Named fund approvers record version-checked decisions through `/approve`; the
fund's minimum-approval rule must be met before a steward can select one or
several accepted complementary proposals through `/select`. Selection enforces
allocation, recipient, and settled-value limits, creates an explicit fund
reservation, and may retain links to ordinary
proposal tasks or a delivery team. Fund reads report reserved value separately.
Requested access is descriptive: neither selection nor compensation grants
repository, secret, credential, merge, environment, or withdrawal authority.

### Visible funded execution

A selected recipient reports milestone execution through the delivery
proposal's `/progress` resource. Each immutable observation carries its
milestone status and percentage, summary, agent compute, forecast date, access
and handoff health, and bounded references to established proposal tasks,
sessions, workspaces, forks, pull requests, checks, previews, or delivery
teams. The funded-outcome detail projects every selected execution with its
current progress, evidence, reservation, approved expense total, compute,
changes, blockers, and remaining-budget forecast, while the cited resources
continue to enforce their own permissions.

Recipients submit evidenced costs through `/expenses`; only a named fund
steward can decide them, and approval atomically moves value from reserved to
spent. Steward `/controls` can pause or resume work, replace the active
recipient, cancel the unspent reservation, or change its budget within the
outcome, fund allocation, recipient, and settled-availability limits. Reported
inactivity, revoked access, failed handoffs, overruns, pauses, and cancellation
block new expense submission or approval. Controls and replacement preserve all
legitimate progress and evidence and never grant repository, credential,
review, merge, environment, or fund-withdrawal authority.

### Evidence-governed milestone settlement

Outcome milestones may name `reviewer_ids`; otherwise the fund's governed
approvers are the designated reviewers. A reviewer posts an immutable decision
to `/repositories/{repository}/funded-outcomes/{outcome}/delivery-proposals/{proposal}/milestones/{milestone}/reviews`.
The decision snapshots bounded references to commits, authorship, handoffs,
checks, previews, releases, deployments, and declared outcome measures, plus a
required rationale and any dissent. Decisions can request correction, reject,
open a dispute, approve a partial award, or accept the remaining milestone.

An accepted award settles only value already reserved for the proposal's
original named recipient. Approved execution expenses count against that
milestone allocation, partial awards cannot exceed its remainder, and neither
evidence nor payment changes ordinary authorship, review, check, merge, release,
deployment, credential, or repository authority. The settlement history and
fund balances make every recognized tranche inspectable.

`/recoveries` records recipient withdrawal, deadline timeout, recipient appeal,
provider payment failure and retry, or a steward refund. Rejection, withdrawal,
and timeout release unearned value; payment failure returns an accepted award
to its reservation until retry; refunds release it back to the fund. Appeals
retain the challenged decision and allow a designated reviewer to assess the
same original allocation again rather than rewriting history.

### Complete backing-to-outcome workflow

The Funds repository workspace combines the governed ledger with roadmap
outcome creation and backing, human or approved-agent delivery offers,
acceptance, approval, selection, reservation, execution blockers, evidence,
costs, replacement controls, and attributable settlement receipts. Its reader
projection follows exact commit, pull request, check, preview, release, and
outcome-measure references without treating any financial record as authority
over those ordinary resources.

`project_funding_workflow_test.go` proves the complete loop through public HTTP
and stock Git. It retains a community-backed roadmap scope change,
complementary developer and approved-agent delivery, a pending agent-cost
overrun that blocks approval, an attributable replacement, a rejected
milestone, a disputed and then accepted milestone, and a refund. Rejected
milestone value is released immediately from the reservation; pending and
approved expenses are both counted when deriving an overrun blocker.

The complete discovery regression is `product_discovery_workflow_test.go`. It
connects released-product feedback, opportunity synthesis and challenge,
transparent roadmap acceptance and rejection, consent-bound validation,
human-agent delivery evidence, measured failure, reciprocal participant
learning, and accountable replanning through the public API and stock Git.

Authenticated repository readers use `POST
/repositories/{repository}/product-feedback` for product needs broader than a
reproducible issue. A submission names project, release, documentation journey,
or preview context and retains the need, desired outcome, frequency, impact,
audience, provenance, identity visibility, evidence visibility, contact
preference, and research/update consent. Release, journey, and preview IDs are
resolved within the repository before acceptance. Private organization feedback
requires accepted organization membership and is invisible outside that
membership.

List and detail responses are viewer-specific projections: reporters may hide
their identity from the general audience, contact values are maintainer-only,
and maintainer-only redacted evidence exposes metadata but not content to other
readers. Discussion is attributable, maintainers can add validated issue and
product-experiment links, and reporters can withdraw consent to clear future
contact while preserving the historical need. The web application exposes the
same contract at `view=feedback`. Feedback and consent do not grant operational,
research, targeting, or repository authority.

## Product opportunities

Repository writers use `POST /repositories/{repository}/product-opportunities`
to publish an inspectable synthesis of permitted feedback, issues, preview
findings, support signals, usage evidence, and experiment outcomes. Each
immutable version states the unmet need, affected audiences, severity, reach,
confidence, expected value, explicit uncertainty, and why every cited source is
supporting, contradicting, a minority need, or a duplicate. Platform-owned
sources must resolve at the cited feedback timestamp, issue version, exact
preview-finding revision, or experiment decision version; bounded external
support and usage observations retain their declared revision identifiers.

Repository readers can inspect complete history and append attributable
corrections or challenges without changing a submission. A feedback reporter
may detach their report through the opportunity's `/feedback/{feedback}/detach`
action; detachment publishes a new opportunity version while the older version
continues to explain what was previously synthesized. Lists project only the
current version. These records deliberately expose contradictory evidence,
minority needs, duplicate classification, uncertainty, and staleness rather
than calculating a popularity score or asking readers to trust hidden agent
reasoning. They grant no operational or research authority.

## Product roadmaps

Repository writers turn current opportunity versions into accountable direction
through `/repositories/{repository}/product-roadmaps`. Immutable versions record
goals, capacity, and sequenced accepted, rejected, or deferred outcomes with
owners, horizons, success measures, dependencies, risks, governance decisions,
commitments, and rationale. Reads derive capacity overflow, missing dependency,
commitment conflict, unavailable owner, and slipped target blockers; resolution
requires an attributed optimistic-concurrency replan. Readers can add version-
bound discussion and human or agent scenarios, but scenarios always carry no
resource, repository, delivery, or governance authority.

### Outcome delivery and measured value

Repository writers promote an accepted outcome through
`POST /repositories/{repository}/product-roadmaps/{roadmap}/outcomes/{outcome}/delivery`.
The request freezes the current roadmap and opportunity versions and an exact
base revision, then creates an ordinary proposal with ordered human- and
agent-owned tasks. Each task carries acceptance criteria, cited evidence, and
the success measures it covers; promotion fails unless the plan collectively
covers every measure that earned the outcome priority.

Writers attach exact pull request, check, preview, integration, release,
deployment, and experiment observations through the delivery's `/links`
collection. Shipping can therefore produce `delivered_not_achieved` while
measures remain absent or failed. Changed assumptions, unresolved user needs,
policy conflicts, and failed measures are explicit blockers. A cited
`/revisit-requests` entry moves the record to `revisit_required`; only delivered
work with all measures passed and no blockers becomes `achieved`. These records
report through existing delivery systems and grant no operational authority.

### Reciprocal product learning

Each delivery exposes its consent-aware learning record at
`/repositories/{repository}/roadmap-deliveries/{delivery}/learning`. Repository
writers publish typed decision, preview, delivery, rejection, or measured-
outcome updates with a plain-language rationale, explicitly cited feedback and
stakeholder recipients, and links for inspecting rationale, validating a
release, or following resulting work. Feedback recipients must still have
active product-update consent. Participant reads include only their addressed
updates, redact non-public link targets, and omit maintainers' broader response
and departure ledger.

A cited feedback reporter can respond once per update with `improved`,
`not_improved`, `mixed`, or `unknown`, a bounded explanation and evidence, and
explicit dissent. Any participant can leave the delivery conversation, while a
reporter may scope departure to one feedback record; departures suppress future
projection but preserve earlier credit, consent history, responses, and source
provenance. Maintainers append optimistic-concurrency lesson versions comparing
promised and observed outcomes, citing dissent and resulting work, binding a
current roadmap revision, and marking the opportunity `open`, `fulfilled`, or
`unsupported`. This record informs later roadmap revisions but grants no
contact, research, repository, roadmap, release, or operational authority.

## Project governance

## Repository accessibility commitments

Repository writers maintain shared accessibility contracts through
`/repositories/{repository}/accessibility-commitments`. Immutable versions
cover repository, journey, component, and release scopes and retain standards,
assistive technologies and platforms, target audiences, required scenarios,
severity/review policy, owners, explicitly approved time-bounded exceptions,
and links to roadmap outcomes, documentation, previews, and release policy.
Revisions require the current version so simultaneous editors cannot silently
replace one another.

Writers append coverage evidence for one exact contract version, required
scenario, and declared assistive environment as `passed`, `failed`,
`unsupported`, or `not_tested`; the record retains its actor, optional exact
revision, evidence, notes, and time. Reads derive missing or failed scenario/environment
coverage, unsupported environments, exceptions expiring within 30 days,
expired exceptions, and conflicting standard versions or levels on overlapping
scopes. The `view=accessibility` repository surface makes scope, judgment,
ownership, evidence, and gaps inspectable before review. State lives beneath
`$ACCESSIBILITY_COMMITMENT_ROOT`; the contract grants no review, merge,
release, credential, or operational authority.

## Repository performance contracts

Collaborators define the meaning of an optimization through repository-scoped
`/performance-goals`. An immutable version selects a repository, release, user
journey, API, command, or service and records representative workloads,
unit-bearing metrics, baseline values and evidence, target ranges, explicit
budgets, correctness constraints, supported environment digests, accountable
owners, baseline freshness, and links to issues, incidents, previews, releases,
and decisions. Revisions use optimistic concurrency so a stale editor cannot
replace a newer agreement.

Measurements append beneath a goal and retain their actor, exact goal version,
metric, value, environment digest, source, optional repository revision, and
measurement time. The API derives `missing_measurement`,
`incomparable_environment`, `stale_baseline`, and conflicting target-range
states on reads instead of treating an isolated result as comparable. The
repository `view=performance` surface publishes, revises, and measures these
contracts; it grants no additional repository or execution authority. Durable
state is rooted beneath `$PERFORMANCE_GOAL_ROOT`.

Authorized collaborators append reproducible trials at
`POST /repositories/{repository}/performance-goals/{goal}/trials`. Each trial
binds the current goal version and an existing exact commit; a supplied release
must attest that same commit. Retained evidence includes benchmark definition
and sanitized-input digests, environment, warmups, raw samples, sampling method,
server-derived mean and sample variance, resource use, bounded traces, logs,
profiles and artifacts, cost, attribution, and rerun lineage. Credential-like
evidence is rejected. Repository readers can inspect this on `view=performance`
without receiving operational access or private production data.

Supported performance-investigation conclusions cross into delivery through
`POST /repositories/{repository}/performance-investigations/{investigation}/changes`.
The endpoint creates an ordinary assigned proposal task and snapshots the goal,
baseline trial, diagnosis, constraints, owner, and base revision. Exact
candidate evidence is attached through a goal's `/comparisons` collection only
when its trial revision equals the linked pull request source and the baseline
and candidate share the benchmark definition, workload, environment, and
sampling method. The result reports a 95% confidence interval plus CPU, memory,
cost, correctness, scenario, command, author, and residual-risk evidence; it
does not alter ordinary repository or pull-request authority.

Repository and organization owners publish governance charter drafts through
their scoped `/governance-charter` API. Each immutable revision records roles,
eligibility, decision classes, participation and quorum, protected resources,
terms, removal and succession procedures, amendment rules, author, reason, and
the live authority preview used at publication. An owner must add a
version-bound approval before activation; activation regenerates that preview
and rejects unsupported resources or membership/quorum requirements the
current project cannot satisfy. Existing ownership, teams, branch protections,
integration queues, releases, environments, security controls, and agent grants
remain authoritative: a charter documents legitimacy and grants no operational
access. Later revisions, approvals, and expiring exceptions append history
without changing completed decisions. Durable state is beneath
`$GOVERNANCE_ROOT`; the web surfaces are repository `view=governance` and the
selected organization workspace.

Active charters also admit evidence-backed participant standing through
`/governance-charter/standings`. Owners may cite bounded contribution, review,
support, ownership, or membership evidence when inviting a principal into a
declared role; acceptance starts the charter-defined term. Recusal, resume,
suspension, expiry, identity revocation, and federation-trust revocation append
attributable events instead of erasing eligibility history. Reads expose the
participant's role, evidence, responsibilities, term, nominations, appeals,
conflicts, and lifecycle state. Every standing has an explicitly empty
operational-authority boundary.

Governed proposals extend an active charter at `/governance-charter/proposals`.
They freeze the applicable charter version and declared decision rules while
showing alternatives, cited evidence, scope, affected resources, disclosures,
discussion deadline, electorate roles, quorum, threshold, and implementation
effects. Current active standing controls who may open and cast one human
ballot; approved agents can contribute only cited analysis. Secret choices and
rationales stay sealed until closure. Tallying re-evaluates standing, excludes
recused, expired, suspended, or revoked voters, treats abstentions separately,
and retains the counts, electorate, exclusions, deterministic digest, contests,
resolutions, and dissent without granting implementation authority. Participation
or a later vote does not grant code, secret, merge, deployment, membership, or
credential access.

An approved tally creates an immutable `decision_receipt` binding the charter
version, tally digest, winner, scope, affected resources, and exact effects. Its
implementation view begins `awaiting_owner_approval`, grants no operational
authority, and names the ordinary policy revision, initiative, task plan, role
transition, or access request required. Resource owners link those artifacts
through the proposal's `/implementation` action; target resources continue to
enforce their own review, integration, release, environment, extension, and
agent controls. Missing artifacts remain blockers, while changed scope, cost,
assumptions, or protected effects require a new or amended decision.

Stewardship recovery extends that charter at `/governance-charter/stewardship`.
Owners retain attributable nomination, election, term-expiry, recall,
succession, deadlock, and emergency cases bound to affected standings and
decision receipts. Removal empties governance-derived authority while
preserving evidence and history. Resource handoffs remain separately approved
external actions. Emergencies require narrow scope, review, and an expiry of at
most thirty days. The derived `/health` view exposes vacancies, expiring terms,
quorum loss, unresolved handoffs, deadlocks, appeals, and emergency powers.

The complete collaboration contract is exercised by
`governance_workflow_test.go`: a project adopts a charter, recognizes proven
contributors, records recusal, failed quorum, evidence, approval, and dissent,
then links human-verified agent work delivered through stock Git and ordinary
checks, owner review, merge, and release. Successor election, a separately
owner-approved resource handoff, appeal, and bounded emergency recovery retain
the full record without changing repository ownership or availability.

## External extensions

## Federation identity

An instance publishes its version-1 signed identity at
`GET /.well-known/komodo-federation`. The document identifies the canonical
HTTPS instance, discovery and actor endpoints, capabilities, operator contacts,
Ed25519 public keys, a digest link to the preceding version, and only public
users, approved agents, or installations selected for federation. Individual
public identities resolve at `/federation/actors/{kind}/{id}` and use stable
instance-qualified subjects such as `agent:reviewer@https://community.example`.

Local operators discover a peer through `POST /federation/peers/discoveries`,
inspect retained observations at `GET /federation/peers`, and explicitly trust
or revoke it through its `/trust` action. Signatures are checked before a peer
document is accepted. Key rotation publishes a new signed version while
retaining the retired key and prior digest; unreachable peers preserve their
last verified identity, and an unchained identity change blocks trust. Remote
subjects never become local principals, credentials, repository members, or
authority grants, and discovery exposes no membership beyond the instance's
deliberately public actor catalog. The Access workspace presents this trust
record. Durable state is rooted beneath `$FEDERATION_ROOT`.

### Federated agent collaboration

Agents improve a federated contribution on the source community, never on the
remote target. A local participant delegates an approved agent against the
exact source revision and branch with an explicit path/evidence context. The
API issues a 24-hour Git credential limited to that local branch and records
worker events, guidance, and revocation only beneath
`$FEDERATED_AGENT_SESSION_ROOT`. It does not preload secrets, remote repository
contents, checks, or private pull-request evidence.

On publication, the API derives the new commits and changed files from local
Git, revokes the credential, and sends a signed, idempotent `agent_session`
pull-request event to the trusted target. Only the declared summary, commands,
evidence, costs, and residual concerns cross the boundary. The remote pull
therefore shows `agent:{id}@{home instance}` and its authorizing user as
verified observations, while all review, check, merge, and repository authority
remains remote and unchanged.

### Federated repositories

Trusted peers advertising `repository.discovery` expose a bounded signed public
snapshot at the discovery document's `repositories` endpoint. References are
stable `repository:{id}@{canonical HTTPS instance}` subjects. A snapshot binds
public repository metadata, visible branches and exact tips, releases,
contributor pathway guidance, public issues, ready contribution opportunities,
and attributable activity to the publishing instance and an exact repository
revision. It explicitly reports supported and unsupported capabilities; source
contents and mutations are not projected.

Authenticated home-instance developers resolve or refresh a reference through
`POST /federation/repositories/resolutions`, list followed observations at
`GET /federation/repositories`, and open the shareable web reader at
`/federation/repositories?ref={reference}`. The home instance verifies the exact
response with the peer's discovered Ed25519 key and retains its content-addressed
cache revision, signature, source URL, and check time beneath `$FEDERATION_ROOT`.
Peer outage or a later private/not-found response preserves the last verified
snapshot as explicitly stale and marks visibility withdrawal; it never invents
local access, copies inaccessible evidence, or presents cached context as a
locally controlled repository.

Peers may separately advertise `repository.contributions`. This capability
adds signed, bounded Git object and proposal endpoints; it does not widen the
read-only discovery snapshot. From a current trusted remote observation, a
developer creates an independently owned local fork with `POST
/federation/repositories/forks`, then clones and pushes to that ordinary local
repository with stock Git. `POST /federation/repositories/{fork}/sync` imports
one selected upstream branch only when it can fast-forward local history.

`POST /federation/repositories/{fork}/proposals` binds an exact local branch tip
to an exact target tip from the last verified snapshot. The home instance signs
the bounded object closure, title/body, remote author subject, and public
contribution context. The upstream verifies its explicitly trusted peer and
every content-addressed Git object before creating a private immutable source
snapshot and ordinary reviewable pull request. Remote subjects remain remote
identifiers: neither instance receives repository membership, reusable
credentials, Git write access, or authority over the other. Stale revisions,
unsupported negotiation, unreachable peers, invalid signatures or objects,
diverged synchronization, and rejected transfer are returned as explicit safe
failures rather than patches or partial authority grants.

### Federated pull-request conversation

Peers advertising `pull_request.exchange` expose `POST
/federation/pull-request-events`. A repository participant publishes through
the local pull request's `/federated-events` collection, so their home instance
first enforces ordinary repository write access and the current exact source
revision. Signed event kinds cover discussion, review, requested changes,
revision updates, checks, previews, and closure, binding both stable pull
references, a remote actor subject, audience, bounded evidence, and an
idempotency key.

The receiver verifies the exact envelope against an explicitly trusted peer key
and applies its own visibility and embargo rules. Exact replay is harmless;
changed content under the same key conflicts, while outage responses instruct
the caller to retry the same key. `GET
/repositories/{repository}/pull-requests/{pull}/federated-events` and the web
discussion show origin, verification, audience, and derived revision
currentness. Synchronization makes old evidence visibly stale without rewriting
it. Imported claims remain advisory: they create no local identity and satisfy
no local review, required check, preview access, closure, or merge policy.

### Federated contribution acceptance

An upstream owner merges an immutable imported source revision only through the
ordinary local review, required-check, preview, conflict, and integration policy.
The complete source object set is linked into upstream storage before its target
reference advances, while the pull permanently retains signed proposal
provenance and the merge commit retains source trailers.

After local publication the upstream signs a version-1 merge receipt binding
both pull references, exact source and merge commits, remote authorship, merging
maintainer, and digests of retained review and check evidence. `POST
/federation/contribution-receipts` treats exact retries as the same receipt and
rejects changed payloads under one idempotency key. Delivery failure is
retryable and cannot roll back or duplicate local history. Authenticated reads
at `GET /federation/contribution-receipts` preserve historical verification
while deriving `current_trust` anew, so source deletion, outage, or later trust
revocation neither erases accepted work nor falsely implies continuing trust.
An upstream owner retries the exact retained envelope at the pull request's
`/federated-merge-receipt/retry` action; retry never re-enters merge execution.

### Complete federation workflow proof

`apps/api/federation_workflow_test.go` runs the collaboration contract against
two independently persisted TLS HTTP applications and their public federation
APIs. It uses an unmodified Git client to clone the public upstream, create and
push a home-owned fork branch, and publish locally governed agent commits. The
test follows the signed remote snapshot, imports an immutable proposal, exchanges
maintainer and agent observations, applies ordinary upstream review and merge
policy, and verifies the signed receipt on the contributor instance.

The recovery portion replays the same receipt, follows a chained Ed25519 key
rotation, retains the last verified peer document through a temporary outage,
and then revokes current trust. The accepted receipt and its historical
signature verification remain visible after revocation while future federated
authority is contained. An agent publication at a newer local revision is
explicitly non-current at the immutable upstream candidate; it supplies
attributed evidence but cannot silently revise the pull request or acquire
remote check, review, or merge authority.

Extensions are platform principals, not user API tokens. Their owners register
them at `POST /extensions` with an operator contact, capabilities, HTTPS
callback and action endpoints, requested resource permissions, event types, and
credential rotation policy. The response returns separate endpoint challenges;
the owner proves control through `POST /extensions/{extension}/endpoint-verifications`.
Both endpoints must be verified before installation.

A repository owner examines a narrowed grant at `POST
/repositories/{repository}/extension-authority-previews`, then installs that
exact permission and event subset at the corresponding
`/extension-installations` collection. The preview names the extension's own
`ext_` actor, declares `can_impersonate: false` and `credential_issued: false`,
and warns while endpoint ownership is unverified. Listing retains installer and
repository attribution. `DELETE .../extension-installations/{installation}`
revokes the grant and removes all effective permissions and events without
erasing history. Registration and inspection are available in the web Access
workspace; data is rooted beneath `$EXTENSION_ROOT`.

Installations also retain approved or denied decisions for every declared
capability, selected resource types, and bounded non-secret settings. Owners
use a version-checked patch to upgrade, suspend, resume, or remove one grant;
each transition appends actor history and suspension or removal immediately
empties effective authority. Organization installs require an explicit set of
repositories, all validated before creation. Ownership transfer does not alter
installation attribution, and no lifecycle action issues a human credential.

Extension callbacks are durable deliveries derived from the repository activity
ledger. Installations filter that ledger by event and resource grants, retain a
schema-version `1` redacted payload with stable source and ordering IDs, and
sign the exact body with an installation-only HMAC key. Repository readers can
inspect payloads, attempts, retry timing, and dead letters, while owners trigger
delivery and replay. Duplicate source events remain one delivery and inactive
installations cannot send retained work. The owner receives the callback signing
secret once alongside the installation credential and provisions it to the
extension operator; neither secret is returned by later installation reads.

Repository participants inspect continuous health at the installation's
`/operations` resource: attributed requests, delivery outcomes, dead letters,
latency, invocations, contributions, hourly consumption, observed permission
use, and configuration history. Actionable notices cover missing or expiring
credentials, broken callbacks, and anomalous consumption. Installation
credentials expire after ninety days; rotation replaces the stored digest and
retains only issuance/expiry metadata and its attributable event.

Owners probe either declared endpoint against schema version 1 through
`/contract-tests`, retaining outcome, response, latency, error, actor, and time.
Quarantine empties effective authority while preserving prior evidence. Pause,
resume, narrowing upgrades, removal, rotation, tests, and health are composed in
the repository Extensions surface. New capabilities or permissions still
require another owner-approved grant.

The complete contract is composed in `extension_workflow_test.go` using only
public HTTP and stock Git. A developer registers and verifies a sample
extension, an owner grants it one repository, and a contributor opens a pull
request whose signed delivery first fails and then succeeds on explicit replay.
The extension publishes an exact-revision check with an annotation and artifact
and declares a repair action that the contributor invokes. That advisory check
does not replace the repository's required check or owner review. A capability
upgrade succeeds only from the owner at the current installation version, and
removal invalidates the installation credential while retaining deliveries,
attempts, contributions, actions, invocations, configuration history, and
extension authorship.

## Repository documentation collections

Repository owners create documentation contracts at `POST
/repositories/{repository}/documentation-collections` and revise them through
the collection's `/versions` action with optimistic concurrency. Each immutable
version declares a repository root and ordered page paths, exact source commits
for supported project versions, optional release mappings, participant owners,
audiences, navigation and rendering behavior, publication review policy, typed
links, attributable author, and change reason. Data is rooted beneath
`$DOCUMENTATION_ROOT`.

Collection reads resolve configured page blobs and Git commit authorship from the
reviewed commit rather than the moving branch. They return explicit
`missing_ownership`, `broken_source`, `rendering_mismatch`, and
`stale_version_mapping` findings. The repository's shareable
`view=documentation&ref={revision}` tab makes the explained version, maintainers,
reviewed source, history, policy, and page content visible together.

Documentation executability is repository-defined in version-1
`.komodo/documentation-checks.json`. Each check declares a unique ordinary
check name, one of the `links`, `symbols`, `build`, `sample`, `command`, or
`tutorial` verification kinds, its collection and pages, the documentation and
code paths that affect its result, and an exact matrix of source commits,
packages, or releases. Optional symbol/link lists and coverage counts make the
resolved surface inspectable; `expected_output` and ordinary artifact paths
retain built pages and output differences.

Opening or synchronizing a pull request runs these commands beside
`.komodo/checks.json` checks in the existing CPU/time-bounded, networkless,
credential-free Bubblewrap environment. Public check-run JSON and event and
artifact endpoints expose the structured documentation specification, logs,
matrix, coverage, outcomes, builds, and diffs. The runner hashes only declared
input path/object identities. When a new source commit changes none of them it
creates a current successful run that explicitly points to the prior successful
run; changing any declared documentation or code input executes the check again.
Repositories use the documentation check's ordinary name in branch
`required_checks`, making the exact current evidence part of standard readiness
and merge policy.

Pull requests compose code and documentation review through immutable
`/documentation-previews`. A preview snapshots the collection version, exact
source revision, rendered page blobs, navigation changes, verified check-run
examples, affected versions, and declared gaps. Repository participants invite
technical or audience stakeholders, comment against an exact rendered path,
blob, and bounded range, and approve or request changes in either required area.
Decisions retain the reviewed blob set. When a pull request synchronizes, reads
compare only those page blobs with the new source revision and report the exact
stale paths; unrelated code changes therefore do not silently carry forward or
needlessly discard documentation acceptance.

After the exact reviewed pull-request revision merges, an owner publishes its
approved preview at `/documentation-publications`. This snapshots rendered
pages, source and merge provenance, audiences, redirects, and version/release
mappings as an immutable edition. `GET /repositories/{repository}/documentation`
selects by `version`, `release`, `path`, or full-text `q`, returns stable web
links, and marks superseded editions as archived.

Authenticated readers report `page_feedback`, `failed_example`, `search_miss`,
or `version_mismatch` beneath an exact edition. Page reports name a page in the
snapshot; bounded log, screenshot, or sample-input evidence is redacted before
persistence. Maintainers link each outcome once to an existing issue, proposal,
or documentation task with reporter and triage attribution intact.

The complete contract is composed in `documentation_workflow_test.go` using
only public HTTP and stock Git. It changes behavior and guidance together from
a contributor proposal, records owner discussion and a source-citing Codex
suggestion, executes the declared documentation matrix, reviews the rendered
page, and publishes the merged release edition. A reader then reports a failed
instruction for the older release; the report is redacted and triaged into a
version-bound documentation task, and an agent-authored repair returns through
the same checks, pull-request review, release, and publication policy. Version
selection retains superseded editions as explicit archives while identifying
the newest corrected guidance, preserving source, reader, reviewer, publisher,
and repair authorship instead of rewriting the original edition.

## Preview acceptance

Repository owners publish preview acceptance requirements with `PUT
/repositories/{repository}/preview-acceptance-requirements`. Requirements select
target branches and optional changed-path globs, identify applicable risk
classes, and define promised-behavior scenarios plus required participant roles.
Invited testers or feedback participants and repository participants record an
attributable acceptance or reasoned rejection on one exact preview. An owner may
override a rejection only with a durable justification.

Pull-request readiness reports applied policy, risk classes, current and stale
acknowledgements, missing roles, rejections, overrides, and blocking findings.
Changing the source revision makes older acknowledgements stale. Current
unresolved blocking findings or an unsatisfied applicable scenario add the
`preview_acceptance_required` blocker and prevent merge alongside reviews and
required checks.

## Contributor pathways

### Ready contribution opportunities

The contribution pathway’s `view=contribute` workspace also presents work
derived from already-governed project resources. Owners publish through `POST
/repositories/{repository}/contribution-opportunities`, identifying a triaged
issue, open proposal, ready unassigned plan task, or current organization
stewardship finding. The API resolves the source before accepting it and pins
the resulting opportunity to the source or current repository revision. Each
entry describes required skills, interests, expected outcome, bounded paths or
areas, dependencies, risk, available mentors, and whether human or agent help
is available. The same source cannot be exposed twice.

An authenticated repository reader records interests, skills, available hours,
maximum risk, and preferred assistance at the singular
`/contribution-opportunity-profile` resource. The `/contribution-opportunity-matches`
read ranks every visible entry with inspectable positive reasons and gaps;
missing skills are shown as learning needs instead of silently hiding useful
work. Matching is advisory and deterministic.

Readers reserve one ready entry at its `/claims` collection for one hour to
fourteen days. A live claim blocks duplicates, retains claimant, note, and
expiry, and can be released early only by that claimant. Expired claims stop
blocking automatically while remaining attributable history. These APIs use
repository-read authentication and every response makes the authority boundary
explicit: a profile, suggestion, or reservation grants no repository write,
Git, agent, review, or merge access. Data is stored beneath
`$CONTRIBUTION_OPPORTUNITY_ROOT`.

An active claimant can turn the match into a reproducible starting point at
the opportunity's `/start` action. The API creates a private contributor-owned
fork and launches `.komodo/workspaces.json` at the recorded commit. The shared
workspace retains the current pathway prerequisites and supported setup,
source issue or proposal evidence, explicit acceptance criteria, and only
sample-data paths that the maintainer allowed and that exist at that commit.
Setup commands run in the existing credential-free, networkless sandbox;
workspace-grounded questions provide revision-specific explanations.

Contributors append `missing_access`, `obsolete_instructions`, or
`non_reproducible_prerequisite` reports to the original opportunity when the
starting point fails. Reports are attributable and repository-visible and must
not copy secrets, private terminal history, or inaccessible evidence. The fork
and workspace provide no upstream write authority.

Starting also creates an opportunity-bound collaboration ledger at
`.../contribution-opportunities/{opportunity}/collaboration`. Its 45-second
presence leases show the claimant and designated mentors in the help thread,
files, setup, or checkpoint surfaces without granting mentors access to the
private fork. Durable typed events distinguish questions, requested early
checkpoints, advice, answers, handoffs, interventions, scope changes, and
resolution. Entries retain actor role and an explicit decision owner, while
mentor availability and the start-selected response deadline make outstanding
attention visible.

The claimant or upstream owner may authorize an organization-approved agent in
one explicit mode: explain, diagnose setup, or edit named workspace paths.
Agent explanations, diagnoses, and edits append attributable actions to the
same ledger; edits use the normal workspace mutation and authorship boundary.
Pausing or revoking the versioned control rejects later agent action. A mentor
never receives fork membership through this workflow, and an agent control is
not a general repository or Git credential. When response inactivity, changed
scope, or lost access makes the current arrangement untenable, participants
record `reassignment_required` or `exited`; that terminal transition revokes
agent controls and presence but retains questions, advice, handoffs, edits,
checkpoints, and other legitimate progress.

### Publishing a guided contribution

The active claimant preflights or publishes a checkpoint at `POST
/repositories/{upstream}/contribution-opportunities/{opportunity}/publication`.
The request identifies the private fork workspace, immutable checkpoint,
source and target branches, pull-request title, commit message, and one
`satisfied` evidence statement for every opportunity acceptance criterion.
Set `dry_run` to inspect the same requirements without writing Git or creating
a pull request.

Preflight reports `blocking_requirements` separately from `coaching_needs`.
Blocking omissions include a missing acknowledgement of the current pathway,
a stale opportunity workspace, a missing or non-reproducible checkpoint, and
incomplete acceptance evidence. Unresolved questions or absent mentor/agent
support stay visible as coaching context but do not impersonate repository
policy or prevent a contributor from requesting review.

Successful publication reconstructs and pushes only the verified checkpoint
content to the contributor-owned fork, then creates an ordinary cross-repository
pull request upstream. Its structured `contribution_context` retains the
opportunity, exact pathway version and acknowledgement, declared setup commands
and dependencies, safe mentor guidance and agent assistance summaries,
acceptance evidence, workspace/checkpoint IDs, and exact file contributors.
The platform immediately starts ordinary commit-bound checks. Discussion,
reviews, reproductions, required checks, owner acknowledgement, protected
integration queues, and merge permissions continue through the established
pull-request workflow.

Once the guided pull request is merged and a release includes its exact merge,
the repository owner completes the opportunity at `POST
/repositories/{repository}/contribution-opportunities/{opportunity}/outcome`.
The API verifies both links before retaining immutable contributor credit,
maintainer feedback, support hours, `ready`, `ready_with_support`, or
`needs_guidance` readiness, and an optional suggested next opportunity. These
trust outcomes are public alongside the opportunity on the Contribute surface
and explicitly grant no repository membership or future authority.

Repository owners publish the project participation contract at `POST
/repositories/{repository}/contributor-pathway/versions`. Each immutable
version gathers project goals, prerequisites, conduct and private-security
guidance, supported setup steps, communication expectations, review policy,
and categories of work suitable for outside humans, agents, or either. An
`expected_version` prevents concurrent maintainers from replacing intervening
guidance, and every version retains its author, reason, and publication time.

References link the contract to exact-revision documentation and workspace
definitions, current repository owners, and existing releases, issues, and
proposals. The API checks those resources on every read and marks them
`current`, `stale`, or `inaccessible`; a default-branch move therefore makes
old setup guidance visibly stale without rewriting its historical version.
Public project guidance is anonymously readable at `GET
/repositories/{repository}/contributor-pathway`. Authenticated developers can
append one acknowledgement per version before investing effort, retaining
their actor and note. The repository web surface is the shareable
`view=contribute&ref={revision}` tab. Publishing or acknowledging guidance
does not grant Git, repository, agent, review, or merge authority.

## Pre-execution delivery teams

Repository collaborators form a temporary delivery team at `POST
/repositories/{repository}/delivery-teams` around a proposal, organization
initiative, accepted decision, incident follow-up, or another named planned
outcome. The first charter and every later version retain the shared outcome,
success measures, operating principles, total hours/cost-unit/agent-run budget,
deadline, default escalation path, author, and reason for change. Team data is
stored beneath `$DELIVERY_TEAM_ROOT`, defaulting to `data/delivery-teams` from
the API working directory.

Participant invitations declare `human` or `agent`, the stable principal ID,
complementary role, why that participant is needed, responsibilities, an
individual budget and deadline, escalation target, and requested actions.
Humans must already participate in the repository. Agents must be registered
with the repository-owning organization and their preview derives from active,
unexpired organization role grants over that repository. In both cases the API
intersects effective actions with the organizer's own authority and reports
unavailable requests explicitly. The preview always states
`grants_authority: false`: forming a team creates neither a credential nor a new
repository, Git, merge, deployment, or agent permission.

Invitees accept or decline explicitly; approved-agent operators respond for an
agent. Accepted human members may collaboratively revise the charter, while
replacement and removal preserve the old participant record. Every mutation
uses `expected_version` and appends an actor-attributed event, so concurrent or
stale changes cannot overwrite an intervening agreement. The repository web
workspace at `view=teams&team={id}` presents the charter, authority boundaries,
acceptance state, history, and escalation contract before execution begins.

Each team has an optional revision-bound decomposition at its `/plan/versions`
collection. A plan version snapshots its charter version and ordered work
streams. Every stream has a stable ID and immutable revision, accepted team
participant owner, explicit inputs, expected artifacts, dependencies,
acceptance criteria, hours/cost/run budget, assumptions, integration order, and
one or more repository scopes pinned to a full commit with paths and required
actions. Stable IDs make later material replanning intelligible without
rewriting the versions collaborators previously considered.

The API derives visible blockers instead of silently accepting incompatible
parallel work: incompatible starting commits or overlapping paths in a shared
repository, missing dependencies, dependencies ordered after their consumers, unavailable owner
actions, individual or total budget overflow, and assumptions pinned to an old
upstream stream revision. A proposal remains `pending_acceptance` until every
current and affected prior owner accepts that exact plan version and all
blockers are resolved. Humans act directly and approved agents act through a
current operator; proposals and acceptances use the team `expected_version` and
append attributed events. Later charter or membership changes can make an
accepted plan visibly `blocked`. Plan history remains immutable, and neither an
accepted plan nor a repository scope issues a credential, starts a run, or
grants access.

An accepted work-stream owner links an already established execution surface at
`POST .../streams/{stream}/contexts`. The context must be a change session,
investigation, decision experiment, or shared workspace pinned to an exact
repository commit already accepted in that stream's scope. This records where
work occurs without starting the resource, issuing a credential, or importing
its private runtime state. Attachment resolves the named resource through its
established store and rejects a missing resource or mismatched captured commit;
change sessions additionally name their pull request and experiments name their
decision so the lookup is unambiguous.

Owners publish findings, questions, checkpoints, artifacts, decisions, and
residual uncertainty at the team's `/timeline`. Every entry retains the plan's
stream revision, acting participant and stable actor, execution context, and at
least one citation. Citations fail closed unless their repository, exact commit,
and optional path fall within the stream's accepted scope. Repository policy
protects the team read itself; evidence from another scoped repository stays
there rather than leaking through the home team's read boundary. The timeline is an explicit projection of
inspectable evidence—not a copy of hidden prompts, terminal input, credentials,
raw logs, or inaccessible output.

A handoff request at the team's `/handoffs` collection names one accepted
recipient, immutable timeline entry IDs, their derived exact revisions, the
source context, acceptance criteria, and residual uncertainty. Only that
recipient (or an approved agent's current operator) accepts through the
handoff's `/acceptance` resource. Acceptance retains the verifying actor, note,
and time; neither request nor acceptance expands authority. The Teams web
workspace presents scoped contexts, cited records, pending criteria and
uncertainty, and recipient verification together.

Once execution is attached, accepted owners publish safe stream checkpoints at
`POST .../streams/{stream}/status`. A checkpoint reports the captured revision,
queued/running/paused/failed/completed/canceled state, current action, open
question, predicted next action, cumulative hours/cost/run use, and coarse
access and output health. It contains no credential, prompt, terminal input, or
raw log. The team read derives one clock-sensitive `runtime` view across all
streams: dependency waits, stale revisions, expired or revoked access,
conflicting output, exhausted accepted budgets, failed executions, disconnected
participants, open questions, active controls, and bounded recovery or explicit
escalation. A running or queued stream becomes disconnected after fifteen
minutes without a checkpoint. One resume is the maximum recovery recommendation
for a failed stream; another failure is an escalation.

Repository-authorized collaborators intervene through the team's `/controls`
collection. The organizer may guide, pause, resume, or cancel one stream or the
whole effort, and may reassign operational follow-up or narrow a stream.
Accepted human stream owners may control their own stream. Reassignment requires
an already accepted participant whose independently effective actions cover the
stream; narrowing must be a subset of its accepted paths. Controls are immutable
and attributable, preserve accepted artifacts, do not rewrite the plan's owner,
and always report `expands_authority: false`. They coordinate the public team
contract but do not impersonate or directly mutate an attached execution
resource; its established control and credential boundaries remain authoritative.

Completed streams become reviewable through an immutable team reconciliation at
`POST .../integration/reconciliations`. Each contribution names its branch,
cited timeline entries, accepted handoffs, criterion evidence, and residual
risks. The API derives exact branch tips and merge conflicts from Git, orders
contributions by the accepted plan, and exposes every missing stream, stale
evidence, unaccepted handoff, unmet criterion, and conflict. Blocked attempts
remain retained and cannot publish; later attempts append instead of rewriting.

A ready stream publishes through the reconciliation's
`/streams/{stream}/pull-request` resource. This creates an ordinary commit-bound
pull request, starts declared checks, and links it back to the exact team plan
and reconciliation. Review-facing evidence retains participants, timeline and
handoff IDs, criteria, stream costs, decisions, and residual risks. Existing
review, owner approval, required-check, protected-queue, release, and repository
permission rules remain authoritative: team membership grants no credential or
merge authority.

`delivery_team_workflow_test.go` composes the complete capability through the
public team, pull-request, check, review, merge, and release APIs plus stock Git.
It begins at an accepted decision, coordinates a developer and three approved
agents, retains a challenged finding and its resolution, redirects a failed
stream within accepted authority, verifies an agent-to-human handoff, publishes
ordered contributions, and releases the merged result. Removing the completed
specialist then causes further team publication on that participant identity to
fail while the charter, plan, evidence, costs, controls, handoff verification,
review links, and release record remain intact.

## Evidence-driven technical decisions

Repository participants open consequential choices at `POST
/repositories/{repository}/decisions`. A pending decision begins in repository,
proposal, investigation, incident, evolution-plan, or stewardship-opportunity
context and names one accountable owner plus the participating repository
collaborators. Its current scope keeps the question, constraints, observable
success measures, optional deadline, and affected resource links together.

Scope edits append a complete attributable version through the decision
resource; they never replace earlier boundaries. Discussion is append-only at
the decision's `/comments` collection. Collection reads can filter by
`context_kind` and `context_id`, allowing related work to show advisory pending
context without making the decision a general contribution or merge gate. The
shareable repository surface is `view=decisions&decision={id}`. Decision data
lives beneath `$DECISION_ROOT`, defaulting to `apps/api/data/decisions` when the
API is started through the documented root command.

Alternatives live beneath each decision's `/alternatives` collection. Their
claims use the common assumption, tradeoff, risk, compatibility, cost, and
expected-outcome criteria, with an explicit dissent kind. Appending a claim may
name the exact claim it supersedes, preserving both authors and both positions.
Evidence citations identify exact code revisions and paths or durable
dependency, release, incident, and usage resources, together with observation
time and an inspectable URL. The API derives criterion gaps, evidence coverage,
dissent counts, and citations stale relative to the current scope version.

Participants can delegate one selected option through its `/agent-runs`
resource. The returned 24-hour credential reads only that decision scope and
alternative through `/decision-research-agent/context`; it can append a finding
only when every citation already belongs to the option. It has no Git or
repository-write authority. The Decisions web surface presents the common
comparison first and retains the complete claims, citations, uncertainty, and
supersession record below it.

Decision participants prototype an active alternative at its `/experiments`
collection. Every experiment names a full commit, a dependency fingerprint, and
one named command declared by that revision's `.komodo/workspaces.json`; the API
creates a policy-bounded shared workspace, runs setup followed by only that
command, and records that the experiment has no publication authority.
Reproduction creates another experiment with `reproduces_experiment_id`.

Experiment checkpoints cite an existing safe workspace checkpoint, ordered
workspace log events, and artifact paths actually present in its diff. Resource
use is derived from the workspace ledger. Posting current code, dependency, and
environment fingerprints to `/validity` reports `code_changed`,
`dependencies_changed`, or `environment_changed` without deleting evidence.
Experiments never publish, open a pull request, or merge implicitly.

Product experimentation planning is separate from those decision-research
workspaces. Repository collaborators use `/product-experiments/signals` to
register versioned, permitted product measures and `/product-experiments` to
agree a complete pre-exposure plan originating from a proposal, issue,
decision, pull request, preview, or release. Each success or guardrail measure
binds an exact signal version; the plan also freezes variants, audience
eligibility and exclusions, consent class, minimum evidence, duration,
accountable owners, participants, stop conditions, assumptions, and overlap
keys. The API and `view=experiments` web surface derive instrumentation and
audience-policy gaps, overlaps, changed assumptions, and current-version
approval gaps without erasing discussion or earlier approvals. Readiness is
advisory agreement to proceed with later implementation and exposure work; it
does not assign users, collect signals, publish variants, or grant release or
operational authority. Storage defaults to `data/product-experiments` and can
be changed with `$PRODUCT_EXPERIMENT_ROOT`.

Variant and instrumentation implementation is attached through the experiment's
`/work-items` and `/implementations` collections. Work items reference existing
human- or agent-owned tasks, sessions, or workspaces at an exact revision; they
do not create or execute those resources. Implementation publication accepts an
ordinary pull request ID and derives its exact source commit on the server. The
review record freezes the implemented variant IDs, exact signal event versions
and properties, exposure rules, privacy classification, removal plan, and the
repository-defined check names that verify assignment, metric capture, variant
isolation, and fallback behavior. Ordinary pull-request review, checks, branch
protection, and merge remain authoritative, and a later plan revision marks old
implementation evidence non-current instead of rewriting it.

Before rollout, the repository owner publishes an audience contract at the
experiment's `/audience-policies` collection. It binds the complete variant set
to an exact release and commit, declares deterministic basis-point allocation,
one mutually exclusive group, consent, regional and organization constraints,
structured inclusion and exclusion attributes, exact signal properties, data
retention, and the maintainers or privacy participants who must approve it.
Reads expose the eligible population and minimal collection policy while
showing only pseudonymous digests for assignment audits. Missing approval,
stale plan or release evidence, unauthorized properties, incomplete weights,
and conflicting groups block assignment. Repeated assignment is consistent and
does not turn the audience contract into deployment, telemetry, or exposure
authority.

### Validate direction before commitment

Repository writers create outcome validations beneath a roadmap's
`/validations` collection. Each captures the current roadmap and opportunity
versions, a technical decision, prototype, documentation concept, or product
experiment, and representative success and guardrail measures traced to cited
feedback. Its preview or research activity has an exact revision, bounded
scope, and time window; optimistic versions retain later changes.

An invitation requires a feedback reporter with active research consent and
returns a purpose-bound participant credential. That credential can read only
`/roadmap-validation-participant/context` and append bounded findings through
`/roadmap-validation-participant/findings`; it grants no repository access.
Findings retain attribution, the invited revision, accessibility needs,
dissent, acceptance, and evidence validity. Writers append assessments citing
exact findings and choose `accept`, `revise`, `defer`, or `reject`, preserving
the roadmap plan and validation versions that preceded the learning.

The accountable owner requests affected-owner acknowledgements or named policy
approvals through the decision's `/approval-requirements` collection. Only the
named participant can respond; pending requirements and rejected approvals are
derived as public blockers and conflicts. The owner publishes at `/commitments`
only after every requirement succeeds and every alternative is explicitly
selected or rejected. Each append-only commitment version freezes its scope
version, rationale, accepted tradeoffs, retained dissent, conditions, review
date, exact retained evidence citations, and approval records. Later material
scope, alternative, claim, or evidence changes move a published decision to
`reopened` without changing an earlier commitment.

An owner may authorize a downstream deviation at `/exceptions` only against an
existing commitment. Exceptions name their scope, reason, conditions, decision
version, authorizer, start, and an expiry no more than one year away; revocation
is retained instead of deleting the record. Active, expired, and revoked
exceptions remain visible in the Decisions workspace and grant no repository,
Git, publication, or merge authority by themselves.

An accepted commitment becomes accountable implementation at the decision's
`/delivery` resource. Promotion creates one ordinary proposal and ordered human-
or Codex-owned tasks at an exact base commit. Their completion criteria must
collectively cover every scope constraint, success measure, and commitment
condition without inventing substitutes. The immutable decision ID, commitment
version, rationale, and evidence flow through the existing task assignment,
session or workspace, pull request, check, integration, release, and deployment
contracts. Release pull-request inclusions retain the decision identity and
version so delivered artifacts remain traceable to the choice they implement.

Publishing a decision-governed task requires a review-facing delivery account
whose criteria exactly match the task and are all evidenced as met. Agent work
also requires its scoped change session; human work retains assignment, Git,
pull-request, and review attribution without manufacturing an agent session. If
an assumption changes, a measure fails, work proves incompatible, or a
constraint must be deviated from, a participant posts an evidence link at
`/revisit-requests`; the request is retained as an actionable blocker and
reopens the decision. A time-bounded owner exception remains the explicit path
for an authorized deviation, never an implicit waiver by implementation.

`decision_workflow_test.go` composes the complete public decision contract. It
proves affected-owner participation, agent research, competing and reproduced
experiments, approval with retained dissent, dependent human and Codex delivery,
ordinary checks and reviews, immutable release inclusion, success-measure
evidence, and a post-release changed-assumption revisit through public HTTP and
stock Git.

## Proactive agent stewardship

An organization owner can define long-running responsibility without turning a
goal into ambient authority. `POST /organizations/{organization}/stewardship-mandates`
creates version 1; later versions are appended at the mandate's `/versions`
collection. Every immutable version records desired outcomes, repository and
branch boundaries, trusted signals, exclusions, monthly hours and daily run
budgets, cadence and expiry, the approved agent, allowed actions, and decisions
that must return to a human.

Versions begin `pending_acceptance`. Only a current operator of the named agent
can accept one, retaining operator identity and time. An organization owner can
pause, resume, or revoke an exact version, and revisions require fresh
acceptance rather than inheriting consent. The schedule makes a retained
version effectively `expired` after its deadline.

The version preview resolves current organization policy and existing agent
role grants separately for every scoped repository. It always reports that the
mandate created no authority and provides no write or merge permission. Any
actual execution must still pass through an independently scoped grant and the
ordinary proposal, session, review, check, and integration controls. The
organization workspace exposes drafting, inspection, preview, operator
acceptance, revision, pause/resume, and revocation while keeping every version
readable for accountability.

### Evidence-backed stewardship backlog

An active mandate's approved agent operator submits evaluations at `POST
/organizations/{organization}/stewardship-opportunities/evaluations`. The
repository and signal must be explicitly trusted by that exact active mandate
version. Each finding includes severity, expected value, confidence, affected
owners and immutable revisions, typed source citations with observation times,
and a plain-language explanation of why the mandate includes it.

The mandate, repository, and stable finding key form the deduplication boundary.
A later evaluation refreshes the same item rather than creating suggestion
noise, while its shared discussion, decisions, and rank remain intact. Evidence
does not silently follow a moving branch: collaborators can mark it stale with
a reason, and only a new exact-revision evaluation clears that state.

Organization collaborators inspect the ranked queue at `GET
/organizations/{organization}/stewardship-opportunities`, discuss items through
their `/comments` collection, and append rank, dismiss, snooze, incorrect,
reopen, or stale decisions through `/decisions`. The organization web workspace
keeps citations, scope reasoning, state, and challenges together. These queue
actions prioritize attention only; they issue no credential and start no work.

### Governed opportunity admission

Organization owners version opportunity-class rules at a mandate version's
`/work-policy` resource. A class either requires an owner decision or permits
the accepted agent operator to request bounded auto-start, with maximum risk,
daily-run, and monthly-hour limits composed with the mandate's smaller budget.
Every request sends the current policy version and opportunity decision version;
stale concurrent decisions fail instead of spending resources twice.

`POST /organizations/{organization}/stewardship-opportunities/{opportunity}/work-decisions`
records accepted and blocked decisions. Stale evidence, changed or inactive
mandates, changed policy, conflicting accepted work, active incidents,
embargoed security evidence, excessive risk, and exhausted budgets remain
explicit blocker codes. Decisions target affected owners through ordinary
repository activity, which also feeds established inbox projections.

An accepted item is promoted through its `/promotion` resource into an ordinary
repository proposal and ordered proposal-plan tasks. The handoff retains the
opportunity link, current evaluated base revision, human or agent owners,
observable outcomes, completion criteria, risk, and verification plan. It does
not create a branch, credential, session, or contributor authority; those begin
only through the existing assignment and change-session contracts. Refreshed
evidence supersedes an unpromoted acceptance and requires a new decision.

### Stewarded delivery evidence

Promotion copies the accepted opportunity ID, exact mandate version, evaluated
commit, citations, rationale, risk, verification plan, and affected-owner
acknowledgements into each task's immutable reasoning context. Starting the
task still uses the ordinary assignment concurrency token and creates the same
branch-only, 24-hour change-session credential as any other Codex task; pause,
guidance, cancellation, revocation, and worker event controls are unchanged.

Publishing a stewarded task through its `/contributions` resource requires the
linked session and a structured delivery account: implementation reasoning,
commands run, residual risks, and one `met`, `unmet`, or `not_applicable`
decision for every original completion criterion. Criteria cannot be renamed or
omitted during publication, and unmet decisions require explanatory evidence.
The API derives source and target commits, files, actor, and recording time and
copies the opportunity context and account onto the ordinary pull request.
Reviewers see that evidence in the pull request overview, while required checks,
reviews, owner acknowledgements, fork boundaries, protected integration queues,
and merge permissions continue to apply without a stewardship exception.

### Stewardship learning and safety

Each accepted mandate version has a member-readable `/report` resource that
rolls up opportunity disposition, accepted and blocked recommendations,
implementation, verification and release results, estimated and actual resource
use, false-positive feedback, and evidence-backed progress or regression for
every original desired outcome. Organization owners append outcome attestations
beneath an opportunity; every record cites an exact repository revision and a
retained delivery, check, release, or other evidence resource. History is
append-only, so later success does not erase failed work or a rejected signal.

Owners may use that history to version class priority, minimum confidence, and
required evidence kinds alongside existing admission, risk, and consumption
rules. These settings can rank or block a recommendation, but cannot alter
repository/branch scope, trusted signals, agent identity, actions, budgets, or
human decision boundaries. Those material changes continue to require a new
immutable mandate version and fresh operator acceptance.

An owner or the accepted agent's operator reports `repeated_failures`,
`inactivity`, `access_revoked`, or `anomalous_consumption` at the exact mandate
version's `/health-events` resource. The store atomically pauses only that
active version and appends an actionable notice to its report. A maintainer must
review the retained detail and explicitly resume through the ordinary mandate
lifecycle; reporting a safety condition never grants replacement access or
quietly broadens another mandate.

### Complete stewardship workflow

`stewardship_workflow_test.go` composes the public stewardship contract as one
regression. A maintainer creates an organization repository and bounded mandate,
an operator accepts it and explains a current usage finding, and collaborators
rank and guide the accepted improvement while dismissing a second finding. The
accepted item passes versioned auto-start policy into an ordinary proposal task,
branch-only agent session, structured contribution, commit-bound check, owner
review, merge, and immutable release build.

The retained mandate report connects estimated and actual effort, run count,
signal citation, decisions, session events, exact code, review-facing reasoning,
verification, release evidence, and goal progress. Revoking the mandate after
delivery proves that the historical record survives while future stewardship
authority stops; neither the mandate nor the completed workflow bypasses normal
repository, review, merge, or release permissions.

## Prospective impact assessment

Impact assessment is a pre-implementation, exact-revision record rather than a
prediction that silently follows a branch. A repository writer starts at
`POST /repositories/{repository}/impact-assessments` with selected paths or
symbols, a current conclusion from a shared investigation, or a proposed
unified diff. The service derives reviewable evidence across references, tests,
last-touch history, published packages and interfaces, permission-visible
consumers, releases, and deployment environments. Scan limits and unsupported
files remain explicit completeness reasons.

Repository participants can add missed impacts and classify each item as open,
mitigated, an accepted risk, or unknown with a durable rationale. Requests to
affected owner IDs add those owners to the assessment—not to the repository—so
they can acknowledge the conclusion or record a concern. A delegated Codex
credential can read only the assessment and append findings citing evidence
already retained by it; it carries no Git or repository mutation authority.
The repository `view=impact&assessment={id}&ref={commit}` surface keeps scope,
decisions, acknowledgements, verification paths, uncertainty, and evidence
together before implementation begins. Data is rooted at
`$IMPACT_ASSESSMENT_ROOT`, defaulting to `apps/api/data/impact-assessments`.

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

## Actionable issue reports

The repository Issues tab is the public entry point for unexpected behavior.
An authenticated reader records expected and observed behavior, severity,
environment, ordered reproduction steps, an optional affected release, and
either public or repository-participant visibility. The API validates a named
release in that repository and snapshots its version into the issue.

Evidence accepts only declared logs, screenshots, traces, or sample inputs,
bounded to ten files, one MiB each and five MiB total. Collection responses omit
attachment bodies. Duplicate suggestions search only reports already visible to
the caller, so private evidence cannot be inferred from public search.
Discussion and status changes remain ordered, actor-attributed history, and
public issue links use `view=issues&issue={id}`.

The resources are `GET/POST /repositories/{id}/issues`, `GET/PATCH
/repositories/{id}/issues/{issue}`, `POST
/repositories/{id}/issues/{issue}/comments`, `GET
/repositories/{id}/issues/suggestions?q=...`, and `GET
/repositories/{id}/issue-templates`. Opening and commenting require an
authenticated repository-read grant; only the reporter or repository owner may
change status.

### Executable issue reproductions

An issue pinned to `affected_commit_id` or `affected_release_id` can become
shared executable evidence. The reporter or repository owner launches a named
command from the exact revision's `.komodo/reproductions.json` with `POST
/repositories/{repository}/issues/{issue}/reproductions`. Schema version `1`
declares the environment, tools, optional setup, CPU/memory/disk limits, and
named reproduction commands with timeouts, expected exit codes, and bounded
artifact paths. A release-backed issue always runs its attested immutable commit;
branch names and caller-selected commands are not accepted.

Each launch materializes only the captured Git tree, writes explicit sanitized
fixtures beneath `.komodo-inputs`, and runs in Bubblewrap without network,
credentials, repository metadata, or host filesystem access. Input names and
content that resemble credentials are rejected. Logs are bounded and redact
credential-shaped output; declared artifacts are regular files limited to one
MiB each and five MiB total and fail closed when credential-shaped. The durable
attempt retains the environment definition and SHA-256 digest, exact commit and
release identity, initiator, input checksums, ordered commands/logs/outcomes,
observed result, failure reason, and checksum-addressed artifact bytes. Failed
setup, command, or artifact collection remains inspectable.

Visible authorized collaborators inspect attempts with `GET .../reproductions`
and `GET .../reproductions/{attempt}`. Any authenticated repository participant
can `POST .../reproductions/{attempt}/reruns` after it becomes terminal; the new
attributed attempt copies the prior immutable definition, revision, command, and
sanitized inputs instead of reading a moved manifest. This makes environment or
fixture differences explicit while preserving every prior failure.

### Shared issue triage and diagnosis

Repository owners classify and prioritize an issue, assign existing repository
participants, apply visible labels, or identify a visible same-repository
duplicate through `PUT .../issues/{issue}/triage`. The issue version is an
optimistic concurrency token, so two maintainers cannot silently replace one
another's decision. Every accepted update records its actor and history event.
Closing remains a separate reporter-or-owner action rather than an implied side
effect of a label or diagnosis.

`POST .../issues/{issue}/relationships` adds an attributable typed connection
to code, a dependency, release, deployment, incident, proposal, pull request,
decision, or existing investigation. Code links require an existing exact
commit in the issue repository and retain an optional path. These links make
affected surfaces and already-planned work inspectable without copying their
evidence or changing their access policy.

Any visible authenticated participant opens a diagnosis with `POST
.../issues/{issue}/investigations`, selecting one retained reproduction attempt;
the record captures that attempt's exact revision. Participants append a
hypothesis, finding, reporter evidence request, conclusion, or challenge at its
`/entries` collection. Every entry has one or more citations to an actual
reproduction event or artifact, exact code revision, or issue relationship and
may identify suspected revisions and participant owners. Challenges name and
visibly dispute an earlier entry rather than overwriting it. Opening an
investigation at another revision marks the older revision's entries stale,
while keeping their authors, citations, dispute state, and chronology.

The owner may start a named read-only agent through the investigation's
`/agent-runs` collection. Its one-time credential expires after 24 hours and
can only read `GET /issue-investigation-agent/context` for that selected issue
and publish cited entries at `POST /issue-investigation-agent/entries`. The
context omits attachment bodies and explicitly carries no repository read,
repository write, or Git authority. Agent conclusions therefore remain visible,
attributed claims that humans can challenge, not hidden truth or triage power.
The owner can revoke an exact run through its agent-run resource; expiry and
revocation remain visible on the issue instead of erasing the agent's entries.

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

## Revision-aware code intelligence

`GET /repositories/{repository}/code-intelligence?ref={branch-or-commit}`
builds a knowledge map from exactly the resolved visible commit. Optional `q`
filters symbol names and paths, while `symbol` expands the matching definition
with source locations for references, function-call candidates, and tests. Each
symbol also exposes the first-parent commit that most recently changed its file,
including author identity and commit evidence. Go, TypeScript/JavaScript, Python,
and Rust are analyzed with deliberately conservative syntax patterns; consumers
must treat the relationships as navigation evidence, not compiler guarantees.

The response always identifies `repository_id`, the requested `revision`, and
the immutable analyzed `commit_id`. Its `analysis` object reports `complete` or
`incomplete`, skipped files, bounded-scan truncation, permission-hidden
relationships, and `stale`; results from different commits are never merged.
Binary, unsupported, or excess files are reported rather than silently omitted.
Declared dependencies are included only when their declaration names the exact
consumer commit and the caller can also read the provider repository. Resolved
interface schema paths remain source evidence owned by the relationship store.

The repository's shareable `view=intelligence&ref={revision}` tab searches and
navigates this map. Definition, caller, reference, and test links open the
ordinary exact-commit blob browser at the cited line, while dependency links
lead to the permitted provider's own map. Branch changes create a fresh request
instead of carrying analysis forward to a new revision.

## Grounded codebase questions

Authenticated collaborators ask from repository, file, proposal, task, pull
request, incident, or development-workspace context with `POST
/repositories/{repository}/questions`. The request carries a question, optional
branch or commit in `revision`, and typed `context`. The server resolves that
revision once and durably records the initiating user, immutable commit, answer,
structured claims, citations, and ordered events beneath `$QUESTION_ROOT`
(default `apps/api/data/questions`).

Every citation names its repository and exact commit. Source and documentation
citations also retain blob object, path, and line; history and
permission-filtered dependency citations retain immutable evidence identities.
Claims classify themselves as `evidence`, `inference`, or `uncertainty`, making
permission gaps explicit. `GET .../questions/{conversation}/events?after={sequence}`
replays server-sent events, while ordinary conversation reads remain
reproducible after a branch moves. The repository web surface derives context
from the current file, proposal, pull request, incident, or workspace and links
citations back to the exact-commit browser.

A completed explanation can continue directly into the shared investigation
surface. `POST /repositories/{repository}/investigations` accepts its
`conversation_id` only when the conversation belongs to the repository and its
immutable commit matches the new canvas revision. That durable link is copied
into reasoning-connected tasks, agent sessions, and pull requests, preserving
the attributed question and permitted evidence as implementation moves through
checks and review.

## Collaborative code investigations

Repository writers open a shared canvas with `POST
/repositories/{repository}/investigations`, naming a title, framing question,
and revision. The revision is resolved once; the record, its ordered runs, and
all entries are durable beneath `$INVESTIGATION_ROOT` (default
`apps/api/data/investigations`). Collection and detail reads require ordinary
repository access plus explicit canvas participation. Existing repository
participants can be invited, but a canvas invitation does not broaden their
repository permissions.

`POST .../{investigation}/entries` appends actor-attributed code references,
reproducible queries, runtime observations, hypotheses, agent findings,
conclusions, or challenges. Code citations are verified against the exact
canvas commit and retain the blob, path, and line. A runtime observation must
cite an existing ordered event from a bounded development workspace founded on
that same commit; the canvas stores the safe collaborator-authored observation
and pointer, never terminal output, runtime files, credentials, or private agent
context. Challenges and supersessions point to immutable prior entry IDs.

`POST .../{investigation}/runs` resolves a new revision and appends a run. It
preserves every prior entry and visibly marks commit-bound evidence stale when
the commit differs, allowing collaborators to re-verify or supersede it. The
shareable web surface is
`view=investigations&investigation={investigation}` and exposes invitations,
reruns, citations, challenges, and the ordered canvas.

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

## Reasoning-connected implementation

Shared understanding enters delivery through `POST
/repositories/{repository}/connected-work`. A repository writer selects
non-stale investigation conclusions and/or impact-assessment items, describes
ordered tasks by earlier task indexes, and may assign each task to a repository
participant or the `codex` agent. The service validates every source and
dependency before creating the proposal, then uses the selected evidence's
exact commit as the assignment base revision.

Each plan task owns an immutable `reasoning_context` snapshot containing the
origin resource and item IDs, claim or risk, decision and rationale,
verification paths, permitted evidence, and owner acknowledgements. Starting
an agent task copies that context into its scoped change session. Publishing
from either a task session or a task-derived workspace copies it into the pull
request, so reviewers do not need to reconstruct why the change exists from a
mutable branch or prose description.

Investigation reruns continue to mark old commit-bound entries stale and append
a new run. They do not rewrite contexts already attached to tasks, sessions, or
pull requests. Consequently the origin clearly reports invalidation while the
delivery record continues to identify the exact understanding that initiated
the work.

## Connected issue repairs

`POST /repositories/{repository}/issues/{issue}/repairs` turns confirmed issue
evidence into an ordinary assigned proposal task. The request selects the
completed reproduction, its exact-revision investigation and undisputed
conclusion, explicit acceptance criteria, and a human participant or `codex`.
Those values are copied into immutable task reasoning context so later branch
movement or investigation reruns cannot silently change the starting evidence.

Assignment issues no credential. Owners use existing task change-session or
shared-workspace paths, then publish an ordinary task pull request. Linking that
exact task request through the repair's `/pull-request` endpoint gives the issue
a durable progress record while repository permissions, checks, reviews,
queues, and owner-only merge policy remain authoritative.

### Candidate repair verification

After linking the task pull request, a participant can `POST
.../repairs/{repair}/verifications` to bind verification to the pull request's
exact source commit. The candidate reproduction manifest must match the retained
definition and environment before the original sanitized inputs and command are
replayed in a new credential-free, networkless Bubblewrap snapshot.

`GET .../verifications/{verification}` joins that attempt with required checks
from the same commit and the unchanged acceptance criteria. Its evidence digest
covers revision, definition, inputs, criteria, reproduction, and check outcomes.
A declared preview artifact is returned only through the issue visibility
boundary as retained evidence. Revision, input, manifest, or check changes make
prior confirmation stale.

Only the reporter appends `confirmed` or `rejected` decisions. Confirmation
requires that the retained failure no longer reproduces and all required checks
succeed. The owner may append a reasoned `override` under the same conditions;
it never replaces reporter dissent.

### Delivered resolution

Candidate confirmation does not claim that a fix has reached users. Once the
linked pull request is merged and included in a newer release, the reporter or
owner can submit that release's `release_id` to the ordinary reproduction
collection. Eligibility is derived from the release's server-recorded pull
request inclusions and an exact-revision reporter confirmation or reasoned owner
override. The runner then reads the newer release manifest and replays the same
named case with explicit sanitized inputs in the existing credential-free,
networkless boundary.

The delivered attempt is retained alongside the original affected-release
failure and candidate attempt. Maintainers link the real release and deployment
resources to the issue; reporter-or-owner status control remains separate, so a
non-reproducible attempt, passing general check, merge, or deployment cannot
silently close the report. The repository issue page presents triage, all safe
attempts and retries, cited human/agent diagnosis, acceptance criteria,
commit-bound decisions, the linked pull request, and delivered release and
deployment evidence as one permission-aware resolution trail.

`issue_resolution_workflow_test.go` proves the public-HTTP and stock-Git loop,
including an initial missing-fixture failure, an explicit cited evidence request
and sanitized retry, read-only agent diagnosis, branch-only agent authorship,
ordinary checks/review/merge, corrected release and deployment, unchanged-case
success against that release, and final reporter closure.

## Revision-exact pull request previews

A source repository opts into collaborative previews with a version-1
`.komodo/previews.json` file at the pull request's snapshotted commit. The file
declares build commands, a long-running start command, a service port, permitted
configuration names, and CPU, memory, disk, build-time, and lifetime bounds.
`POST /repositories/{repository}/pull-requests/{pull}/previews` resolves that
file from the pull request's source repository and exact source commit, accepts
only declared configuration, and records definition and configuration digests
before starting work.

Each attempt materializes only that Git tree, captures build and service logs,
and retains its creator, revision, setup lifecycle, authenticated same-origin
URL, expiry, and failure. Configuration values exist only in the launched
isolate; the durable/public record contains the declared names and an
attestation digest. Repository read policy also guards the preview gateway.
When pull request synchronization records a newer source commit, earlier
attempts remain intact and derive `stale: true` instead of being replaced. The
shareable web surface is `view=pulls&pull={id}&section=previews`.

Preview owners invite affected people with `POST .../previews/{preview}/invitations`.
An invitation names a user, one of `view`, `test`, or `feedback`, an expiry no
later than the attempt, and either a direct-user audience or an existing issue,
decision, or proposal whose retained participants prove why that user belongs.
`DELETE .../invitations/{invitation}` revokes it immediately. The attempt keeps
attributable invitation, gateway-entry, and revocation events.
An invitee can inspect `GET .../previews/{preview}/audience` for their role,
expiry, exact revision, effective policy, and an explicit matrix showing that
repository, Git, workspace, deployment, and production access remain false.

The optional version-1 `audience` object declares `network` (`none` or
`repository_allowlist`), `data` (`synthetic` or `masked`), `identity`
(`anonymous` or `preview_alias`), and actions (`navigate`, `submit_test_data`,
or `comment`). Omission uses anonymous, synthetic, networkless, navigate-only
defaults. The gateway removes authorization, cookies, and forwarded identity,
applies a restrictive CSP, and permits mutating test requests only when both
the manifest and a `test` invitation allow them. Invitation roles cannot read
code, open a workspace, inspect environment configuration, deploy, or reach
production/private services; the web surface states that effective boundary.

### Preview findings

`POST .../previews/{preview}/findings` turns hands-on evaluation into a durable
finding on that exact attempt. A current feedback invitation (or an existing
repository participant) supplies a route, title, description, ordered
reproduction steps, and up to ten permitted screenshots, recordings, console
captures, traces, or annotations. The server binds the finding to the attempt's
immutable source revision, redacts credential-like textual fields and sensitive
route parameters, and automatically links an open finding with the same title
and route as a duplicate.

Finding discussion is attributable and available to the exact-preview audience.
Repository participants classify findings as bugs, usability, accessibility,
content, performance, or questions; relate or explicitly deduplicate them; and
resolve or reopen them without losing history. Evidence metadata is safe to show
beside the pull request, but bytes remain separately stored with an
`exact_preview` audience and require the caller to pass the preview invitation
or repository-participant check again. Thus a finding can coordinate review on
the pull request without copying inaccessible evidence into a broader comment
audience.
### Preview finding repairs

Validated preview feedback can move into delivery without copying its context.
A repository participant can turn a finding into acceptance criteria and an
assigned human- or agent-owned proposal task at the exact observed source
revision. The participant may also prepare a pull-request change session or a
shared workspace; those resources receive only the selected evidence IDs, while
the evidence bodies stay behind the preview's dedicated access check.

After ordinary branch publication and pull-request synchronization, the repair
action requires that exact current source revision, validates every cited commit,
and retains the commands, checks, authors, session, and workspace that connect the
observation to the fix. It then starts a new normal preview attempt and resolves
the finding with that link. The action neither pushes nor synchronizes a branch,
issues no credential, and cannot bypass task, workspace, agent, review, or merge
permissions.

Evidence creation accepts base64 `content` only on the finding-creation request.
The preview store redacts permitted textual evidence, writes the result behind the
exact-preview access check, and clears the request body before retaining or
returning finding metadata. This asymmetric wire contract is intentional: later
finding, pull-request, and release reads can expose checksums and provenance but
never replay the submitted evidence body.

`collaborative_preview_workflow_test.go` is the black-box regression boundary for
the complete proposal-to-release loop. It proves a failed preview build and retry,
expired and renewed outsider access, revision-grounded redacted feedback, agent
repair through a branch credential, a new exact preview, stale then renewed
stakeholder acceptance, required checks and review, merge, release attribution,
and the absence of repository authority for the invited stakeholder.
# Collaborative performance diagnosis

Performance delivery policies apply current goal versions to target branches,
path globs, and risk classes. Pull-request readiness and merge require the same
exact candidate comparison to meet declared regression and confidence
thresholds. Comparison confidence intervals and policy thresholds use percentage
change from the baseline; a comparison with declared correctness failures or a
candidate trial that no longer matches the current pull revision blocks delivery.
Staged observations preserve that comparison through release and
deployment, retaining environment, health, assumptions, uncertainty, and
explicit pause, restore, linked repair, or decision-revisit outcomes without
granting operational authority.

Performance investigations extend exact-version performance goals and their
reproducible trials through
`/repositories/{repository}/performance-investigations`. A diagnosis selects
trial IDs with their exact revision, workload source, environment digest, and
audience before discussion begins. Its entries require inspectable citations
and retain hypotheses, folded flame graphs, structured comparisons, runtime
paths, uncertainty, challenges, confirmations, conclusions, and attribution.

Code and symbol citations resolve against exact Git blobs. Trial/profile/trace
citations must belong to the selected evidence set; restricted operational
evidence cannot be promoted into repository-audience discussion. Current reads
mark affected entries stale when the goal contract changes or selected evidence
is unavailable or differs in revision, workload, or environment. These records
are explanations only: they issue no credential and grant no repository,
execution, review, or merge authority.

`performance_workflow_test.go` proves the complete public concern-to-production
journey with stock Git: a production-linked goal and sanitized baseline lead to
an agent/owner diagnosis, agent-owned optimization, required check, review,
exact-revision comparison, release, and staged observations. A noisy trial and
correctness-regressed trial are retained as blockers, and a missed canary target
is retained with its pause before a retry and production measurement confirm the
user-facing improvement.

# Live product experiments

Approved audience policies launch only against an existing successful
deployment whose environment, release, and exact commit match the policy. Each
run retains ordered allocation stages and observations for live exposure,
declared measures and uncertainty, data and instrumentation quality, guardrails,
consent, operational health, evidence, and cost. Plan participants may advance,
pause, resume, or stop an active attempt.

Guardrail breaches, failed deployments, lost instrumentation or data, sample
imbalance, and revoked consent synchronously contain the affected attempt.
Containment does not alter stable assignments or discard prior evidence, and a
contained attempt cannot resume; retry is a separately identified run.

Once a declared threshold or stop condition is reached, participants publish an
analysis bound to one run observation. It retains segment effects, uncertainty,
exclusions, guardrail outcomes, bounded human or agent interpretation, dissent,
and aggregate evidence. Versioned outcome decisions adopt a variant, retain the
control, extend, or remain inconclusive and create rollout, rollback, follow-up,
or cleanup tasks without granting operational authority. Tasks require ordinary
pull-request, release, deployment, and evidence links. After all are complete, a
cleanup receipt records retired variants, targeting, credentials, and collection;
future assignment and launch are blocked while aggregate evidence, provenance,
user protections, and delivery links remain inspectable.

`product_experiment_workflow_test.go` composes this contract through public HTTP
and stock Git. It starts from user feedback, retains separate human variant and
agent instrumentation commits through ordinary checks and review, binds the
merged release to a successful deployment and consent-minimized audience,
contains a guardrail-breaching attempt, and progressively runs a separately
identified retry. The final analysis preserves agent uncertainty and human
dissent; the acknowledged decision links rollout evidence before cleanup retires
obsolete targeting and collection without deleting aggregate learning.

# Versioned locale support plans

`/repositories/{repository}/locale-plans` gives collaborators one attributable
contract for which repository, product, documentation, and release experiences
must work in each locale. Immutable versions declare language and region
targets, ordered fallback locales, preferred and avoided terminology,
formatting requirements, covered journeys, accountable owners and reviewers,
and per-locale completion thresholds. Readers can inspect the current agreement
and its complete history at `view=locales`.

Every translatable resource names its path, resource kind, format, journeys,
owners, and an exact source commit that the server verifies before publication.
Coverage evidence is separately attributable and pins the plan version,
resource, locale, journey, completion percentage, status, and exact source
revision. The reader derives missing ownership or coverage, unsupported
formats, conflicting preferred terminology, evidence stale against the current
resource commit, and release thresholds that remain unmet. These records make
support claims reviewable; they do not grant source, review, merge, release,
translation-provider, credential, or operational authority. Durable state
lives beneath `$LOCALE_PLAN_ROOT` (default `apps/api/data/locale-plans`).

### Revision-exact translation work

A repository defines extraction in schema-version `1`
`.komodo/localization.json`. The document names one source locale, target
locales, and JSON resources with stable IDs, source paths, `{locale}`
translation path templates, context, screenshot URLs, and plural rules. `POST
/repositories/{repository}/pull-requests/{pull}/translation-units/extract`
accepts only the pull request's exact source revision and server-captures the
configuration and source blob identities. Stable resource/key units expose
source locations, variables, prior messages, and added, changed, removed, or
reused state together with translated, untranslated, superseded, or removed
state for every locale.

Readers use the same pull-request surface to inspect extraction and submit a
translation proposal for an individual unit and locale. Proposals retain actor,
revision, source message, and prior versions. Re-extraction after source edits
supersedes affected proposals without erasing them; it never grants Git or
unrelated repository write access. Durable state lives beneath
`$TRANSLATION_UNIT_ROOT` (default `apps/api/data/translation-units`) and the web
workspace is `view=locales`.

### Grounded collaborative translation

Extraction can bind `locale_plan_id` and its current
`locale_plan_version`, freezing preferred and avoided terminology plus the
plan's locale-specific reviewers. The localization configuration may also
declare bounded `product_context`, `protected` or `embargoed` content, and
`permitted_actor_ids`; inaccessible work is omitted from repository extraction
lists as well as rejected at its detail and mutation routes.

Readers coordinate a locale through `/translation-units/claims` using the
latest claim version, with attributable release and permission-checked handoff.
Unit `/discussion` retains linguistic judgment next to its exact source.
`/suggestions` records the requesting human, scoped agent, exact revision,
suggested text, evidence references, and uncertainty. A human other than that
agent must approve, edit, reject, or escalate the result. Approval or editing
creates a human-authored proposal that retains its agent-suggestion origin;
the agent cannot approve itself. Proposal `/reviews` enforce the reviewers
frozen from the locale plan and retain rationale. Concurrent claims, protected
content, embargoes, source supersession, handoffs, and review decisions remain
explicit without granting Git, translation-provider, credential, merge, or
release authority.

### Locale publication governance and regional correction

`/repositories/{repository}/localization-delivery-policies` binds target
branches and changed paths to locale requirements selected by audience and risk
class. Each requirement names a coverage threshold, current localization check
names, and regional reviewers. A writer records an exact pull candidate at
`/pull-requests/{pull}/locale-publication`, assigning every included locale the
`staged`, `deferred`, or `withdrawn` state with a rationale and optional
fallback. Updates use an expected version. Readiness and merge evaluate only
staged locales against the exact candidate verification; deferred and withdrawn
locales stay explicit without blocking unaffected delivery or being presented
as supported.

After an application release or documentation collection is published,
`/locale-publications` retains its locale, public version, exact source
revision, candidate provenance, fallback, and published or withdrawn state.
The repository Locales surface presents those facts to readers. A permitted
reader can attach a `mistranslation`, `cultural_mismatch`, `broken_formatting`,
or `missing_content` finding to that exact publication and path. Repository
writers retain validation or rejection; only a validated finding can link an
existing ordinary proposal task with a named human or approved-agent owner and
immutable acceptance criteria. Publication, feedback, validation, and repair
links grant no source, review, merge, release, documentation, credential,
translation-provider, or operational authority. Durable state lives beneath
`$LOCALIZATION_DELIVERY_ROOT` (default
`apps/api/data/localization-delivery`).

`localization_workflow_test.go` composes the complete public contract through
stock Git and HTTP: evolving source supersedes prior translation evidence;
human and grounded-agent language decisions remain attributable; exact-preview
checks and named regional review govern only staged locales; a failed RTL
locale is withdrawn without widening the support claim or blocking French;
and post-release reader feedback returns through an ordinary agent-owned task,
reviewed repair, release, and corrected publication. A publication verifies
the locale candidate against the pull's exact source revision and the released
application against its merge revision, preserving both sides of provenance.

# Versioned data-use commitments

Repository privacy engineering begins at
`/repositories/{repository}/data-commitments`. Writers define immutable terms
for repository, release, extension, experiment, and environment scopes; readers
see the current version and its full attributable history. Every permitted use
names its data categories, purposes, subjects, collection mechanism, processing,
recipients, retention, residency, deletion behavior, consent basis, and owners.
Each version must link both an applicable policy and a user-facing notice.

Guarantees are explicit as `supported`, `partial`, or `unsupported`; partial and
unsupported claims require a rationale and remain visible blockers. Missing
commitment or per-use ownership is also derived rather than hidden. Time-bounded
exceptions identify affected uses and guarantees, approver, reason, and expiry;
the reader flags them during their last 30 days and after expiry. When current
commitments overlap on the same scope, category, and subject but disagree on
purpose, retention, deletion, or consent, both retain a reciprocal conflict
blocker. These records explain permitted handling before implementation and do
not themselves authorize data access, collection, release, deployment, extension
execution, or experiment exposure. The repository web surface is `view=privacy`.

## Revision-exact data-flow maps

`/repositories/{repository}/data-flows` turns repository-defined flow manifests
into an inspectable map at one exact Git commit. A declaration captures the
manifest and code blob identities, cites exact commitment versions and data-use
IDs, and connects interactions, interfaces, packages, stores, extensions,
releases, environments, audiences, and external recipients with explicit entry,
movement, persistence, and exit edges. This makes every retained copy and
processor traceable from current project evidence rather than a detached
inventory.

Repository readers and read-only agents can append bounded findings with exact
code citations, observed edges, and uncertainty. The reader projection reports
new undeclared movement, category or purpose differences, declared paths that
were not observed, newer declarations that stale prior analysis, and dependency
evidence the viewer cannot inspect. Inaccessible evidence is represented only
by a typed bounded reference: its body is never copied into the data-flow store
or broader privacy workspace. Flow evidence is explanatory and grants no data,
source, extension, release, environment, credential, or operational authority.

## Synthetic runtime privacy verification

Repositories declare version-1 privacy checks in
`.komodo/privacy-checks.json`. Each check names synthetic journeys, exact code
and fixture inputs, applicable data-use commitments, retained sanitized
artifacts, and the behaviors it covers: collection, consent, minimization,
access, retention, export, deletion, telemetry, and recipients. The ordinary
check runner materializes only the candidate commit in a credential-free,
networkless sandbox. Privacy checks cannot inject environment secrets; their
logs and artifacts are sanitized before durable pull-request evidence is
published. Successful evidence can be reused only when every declared input
blob is unchanged, with the original run retained as explicit provenance.

Repository owners configure `/privacy-verification-policies` against an exact
data-use commitment version, target branches and paths, required check names,
coverage, and named privacy owners. A named owner acknowledges only an exact
pull-request preview revision. Readiness rejects missing, failed, stale, or
incomplete evidence and stale or rejected owner acknowledgement. A justified
exception is limited to named checks or dimensions, expires within 90 days, and
must link an issue, proposal, or task; expiry restores the blocker. Pull-request
readiness and merge use this assessment directly, and
`/releases/privacy-readiness` applies the same evidence to an exact release
candidate. These records demonstrate bounded runtime behavior but grant no
production data, preview, repository, merge, release, or operational authority.

## Pull-request privacy impact assessments

`/repositories/{repository}/pull-requests/{pull}/privacy-assessments` compares
one exact candidate with the pull request's exact target while behavior can
still change. Assessments classify changed collection, purposes, recipients,
retention, access, and user controls and link target/candidate flow maps,
commitment versions, and server-captured candidate source blobs. Explicit
requirements assign owners for acknowledgement, notices, consent changes,
migrations, tests, or exceptions instead of hiding follow-up in an approval.

Readers and scoped read-only agents may append revision-grounded challenges,
mitigations, and residual risk. Only a requirement's named owner can accept it
or request changes. A changed candidate or cited blob makes evidence and prior
acknowledgement stale and restores visible blockers. Privacy review grants no
data access, repository write, review, merge, release, or operational authority.

## Post-release privacy drift and correction

Repository owners define permitted production monitors at
`/repositories/{repository}/privacy-drift/monitors`. A monitor pins an exact
data-use commitment version and declared use to one release and revision,
environment, optional extension, responsible owners, notification participants,
allowed drift classes, and evidence-retention limit. This allowlist makes a
production-derived signal admissible; it does not authorize collection or
access.

Authorized collaborators report detections beneath `/privacy-drift/signals`.
The schema accepts only a permitted signal reference, aggregate metric and
count, bounded observation window, content digest, and sanitized summary for an
undeclared flow, excessive retention, failed deletion, consent mismatch, or
unexpected recipient. It has no field for a subject, raw event, payload,
contact detail, or attachment, and rejects evidence not explicitly marked
sanitized. Reads require repository write authority.

The append-only ledger inherits the responsible release, environment,
extension, and owners from the monitor and records containment, notifications
limited to named participants, private incident links, governed-exception
decisions, and resolution. A repair creates an ordinary proposal and human- or
approved-agent-owned task at the affected release revision. Its immutable
reasoning context retains the expected behavior, verification criteria, and
sanitized signal reference—not the raw production data—so later task and pull
request readers can reconstruct why the correction exists. Pull requests,
privacy verification, review, release, and deployment continue through their
existing contracts and may be cited back as ledger events. Revoking a connected
extension empties its effective authority without deleting the historical flow
or correction trail. No drift resource grants data, environment, extension,
credential, review, merge, release, or deployment authority.

# Developer support questions

`POST /repositories/{repository}/support-questions` opens an authenticated
support thread against a repository, package, release, API, documentation
journey, or error. The request records the developer's question and goal,
software version, environment, attempted steps, urgency, public or repository
audience, and none/thread/email contact preference. Missing version,
environment, and attempts are returned in `missing_context` and initially place
the thread in `needs_context`; they are not silently inferred.

Attachments are limited to sanitized logs, configuration, and sample code (ten
files, 1 MiB each and 5 MiB total) with either audience or maintainer-only
visibility. List responses omit all attachment content. Detail reads clear
maintainer-only content and email addresses unless the reader is the asker,
owner, or collaborator. Suggestions compare only visible question/goal/title
and issue summaries, never private attachment content. Detail, comment, status,
and suggestion routes retain attributable discussion and immutable history.
State defaults to `$SUPPORT_QUESTION_ROOT`; the repository workspace is
`view=support`.

An authenticated reader proposes an answer with `POST
/repositories/{repository}/support-questions/{question}/answers`. The first
revision creates an answer; later revisions provide its `answer_id` and exact
current `supersedes_id`, preventing a collaborator from silently overwriting a
newer correction. Each revision freezes its author and human/agent kind,
instructions, applicable versions, uncertainty, and claim-level evidence.
Claims distinguish `verified`, `inference`, and `uncertainty`; inference and
uncertainty require an explanation, and agent revisions additionally require
overall uncertainty.

Citations identify an exact revision and one of `source`, `symbol`,
`documentation`, `package`, `release`, `support_question`, or `issue`. Source
and symbol paths (and optional bounded lines) are resolved from the cited Git
commit. Other resources must exist and be visible to the contributor. Citation
visibility must equal the thread audience, which prevents maintainer-only or
otherwise inaccessible evidence from becoming implicit public support.
Participants use `POST .../answers/{answer}/feedback` to attach an endorsement,
challenge, clarification request, or comment to an exact answer revision and
optional claim. The complete revision and feedback history is returned on
thread detail and rendered in the Support workspace; it is collaborative
guidance, not execution or project authority.

Participants launch an immutable verification attempt with `POST
/repositories/{repository}/support-questions/{question}/verifications`. The
request binds one answer revision to an exact repository revision and
applicable software version, the thread's declared environment plus image
digest and bounded resources, exact dependencies, sanitized inputs, artifact
paths, and cost. Frozen answer instructions execute in a credential-free,
networkless Bubblewrap workspace. Reads retain commands, redacted stdout and
stderr, outputs, content-addressed artifacts, result, actor, and cost. `POST
.../verifications/{verification}/reruns` lets another participant replay the
exact record. Reads mark evidence stale after answer/instruction, source,
software, environment image, dependency, or input changes. Credential-like
inputs and artifacts are rejected, detected log content is redacted, and the
workflow grants no repository, environment, credential, or operational
authority. Launch and evidence history are also available in `view=support`.

The complete developer-support contract is exercised by
`support_workflow_test.go`. A public package user and maintainer refine a
version-bound integration question, an agent publishes source-cited guidance,
a failed attempt is superseded by clean verification, and the accepted answer
becomes searchable project knowledge. A duplicate is merged without erasing
its trail. A separate missing-example question creates dependent agent and
human proposal tasks; stock Git, a required check, owner review, merge, and a
release deliver the code-and-documentation repair before updated guidance is
verified and the asker sees their projected notification. Maintainer-only
evidence remains absent from anonymous reads throughout.

# Public agent profiles

`GET /agent-profiles` and `GET /agent-profiles/{id}` are public catalog reads.
An authenticated operator publishes with `POST /agent-profiles` and adds an
optimistically concurrency-checked immutable revision at
`POST /agent-profiles/{id}/versions`. Profiles disclose operator ownership,
supported work and tools, exact model and execution provenance, context use and
retention, subprocessors, remote boundaries, cost/resources, requested
capabilities, availability, support, and change history. Stable IDs are
server-generated and handles cannot collide with human or agent identities.
The response deliberately separates `platform_verified_evidence` from
unverified operator claims and declares that the catalog grants no authority.
State defaults to `$AGENT_PROFILE_ROOT`; repositories expose the reader and
publisher workspace at `view=agents`.

## Explainable agent discovery

Repository writers attach attributable evaluation or delivered-outcome
observations with `POST /repositories/{repository}/agent-discovery/evidence`.
Each observation names an exact agent profile version, comparable workflow and
tags, source reference, result, observed cost and time, conflict of interest,
and either `public` or `repository` audience. These are platform-retained
observations, not a claim that the underlying result was independently proven.

Repository readers create a comparison at
`POST /repositories/{repository}/agent-discovery/searches`, binding one task,
proposal, issue, decision, incident, stewardship mandate, or delivery-team role
to explicit workflow, permitted capabilities, deployment boundaries, policy
terms, price ceiling, availability, and comparable-work tags. The response is
alphabetical and has no aggregate score: every profile exposes a match or
conflict explanation for each constraint plus accessible evaluations, outcomes,
staleness, missing evidence, and conflicts of interest. Repository readers use
the repository-scoped GET route. Public comparisons are shareable at
`GET /agent-searches/{id}`; that projection removes the private work identifier,
creator, repository-only evidence, and even private evidence's effect on the
reported gaps. Discovery grants no repository or operational authority. State
defaults to `$AGENT_DISCOVERY_ROOT` and the workflow appears at `view=agents`.

## Bounded project agent evaluations

Repository writers define suites at
`/repositories/{repository}/agent-evaluations/suites`. Every immutable version
freezes sanitized representative inputs at exact repository revisions,
expected outcomes, correctness and policy checks, human-review criteria,
prohibited actions, and cost, latency, and tool-action budgets. Hidden checks
may retain private expectations and contamination canaries in the suite store,
but repository reader responses replace their descriptions and omit both the
expectation and canary.

Writers start an exact suite/profile version through `/trials`. The resulting
session contract is always isolated with publish, secret, merge, and environment
authority false. Completion retains scenario outputs, tool actions, artifact
digests, check summaries, cost, latency, runtime failure, and reproducibility
notes. The service derives budget overruns, prohibited-action attempts, and
hidden-canary contamination and preserves attributable accept, reject, or
needs-review decisions. An input digest labels duplicate runs as repeated;
operator-supplied inputs and explicit reproductions receive their own labels so
they cannot appear to be independent first-party proof. Records live beneath
`$AGENT_EVALUATION_ROOT` and are presented in `view=agents`; evaluation evidence
does not itself approve, install, credential, or authorize the candidate.
