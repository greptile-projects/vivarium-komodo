/* eslint-disable react-hooks/set-state-in-effect */
"use client";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";

type Participant = {
  id: string;
  kind: "human" | "agent";
  user_id?: string;
  agent_identity?: string;
  role: string;
  consent: string;
  invited_by_id: string;
};
type Incubator = {
  id: string;
  title: string;
  audience: string;
  problem: string;
  desired_outcome: string;
  constraints: string[];
  success_measures: string[];
  sponsor_ids: string[];
  decision_rights: string[];
  visibility: string;
  source: { kind: string; label?: string; status: string; detail?: string };
  created_by_id: string;
  participants: Participant[];
  discussion: {
    id: string;
    author_id: string;
    body: string;
    created_at: string;
  }[];
  evidence: {
    id: string;
    kind: string;
    reference: string;
    summary: string;
    added_by_id: string;
  }[];
  assumptions: {
    id: string;
    statement: string;
    status: string;
    added_by_id: string;
  }[];
  scope_changes: {
    id: string;
    rationale: string;
    actor_id: string;
    created_at: string;
  }[];
  duplicate_incubator_ids: string[];
  authority_granted: boolean;
  updated_at: string;
};
const lines = (v: FormDataEntryValue | null) =>
  String(v || "")
    .split("\n")
    .map((x) => x.trim())
    .filter(Boolean);
async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const r = await fetch(`/api${path}`, {
    ...init,
    headers: { "content-type": "application/json", ...init?.headers },
  });
  if (!r.ok) {
    const x = await r.json().catch(() => ({ error: "unavailable" }));
    throw new Error(String(x.error).replaceAll("_", " "));
  }
  return r.json();
}
export function ProjectIncubators() {
  const [items, setItems] = useState<Incubator[]>([]),
    [selected, setSelected] = useState(""),
    [error, setError] = useState("");
  const load = useCallback(async () => {
    try {
      const x = await request<{ items: Incubator[] }>("/project-incubators");
      setItems(x.items);
      if (!selected && x.items[0]) setSelected(x.items[0].id);
    } catch (e) {
      setError((e as Error).message);
    }
  }, [selected]);
  useEffect(() => {
    void load();
  }, [load]);
  async function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget),
      kind = String(f.get("source_kind")),
      body = {
        title: f.get("title"),
        audience: f.get("audience"),
        problem: f.get("problem"),
        desired_outcome: f.get("outcome"),
        constraints: lines(f.get("constraints")),
        success_measures: lines(f.get("measures")),
        sponsor_ids: lines(f.get("sponsors")),
        decision_rights: lines(f.get("rights")),
        visibility: f.get("visibility"),
        source: {
          kind,
          repository_id: f.get("repository_id") || undefined,
          organization_id: f.get("organization_id") || undefined,
          resource_id: kind === "idea" ? undefined : f.get("resource_id"),
          label: f.get("source_label") || undefined,
        },
      };
    try {
      const x = await request<Incubator>("/project-incubators", {
        method: "POST",
        body: JSON.stringify(body),
      });
      setSelected(x.id);
      setError("");
      e.currentTarget.reset();
      await load();
    } catch (x) {
      setError((x as Error).message);
    }
  }
  async function comment(e: FormEvent<HTMLFormElement>, id: string) {
    e.preventDefault();
    const form = e.currentTarget,
      f = new FormData(form);
    try {
      await request(`/project-incubators/${id}/comments`, {
        method: "POST",
        body: JSON.stringify({ body: f.get("body") }),
      });
      form.reset();
      await load();
    } catch (x) {
      setError((x as Error).message);
    }
  }
  async function append(
    e: FormEvent<HTMLFormElement>,
    path: string,
    make: (f: FormData) => unknown,
  ) {
    e.preventDefault();
    const form = e.currentTarget;
    try {
      await request(path, {
        method: "POST",
        body: JSON.stringify(make(new FormData(form))),
      });
      form.reset();
      setError("");
      await load();
    } catch (x) {
      setError((x as Error).message);
    }
  }
  const current = items.find((x) => x.id === selected);
  return (
    <section className="package-catalog">
      <div className="package-catalog-hero">
        <span className="eyebrow">
          Shared need → consented community → deliberate project
        </span>
        <h1>Project incubators</h1>
        <p>
          Establish why a project should exist and who may shape it before
          choosing a repository, framework, or owner.
        </p>
        <details className="panel">
          <summary>Open an incubator</summary>
          <form className="stacked-form" onSubmit={create}>
            <label>
              Working title
              <input name="title" required />
            </label>
            <label>
              Who is affected?
              <textarea name="audience" required />
            </label>
            <label>
              Problem
              <textarea name="problem" required />
            </label>
            <label>
              Desired outcome
              <textarea name="outcome" required />
            </label>
            <label>
              Constraints <small>one per line</small>
              <textarea name="constraints" />
            </label>
            <label>
              Success measures <small>one per line</small>
              <textarea name="measures" required />
            </label>
            <label>
              Sponsor user IDs <small>one per line</small>
              <textarea name="sponsors" required />
            </label>
            <label>
              Decision rights <small>who decides what, one per line</small>
              <textarea name="rights" required />
            </label>
            <label>
              Visibility
              <select name="visibility">
                <option value="participants">Participants</option>
                <option value="public">Public</option>
              </select>
            </label>
            <label>
              Origin
              <select name="source_kind">
                <option value="idea">New idea</option>
                <option value="feedback">Product feedback</option>
                <option value="support_gap">Support gap</option>
                <option value="governed_proposal">Governed proposal</option>
              </select>
            </label>
            <label>
              Source repository ID <input name="repository_id" />
            </label>
            <label>
              Source organization ID <input name="organization_id" />
            </label>
            <label>
              Source resource ID <input name="resource_id" />
            </label>
            <label>
              Source label <input name="source_label" />
            </label>
            <Button type="submit">Open collaborative home</Button>
          </form>
        </details>
      </div>
      {error && <p className="form-error">{error}</p>}
      <div className="package-catalog-layout">
        <section className="package-results">
          <header>
            <h2>Incubators</h2>
            <span>{items.length}</span>
          </header>
          {items.map((x) => (
            <button
              className={x.id === selected ? "selected" : ""}
              onClick={() => setSelected(x.id)}
              key={x.id}
            >
              <span>
                <strong>{x.title}</strong>
                <Badge>{x.visibility}</Badge>
              </span>
              <small>{x.problem}</small>
              {x.duplicate_incubator_ids.length > 0 && (
                <small>
                  Possible duplicate · {x.duplicate_incubator_ids.length}
                </small>
              )}
            </button>
          ))}
        </section>
        <article className="package-inspector">
          {current ? (
            <>
              <header>
                <div>
                  <span className="eyebrow">
                    {current.source.kind} · {current.source.status}
                  </span>
                  <h2>{current.title}</h2>
                </div>
                <Badge>
                  {
                    current.participants.filter((x) => x.consent === "accepted")
                      .length
                  }{" "}
                  shaping
                </Badge>
              </header>
              {current.source.status === "inaccessible" && (
                <p className="package-warning">{current.source.detail}</p>
              )}
              {current.duplicate_incubator_ids.length > 0 && (
                <p className="package-warning">
                  <strong>Potential duplicate initiatives:</strong>{" "}
                  {current.duplicate_incubator_ids.join(", ")}
                </p>
              )}
              <section>
                <h3>Why this project?</h3>
                <p>
                  <strong>Audience:</strong> {current.audience}
                </p>
                <p>
                  <strong>Problem:</strong> {current.problem}
                </p>
                <p>
                  <strong>Desired outcome:</strong> {current.desired_outcome}
                </p>
                <p>
                  <strong>Success:</strong>{" "}
                  {current.success_measures.join(" · ")}
                </p>
                <p>
                  <strong>Constraints:</strong>{" "}
                  {current.constraints.join(" · ") || "None recorded"}
                </p>
              </section>
              <section>
                <h3>Governance before ownership</h3>
                <p>
                  <strong>Sponsors:</strong> {current.sponsor_ids.join(", ")}
                </p>
                <p>
                  <strong>Decision rights:</strong>{" "}
                  {current.decision_rights.join(" · ")}
                </p>
                <p>
                  <strong>No authority granted:</strong> incubator participation
                  creates no repository, Git, agent, credential, approval, or
                  operational access.
                </p>
                {current.participants.map((p) => (
                  <div key={p.id}>
                    <p>
                      <Badge>{p.consent}</Badge> {p.kind}{" "}
                      {p.user_id || p.agent_identity} · {p.role} · invited by{" "}
                      {p.invited_by_id}
                    </p>
                    {p.kind === "human" && p.consent === "pending" && (
                      <p>
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() =>
                            void request(
                              `/project-incubators/${current.id}/participants/${p.id}/consent`,
                              {
                                method: "POST",
                                body: JSON.stringify({ decision: "accepted" }),
                              },
                            )
                              .then(load)
                              .catch((x) => setError((x as Error).message))
                          }
                        >
                          Accept as me
                        </Button>{" "}
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={() =>
                            void request(
                              `/project-incubators/${current.id}/participants/${p.id}/consent`,
                              {
                                method: "POST",
                                body: JSON.stringify({ decision: "declined" }),
                              },
                            )
                              .then(load)
                              .catch((x) => setError((x as Error).message))
                          }
                        >
                          Decline as me
                        </Button>
                      </p>
                    )}
                  </div>
                ))}
              </section>
              <section>
                <h3>Evidence and assumptions</h3>
                {current.evidence.map((x) => (
                  <p key={x.id}>
                    <strong>{x.kind}:</strong> {x.summary} ({x.reference}) ·{" "}
                    {x.added_by_id}
                  </p>
                ))}
                {current.assumptions.map((x) => (
                  <p key={x.id}>
                    <Badge>{x.status}</Badge> {x.statement} · {x.added_by_id}
                  </p>
                ))}
                {current.scope_changes.map((x) => (
                  <p key={x.id}>
                    <strong>Scope changed:</strong> {x.rationale} · {x.actor_id}
                  </p>
                ))}
              </section>
              <section>
                <h3>Invite collaborators</h3>
                <p>
                  Human consent stays pending until the named user responds.
                  Agents must cite an exact active onboarding identity.
                </p>
                <form
                  className="stacked-form"
                  onSubmit={(e) =>
                    append(
                      e,
                      `/project-incubators/${current.id}/participants`,
                      (f) => ({
                        kind: f.get("kind"),
                        user_id: f.get("user_id") || undefined,
                        agent_identity: f.get("agent_identity") || undefined,
                        onboarding_scope_kind: f.get("scope_kind") || undefined,
                        onboarding_scope_id: f.get("scope_id") || undefined,
                        onboarding_id: f.get("onboarding_id") || undefined,
                        role: f.get("role"),
                      }),
                    )
                  }
                >
                  <label>
                    Kind
                    <select name="kind">
                      <option value="human">Human</option>
                      <option value="agent">Approved agent</option>
                    </select>
                  </label>
                  <label>
                    Human user ID
                    <input name="user_id" />
                  </label>
                  <label>
                    Agent identity
                    <input name="agent_identity" />
                  </label>
                  <label>
                    Onboarding scope kind
                    <input
                      name="scope_kind"
                      placeholder="repository or organization"
                    />
                  </label>
                  <label>
                    Scope ID
                    <input name="scope_id" />
                  </label>
                  <label>
                    Active onboarding ID
                    <input name="onboarding_id" />
                  </label>
                  <label>
                    Shaping role
                    <input name="role" required />
                  </label>
                  <Button type="submit" size="sm">
                    Invite
                  </Button>
                </form>
              </section>
              <section>
                <h3>Add evidence or an assumption</h3>
                <form
                  className="stacked-form"
                  onSubmit={(e) =>
                    append(
                      e,
                      `/project-incubators/${current.id}/evidence`,
                      (f) => ({
                        kind: f.get("kind"),
                        reference: f.get("reference"),
                        summary: f.get("summary"),
                        visibility: f.get("visibility"),
                      }),
                    )
                  }
                >
                  <label>
                    Evidence kind
                    <input name="kind" required />
                  </label>
                  <label>
                    Citable reference
                    <input name="reference" required />
                  </label>
                  <label>
                    Summary
                    <textarea name="summary" required />
                  </label>
                  <label>
                    Visibility
                    <select name="visibility">
                      <option value="participants">Participants</option>
                      <option value="public">Public</option>
                    </select>
                  </label>
                  <Button type="submit" size="sm">
                    Add evidence
                  </Button>
                </form>
                <form
                  className="stacked-form"
                  onSubmit={(e) =>
                    append(
                      e,
                      `/project-incubators/${current.id}/assumptions`,
                      (f) => ({ statement: f.get("statement") }),
                    )
                  }
                >
                  <label>
                    Testable assumption
                    <textarea name="statement" required />
                  </label>
                  <Button type="submit" size="sm">
                    Record assumption
                  </Button>
                </form>
              </section>
              <section>
                <h3>Discussion</h3>
                {current.discussion.map((x) => (
                  <p key={x.id}>
                    <strong>{x.author_id}</strong> {x.body}
                  </p>
                ))}
                <form
                  className="stacked-form"
                  onSubmit={(e) => comment(e, current.id)}
                >
                  <label>
                    Add to the discussion
                    <textarea name="body" required />
                  </label>
                  <Button type="submit" size="sm">
                    Comment
                  </Button>
                </form>
              </section>
            </>
          ) : (
            <div className="empty-state">
              Open the first pre-repository project home.
            </div>
          )}
        </article>
      </div>
    </section>
  );
}
