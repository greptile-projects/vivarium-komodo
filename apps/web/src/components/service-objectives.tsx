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
    [error, setError] = useState("");
  const load = useCallback(async () => {
    const r = await fetch(root);
    if (r.ok) setItems(((await r.json()) as { items: O[] }).items);
  }, [root]);
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
