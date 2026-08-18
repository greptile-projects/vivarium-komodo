"use client";
/* eslint-disable react-hooks/set-state-in-effect, react-hooks/purity */
import { FormEvent, useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";
import { RuntimeInvestigations } from "@/components/runtime-investigations";
type Binding = {
  kind: string;
  resource_id?: string;
  revision?: string;
  status: string;
  reason?: string;
};
type W = {
  id: string;
  title: string;
  origin: { kind: string; resource_id?: string; summary: string };
  release_id: string;
  release_revision: string;
  environment: string;
  time_window: { start: string; end: string };
  user_journey: string;
  owner_ids: string[];
  severity: string;
  source_revision: string;
  bindings: Binding[];
  permitted_evidence: {
    kind: string;
    audience: string;
    access: string;
    reason?: string;
  }[];
  audience: string;
  participant_ids: string[];
  status: string;
  unavailable_context: Binding[];
  hypotheses: {
    id: string;
    summary: string;
    status: string;
    uncertainty?: string;
    actor_id: string;
  }[];
  history: {
    id: string;
    kind: string;
    detail: string;
    actor_id: string;
    created_at: string;
  }[];
  authority: string[];
};
type Probe = {
  id: string;
  kind: string;
  scope: string[];
  status: string;
  expires_at: string;
  approved_by?: string;
  preview: {
    data_categories: string[];
    estimated_cost: number;
    estimated_load: string;
    audience: string;
    sampling_rate: number;
    retention_hours: number;
    privacy_policy: string;
    security_policy: string;
  };
  captures: {
    id: string;
    status: string;
    completeness: string;
    records_captured: number;
    records_expected: number;
    gaps: string[];
    transformations: { kind: string; count: number }[];
    provenance: string;
  }[];
  actions: {
    id: string;
    kind: string;
    detail: string;
    actor_id: string;
    created_at: string;
  }[];
};
const csv = (v: string) =>
  v
    .split(",")
    .map((x) => x.trim())
    .filter(Boolean);
export function DebuggingWorkspaces({
  repository,
  actor,
}: {
  repository: string;
  actor: string;
}) {
  const root = `/api/repositories/${repository}/debugging-workspaces`,
    [items, setItems] = useState<W[]>([]),
    [probes, setProbes] = useState<Record<string, Probe[]>>({}),
    [error, setError] = useState("");
  const load = useCallback(async () => {
    const r = await fetch(root);
    if (r.ok) {
      const next = ((await r.json()) as { items: W[] }).items;
      setItems(next);
      const entries = await Promise.all(
        next.map(async (w) => {
          const p = await fetch(`${root}/${w.id}/probes`);
          return [
            w.id,
            p.ok ? ((await p.json()) as { items: Probe[] }).items : [],
          ] as const;
        }),
      );
      setProbes(Object.fromEntries(entries));
    }
  }, [root]);
  useEffect(() => {
    void load();
  }, [load]);
  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget),
      revision = String(f.get("revision")),
      missing = csv(String(f.get("unavailable"))),
      binding = (kind: string): Binding =>
        missing.includes(kind)
          ? {
              kind,
              status: "unavailable",
              reason: `${kind} context is not accessible to this audience`,
            }
          : {
              kind,
              resource_id: String(f.get(kind)),
              revision,
              status: "available",
            },
      body = {
        title: f.get("title"),
        origin: {
          kind: f.get("origin_kind"),
          resource_id: f.get("origin_id") || undefined,
          revision: f.get("origin_revision") || undefined,
          summary: f.get("summary"),
        },
        release_id: f.get("release"),
        release_revision: revision,
        environment: f.get("environment"),
        time_window: {
          start: new Date(String(f.get("start"))).toISOString(),
          end: new Date(String(f.get("end"))).toISOString(),
        },
        user_journey: f.get("journey"),
        owner_ids: csv(String(f.get("owners"))),
        severity: f.get("severity"),
        source_revision: revision,
        bindings: [
          binding("package"),
          binding("configuration"),
          binding("infrastructure"),
        ],
        permitted_evidence: csv(String(f.get("evidence"))).map((kind) => ({
          kind,
          audience: f.get("audience"),
          access: "permitted",
        })),
        audience: f.get("audience"),
        participant_ids: csv(String(f.get("participants"))),
      };
    const r = await fetch(root, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!r.ok) {
      setError(((await r.json()) as { error: string }).error);
      return;
    }
    setError("");
    e.currentTarget.reset();
    await load();
  }
  async function hypothesis(e: FormEvent<HTMLFormElement>, w: W) {
    e.preventDefault();
    const f = new FormData(e.currentTarget),
      r = await fetch(`${root}/${w.id}/hypotheses`, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          summary: f.get("summary"),
          status: "proposed",
          uncertainty: f.get("uncertainty"),
        }),
      });
    if (!r.ok) {
      setError(((await r.json()) as { error: string }).error);
      return;
    }
    e.currentTarget.reset();
    await load();
  }
  async function requestProbe(e: FormEvent<HTMLFormElement>, w: W) {
    e.preventDefault();
    const f = new FormData(e.currentTarget),
      kind = String(f.get("kind")),
      body = {
        environment: w.environment,
        kind,
        scope: csv(String(f.get("scope"))),
        purpose: f.get("purpose"),
        consent_actor_ids: csv(String(f.get("consent"))),
        expires_at: new Date(
          Date.now() + Number(f.get("hours")) * 3600000,
        ).toISOString(),
        diagnostic:
          kind === "dynamic_diagnostic"
            ? {
                name: f.get("diagnostic"),
                path: ".komodo/diagnostics.json",
                revision: w.source_revision,
              }
            : undefined,
        preview: {
          data_categories: csv(String(f.get("categories"))),
          estimated_cost: Number(f.get("cost")),
          estimated_load: f.get("load"),
          audience: f.get("audience"),
          sampling_rate: Number(f.get("sampling")),
          retention_hours: Number(f.get("retention")),
          privacy_policy: f.get("privacy"),
          security_policy: f.get("security"),
        },
      };
    const r = await fetch(`${root}/${w.id}/probes`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!r.ok) {
      setError(((await r.json()) as { error: string }).error);
      return;
    }
    setError("");
    e.currentTarget.reset();
    await load();
  }
  async function probeAction(w: W, p: Probe, path: string, body: object) {
    const r = await fetch(`${root}/${w.id}/probes/${p.id}/${path}`, {
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
            Observed behavior → running release → exact code
          </span>
          <h2>Debugging workspaces</h2>
          <p>
            Establish shared runtime context without copying protected evidence
            or granting production authority.
          </p>
        </div>
        <Badge>{items.length} workspaces</Badge>
      </div>
      <details className="panel">
        <summary>Open a debugging workspace</summary>
        <form className="stacked-form" onSubmit={create}>
          <label>
            Title
            <input name="title" required />
          </label>
          <label>
            Observed from
            <select name="origin_kind">
              <option>issue</option>
              <option>incident</option>
              <option value="support_thread">support thread</option>
              <option>deployment</option>
              <option value="service_objective">service objective</option>
              <option>trace</option>
              <option value="manual_observation">manual observation</option>
            </select>
          </label>
          <label>
            Origin resource ID
            <input name="origin_id" />
          </label>
          <label>
            Origin revision
            <input name="origin_revision" />
          </label>
          <label>
            Observed behavior
            <textarea name="summary" required />
          </label>
          <label>
            Affected release ID
            <input name="release" required />
          </label>
          <label>
            Exact release/source revision
            <input name="revision" required />
          </label>
          <label>
            Environment
            <input name="environment" required />
          </label>
          <label>
            Window start
            <input name="start" type="datetime-local" required />
          </label>
          <label>
            Window end
            <input name="end" type="datetime-local" required />
          </label>
          <label>
            User journey
            <input name="journey" required />
          </label>
          <label>
            Severity
            <select name="severity">
              <option>critical</option>
              <option>high</option>
              <option>medium</option>
              <option>low</option>
            </select>
          </label>
          <label>
            Owners
            <input name="owners" defaultValue={actor} required />
          </label>
          <label>
            Participants
            <input name="participants" defaultValue={actor} />
          </label>
          <label>
            Package revision reference
            <input name="package" />
          </label>
          <label>
            Configuration revision reference
            <input name="configuration" />
          </label>
          <label>
            Infrastructure revision reference
            <input name="infrastructure" />
          </label>
          <label>
            Unavailable context{" "}
            <small>
              comma-separated package, configuration, infrastructure
            </small>
            <input name="unavailable" />
          </label>
          <label>
            Permitted evidence kinds
            <input
              name="evidence"
              placeholder="logs,traces,profile,state_snapshot,dynamic_diagnostic"
              required
            />
          </label>
          <label>
            Audience
            <select name="audience">
              <option>repository</option>
              <option>participants</option>
            </select>
          </label>
          <Button type="submit">Establish exact context</Button>
        </form>
      </details>
      {error && <p className="form-error">{error.replaceAll("_", " ")}</p>}
      {items.map((w) => (
        <article className="panel" key={w.id}>
          <header>
            <div>
              <span className="eyebrow">
                {w.origin.kind.replaceAll("_", " ")} · {w.environment} ·{" "}
                {w.release_id}
              </span>
              <h3>{w.title}</h3>
              <p>{w.origin.summary}</p>
            </div>
            <Badge>
              {w.severity} · {w.status}
            </Badge>
          </header>
          <p>
            <strong>Running code:</strong> {w.source_revision} · release{" "}
            {w.release_revision}
          </p>
          <p>
            <strong>Journey:</strong> {w.user_journey} ·{" "}
            <strong>window:</strong>{" "}
            {new Date(w.time_window.start).toLocaleString()}–
            {new Date(w.time_window.end).toLocaleString()}
          </p>
          <p>
            <strong>Owners:</strong> {w.owner_ids.join(", ")} ·{" "}
            <strong>audience:</strong> {w.audience} · participants{" "}
            {w.participant_ids.join(", ")}
          </p>
          {w.bindings.map((b, i) => (
            <p key={i}>
              <Badge>{b.kind}</Badge>{" "}
              {b.status === "available"
                ? `${b.resource_id} @ ${b.revision}`
                : `unavailable: ${b.reason}`}
            </p>
          ))}
          <h4>Permitted evidence</h4>
          {w.permitted_evidence.map((x, i) => (
            <Badge key={i}>
              {x.kind} · {x.access} · {x.audience}
            </Badge>
          ))}
          {w.unavailable_context.map((x, i) => (
            <p className="form-error" key={i}>
              {x.kind}: {x.reason}
            </p>
          ))}
          <h4>Scoped runtime probes</h4>
          {(probes[w.id] || []).map((p) => (
            <div className="panel" key={p.id}>
              <p>
                <Badge>
                  {p.kind.replaceAll("_", " ")} · {p.status}
                </Badge>{" "}
                {p.scope.join(", ")} · expires{" "}
                {new Date(p.expires_at).toLocaleString()}
              </p>
              <p>
                <strong>Preview:</strong> {p.preview.data_categories.join(", ")}{" "}
                · {p.preview.estimated_load} load · cost{" "}
                {p.preview.estimated_cost} · sample{" "}
                {p.preview.sampling_rate * 100}% · retain{" "}
                {p.preview.retention_hours}h · {p.preview.audience}
              </p>
              <p>
                <small>
                  Privacy {p.preview.privacy_policy} · security{" "}
                  {p.preview.security_policy}
                </small>
              </p>
              {p.captures.map((c) => (
                <p key={c.id}>
                  <Badge>{c.completeness}</Badge> {c.records_captured}/
                  {c.records_expected} records · {c.provenance}
                  {c.gaps.length > 0 && ` · gaps: ${c.gaps.join(", ")}`}
                </p>
              ))}
              {w.owner_ids.includes(actor) &&
                p.status === "pending_approval" && (
                  <>
                    <Button
                      onClick={() =>
                        probeAction(w, p, "decision", {
                          decision: "approved",
                          reason:
                            "environment owner accepted the bounded preview",
                        })
                      }
                    >
                      Approve bounded probe
                    </Button>{" "}
                    <Button
                      onClick={() =>
                        probeAction(w, p, "decision", {
                          decision: "denied",
                          reason: "environment policy denied collection",
                        })
                      }
                    >
                      Deny
                    </Button>
                  </>
                )}
              {w.owner_ids.includes(actor) &&
                (p.status === "approved" || p.status === "narrowed") && (
                  <Button
                    onClick={() =>
                      probeAction(w, p, "controls", {
                        kind: "revoke",
                        detail: "collection authority revoked",
                      })
                    }
                  >
                    Revoke probe
                  </Button>
                )}
              <details>
                <summary>Probe actions</summary>
                {p.actions.map((a) => (
                  <p key={a.id}>
                    {new Date(a.created_at).toLocaleString()} · {a.actor_id} ·{" "}
                    {a.kind.replaceAll("_", " ")} · {a.detail}
                  </p>
                ))}
              </details>
            </div>
          ))}
          {w.participant_ids.includes(actor) && (
            <details>
              <summary>Request privacy-safe runtime evidence</summary>
              <form
                className="stacked-form"
                onSubmit={(e) => requestProbe(e, w)}
              >
                <label>
                  Evidence kind
                  <select name="kind">
                    <option>logs</option>
                    <option>traces</option>
                    <option>profile</option>
                    <option value="state_snapshot">state snapshot</option>
                    <option value="dynamic_diagnostic">
                      repository diagnostic
                    </option>
                  </select>
                </label>
                <label>
                  Bounded scope
                  <input
                    name="scope"
                    placeholder="service:api,route:/checkout"
                    required
                  />
                </label>
                <label>
                  Purpose
                  <textarea name="purpose" required />
                </label>
                <label>
                  Data categories
                  <input name="categories" required />
                </label>
                <label>
                  Cost estimate
                  <input
                    name="cost"
                    type="number"
                    min="0"
                    step="0.01"
                    defaultValue="0"
                    required
                  />
                </label>
                <label>
                  Load
                  <select name="load">
                    <option>low</option>
                    <option>moderate</option>
                    <option>high</option>
                  </select>
                </label>
                <label>
                  Sampling rate
                  <input
                    name="sampling"
                    type="number"
                    min="0.01"
                    max="1"
                    step="0.01"
                    defaultValue="0.1"
                    required
                  />
                </label>
                <label>
                  Retention hours
                  <input
                    name="retention"
                    type="number"
                    min="1"
                    max="720"
                    defaultValue="24"
                    required
                  />
                </label>
                <label>
                  Probe hours
                  <input
                    name="hours"
                    type="number"
                    min="1"
                    max="24"
                    defaultValue="1"
                    required
                  />
                </label>
                <label>
                  Audience
                  <select name="audience">
                    <option>participants</option>
                    <option>repository</option>
                  </select>
                </label>
                <label>
                  Privacy policy
                  <input name="privacy" required />
                </label>
                <label>
                  Security policy
                  <input name="security" required />
                </label>
                <label>
                  Consent actors
                  <input name="consent" />
                </label>
                <label>
                  Diagnostic name
                  <input name="diagnostic" />
                </label>
                <Button type="submit">Preview and request approval</Button>
              </form>
            </details>
          )}
          <RuntimeInvestigations
            repository={repository}
            workspace={w.id}
            revision={w.source_revision}
            participants={w.participant_ids.includes(actor)}
            probes={probes[w.id] || []}
          />
          <h4>Hypotheses</h4>
          {w.hypotheses.map((x) => (
            <p key={x.id}>
              <Badge>{x.status}</Badge> {x.summary} — {x.actor_id}
              {x.uncertainty && ` · uncertainty: ${x.uncertainty}`}
            </p>
          ))}
          {w.participant_ids.includes(actor) && (
            <form className="stacked-form" onSubmit={(e) => hypothesis(e, w)}>
              <label>
                Shared hypothesis
                <textarea name="summary" required />
              </label>
              <label>
                Uncertainty
                <input name="uncertainty" />
              </label>
              <Button type="submit">Add attributed hypothesis</Button>
            </form>
          )}
          <details>
            <summary>Attributable history</summary>
            {w.history.map((x) => (
              <p key={x.id}>
                {new Date(x.created_at).toLocaleString()} · {x.actor_id} ·{" "}
                {x.kind.replaceAll("_", " ")} · {x.detail}
              </p>
            ))}
          </details>
          <small>
            Probe records grant no provider, deployment, environment,
            credential, or mutation authority.
          </small>
        </article>
      ))}
    </section>
  );
}
