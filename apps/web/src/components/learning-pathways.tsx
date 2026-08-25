"use client";

import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import Link from "next/link";
import { Badge, Button, EmptyState } from "@/components/ui";
import { Book } from "@/components/icons";

type Resource = {
  kind: string;
  label: string;
  url?: string;
  resource_id?: string;
  path?: string;
  symbol?: string;
  revision: string;
  status: "current" | "stale" | "inaccessible";
  detail: string;
};
type Exercise = {
  title: string;
  kinds?: string[];
  instructions: string;
  acceptance_criteria: string[];
  tools?: Array<{ name: string; version: string }>;
  data?: Array<{ name: string; kind: string; digest: string }>;
  setup_commands?: string[];
  maximum_cost?: number;
};
type Module = {
  id: string;
  title: string;
  why_it_matters: string;
  objectives: string[];
  expected_effort_minutes: number;
  exercises: Exercise[];
  resources: Resource[];
};
type Finding = {
  kind: string;
  module_id?: string;
  resource_label?: string;
  owner_id?: string;
  detail: string;
};
type Version = {
  number: number;
  role: string;
  outcome: string;
  prerequisites: string[];
  objectives: string[];
  supported_revisions: string[];
  modules: Module[];
  mentor_ids: string[];
  expected_effort_minutes: number;
  accessibility_needs: string[];
  localization_needs: string[];
  learner_environments: Array<{
    name: string;
    requirement: string;
    supported: boolean;
  }>;
  completion_evidence: string[];
  author_id: string;
  change_reason: string;
  created_at: string;
  findings?: Finding[];
};
type Pathway = {
  repository_id: string;
  id: string;
  current_version: number;
  versions: Version[];
};
type Attempt = {
  id: string;
  pathway_id: string;
  help_timeline: Array<{
    number: number;
    kind: string;
    author_id: string;
    guidance_kind?: string;
    body?: string;
    recipient_id?: string;
  }>;
  agent_states: Record<string, string>;
};
const lines = (v: FormDataEntryValue | null) =>
  String(v ?? "")
    .split("\n")
    .map((x) => x.trim())
    .filter(Boolean);
async function json<T>(url: string, init?: RequestInit): Promise<T> {
  const r = await fetch(`/api${url}`, init);
  const b = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(b.error || "Request failed");
  return b;
}

export function LearningPathways({
  repository,
  actor,
  revision,
}: {
  repository: string;
  actor: string;
  revision: string;
}) {
  const [items, setItems] = useState<Pathway[]>([]),
    [editing, setEditing] = useState(false),
    [error, setError] = useState(""),
    [launched, setLaunched] = useState<Attempt | null>(null);
  useEffect(() => {
    let active = true;
    json<{ items: Pathway[] }>(`/repositories/${repository}/learning-pathways`)
      .then((v) => {
        if (active) setItems(v.items);
      })
      .catch((e) => {
        if (active)
          setError(
            e instanceof Error ? e.message : "Learning pathways unavailable",
          );
      });
    return () => {
      active = false;
    };
  }, [repository]);
  async function publish(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget),
      id = String(f.get("id")),
      existing = items.find((x) => x.id === id);
    const resourceKind = String(f.get("resource_kind"));
    const resource: Omit<Resource, "status" | "detail"> = {
      kind: resourceKind,
      label: String(f.get("resource_label")),
      revision,
      path: String(f.get("resource_path")),
      symbol: String(f.get("resource_symbol")),
      resource_id: String(f.get("resource_id")),
      url: String(f.get("resource_url")),
    };
    try {
      const value = await json<Pathway>(
        `/repositories/${repository}/learning-pathways/${encodeURIComponent(id)}/versions`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            expected_version: existing?.current_version ?? 0,
            role: f.get("role"),
            outcome: f.get("outcome"),
            prerequisites: lines(f.get("prerequisites")),
            objectives: lines(f.get("objectives")),
            supported_revisions: [revision],
            mentor_ids: lines(f.get("mentors")),
            expected_effort_minutes: Number(f.get("effort")),
            accessibility_needs: lines(f.get("accessibility")),
            localization_needs: lines(f.get("localization")),
            learner_environments: [
              {
                name: f.get("environment"),
                requirement: f.get("environment_requirement"),
                supported: f.get("environment_supported") === "on",
              },
            ],
            completion_evidence: lines(f.get("evidence")),
            modules: [
              {
                id: f.get("module_id"),
                title: f.get("module_title"),
                why_it_matters: f.get("why"),
                objectives: lines(f.get("module_objectives")),
                expected_effort_minutes: Number(f.get("module_effort")),
                exercises: [
                  {
                    title: f.get("exercise_title"),
                    kinds: lines(f.get("exercise_kinds")),
                    instructions: f.get("exercise_instructions"),
                    acceptance_criteria: lines(f.get("exercise_acceptance")),
                    tools: [
                      {
                        name: String(f.get("tool_name")),
                        version: String(f.get("tool_version")),
                      },
                    ],
                    data: [
                      {
                        name: String(f.get("data_name")),
                        kind: "synthetic",
                        digest: String(f.get("data_digest")),
                      },
                    ],
                    setup_commands: lines(f.get("setup_commands")),
                    maximum_cost: Number(f.get("maximum_cost")),
                  },
                ],
                resources: [resource],
              },
            ],
            change_reason: f.get("reason"),
          }),
        },
      );
      setItems((old) => [...old.filter((x) => x.id !== value.id), value]);
      setEditing(false);
      setError("");
    } catch (x) {
      setError(x instanceof Error ? x.message : "Could not publish");
    }
  }
  async function launch(pathway: Pathway, module: Module, index: number) {
    try {
      const a = await json<Attempt>(
        `/repositories/${repository}/learning-pathways/${pathway.id}/attempts`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            pathway_version: pathway.current_version,
            module_id: module.id,
            exercise_index: index,
          }),
        },
      );
      setLaunched(a);
      setError("");
    } catch (x) {
      setError(x instanceof Error ? x.message : "Could not launch exercise");
    }
  }
  async function askForHelp(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!launched) return;
    const f = new FormData(e.currentTarget),
      kind = String(f.get("recipient_kind"));
    try {
      const a = await json<Attempt>(
        `/repositories/${repository}/learning-pathways/${String(f.get("pathway"))}/attempts/${launched.id}/help`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            kind: "question",
            recipient_kind: kind,
            recipient_id: f.get("recipient_id"),
            agent_approval_id:
              kind === "agent" ? f.get("approval_id") : undefined,
            body: f.get("question"),
            shared_event_numbers: lines(f.get("events")).map(Number),
            workspace_access: f.get("workspace_access"),
            learner_authorized: f.get("learner_authorized") === "on",
          }),
        },
      );
      setLaunched(a);
      setError("");
    } catch (x) {
      setError(x instanceof Error ? x.message : "Could not request help");
    }
  }
  async function controlAgent(
    agent: string,
    kind: "pause_agent" | "revoke_agent",
  ) {
    if (!launched) return;
    try {
      setLaunched(
        await json<Attempt>(
          `/repositories/${repository}/learning-pathways/${launched.pathway_id}/attempts/${launched.id}/help`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ kind, recipient_id: agent }),
          },
        ),
      );
      setError("");
    } catch (x) {
      setError(x instanceof Error ? x.message : "Could not control agent");
    }
  }
  if (editing)
    return (
      <section className="workspace">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Project-native curriculum</p>
            <h2>Publish learning pathway</h2>
          </div>
          <Button variant="secondary" onClick={() => setEditing(false)}>
            Cancel
          </Button>
        </div>
        {error && <p className="form-error">{error}</p>}
        <form className="card form-stack" onSubmit={publish}>
          <label>
            Pathway ID
            <input name="id" required placeholder="backend-maintainer" />
          </label>
          <label>
            Project role
            <input name="role" required />
          </label>
          <label>
            Outcome
            <textarea name="outcome" required />
          </label>
          <label>
            Prerequisites
            <textarea name="prerequisites" required />
          </label>
          <label>
            Pathway objectives
            <textarea name="objectives" required />
          </label>
          <label>
            Total expected effort (minutes)
            <input name="effort" type="number" min="1" required />
          </label>
          <label>
            Mentor IDs
            <textarea
              name="mentors"
              placeholder="One repository participant per line"
            />
          </label>
          <label>
            Accessibility needs
            <textarea
              name="accessibility"
              placeholder="One supported need per line"
            />
          </label>
          <label>
            Localization needs
            <textarea
              name="localization"
              placeholder="One language or locale need per line"
            />
          </label>
          <label>
            Learner environment
            <input
              name="environment"
              required
              placeholder="Linux development container"
            />
          </label>
          <label>
            Environment requirement
            <input name="environment_requirement" required />
          </label>
          <label>
            <input
              name="environment_supported"
              type="checkbox"
              defaultChecked
            />{" "}
            This environment is supported
          </label>
          <label>
            Required completion evidence
            <textarea name="evidence" required />
          </label>
          <h3>First ordered module</h3>
          <label>
            Module ID
            <input name="module_id" required />
          </label>
          <label>
            Title
            <input name="module_title" required />
          </label>
          <label>
            Why it matters to project work
            <textarea name="why" required />
          </label>
          <label>
            Objectives
            <textarea name="module_objectives" required />
          </label>
          <label>
            Expected effort (minutes)
            <input name="module_effort" type="number" min="1" required />
          </label>
          <label>
            Exercise title
            <input name="exercise_title" required />
          </label>
          <label>
            Exercise instructions
            <textarea name="exercise_instructions" required />
          </label>
          <label>
            Exercise acceptance criteria
            <textarea name="exercise_acceptance" required />
          </label>
          <label>
            Practice kinds
            <textarea
              name="exercise_kinds"
              placeholder={"exploration\ndebugging\ntests"}
            />
          </label>
          <label>
            Exact tool name
            <input name="tool_name" required placeholder="go" />
          </label>
          <label>
            Exact tool version
            <input name="tool_version" required placeholder="1.25.0" />
          </label>
          <label>
            Synthetic dataset name
            <input name="data_name" required />
          </label>
          <label>
            Dataset digest
            <input name="data_digest" required placeholder="sha256:…" />
          </label>
          <label>
            Setup commands
            <textarea name="setup_commands" />
          </label>
          <label>
            Maximum practice cost
            <input
              name="maximum_cost"
              type="number"
              min="0"
              step="0.01"
              defaultValue="10"
            />
          </label>
          <label>
            Exact resource type
            <select name="resource_kind">
              <option value="documentation">Documentation</option>
              <option value="symbol">Symbol</option>
              <option value="decision">Decision</option>
              <option value="issue">Issue</option>
              <option value="api">API</option>
              <option value="package">Package</option>
              <option value="contributor_guidance">Contributor guidance</option>
            </select>
          </label>
          <label>
            Resource label
            <input name="resource_label" required />
          </label>
          <label>
            Repository path
            <input name="resource_path" />
          </label>
          <label>
            Symbol locator
            <input name="resource_symbol" />
          </label>
          <label>
            Resource ID
            <input name="resource_id" />
          </label>
          <label>
            Resource URL
            <input name="resource_url" />
          </label>
          <label>
            Exact revision
            <input value={revision} readOnly />
          </label>
          <label>
            Reason for version
            <textarea name="reason" required />
          </label>
          <Button type="submit">Publish immutable version</Button>
        </form>
      </section>
    );
  return (
    <section className="workspace">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Learn through real project work</p>
          <h2>Learning pathways</h2>
        </div>
        {actor && (
          <Button onClick={() => setEditing(true)}>Publish pathway</Button>
        )}
      </div>
      {error && <p className="form-error">{error}</p>}
      {launched && (
        <section className="card">
          <p>
            <Badge>Practice ready</Badge> Detached attempt {launched.id} is
            networkless, unpublished, and isolated from authoritative branches.
          </p>
          <h3>Permission-aware help timeline</h3>
          {launched.help_timeline?.map((x) => (
            <p key={x.number}>
              <Badge>{x.guidance_kind || x.kind}</Badge> {x.author_id}:{" "}
              {x.body || `${x.kind} ${x.recipient_id || ""}`}
            </p>
          ))}
          {Object.entries(launched.agent_states || {}).map(([agent, state]) => (
            <p key={agent}>
              <Badge>{state}</Badge> {agent}{" "}
              {state === "active" && (
                <>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => controlAgent(agent, "pause_agent")}
                  >
                    Pause
                  </Button>{" "}
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => controlAgent(agent, "revoke_agent")}
                  >
                    Revoke
                  </Button>
                </>
              )}
            </p>
          ))}
          <form className="form-stack" onSubmit={askForHelp}>
            <input type="hidden" name="pathway" value={launched.pathway_id} />
            <label>
              Ask a revision-grounded question
              <textarea name="question" required />
            </label>
            <label>
              Helper type
              <select name="recipient_kind">
                <option value="mentor">Designated mentor</option>
                <option value="agent">Approved agent</option>
              </select>
            </label>
            <label>
              Mentor ID or active agent identity
              <input name="recipient_id" required />
            </label>
            <label>
              Agent approval ID
              <input name="approval_id" />
            </label>
            <label>
              Share selected exercise event numbers only
              <input
                name="events"
                placeholder="1&#10;3"
              />
            </label>
            <label>
              Workspace access
              <select name="workspace_access">
                <option value="">Timeline only</option>
                <option value="observe">May observe</option>
                <option value="join">May join</option>
              </select>
            </label>
            <label>
              <input type="checkbox" name="learner_authorized" /> Allow a
              bounded demonstration (mentors only)
            </label>
            <Button type="submit" size="sm">
              Request accountable guidance
            </Button>
          </form>
        </section>
      )}
      {items.length === 0 ? (
        <EmptyState
          icon={<Book />}
          title="No learning pathways"
          description="Collaborators have not yet made project knowledge into a maintained curriculum."
        />
      ) : (
        items.map((p) => {
          const v = p.versions.at(-1)!;
          return (
            <article className="card" key={p.id}>
              <p>
                <Badge>Version {v.number}</Badge> {v.expected_effort_minutes}{" "}
                minutes · maintained by {v.author_id}
              </p>
              <h3>{v.role}</h3>
              <p>{v.outcome}</p>
              <h4>What you will learn</h4>
              <ul>
                {v.objectives.map((x) => (
                  <li key={x}>{x}</li>
                ))}
              </ul>
              {v.findings?.map((x, i) => (
                <p className="form-error" key={`${x.kind}-${i}`}>
                  <strong>{x.kind.replaceAll("_", " ")}:</strong> {x.detail}
                </p>
              ))}
              <h4>Ordered modules</h4>
              {v.modules.map((m, i) => (
                <section key={m.id}>
                  <h5>
                    {i + 1}. {m.title}
                  </h5>
                  <p>{m.why_it_matters}</p>
                  <p>{m.expected_effort_minutes} minutes</p>
                  {m.resources.map((r, j) => (
                    <p key={j}>
                      <Badge>{r.status}</Badge>{" "}
                      {r.url ? <Link href={r.url}>{r.label}</Link> : r.label} ·{" "}
                      {r.detail}
                    </p>
                  ))}
                  <details>
                    <summary>Exercise and evidence</summary>
                    {m.exercises.map((e, j) => (
                      <div key={e.title}>
                        <strong>{e.title}</strong>
                        <p>{e.instructions}</p>
                        <p>
                          {e.tools
                            ?.map((t) => `${t.name} ${t.version}`)
                            .join(", ")}{" "}
                          ·{" "}
                          {e.data?.map((d) => `${d.kind} ${d.name}`).join(", ")}
                        </p>
                        <ul>
                          {e.acceptance_criteria.map((x) => (
                            <li key={x}>{x}</li>
                          ))}
                        </ul>
                        {actor && (
                          <Button
                            variant="secondary"
                            onClick={() => launch(p, m, j)}
                          >
                            Launch bounded practice
                          </Button>
                        )}
                      </div>
                    ))}
                  </details>
                </section>
              ))}
              <h4>Completion evidence</h4>
              <ul>
                {v.completion_evidence.map((x) => (
                  <li key={x}>{x}</li>
                ))}
              </ul>
              <p>
                <strong>Accessibility:</strong>{" "}
                {v.accessibility_needs.join(", ") || "No needs recorded"}
              </p>
              <p>
                <strong>Localization:</strong>{" "}
                {v.localization_needs.join(", ") || "No needs recorded"}
              </p>
            </article>
          );
        })
      )}
    </section>
  );
}
