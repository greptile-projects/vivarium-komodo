"use client";
import { useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";
type Signal = {
  id: string;
  current_version: number;
  versions: Array<{
    name: string;
    unit: string;
    instrumented: boolean;
    permitted_audiences: string[];
  }>;
};
type Experiment = {
  id: string;
  current_version: number;
  ready: boolean;
  versions: Array<{
    title: string;
    hypothesis: string;
    variants: Array<{ id: string; name: string; control: boolean }>;
    target_audience: { description: string };
    measures: Array<{ name: string; kind: string; threshold: string }>;
    minimum_evidence: string;
    duration_hours: number;
    owner_ids: string[];
    stop_conditions: string[];
  }>;
  blockers: Array<{ kind: string; detail: string }>;
  comments: Array<{ id: string; body: string; author_id: string }>;
  approvals: Array<{
    id: string;
    actor_id: string;
    decision: string;
    version: number;
  }>;
  audience_policies: Array<{
    id: string;
    version: number;
    ready: boolean;
    release_id: string;
    release_commit_id: string;
    mutual_exclusion_group: string;
    eligibility: {
      consent_class: string;
      regions: string[];
      organization_ids: string[];
      required_attributes: string[];
      excluded_attributes: string[];
    };
    allocation: Array<{ variant_id: string; basis_points: number }>;
    collection: Array<{
      signal_id: string;
      signal_version: number;
      properties: string[];
    }>;
    retention_days: number;
    blockers: Array<{ kind: string; detail: string }>;
    assignment_audits: Array<{
      id: string;
      subject_digest: string;
      variant_id?: string;
      decision: string;
      reason: string;
    }>;
  }>;
  runs: Array<{
    id: string;
    status: string;
    current_stage: number;
    environment_id: string;
    containment_reason?: string;
    stages: Array<{
      name: string;
      max_exposure: number;
      allocation: Array<{ variant_id: string; basis_points: number }>;
    }>;
    observations: Array<{
      id: string;
      exposure_by_variant: Record<string, number>;
      measure_values: Record<string, number>;
      uncertainty: Record<string, number>;
      data_quality: string;
      operational_health: string;
      instrumentation_health: string;
      consent_health: string;
      cost_units: number;
      containment_triggered: boolean;
    }>;
  }>;
};
export function ProductExperiments({
  repository,
  actor,
}: {
  repository: string;
  actor: string;
}) {
  const [items, setItems] = useState<Experiment[]>([]),
    [signals, setSignals] = useState<Signal[]>([]),
    [error, setError] = useState("");
  const base = `/api/repositories/${repository}/product-experiments`;
  const load = useCallback(async () => {
    const [a, b] = await Promise.all([fetch(base), fetch(`${base}/signals`)]);
    if (a.ok) setItems((await a.json()).items);
    if (b.ok) setSignals((await b.json()).items);
  }, [base]);
  useEffect(() => {
    const timer = setTimeout(() => void load(), 0);
    return () => clearTimeout(timer);
  }, [load]);
  async function post(path: string, body: unknown) {
    setError("");
    const r = await fetch(base + path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!r.ok) {
      setError((await r.json()).error);
      return;
    }
    await load();
  }
  return (
    <section>
      <header className="section-heading">
        <div>
          <p className="eyebrow">Pre-exposure contract</p>
          <h2>Product experiments</h2>
          <p>
            Agree what a test proves, what harm stops it, and who owns it before
            anyone receives a variant.
          </p>
        </div>
      </header>
      {error && (
        <p role="alert" className="form-error">
          {error}
        </p>
      )}
      <details className="card">
        <summary>Register a permitted product signal</summary>
        <form
          className="repository-form"
          onSubmit={(e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget);
            void post("/signals", {
              name: f.get("name"),
              description: f.get("description"),
              unit: f.get("unit"),
              event: f.get("event"),
              properties: String(f.get("properties"))
                .split("\n")
                .filter(Boolean),
              permitted_audiences: String(f.get("audiences"))
                .split("\n")
                .filter(Boolean),
              instrumented: f.get("instrumented") === "on",
              change_reason: f.get("reason"),
            });
          }}
        >
          <label>
            Name
            <input name="name" required />
          </label>
          <label>
            Unit
            <input name="unit" required />
          </label>
          <label>
            Versioned event
            <input name="event" required />
          </label>
          <label className="wide">
            Definition
            <textarea name="description" required />
          </label>
          <label>
            Permitted consent classes
            <textarea
              name="audiences"
              defaultValue="product_analytics"
              required
            />
          </label>
          <label>
            Permitted properties
            <textarea name="properties" />
          </label>
          <label>
            <input name="instrumented" type="checkbox" /> Instrumentation is
            available
          </label>
          <label>
            Change reason
            <input name="reason" required />
          </label>
          <Button type="submit">Register signal</Button>
        </form>
      </details>
      <details className="card">
        <summary>Open a product experiment</summary>
        <form
          className="repository-form"
          onSubmit={(e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget),
              signal = String(f.get("signal"));
            void post("", {
              title: f.get("title"),
              source: { kind: f.get("source_kind"), id: f.get("source_id") },
              hypothesis: f.get("hypothesis"),
              variants: [
                { id: "control", name: f.get("control"), control: true },
                { id: "variant", name: f.get("variant"), control: false },
              ],
              target_audience: {
                description: f.get("audience"),
                eligibility: String(f.get("eligibility"))
                  .split("\n")
                  .filter(Boolean),
                exclusions: String(f.get("exclusions"))
                  .split("\n")
                  .filter(Boolean),
                consent: f.get("consent"),
                estimated_size: Number(f.get("size")),
              },
              measures: [
                {
                  id: "success",
                  name: "Success",
                  kind: "success",
                  signal_id: signal,
                  signal_version: Number(f.get("signal_version")),
                  aggregation: f.get("success_aggregation"),
                  threshold: f.get("success_threshold"),
                },
                {
                  id: "guardrail",
                  name: "Guardrail",
                  kind: "guardrail",
                  signal_id: signal,
                  signal_version: Number(f.get("signal_version")),
                  aggregation: f.get("guardrail_aggregation"),
                  threshold: f.get("guardrail_threshold"),
                },
              ],
              minimum_evidence: f.get("evidence"),
              duration_hours: Number(f.get("duration")),
              owner_ids: String(f.get("owners")).split("\n").filter(Boolean),
              participant_ids: String(f.get("participants"))
                .split("\n")
                .filter(Boolean),
              stop_conditions: String(f.get("stops"))
                .split("\n")
                .filter(Boolean),
              assumptions: String(f.get("assumptions"))
                .split("\n")
                .filter(Boolean),
              overlap_keys: String(f.get("overlaps"))
                .split("\n")
                .filter(Boolean),
              change_reason: "Initial shared plan",
            });
          }}
        >
          <label>
            Title
            <input name="title" required />
          </label>
          <label>
            Origin
            <select name="source_kind">
              <option>proposal</option>
              <option>issue</option>
              <option>decision</option>
              <option value="pull_request">pull request</option>
              <option>preview</option>
              <option>release</option>
            </select>
          </label>
          <label>
            Origin ID
            <input name="source_id" required />
          </label>
          <label className="wide">
            Testable hypothesis
            <textarea name="hypothesis" required />
          </label>
          <label>
            Control
            <input name="control" required />
          </label>
          <label>
            Variant
            <input name="variant" required />
          </label>
          <label>
            Audience
            <input name="audience" required />
          </label>
          <label>
            Consent class
            <input name="consent" defaultValue="product_analytics" required />
          </label>
          <label>
            Eligibility
            <textarea name="eligibility" required />
          </label>
          <label>
            Exclusions
            <textarea name="exclusions" />
          </label>
          <label>
            Estimated size
            <input name="size" type="number" min="1" required />
          </label>
          <label>
            Product signal
            <select name="signal" required>
              {signals.map((s) => (
                <option value={s.id} key={s.id}>
                  {s.versions.at(-1)?.name}
                </option>
              ))}
            </select>
          </label>
          <input
            type="hidden"
            name="signal_version"
            value={signals[0]?.current_version || 1}
          />
          <label>
            Success aggregation
            <input name="success_aggregation" required />
          </label>
          <label>
            Success threshold
            <input name="success_threshold" required />
          </label>
          <label>
            Guardrail aggregation
            <input name="guardrail_aggregation" required />
          </label>
          <label>
            Guardrail threshold
            <input name="guardrail_threshold" required />
          </label>
          <label>
            Minimum evidence
            <input name="evidence" required />
          </label>
          <label>
            Duration hours
            <input name="duration" type="number" min="1" required />
          </label>
          <label>
            Owners
            <textarea name="owners" defaultValue={actor} required />
          </label>
          <label>
            Approving participants
            <textarea name="participants" defaultValue={actor} required />
          </label>
          <label>
            Stop conditions
            <textarea name="stops" required />
          </label>
          <label>
            Assumptions
            <textarea name="assumptions" />
          </label>
          <label>
            Overlap keys
            <textarea name="overlaps" placeholder="surface:audience" />
          </label>
          <Button type="submit">Open experiment plan</Button>
        </form>
      </details>
      {items.map((x) => {
        const p = x.versions.at(-1)!;
        return (
          <article className="card" key={x.id}>
            <p>
              <Badge tone={x.ready ? "accent" : "warning"}>
                {x.ready ? "approved to build" : "blocked before exposure"}
              </Badge>{" "}
              plan v{x.current_version}
            </p>
            <h3>{p.title}</h3>
            <p>{p.hypothesis}</p>
            <p>
              <strong>Audience:</strong> {p.target_audience.description} ·{" "}
              {p.duration_hours} hours
            </p>
            <p>
              <strong>Variants:</strong>{" "}
              {p.variants
                .map((v) => `${v.name}${v.control ? " (control)" : ""}`)
                .join(" · ")}
            </p>
            <p>
              <strong>Minimum evidence:</strong> {p.minimum_evidence}
            </p>
            <p>
              <strong>Stop immediately:</strong> {p.stop_conditions.join(" · ")}
            </p>
            {x.blockers.map((b, i) => (
              <p className="form-error" key={i}>
                {b.kind.replaceAll("_", " ")}: {b.detail}
              </p>
            ))}
            {x.audience_policies?.map((a) => (
              <section key={a.id}>
                <p>
                  <Badge tone={a.ready ? "accent" : "warning"}>
                    {a.ready ? "audience approved" : "allocation blocked"}
                  </Badge>{" "}
                  audience v{a.version}
                </p>
                <p>
                  <strong>Exact release:</strong> {a.release_id} ·{" "}
                  {a.release_commit_id}
                </p>
                <p>
                  <strong>Eligibility:</strong> consent{" "}
                  {a.eligibility.consent_class}; regions{" "}
                  {a.eligibility.regions.join(", ") || "any"}; organizations{" "}
                  {a.eligibility.organization_ids.join(", ") || "any"}
                </p>
                <p>
                  <strong>Allocation:</strong>{" "}
                  {a.allocation
                    .map((v) => `${v.variant_id} ${v.basis_points / 100}%`)
                    .join(" · ")}{" "}
                  · exclusive group {a.mutual_exclusion_group}
                </p>
                <p>
                  <strong>Collected evidence:</strong>{" "}
                  {a.collection.flatMap((c) => c.properties).join(", ")} ·
                  retained {a.retention_days} days
                </p>
                {a.blockers.map((b, i) => (
                  <p className="form-error" key={i}>
                    {b.kind.replaceAll("_", " ")}: {b.detail}
                  </p>
                ))}
                <p>
                  {a.assignment_audits.length} pseudonymous assignment decisions
                  retained; sensitive membership is not displayed.
                </p>
              </section>
            ))}
            {x.runs?.map((run) => {
              const latest = run.observations.at(-1);
              return (
                <section className="experiment-live" key={run.id}>
                  <p>
                    <Badge
                      tone={run.status === "running" ? "accent" : "warning"}
                    >
                      {run.status}
                    </Badge>{" "}
                    live attempt · {run.environment_id}
                  </p>
                  <p>
                    <strong>Stage:</strong>{" "}
                    {run.stages[run.current_stage - 1]?.name} · up to{" "}
                    {run.stages[run.current_stage - 1]?.max_exposure / 100}%
                    exposure
                  </p>
                  {run.containment_reason && (
                    <p className="form-error">
                      Contained: {run.containment_reason}. Existing assignments
                      and evidence were retained.
                    </p>
                  )}
                  {latest && (
                    <>
                      <p>
                        <strong>Exposure:</strong>{" "}
                        {Object.entries(latest.exposure_by_variant)
                          .map(([k, v]) => `${k} ${v}`)
                          .join(" · ")}
                      </p>
                      <p>
                        <strong>Evidence:</strong>{" "}
                        {Object.entries(latest.measure_values)
                          .map(
                            ([k, v]) =>
                              `${k} ${v} ± ${latest.uncertainty[k] ?? "?"}`,
                          )
                          .join(" · ")}{" "}
                        · cost {latest.cost_units}
                      </p>
                      <p>
                        <strong>Health:</strong> data {latest.data_quality} ·
                        instrumentation {latest.instrumentation_health} ·
                        operations {latest.operational_health} · consent{" "}
                        {latest.consent_health}
                      </p>
                    </>
                  )}
                  <div className="form-actions">
                    {run.status === "running" && (
                      <Button
                        variant="secondary"
                        onClick={() =>
                          void post(`/${x.id}/runs/${run.id}/controls`, {
                            action: "pause",
                            reason: "Participant paused live exposure",
                          })
                        }
                      >
                        Pause
                      </Button>
                    )}
                    {run.status === "paused" && (
                      <Button
                        onClick={() =>
                          void post(`/${x.id}/runs/${run.id}/controls`, {
                            action: "resume",
                            reason: "Participant resumed after review",
                          })
                        }
                      >
                        Resume
                      </Button>
                    )}
                    {(run.status === "running" || run.status === "paused") && (
                      <Button
                        variant="secondary"
                        onClick={() =>
                          void post(`/${x.id}/runs/${run.id}/controls`, {
                            action: "stop",
                            reason: "Participant ended this attempt",
                          })
                        }
                      >
                        Stop
                      </Button>
                    )}
                  </div>
                </section>
              );
            })}
            <form
              className="form-stack"
              onSubmit={(e) => {
                e.preventDefault();
                const f = new FormData(e.currentTarget);
                void post(`/${x.id}/comments`, { body: f.get("body") });
              }}
            >
              <label>
                Discuss the plan
                <textarea name="body" required />
              </label>
              <Button variant="secondary" type="submit">
                Comment
              </Button>
            </form>
            {x.comments.map((c) => (
              <blockquote key={c.id}>
                {c.body}
                <footer>{c.author_id}</footer>
              </blockquote>
            ))}
            <div className="form-actions">
              <Button
                onClick={() =>
                  void post(`/${x.id}/approvals`, {
                    decision: "approved",
                    note: "Current plan is safe and testable",
                  })
                }
              >
                Approve v{x.current_version}
              </Button>
              <Button
                variant="secondary"
                onClick={() =>
                  void post(`/${x.id}/approvals`, {
                    decision: "changes_requested",
                    note: "Plan needs revision",
                  })
                }
              >
                Request changes
              </Button>
            </div>
          </article>
        );
      })}
    </section>
  );
}
