"use client";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";

type Standing = {
  id: string;
  principal_id: string;
  role: string;
  state: string;
  responsibilities: string[];
  evidence: { kind: string; reference: string; summary: string }[];
  available_nominations: string[];
  available_appeals: string[];
  operational_authority: string[];
  term_ends_at?: string;
};
type Charter = {
  active_version?: number;
  current: {
    version: number;
    state: string;
    title: string;
    purpose: string;
    roles: {
      name: string;
      purpose: string;
      eligibility: string[];
      responsibilities: string[];
      minimum_members: number;
      term_days?: number;
    }[];
    decision_classes: {
      name: string;
      description: string;
      eligible_roles: string[];
      participation: string;
      quorum: number;
      threshold: number;
      protected_resources: string[];
    }[];
    participation_rules: string[];
    protected_resources: string[];
    procedures: { removal: string; succession: string; vacancy: string };
    amendment_policy: {
      eligible_roles: string[];
      notice_days: number;
      quorum: number;
      threshold: number;
    };
    preview: {
      items: {
        kind: string;
        resource: string;
        state: string;
        detail: string;
        blocking: boolean;
      }[];
      blockers: string[];
      authority_granted: boolean;
    };
    author_id: string;
    change_reason: string;
    created_at: string;
  };
  history: Charter["current"][];
  approvals: {
    actor_id: string;
    version: number;
    note?: string;
    created_at: string;
  }[];
  exceptions: {
    id: string;
    version: number;
    scope: string;
    reason: string;
    expires_at: string;
    actor_id: string;
  }[];
  standings: Standing[];
};
const lines = (v: FormDataEntryValue | null) =>
  String(v || "")
    .split("\n")
    .map((x) => x.trim())
    .filter(Boolean);
export function GovernanceCharter({
  scope,
  id,
  canManage,
}: {
  scope: "repositories" | "organizations";
  id: string;
  canManage: boolean;
}) {
  const base = `/${scope}/${id}/governance-charter`,
    [charter, setCharter] = useState<Charter>(),
    [error, setError] = useState("");
  const load = useCallback(async () => {
    const r = await fetch(`/api${base}`);
    if (r.ok) setCharter(await r.json());
    else if (r.status !== 404)
      setError("Governance charter could not be loaded.");
  }, [base]);
  useEffect(() => {
    const timer = setTimeout(() => void load(), 0);
    return () => clearTimeout(timer);
  }, [load]);
  async function mutate(path: string, body: unknown) {
    setError("");
    const r = await fetch(`/api${base}${path}`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!r.ok) {
      const v = await r.json().catch(() => ({}));
      setError(
        v.error === "charter_conflict"
          ? "Activation is blocked by missing approval, stale version, or an impossible rule."
          : "The charter could not be updated.",
      );
      return;
    }
    setCharter(await r.json());
  }
  async function publish(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget),
      role = String(f.get("role")),
      resource = lines(f.get("resources"));
    await mutate("", {
      expected_version: charter?.current.version || 0,
      title: f.get("title"),
      purpose: f.get("purpose"),
      roles: [
        {
          name: role,
          purpose: f.get("role_purpose"),
          eligibility: lines(f.get("eligibility")),
          responsibilities: lines(f.get("responsibilities")),
          minimum_members: Number(f.get("minimum")),
          term_days: Number(f.get("term")),
        },
      ],
      decision_classes: [
        {
          name: f.get("decision"),
          description: f.get("decision_description"),
          eligible_roles: [role],
          participation: f.get("participation"),
          quorum: Number(f.get("quorum")),
          threshold: Number(f.get("threshold")),
          protected_resources: resource,
        },
      ],
      participation_rules: lines(f.get("rules")),
      protected_resources: resource,
      procedures: {
        removal: f.get("removal"),
        succession: f.get("succession"),
        vacancy: f.get("vacancy"),
      },
      amendment_policy: {
        eligible_roles: [role],
        notice_days: Number(f.get("notice")),
        quorum: Number(f.get("amendment_quorum")),
        threshold: Number(f.get("amendment_threshold")),
      },
      change_reason: f.get("reason"),
    });
  }
  return (
    <section className="investigation-workspace">
      <div className="section-heading">
        <div>
          <span className="eyebrow">Decision rights</span>
          <h2>Project governance</h2>
          <p>
            Published rules explain who may decide what and how stewardship
            changes. They never replace repository, organization, branch,
            release, environment, security, or agent authority.
          </p>
        </div>
        {charter && (
          <Badge>
            {charter.current.state} · v{charter.current.version}
          </Badge>
        )}
      </div>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      {charter && (
        <div className="panel stack">
          <h3>{charter.current.title}</h3>
          <p>{charter.current.purpose}</p>
          <p>
            <strong>Protected resources:</strong>{" "}
            {charter.current.protected_resources.join(" · ")}
          </p>
          {charter.current.roles.map((r) => (
            <div key={r.name}>
              <h4>{r.name}</h4>
              <p>
                {r.purpose} · minimum {r.minimum_members}
                {r.term_days ? ` · ${r.term_days}-day term` : ""}
              </p>
              <p>
                <strong>Eligible:</strong> {r.eligibility.join(" · ")}
                <br />
                <strong>Responsible:</strong> {r.responsibilities.join(" · ")}
              </p>
            </div>
          ))}
          {charter.current.decision_classes.map((d) => (
            <div key={d.name}>
              <h4>{d.name}</h4>
              <p>
                {d.description} · {d.participation} · quorum {d.quorum} ·
                threshold {d.threshold}%
              </p>
            </div>
          ))}
          <p>
            <strong>Removal:</strong> {charter.current.procedures.removal}
            <br />
            <strong>Succession:</strong> {charter.current.procedures.succession}
            <br />
            <strong>Vacancy:</strong> {charter.current.procedures.vacancy}
          </p>
          <p>
            <strong>Amendments:</strong>{" "}
            {charter.current.amendment_policy.notice_days} days notice · quorum{" "}
            {charter.current.amendment_policy.quorum} ·{" "}
            {charter.current.amendment_policy.threshold}% threshold
          </p>
          <h4>Live authority preview</h4>
          {charter.current.preview.items.map((x, i) => (
            <p
              className={x.blocking ? "form-error" : ""}
              key={`${x.resource}:${i}`}
            >
              <strong>
                {x.kind} · {x.state}:
              </strong>{" "}
              {x.detail}
            </p>
          ))}
          <small>
            Authority granted by charter: no · authored by{" "}
            {charter.current.author_id}
          </small>
          {charter.approvals
            .filter((a) => a.version === charter.current.version)
            .map((a) => (
              <p key={a.actor_id}>
                Approved by {a.actor_id}
                {a.note ? ` — ${a.note}` : ""}
              </p>
            ))}
          {charter.exceptions.map((x) => (
            <p key={x.id}>
              Exception to v{x.version}: {x.scope} — {x.reason} (expires{" "}
              {new Date(x.expires_at).toLocaleString()})
            </p>
          ))}
          {canManage && charter.current.state !== "active" && (
            <div className="button-row">
              <Button
                onClick={() =>
                  mutate("/approvals", {
                    version: charter.current.version,
                    note: "Maintainer approval",
                  })
                }
              >
                Approve revision
              </Button>
              <Button
                onClick={() =>
                  mutate("/activation", { version: charter.current.version })
                }
              >
                Activate after fresh preview
              </Button>
            </div>
          )}
          <details>
            <summary>Immutable revision history</summary>
            {charter.history.map((x) => (
              <p key={x.version}>
                Version {x.version} · {x.state} · {x.change_reason} ·{" "}
                {x.author_id}
              </p>
            ))}
          </details>
        </div>
      )}
      {charter?.current.state === "active" && (
        <GovernedProposals
          base={base}
          charter={charter}
          canManage={canManage}
        />
      )}
      {canManage && (
        <details className="panel">
          <summary>
            {charter
              ? "Draft a new immutable revision"
              : "Publish the first charter draft"}
          </summary>
          <form className="stack" onSubmit={publish}>
            <label>
              Charter title
              <input
                required
                name="title"
                defaultValue={charter?.current.title}
              />
            </label>
            <label>
              Purpose
              <textarea
                required
                name="purpose"
                defaultValue={charter?.current.purpose}
              />
            </label>
            <div className="content-grid">
              <label>
                Governance role
                <input required name="role" defaultValue="Maintainer" />
              </label>
              <label>
                Minimum members
                <input
                  required
                  type="number"
                  min="1"
                  name="minimum"
                  defaultValue="1"
                />
              </label>
              <label>
                Term days
                <input
                  required
                  type="number"
                  min="1"
                  name="term"
                  defaultValue="365"
                />
              </label>
            </div>
            <label>
              Role purpose
              <textarea required name="role_purpose" />
            </label>
            <label>
              Eligibility, one per line
              <textarea required name="eligibility" />
            </label>
            <label>
              Responsibilities, one per line
              <textarea required name="responsibilities" />
            </label>
            <div className="content-grid">
              <label>
                Decision class
                <input required name="decision" defaultValue="Project policy" />
              </label>
              <label>
                Participation
                <select name="participation">
                  <option>open_deliberation</option>
                  <option>eligible_roles_only</option>
                </select>
              </label>
              <label>
                Quorum
                <input
                  required
                  type="number"
                  min="1"
                  name="quorum"
                  defaultValue="1"
                />
              </label>
              <label>
                Threshold %
                <input
                  required
                  type="number"
                  min="1"
                  max="100"
                  name="threshold"
                  defaultValue="50"
                />
              </label>
            </div>
            <label>
              Decision description
              <textarea required name="decision_description" />
            </label>
            <label>
              Protected resource classes, one per line
              <textarea
                required
                name="resources"
                placeholder={
                  "ownership\nbranches:main\nreleases\nsecurity\nagents"
                }
              />
            </label>
            <label>
              Participation rules
              <textarea required name="rules" />
            </label>
            <label>
              Removal procedure
              <textarea required name="removal" />
            </label>
            <label>
              Succession procedure
              <textarea required name="succession" />
            </label>
            <label>
              Vacancy procedure
              <textarea required name="vacancy" />
            </label>
            <div className="content-grid">
              <label>
                Amendment notice days
                <input
                  required
                  type="number"
                  min="0"
                  name="notice"
                  defaultValue="7"
                />
              </label>
              <label>
                Amendment quorum
                <input
                  required
                  type="number"
                  min="1"
                  name="amendment_quorum"
                  defaultValue="1"
                />
              </label>
              <label>
                Amendment threshold %
                <input
                  required
                  type="number"
                  min="1"
                  max="100"
                  name="amendment_threshold"
                  defaultValue="67"
                />
              </label>
            </div>
            <label>
              Change reason
              <input required name="reason" />
            </label>
            <Button type="submit">Publish draft and preview conflicts</Button>
          </form>
        </details>
      )}
    </section>
  );
}

type GovernedProposal = {
  id: string;
  kind: string;
  title: string;
  summary: string;
  scope: string;
  charter_version: number;
  decision_class: string;
  alternatives: {
    id: string;
    title: string;
    description: string;
    implementation_effects: string[];
  }[];
  evidence: { kind: string; reference: string; summary: string }[];
  affected_resources: string[];
  disclosure_requirements: string[];
  implementation_effects: string[];
  eligible_roles: string[];
  quorum: number;
  threshold: number;
  secret_ballot: boolean;
  closes_at: string;
  state: string;
  discussion: {
    id: string;
    actor_id: string;
    actor_kind: string;
    body: string;
    citations: { reference: string; summary: string }[];
  }[];
  ballots: {
    actor_id: string;
    choice?: string;
    choice_digest: string;
    abstain: boolean;
  }[];
  tally?: {
    electorate: string[];
    counted_ballots: number;
    abstentions: number;
    excluded_ballots: string[];
    counts: Record<string, number>;
    quorum_met: boolean;
    threshold_met: boolean;
    outcome: string;
    digest: string;
  };
  contests: {
    id: string;
    actor_id: string;
    reason: string;
    state: string;
    resolution?: string;
  }[];
};
export function GovernedProposals({
  base,
  charter,
  canManage,
}: {
  base: string;
  charter: Charter;
  canManage: boolean;
}) {
  const [items, setItems] = useState<GovernedProposal[]>([]),
    [error, setError] = useState("");
  const load = useCallback(async () => {
    const r = await fetch(`/api${base}/proposals`);
    if (r.ok) setItems((await r.json()).items);
  }, [base]);
  useEffect(() => {
    void load();
  }, [load]);
  async function post(path: string, body: unknown) {
    setError("");
    const r = await fetch(`/api${base}/proposals${path}`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!r.ok) {
      setError(
        "The governed action was rejected by the charter, current eligibility, or deadline.",
      );
      return;
    }
    await load();
  }
  return (
    <section className="panel stack">
      <div className="section-heading">
        <div>
          <h3>Governed proposals</h3>
          <p>
            Consequential choices retain their charter rules, evidence, current
            electorate, ballots, dissent, and verifiable result.
          </p>
        </div>
        <Badge>{items.filter((x) => x.state === "open").length} open</Badge>
      </div>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <details>
        <summary>Open an evidence-backed proposal</summary>
        <form
          className="stack"
          onSubmit={(e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget),
              alts = lines(f.get("alternatives")).map((x, i) => {
                const [title, description] = x.split("|").map((v) => v.trim());
                return {
                  id: `option-${i + 1}`,
                  title,
                  description,
                  implementation_effects: [],
                };
              });
            void post("", {
              kind: f.get("kind"),
              title: f.get("title"),
              summary: f.get("summary"),
              scope: f.get("scope"),
              decision_class: f.get("decision_class"),
              alternatives: alts,
              evidence: [
                {
                  kind: f.get("evidence_kind"),
                  reference: f.get("evidence_reference"),
                  summary: f.get("evidence_summary"),
                },
              ],
              affected_resources: lines(f.get("resources")),
              disclosure_requirements: lines(f.get("disclosures")),
              implementation_effects: lines(f.get("effects")),
              secret_ballot: f.get("secret") === "on",
              discussion_hours: Number(f.get("hours")),
            });
          }}
        >
          <label>
            Choice type
            <select name="kind">
              <option value="technical_decision">Technical decision</option>
              <option value="initiative">Initiative</option>
              <option value="policy_exception">Policy exception</option>
              <option value="funding_request">Funding request</option>
              <option value="resource_request">Resource request</option>
              <option value="leadership_nomination">
                Leadership nomination
              </option>
              <option value="charter_amendment">Charter amendment</option>
            </select>
          </label>
          <label>
            Title
            <input name="title" required />
          </label>
          <label>
            Scope and question
            <textarea name="summary" required />
          </label>
          <label>
            Scope boundary
            <input name="scope" required />
          </label>
          <label>
            Decision class
            <select name="decision_class">
              {charter.current.decision_classes.map((x) => (
                <option key={x.name}>{x.name}</option>
              ))}
            </select>
          </label>
          <label>
            Alternatives, one per line as title | description
            <textarea name="alternatives" required />
          </label>
          <label>
            Evidence kind
            <input name="evidence_kind" required />
          </label>
          <label>
            Evidence reference
            <input name="evidence_reference" required />
          </label>
          <label>
            Evidence summary
            <textarea name="evidence_summary" required />
          </label>
          <label>
            Affected resources
            <textarea name="resources" required />
          </label>
          <label>
            Disclosure requirements
            <textarea name="disclosures" />
          </label>
          <label>
            Implementation effects
            <textarea name="effects" />
          </label>
          <label>
            Discussion hours
            <input
              name="hours"
              type="number"
              min="1"
              max="2160"
              defaultValue="168"
              required
            />
          </label>
          <label>
            <input name="secret" type="checkbox" /> Charter-permitted secret
            ballot
          </label>
          <Button type="submit">Open governed proposal</Button>
        </form>
      </details>
      {items.map((p) => (
        <article className="card stack" key={p.id}>
          <header>
            <Badge>{p.state}</Badge> <strong>{p.title}</strong>
          </header>
          <p>{p.summary}</p>
          <small>
            {p.kind.replaceAll("_", " ")} · charter v{p.charter_version} ·{" "}
            {p.decision_class} · closes {new Date(p.closes_at).toLocaleString()}
          </small>
          <p>
            <strong>Scope:</strong> {p.scope}
            <br />
            <strong>Affected:</strong> {p.affected_resources.join(" · ")}
            <br />
            <strong>Electorate:</strong> {p.eligible_roles.join(" · ")} · quorum{" "}
            {p.quorum} · threshold {p.threshold}%<br />
            <strong>Disclosures:</strong>{" "}
            {p.disclosure_requirements.join(" · ") || "none declared"}
            <br />
            <strong>Effects:</strong>{" "}
            {p.implementation_effects.join(" · ") || "none declared"}
          </p>
          {p.evidence.map((x) => (
            <p key={x.reference}>
              <strong>{x.kind}:</strong> {x.summary} ·{" "}
              <code>{x.reference}</code>
            </p>
          ))}
          <h4>Alternatives</h4>
          {p.alternatives.map((a) => (
            <div key={a.id}>
              <strong>{a.title}</strong>
              <p>{a.description}</p>
              {p.state === "open" && (
                <Button
                  variant="secondary"
                  onClick={() => post(`/${p.id}/ballots`, { choice: a.id })}
                >
                  Vote {p.secret_ballot ? "secretly" : "attributably"}
                </Button>
              )}
            </div>
          ))}
          {p.state === "open" && (
            <Button
              variant="secondary"
              onClick={() =>
                post(`/${p.id}/ballots`, {
                  abstain: true,
                  reason: "Recorded abstention",
                })
              }
            >
              Abstain
            </Button>
          )}
          <h4>Deliberation</h4>
          {p.discussion.map((x) => (
            <p key={x.id}>
              <strong>
                {x.actor_id} · {x.actor_kind}:
              </strong>{" "}
              {x.body}
              {x.citations.map((c) => (
                <small key={c.reference}>
                  {" "}
                  · {c.summary} ({c.reference})
                </small>
              ))}
            </p>
          ))}
          {p.state === "open" && (
            <form
              className="stack"
              onSubmit={(e) => {
                e.preventDefault();
                const f = new FormData(e.currentTarget);
                void post(`/${p.id}/discussion`, {
                  body: f.get("body"),
                  actor_kind: "human",
                  citations: [],
                });
              }}
            >
              <label>
                Add to the record
                <textarea name="body" required />
              </label>
              <Button type="submit">Comment</Button>
            </form>
          )}
          <p>
            <strong>Ballots:</strong> {p.ballots.length}
            {p.secret_ballot && p.state === "open"
              ? " sealed until tally"
              : " attributable"}
          </p>
          {canManage && p.state === "open" && (
            <Button
              onClick={() => post(`/${p.id}/tally`, { close_early: true })}
            >
              Close and compute tally
            </Button>
          )}
          {p.tally && (
            <div className="notice">
              <strong>{p.tally.outcome}</strong> · quorum{" "}
              {p.tally.quorum_met ? "met" : "not met"} · threshold{" "}
              {p.tally.threshold_met ? "met" : "not met"}
              <p>
                {Object.entries(p.tally.counts)
                  .map(([k, v]) => `${k}: ${v}`)
                  .join(" · ")}{" "}
                · {p.tally.abstentions} abstentions ·{" "}
                {p.tally.excluded_ballots.length} ineligible ballots excluded
              </p>
              <small>
                Verifiable tally <code>{p.tally.digest}</code>
              </small>
            </div>
          )}
          {p.contests.map((x) => (
            <p className="form-error" key={x.id}>
              <strong>Contested by {x.actor_id}:</strong> {x.reason} · {x.state}
              {x.resolution ? ` — ${x.resolution}` : ""}
            </p>
          ))}
        </article>
      ))}
    </section>
  );
}
