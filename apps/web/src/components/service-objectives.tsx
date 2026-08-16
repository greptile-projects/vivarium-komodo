/* eslint-disable react-hooks/set-state-in-effect */
"use client";
import { useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";
type V = {
  number: number;
  title: string;
  description: string;
  scopes: { kind: string; resource_id?: string; name: string }[];
  indicators: {
    id: string;
    name: string;
    signal: string;
    signal_status: string;
    calculation: string;
    unit: string;
  }[];
  measurement_windows: { id: string; duration: string }[];
  targets: {
    indicator_id: string;
    window_id: string;
    comparator: string;
    value: number;
    error_budget_percent: number;
  }[];
  journeys: {
    id: string;
    name: string;
    behavior: string;
    owner_ids: string[];
  }[];
  dependencies: {
    id: string;
    name: string;
    required: boolean;
    owner_ids: string[];
  }[];
  severity_thresholds: {
    level: string;
    budget_consumed_percent: number;
    response: string;
    owner_ids: string[];
  }[];
  owner_ids: string[];
  commitment_links: {
    kind: string;
    resource_id: string;
    label: string;
    status: string;
  }[];
  exceptions: {
    id: string;
    reason: string;
    approved_by: string;
    owner_id: string;
    expires_at: string;
  }[];
  exception_policy: string;
  change_reason: string;
  author_id: string;
};
type O = {
  id: string;
  current_version: number;
  versions: V[];
  blockers: { kind: string; detail: string }[];
  signal_mappings: {
    id: string;
    current_version: number;
    versions: {
      number: number;
      objective_version: number;
      indicator_id: string;
      window_id: string;
      instrumentation_revision: string;
      sources: { id: string; kind: string; name: string }[];
      author_id: string;
    }[];
  }[];
  attainment_history: {
    id: string;
    indicator_id: string;
    window_id: string;
    objective_version: number;
    mapping_version: number;
    instrumentation_revision: string;
    window_start: string;
    window_end: string;
    value: number;
    error_budget_consumed_percent: number;
    uncertainty: string;
    comparable_to_previous: boolean;
    status: string;
    gap_kinds: string[];
    audience: string;
    evidence: {
      kind: string;
      resource_id: string;
      revision: string;
      label: string;
    }[];
  }[];
};
type Policy = {
  id: string;
  name: string;
  objective_id: string;
  objective_version: number;
  environments?: string[];
  required_owner_ids: string[];
  rules: { condition: string; action: string }[];
};
type Investigation = {
  id: string;
  title: string;
  objective_id: string;
  objective_version: number;
  revision: string;
  question: string;
  participants: string[];
  blockers: string[];
  entries: {
    id: string;
    kind: string;
    body: string;
    verdict?: string;
    actor_id: string;
    uncertainty?: string;
  }[];
  input_requests: {
    owner_id: string;
    owner_kind: string;
    question: string;
    status: string;
  }[];
};
type Improvement = {
  id: string;
  title: string;
  objective_id: string;
  objective_version: number;
  proposal_id: string;
  state: string;
  budget_state: string;
  affected_revisions: string[];
  dependency_context: string[];
  delivery_links: {
    kind: string;
    resource_id: string;
    revision: string;
    summary: string;
  }[];
  rollouts: {
    id: string;
    stage: string;
    state: string;
    required_action: string;
    deployment_id: string;
    revision: string;
    rationale: string;
    measurements: {
      indicator: string;
      value: number;
      unit: string;
      passed: boolean;
    }[];
  }[];
};
const lines = (x: FormDataEntryValue | null) =>
    String(x || "")
      .split("\n")
      .map((v) => v.trim())
      .filter(Boolean),
  rows = (x: FormDataEntryValue | null) =>
    lines(x).map((v) => v.split("|").map((y) => y.trim())),
  csv = (x: string) =>
    x
      .split(",")
      .map((v) => v.trim())
      .filter(Boolean);
export function ServiceObjectives({
  repository,
  actor,
}: {
  repository: string;
  actor: string;
}) {
  const root = `/api/repositories/${repository}/service-objectives`,
    [items, setItems] = useState<O[]>([]),
    [policies, setPolicies] = useState<Policy[]>([]),
    [investigations, setInvestigations] = useState<Investigation[]>([]),
    [improvements, setImprovements] = useState<Improvement[]>([]),
    [error, setError] = useState("");
  const load = useCallback(async () => {
    const base = `/api/repositories/${repository}`;
    const [
      objectiveResponse,
      policyResponse,
      investigationResponse,
      improvementResponse,
    ] = await Promise.all([
      fetch(root),
      fetch(`${base}/reliability-delivery-policies`),
      fetch(`${base}/reliability-investigations`),
      fetch(`${base}/reliability-improvements`),
    ]);
    if (objectiveResponse.ok)
      setItems(((await objectiveResponse.json()) as { items: O[] }).items);
    if (policyResponse.ok)
      setPolicies(((await policyResponse.json()) as { items: Policy[] }).items);
    if (investigationResponse.ok)
      setInvestigations(
        ((await investigationResponse.json()) as { items: Investigation[] })
          .items,
      );
    if (improvementResponse.ok)
      setImprovements(
        ((await improvementResponse.json()) as { items: Improvement[] }).items,
      );
  }, [repository, root]);
  useEffect(() => {
    void load();
  }, [load]);
  async function save(path: string, body: unknown) {
    const r = await fetch(root + path, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!r.ok) {
      setError(((await r.json()) as { error: string }).error);
      return;
    }
    setError("");
    await load();
  }
  return (
    <section className="investigation-workspace">
      <div className="section-heading">
        <div>
          <span className="eyebrow">
            Journey → indicator → objective → accountable response
          </span>
          <h2>Service objectives</h2>
          <p>
            Define which user-visible behavior must remain dependable, how it is
            measured, its error budget, and who acts when reliability is at
            risk.
          </p>
        </div>
        <Badge>{items.length} objectives</Badge>
      </div>
      <details className="panel">
        <summary>Define a service objective</summary>
        <Form actor={actor} submit={(x) => save("", x)} />
      </details>
      {error && <p className="form-error">{error.replaceAll("_", " ")}</p>}
      {items.map((x) => {
        const v = x.versions.at(-1)!;
        return (
          <article className="panel" key={x.id}>
            <header>
              <div>
                <span className="eyebrow">
                  version {v.number} · {v.author_id}
                </span>
                <h3>{v.title}</h3>
                <p>{v.description}</p>
              </div>
              <Badge>
                {x.blockers.length
                  ? `${x.blockers.length} explicit blockers`
                  : "contract supported"}
              </Badge>
            </header>
            {x.blockers.map((b, i) => (
              <p className="form-error" key={i}>
                <strong>{b.kind.replaceAll("_", " ")}:</strong> {b.detail}
              </p>
            ))}
            <p>
              <strong>Owners:</strong> {v.owner_ids.join(", ") || "missing"}
            </p>
            <h4>User journeys</h4>
            {v.journeys.map((j) => (
              <p key={j.id}>
                <Badge>{j.id}</Badge> <strong>{j.name}</strong> — {j.behavior} ·{" "}
                {j.owner_ids.join(", ") || "missing owner"}
              </p>
            ))}
            <h4>Indicators, windows, and budgets</h4>
            {v.targets.map((t, i) => {
              const q = v.indicators.find((y) => y.id === t.indicator_id),
                w = v.measurement_windows.find((y) => y.id === t.window_id);
              return (
                <p key={i}>
                  <Badge>{q?.signal_status}</Badge> {q?.name}:{" "}
                  <code>{q?.signal}</code> · {q?.calculation} {t.comparator}{" "}
                  {t.value} {q?.unit} / {w?.duration} · {t.error_budget_percent}
                  % budget
                </p>
              );
            })}
            <h4>Dependencies and severity</h4>
            {v.dependencies.map((d) => (
              <p key={d.id}>
                <Badge>{d.required ? "required" : "supporting"}</Badge> {d.name}{" "}
                · {d.owner_ids.join(", ") || "missing owner"}
              </p>
            ))}
            {v.severity_thresholds.map((s) => (
              <p key={s.level}>
                <Badge>{s.level}</Badge> {s.budget_consumed_percent}% consumed:{" "}
                {s.response} · {s.owner_ids.join(", ")}
              </p>
            ))}
            <h4>Revision-exact operational evidence</h4>
            {!x.signal_mappings?.length && <p>No signal mapping published.</p>}
            {x.signal_mappings?.map((m) => {
              const q = m.versions.at(-1)!;
              return (
                <p key={m.id}>
                  <Badge>mapping v{q.number}</Badge> {q.indicator_id} /{" "}
                  {q.window_id} · instrumentation{" "}
                  <code>{q.instrumentation_revision}</code> ·{" "}
                  {q.sources.map((s) => `${s.kind}: ${s.name}`).join(", ")}
                </p>
              );
            })}
            {!x.attainment_history?.length && (
              <p>No sanitized attainment recorded.</p>
            )}
            {x.attainment_history?.map((a) => (
              <div key={a.id} className="timeline-card">
                <p>
                  <Badge>{a.status}</Badge> <strong>{a.value}</strong> ·{" "}
                  {a.error_budget_consumed_percent}% error budget consumed ·{" "}
                  {new Date(a.window_start).toLocaleDateString()}–
                  {new Date(a.window_end).toLocaleDateString()}
                </p>
                <p>
                  Objective v{a.objective_version}, mapping v{a.mapping_version}
                  , instrumentation <code>{a.instrumentation_revision}</code> ·{" "}
                  {a.audience} evidence
                </p>
                {a.uncertainty && (
                  <p>
                    <strong>Uncertainty:</strong> {a.uncertainty}
                  </p>
                )}
                {a.gap_kinds.map((g) => (
                  <p className="form-error" key={g}>
                    {g.replaceAll("_", " ")}
                  </p>
                ))}
                {a.evidence.map((e, i) => (
                  <p key={i}>
                    <Badge>{e.kind}</Badge> {e.label} ·{" "}
                    <code>
                      {e.resource_id}@{e.revision}
                    </code>
                  </p>
                ))}
              </div>
            ))}
            <h4>Linked commitments</h4>
            {v.commitment_links.map((l, i) => (
              <p key={i}>
                <Badge>{l.status}</Badge> {l.kind}: {l.label} ·{" "}
                <code>{l.resource_id}</code>
              </p>
            ))}
            <h4>Exception policy</h4>
            <p>{v.exception_policy}</p>
            {v.exceptions.map((e) => (
              <p key={e.id}>
                <Badge>{e.id}</Badge> {e.reason} · owner {e.owner_id} · approved{" "}
                {e.approved_by} · expires{" "}
                {new Date(e.expires_at).toLocaleDateString()}
              </p>
            ))}
            <details>
              <summary>Publish a revised immutable version</summary>
              <Form
                actor={actor}
                initial={v}
                submit={(body) =>
                  save(`/${x.id}/versions`, {
                    expected_version: x.current_version,
                    ...body,
                  })
                }
              />
            </details>
            <details>
              <summary>Version history</summary>
              {x.versions.map((y) => (
                <p key={y.number}>
                  Version {y.number} by {y.author_id} · {y.change_reason}
                </p>
              ))}
            </details>
          </article>
        );
      })}
      <div className="section-heading">
        <div>
          <span className="eyebrow">
            Burn → investigation → governed repair → recovery
          </span>
          <h2>Reliability stewardship trail</h2>
          <p>
            Inspect how current delivery policy, revision-exact human-agent
            diagnosis, ordinary reviewed work, and staged recovery remain
            connected.
          </p>
        </div>
        <Badge>
          {investigations.length + improvements.length} retained records
        </Badge>
      </div>
      {policies.map((policy) => (
        <article className="panel" key={policy.id}>
          <header>
            <div>
              <span className="eyebrow">
                delivery policy · objective v{policy.objective_version}
              </span>
              <h3>{policy.name}</h3>
            </div>
            <Badge>
              {policy.environments?.join(", ") || "all environments"}
            </Badge>
          </header>
          <p>
            <strong>Required owners:</strong>{" "}
            {policy.required_owner_ids.join(", ")}
          </p>
          {policy.rules.map((rule, index) => (
            <p key={index}>
              <Badge>{rule.action}</Badge> {rule.condition.replaceAll("_", " ")}
            </p>
          ))}
        </article>
      ))}
      {investigations.map((investigation) => (
        <article className="panel" key={investigation.id}>
          <header>
            <div>
              <span className="eyebrow">
                investigation · objective v{investigation.objective_version} ·{" "}
                <code>{investigation.revision}</code>
              </span>
              <h3>{investigation.title}</h3>
              <p>{investigation.question}</p>
            </div>
            <Badge>{investigation.participants.length} participants</Badge>
          </header>
          {investigation.blockers.map((blocker) => (
            <p className="form-error" key={blocker}>
              {blocker.replaceAll("_", " ")}
            </p>
          ))}
          {investigation.input_requests.map((request, index) => (
            <p key={index}>
              <Badge>{request.status}</Badge> {request.owner_kind} owner{" "}
              {request.owner_id}: {request.question}
            </p>
          ))}
          {investigation.entries.map((entry) => (
            <div className="timeline-card" key={entry.id}>
              <p>
                <Badge>{entry.verdict || entry.kind}</Badge>{" "}
                <strong>{entry.actor_id}</strong>: {entry.body}
              </p>
              {entry.uncertainty && (
                <p>
                  <strong>Uncertainty:</strong> {entry.uncertainty}
                </p>
              )}
            </div>
          ))}
        </article>
      ))}
      {improvements.map((improvement) => (
        <article className="panel" key={improvement.id}>
          <header>
            <div>
              <span className="eyebrow">
                accountable improvement · objective v
                {improvement.objective_version}
              </span>
              <h3>{improvement.title}</h3>
              <p>
                Proposal <code>{improvement.proposal_id}</code> · affected{" "}
                {improvement.affected_revisions.join(", ")}
              </p>
            </div>
            <Badge>{improvement.budget_state} budget</Badge>
          </header>
          <p>
            <strong>Dependency context:</strong>{" "}
            {improvement.dependency_context.join("; ")}
          </p>
          {improvement.delivery_links.map((link, index) => (
            <p key={index}>
              <Badge>{link.kind}</Badge> {link.summary} ·{" "}
              <code>
                {link.resource_id}@{link.revision}
              </code>
            </p>
          ))}
          {improvement.rollouts.map((rollout) => (
            <div className="timeline-card" key={rollout.id}>
              <p>
                <Badge>{rollout.state}</Badge> <strong>{rollout.stage}</strong>{" "}
                · {rollout.required_action.replaceAll("_", " ")} ·{" "}
                <code>
                  {rollout.deployment_id}@{rollout.revision}
                </code>
              </p>
              <p>{rollout.rationale}</p>
              {rollout.measurements.map((measurement, index) => (
                <p key={index}>
                  {measurement.indicator}: {measurement.value}{" "}
                  {measurement.unit} ·{" "}
                  {measurement.passed ? "passed" : "failed"}
                </p>
              ))}
            </div>
          ))}
        </article>
      ))}
    </section>
  );
}
function Form({
  actor,
  initial,
  submit,
}: {
  actor: string;
  initial?: V;
  submit: (x: Record<string, unknown>) => Promise<void>;
}) {
  function terms(f: FormData) {
    return {
      title: f.get("title"),
      description: f.get("description"),
      scopes: rows(f.get("scopes")).map(([kind, resource_id, name]) => ({
        kind,
        resource_id,
        name,
      })),
      indicators: rows(f.get("indicators")).map(
        ([
          id,
          name,
          description,
          signal,
          signal_status,
          calculation,
          unit,
          good_event,
          total_event,
        ]) => ({
          id,
          name,
          description,
          signal,
          signal_status,
          calculation,
          unit,
          good_event,
          total_event,
        }),
      ),
      measurement_windows: rows(f.get("windows")).map(
        ([id, kind, duration, alignment]) => ({
          id,
          kind,
          duration,
          alignment,
        }),
      ),
      targets: rows(f.get("targets")).map(
        ([indicator_id, window_id, comparator, value, budget]) => ({
          indicator_id,
          window_id,
          comparator,
          value: Number(value),
          error_budget_percent: Number(budget),
        }),
      ),
      journeys: rows(f.get("journeys")).map(([id, name, behavior, owners]) => ({
        id,
        name,
        behavior,
        owner_ids: csv(owners),
      })),
      dependencies: rows(f.get("dependencies")).map(
        ([id, name, kind, required, owners]) => ({
          id,
          name,
          kind,
          required: required === "true",
          owner_ids: csv(owners),
        }),
      ),
      severity_thresholds: rows(f.get("severities")).map(
        ([level, budget, response, owners]) => ({
          level,
          budget_consumed_percent: Number(budget),
          response,
          owner_ids: csv(owners),
        }),
      ),
      owner_ids: lines(f.get("owners")),
      commitment_links: rows(f.get("links")).map(
        ([kind, resource_id, label, status]) => ({
          kind,
          resource_id,
          label,
          status,
        }),
      ),
      exceptions: rows(f.get("exceptions")).map(
        ([id, reason, approved_by, owner_id, expires_at]) => ({
          id,
          reason,
          approved_by,
          owner_id,
          expires_at: new Date(expires_at).toISOString(),
        }),
      ),
      exception_policy: f.get("policy"),
      change_reason: f.get("reason"),
    };
  }
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        void submit(terms(new FormData(e.currentTarget)));
      }}
    >
      <label>
        Title
        <input name="title" defaultValue={initial?.title} required />
      </label>
      <label>
        User-visible promise
        <textarea
          name="description"
          defaultValue={initial?.description}
          required
        />
      </label>
      <label>
        Scopes: kind | resource ID | name
        <textarea
          name="scopes"
          defaultValue={initial?.scopes
            .map((x) => `${x.kind} | ${x.resource_id || ""} | ${x.name}`)
            .join("\n")}
          required
        />
      </label>
      <label>
        Indicators: ID | name | description | signal | available/missing |
        calculation | unit | good event | total event
        <textarea name="indicators" required />
      </label>
      <label>
        Windows: ID | rolling/calendar | duration | alignment
        <textarea name="windows" required />
      </label>
      <label>
        Targets: indicator | window | gte/lte | value | error budget %
        <textarea name="targets" required />
      </label>
      <label>
        Journeys: ID | name | behavior | owners
        <textarea name="journeys" required />
      </label>
      <label>
        Dependencies: ID | name | kind | true/false required | owners
        <textarea name="dependencies" />
      </label>
      <label>
        Severity: warning/critical/exhausted | consumed % | response | owners
        <textarea name="severities" required />
      </label>
      <label>
        Owners
        <textarea
          name="owners"
          defaultValue={initial?.owner_ids.join("\n") || actor}
        />
      </label>
      <label>
        Commitments: kind | resource ID | label | linked/missing
        <textarea name="links" />
      </label>
      <label>
        Exception policy
        <textarea
          name="policy"
          defaultValue={initial?.exception_policy}
          required
        />
      </label>
      <label>
        Exceptions: ID | reason | approver | owner | expiry
        <textarea name="exceptions" />
      </label>
      <label>
        Version reason
        <textarea name="reason" required />
      </label>
      <Button type="submit">Publish service objective</Button>
    </form>
  );
}
