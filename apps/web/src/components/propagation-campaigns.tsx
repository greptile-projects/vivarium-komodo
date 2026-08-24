/* eslint-disable react-hooks/set-state-in-effect */
"use client";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";
type Target = {
  id: string;
  repository_id?: string;
  repository_reference?: string;
  release_line: string;
  revision?: string;
  package_ids: string[];
  owner_ids: string[];
  deadline: string;
  depends_on: string[];
  disposition: string;
  disposition_reason?: string;
  authority: {
    owner_ids: string[];
    access: string;
    basis: string;
    observed_at: string;
  };
};
type Citation = {
  kind: string;
  reference: string;
  revision: string;
  path?: string;
  symbol?: string;
};
type Assessment = {
  id: string;
  target_id: string;
  author_id: string;
  target_revision: string;
  source_revision: string;
  classification: string;
  rationale: string;
  comparisons: {
    kind: string;
    source_summary: string;
    target_summary: string;
    conclusion: string;
    behavioral_proof: boolean;
    citations: Citation[];
  }[];
  risks: string[];
  uncertainty?: string;
  assumptions_still_hold: boolean;
  stale: boolean;
  findings: {
    id: string;
    actor_id: string;
    actor_kind: string;
    summary: string;
    risk?: string;
    uncertainty?: string;
    citations: Citation[];
  }[];
  acknowledgements: {
    id: string;
    owner_id: string;
    decision: string;
    rationale: string;
  }[];
};
type Scenario = {
  id: string;
  behavior: string;
  source_evidence: string[];
  commands: string[];
  required_coverage: string[];
  ordinary_check_names: string[];
  substitute_allowed: boolean;
  substitute_requirement?: string;
};
type Specification = {
  id: string;
  source_revision: string;
  environment: string;
  maximum_cost: number;
  currency: string;
  timeout_seconds: number;
  scenarios: Scenario[];
};
type Attempt = {
  id: string;
  target_id: string;
  runner_id: string;
  specification_id: string;
  assessment_id: string;
  source_revision: string;
  target_revision: string;
  adaptation_revision?: string;
  environment: string;
  bound_inputs: { key: string; revision: string }[];
  evidence: {
    scenario_id: string;
    status: string;
    commands: string[];
    ordinary_checks: string[];
    logs: string[];
    artifacts: {
      name: string;
      digest: string;
      media_type: string;
      size: number;
    }[];
    coverage: string[];
    substitute_evidence: string[];
    residual_difference?: string;
  }[];
  cost: number;
  currency: string;
  duration_seconds: number;
  stale: boolean;
  passing: boolean;
  blockers: { kind: string; detail: string }[];
  owner_decisions: {
    id: string;
    owner_id: string;
    decision: string;
    rationale: string;
  }[];
};
type CoverageTarget = {
  target_id: string;
  state: string;
  paused: boolean;
  supported_users: number;
  reached_users: number;
  exposure_unit?: string;
  observed_outcome?: string;
  next_actions: string[];
  blockers?: { kind: string; detail: string }[];
};
type Campaign = {
  id: string;
  title: string;
  intent: string;
  acceptance_criteria: string[];
  creator_id: string;
  created_at: string;
  source: {
    kind: string;
    resource_id: string;
    revision: string;
    commit_ids: string[];
    evidence_references: string[];
  };
  targets: Target[];
  completion_policy: {
    mode: string;
    required_target_ids: string[];
    allow_already_equivalent: boolean;
    exception_requires_owner: boolean;
  };
  blockers: { target_id: string; kind: string; detail: string }[];
  assessments: Assessment[];
  equivalence_specifications: Specification[];
  equivalence_attempts: Attempt[];
  coverage: {
    complete: boolean;
    supported_users: number;
    reached_users: number;
    blockers: { target_id: string; kind: string; detail: string }[];
    targets: CoverageTarget[];
  };
};
const csv = (x: string | undefined) =>
  (x || "")
    .split(",")
    .map((v) => v.trim())
    .filter(Boolean);
const rows = (x: FormDataEntryValue | null) =>
  String(x || "")
    .split("\n")
    .map((v) => v.trim())
    .filter(Boolean)
    .map((v) => v.split("|").map((y) => y.trim()));
export function PropagationCampaigns({
  repository,
  actor,
}: {
  repository: string;
  actor: string;
}) {
  const root = `/api/repositories/${repository}/propagation-campaigns`,
    [items, setItems] = useState<Campaign[]>([]),
    [error, setError] = useState("");
  const load = useCallback(async () => {
    const r = await fetch(root);
    if (r.ok) setItems(((await r.json()) as { items: Campaign[] }).items);
  }, [root]);
  useEffect(() => {
    void load();
  }, [load]);
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget),
      observed = new Date().toISOString();
    const targets = rows(f.get("targets")).map((x) => ({
      id: x[0],
      repository_id: x[1] || undefined,
      repository_reference: x[2] || undefined,
      release_line: x[3],
      revision: x[4] || undefined,
      package_ids: csv(x[5]),
      owner_ids: csv(x[6]),
      deadline: new Date(x[7]).toISOString(),
      depends_on: csv(x[8]),
      disposition: x[9],
      disposition_reason: x[10] || undefined,
      authority: {
        owner_ids: csv(x[6]),
        access: x[11],
        basis: x[12],
        observed_at: observed,
      },
    }));
    const required = csv(String(f.get("required") || ""));
    const body = {
      title: f.get("title"),
      intent: f.get("intent"),
      acceptance_criteria: String(f.get("criteria") || "")
        .split("\n")
        .map((x) => x.trim())
        .filter(Boolean),
      source: {
        kind: f.get("kind"),
        repository_id: repository,
        resource_id: f.get("resource"),
        revision: f.get("revision"),
        commit_ids: csv(String(f.get("commits") || "")),
        evidence_references: csv(String(f.get("evidence") || "")),
      },
      targets,
      completion_policy: {
        mode: required.length ? "required_targets" : "all_supported",
        required_target_ids: required,
        allow_already_equivalent: f.get("equivalent") === "on",
        exception_requires_owner: true,
      },
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
  async function post(path: string, body: unknown) {
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
  const starter = (x: Campaign, t: Target) =>
    JSON.stringify(
      {
        target_revision: t.revision || "exact-target-revision",
        source_revision: x.source.revision,
        classification: "adaptation_required",
        rationale: "Explain why the source assumptions do or do not hold here.",
        comparisons: [
          "history",
          "symbols",
          "dependencies",
          "interfaces",
          "schemas",
          "prior_fixes",
          "release_commitments",
        ].map((kind) => ({
          kind,
          source_summary: "source observation",
          target_summary: "target observation",
          conclusion: "different",
          behavioral_proof: false,
          citations: [
            {
              kind,
              reference: `evidence:${kind}`,
              revision: t.revision || "exact-target-revision",
            },
          ],
        })),
        risks: [],
        uncertainty: "",
        assumptions_still_hold: false,
      },
      null,
      2,
    );
  return (
    <section className="investigation-workspace">
      <div className="section-heading">
        <div>
          <span className="eyebrow">
            Proven source → maintained lines → shared outcome
          </span>
          <h2>Propagation campaigns</h2>
          <p>
            Agree which exact outcome must reach each release line and
            independently owned project before anyone copies commits. Target
            authority is recorded, never granted.
          </p>
        </div>
        <Badge>{items.length} campaigns</Badge>
      </div>
      <details className="panel">
        <summary>Open a propagation campaign</summary>
        <form className="stacked-form" onSubmit={submit}>
          <label>
            Title
            <input name="title" required />
          </label>
          <label>
            Intent
            <textarea name="intent" required />
          </label>
          <label>
            Acceptance criteria <small>one per line</small>
            <textarea name="criteria" required />
          </label>
          <label>
            Proven source
            <select name="kind">
              <option value="merged_pull_request">Merged pull request</option>
              <option value="security_repair">Security repair</option>
              <option value="regression_correction">
                Regression correction
              </option>
              <option value="policy_change">Policy change</option>
              <option value="package_release">Package release</option>
              <option value="interface_evolution">Interface evolution</option>
            </select>
          </label>
          <label>
            Source resource ID
            <input name="resource" required />
          </label>
          <label>
            Exact source revision
            <input name="revision" required />
          </label>
          <label>
            Source commits <small>comma separated, exact Git object IDs</small>
            <input name="commits" required />
          </label>
          <label>
            Evidence references <small>comma separated</small>
            <input name="evidence" />
          </label>
          <label>
            Targets{" "}
            <small>
              id | local repository id | external repository reference | release
              line | revision | packages | owners | deadline | dependencies |
              pending/unknown/unsupported/inaccessible/already_equivalent |
              reason | access | authority basis
            </small>
            <textarea
              name="targets"
              rows={6}
              placeholder={`stable | ${repository} | | v2.x | commit | parser | ${actor} | 2026-09-30T00:00:00Z | | pending | | write | repository collaborator`}
              required
            />
          </label>
          <label>
            Only these targets are required{" "}
            <small>comma separated; blank means every supported target</small>
            <input name="required" />
          </label>
          <label>
            <input type="checkbox" name="equivalent" /> Count explicitly
            already-equivalent targets as complete
          </label>
          <Button type="submit">Open shared campaign</Button>
        </form>
      </details>
      {error && <p className="form-error">{error.replaceAll("_", " ")}</p>}
      {items.map((x) => (
        <article className="panel" key={x.id}>
          <header>
            <div>
              <span className="eyebrow">
                {x.source.kind.replaceAll("_", " ")} · {x.source.revision} ·
                opened by {x.creator_id}
              </span>
              <h3>{x.title}</h3>
              <p>{x.intent}</p>
            </div>
            <Badge>
              {x.coverage?.complete
                ? "coverage complete"
                : `${x.coverage?.blockers.length || x.blockers.length} blockers`}
            </Badge>
          </header>
          <p>
            <strong>User exposure:</strong> {x.coverage?.reached_users || 0} /{" "}
            {x.coverage?.supported_users || 0} supported ·{" "}
            <strong>Acceptance:</strong> {x.acceptance_criteria.join(" · ")}
          </p>
          <p>
            <strong>Source commits:</strong> {x.source.commit_ids.join(", ")} ·{" "}
            <strong>Proof:</strong> {x.source.resource_id}
          </p>
          <details>
            <summary>Define reusable behavioral proof</summary>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                const f = new FormData(e.currentTarget);
                void post(
                  `/${x.id}/equivalence-specifications`,
                  JSON.parse(String(f.get("specification"))),
                );
              }}
            >
              <textarea
                name="specification"
                rows={16}
                defaultValue={JSON.stringify(
                  {
                    source_revision: x.source.revision,
                    environment: "bounded networkless runner",
                    maximum_cost: 10,
                    currency: "USD",
                    timeout_seconds: 900,
                    scenarios: [
                      {
                        id: "required-behavior",
                        behavior: x.acceptance_criteria[0],
                        source_evidence: x.source.evidence_references.length
                          ? x.source.evidence_references
                          : [x.source.resource_id],
                        commands: ["ordinary target test command"],
                        required_coverage: ["required behavior"],
                        ordinary_check_names: ["unit"],
                        substitute_allowed: false,
                      },
                    ],
                  },
                  null,
                  2,
                )}
                required
              />
              <Button type="submit">Freeze equivalence requirements</Button>
            </form>
          </details>
          <h4>Target delivery and equivalence</h4>
          {x.targets.map((t) => {
            const coverage = x.coverage?.targets.find(
              (v) => v.target_id === t.id,
            );
            return (
              <section className="panel" key={t.id}>
                <p>
                  <Badge>
                    {coverage?.paused
                      ? "paused"
                      : coverage?.state || t.disposition.replaceAll("_", " ")}
                  </Badge>{" "}
                  <strong>{t.release_line}</strong> ·{" "}
                  {t.repository_id || t.repository_reference} ·{" "}
                  {coverage?.reached_users || 0}/
                  {coverage?.supported_users || 0}{" "}
                  {coverage?.exposure_unit || "supported users"}
                </p>
                <p>
                  Owners {t.owner_ids.join(", ") || "unknown"} · next:{" "}
                  {coverage?.next_actions.join(" · ") || "assess target"}
                </p>
                {coverage?.observed_outcome && (
                  <p>
                    <strong>Observed outcome:</strong>{" "}
                    {coverage.observed_outcome}
                  </p>
                )}
                {(coverage?.blockers || []).map((b, i) => (
                  <p className="form-error" key={i}>
                    {b.kind.replaceAll("_", " ")}: {b.detail}
                  </p>
                ))}
                <details>
                  <summary>Record target-owner delivery receipt</summary>
                  <form
                    onSubmit={(e) => {
                      e.preventDefault();
                      const f = new FormData(e.currentTarget);
                      void post(
                        `/${x.id}/targets/${t.id}/delivery-events`,
                        JSON.parse(String(f.get("delivery"))),
                      );
                    }}
                  >
                    <textarea
                      name="delivery"
                      rows={12}
                      defaultValue={JSON.stringify(
                        {
                          kind: "review",
                          status: "succeeded",
                          resource_reference:
                            "pull request, queue, release, deployment, or observation ID",
                          revision: t.revision || "exact delivered revision",
                          summary:
                            "Result from the target's ordinary policy surface",
                          supported_users: 0,
                          reached_users: 0,
                          exposure_unit: "supported installations",
                          outcome: "",
                        },
                        null,
                        2,
                      )}
                      required
                    />
                    <Button type="submit">Append owner receipt</Button>
                  </form>
                </details>
                {(x.equivalence_attempts || [])
                  .filter((v) => v.target_id === t.id)
                  .map((v) => (
                    <p key={v.id}>
                      <Badge>
                        {v.stale
                          ? "stale"
                          : v.passing
                            ? "equivalent"
                            : "not proven"}
                      </Badge>{" "}
                      target <code>{v.target_revision}</code> · {v.cost}{" "}
                      {v.currency} · decisions{" "}
                      {v.owner_decisions.map((d) => d.decision).join(", ") ||
                        "pending"}
                    </p>
                  ))}
                {t.authority.access !== "inaccessible" && (
                  <details>
                    <summary>Compare this exact target</summary>
                    <form
                      onSubmit={(e) => {
                        e.preventDefault();
                        const f = new FormData(e.currentTarget);
                        void post(
                          `/${x.id}/targets/${t.id}/assessments`,
                          JSON.parse(String(f.get("assessment"))),
                        );
                      }}
                    >
                      <textarea
                        name="assessment"
                        rows={18}
                        defaultValue={starter(x, t)}
                        required
                      />
                      <Button type="submit">
                        Record applicability assessment
                      </Button>
                    </form>
                  </details>
                )}
              </section>
            );
          })}
        </article>
      ))}
    </section>
  );
}
