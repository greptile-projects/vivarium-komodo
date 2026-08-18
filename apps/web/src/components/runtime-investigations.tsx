"use client";
/* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */
import { FormEvent, useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";

type Probe = {
  id: string;
  kind: string;
  preview: { audience: string };
  captures: { id: string; completeness: string; provenance: string }[];
};
type Investigation = {
  id: string;
  title: string;
  question: string;
  revision: string;
  evidence: {
    id: string;
    kind: string;
    summary: string;
    accessible: boolean;
    reason?: string;
  }[];
  correlations: {
    id: string;
    kind: string;
    resource_id: string;
    revision: string;
    path?: string;
    symbol?: string;
    relationship: string;
    status: string;
    reason?: string;
  }[];
  claims: {
    id: string;
    kind: string;
    body: string;
    actor_id: string;
    agent_session_id?: string;
    uncertainty?: string;
    status: string;
    stale: boolean;
    blocked_reasons?: string[];
    citations: {
      evidence_id?: string;
      correlation_id?: string;
      claim_id?: string;
    }[];
  }[];
  owner_requests: {
    id: string;
    owner_kind: string;
    owner_id: string;
    question: string;
    status: string;
  }[];
  agent_sessions: {
    id: string;
    agent_id: string;
    status: string;
    guidance: string[];
    expires_at: string;
  }[];
  events: { sequence: number }[];
};

export function RuntimeInvestigations({
  repository,
  workspace,
  revision,
  participants,
  probes,
}: {
  repository: string;
  workspace: string;
  revision: string;
  participants: boolean;
  probes: Probe[];
}) {
  const root = `/api/repositories/${repository}/debugging-workspaces/${workspace}/investigations`,
    [items, setItems] = useState<Investigation[]>([]),
    [error, setError] = useState(""),
    [live, setLive] = useState(0),
    [agentCredential, setAgentCredential] = useState("");
  const load = useCallback(async () => {
    const r = await fetch(root);
    if (r.ok) setItems(((await r.json()) as { items: Investigation[] }).items);
  }, [root]);
  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    const sources = items.map((i) => {
      const after = i.events.at(-1)?.sequence || 0,
        s = new EventSource(`${root}/${i.id}/events?after=${after}`);
      s.onmessage = () => {
        setLive((x) => x + 1);
        void load();
      };
      return s;
    });
    return () => sources.forEach((s) => s.close());
  }, [items.length, load, root]);
  async function post(path: string, body: unknown) {
    const r = await fetch(path, {
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
  function create(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget),
      probe = probes.find((x) => x.id === f.get("probe")),
      capture = probe?.captures.find((x) => x.id === f.get("capture"));
    void post(root, {
      title: f.get("title"),
      question: f.get("question"),
      audience: f.get("audience"),
      evidence: [
        {
          probe_id: probe?.id,
          capture_id: capture?.id,
          kind: probe?.kind,
          summary: f.get("evidence_summary"),
          audience: probe?.preview.audience,
          accessible: Boolean(capture),
          reason: capture
            ? undefined
            : "Selected evidence has no accessible sanitized capture",
        },
      ],
      correlations: [
        {
          kind: f.get("kind"),
          resource_id: f.get("resource"),
          revision: f.get("correlation_revision"),
          path: f.get("path") || undefined,
          symbol: f.get("symbol") || undefined,
          relationship: f.get("relationship"),
          status: f.get("status"),
          reason:
            f.get("status") === "inaccessible" ? f.get("reason") : undefined,
        },
      ],
    });
  }
  return (
    <section className="panel">
      <header>
        <div>
          <h4>Challengeable runtime explanation</h4>
          <small>
            {live
              ? `${live} live updates received`
              : "Streaming attributable investigation events"}
          </small>
        </div>
        <Badge>{items.length} investigations</Badge>
      </header>
      {error && (
        <p className="form-error" role="alert">
          {error.replaceAll("_", " ")}
        </p>
      )}
      {participants && probes.length > 0 && (
        <details>
          <summary>Correlate runtime evidence with code and operations</summary>
          <form className="stacked-form" onSubmit={create}>
            <label>
              Investigation title
              <input name="title" required />
            </label>
            <label>
              Framing question
              <textarea name="question" required />
            </label>
            <label>
              Sanitized probe
              <select name="probe">
                {probes.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.kind} · {p.id}
                  </option>
                ))}
              </select>
            </label>
            <label>
              Capture
              <select name="capture">
                {probes.flatMap((p) =>
                  p.captures.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.completeness} · {c.provenance}
                    </option>
                  )),
                )}
              </select>
            </label>
            <label>
              What this evidence shows
              <textarea name="evidence_summary" required />
            </label>
            <label>
              Correlation kind
              <select name="kind">
                <option>symbol</option>
                <option>commit</option>
                <option>dependency</option>
                <option>configuration</option>
                <option>infrastructure</option>
                <option>deployment</option>
                <option value="known_issue">known issue</option>
              </select>
            </label>
            <label>
              Resource ID
              <input name="resource" required />
            </label>
            <label>
              Exact revision
              <input
                name="correlation_revision"
                defaultValue={revision}
                required
              />
            </label>
            <label>
              Source path
              <input name="path" />
            </label>
            <label>
              Symbol
              <input name="symbol" />
            </label>
            <label>
              Why they are related
              <textarea name="relationship" required />
            </label>
            <label>
              Resolution
              <select name="status">
                <option>resolved</option>
                <option>inaccessible</option>
              </select>
            </label>
            <label>
              Inaccessible reason
              <input name="reason" />
            </label>
            <label>
              Audience
              <select name="audience">
                <option>participants</option>
                <option>repository</option>
              </select>
            </label>
            <Button type="submit">Open cited investigation</Button>
          </form>
        </details>
      )}
      {items.map((i) => (
        <article className="investigation-entry" key={i.id}>
          <header>
            <div>
              <strong>{i.title}</strong>
              <p>{i.question}</p>
            </div>
            <Badge>{i.claims.length} claims</Badge>
          </header>
          <p>
            <code>{i.revision}</code>
          </p>
          {i.evidence.map((e) => (
            <p key={e.id}>
              <Badge>{e.accessible ? "accessible" : "blocked"}</Badge> {e.kind}:{" "}
              {e.summary}
              {e.reason && ` · ${e.reason}`}
            </p>
          ))}
          {i.correlations.map((c) => (
            <p key={c.id}>
              <Badge>{c.kind}</Badge> {c.resource_id}@{c.revision}
              {c.path && ` · ${c.path}`}
              {c.symbol && `#${c.symbol}`} — {c.relationship} · {c.status}
              {c.reason && `: ${c.reason}`}
            </p>
          ))}
          {i.claims.map((c) => (
            <div className="panel" key={c.id}>
              <p>
                <Badge>{c.stale ? "stale" : c.status}</Badge>{" "}
                <strong>{c.kind}</strong> by {c.actor_id}
                {c.agent_session_id &&
                  ` through agent session ${c.agent_session_id}`}
              </p>
              <p>{c.body}</p>
              {c.uncertainty && <p>Uncertainty: {c.uncertainty}</p>}
              {c.blocked_reasons?.map((x) => (
                <p className="form-error" key={x}>
                  Blocked: {x}
                </p>
              ))}
            </div>
          ))}
          {participants && (
            <>
              <details>
                <summary>
                  Publish a cited hypothesis, query, finding, or challenge
                </summary>
                <form
                  className="stacked-form"
                  onSubmit={(e) => {
                    e.preventDefault();
                    const f = new FormData(e.currentTarget),
                      kind = String(f.get("kind")),
                      claim = String(f.get("claim"));
                    void post(`${root}/${i.id}/claims`, {
                      kind,
                      body: f.get("body"),
                      uncertainty: f.get("uncertainty"),
                      citations: claim
                        ? [{ claim_id: claim }]
                        : [
                            { evidence_id: i.evidence[0]?.id },
                            { correlation_id: i.correlations[0]?.id },
                          ],
                    });
                  }}
                >
                  <label>
                    Claim kind
                    <select name="kind">
                      <option>hypothesis</option>
                      <option>query</option>
                      <option>finding</option>
                      <option>uncertainty</option>
                      <option>challenge</option>
                      <option>support</option>
                    </select>
                  </label>
                  <label>
                    Explanation or reproducible query
                    <textarea name="body" required />
                  </label>
                  <label>
                    Uncertainty
                    <input name="uncertainty" />
                  </label>
                  <label>
                    Claim to challenge/support
                    <input
                      name="claim"
                      placeholder="Leave blank to cite evidence and correlation"
                    />
                  </label>
                  <Button type="submit">Publish attributable claim</Button>
                </form>
              </details>
              <details>
                <summary>Delegate bounded read-only analysis</summary>
                <form
                  onSubmit={async (e) => {
                    e.preventDefault();
                    const f = new FormData(e.currentTarget);
                    const r = await fetch(`${root}/${i.id}/agents`, {
                      method: "POST",
                      headers: { "content-type": "application/json" },
                      body: JSON.stringify({
                        agent_id: f.get("agent"),
                        mandate: f.get("mandate"),
                        evidence_ids: i.evidence.map((x) => x.id),
                        correlation_ids: i.correlations.map((x) => x.id),
                        expires_at: new Date(
                          Date.now() + Number(f.get("hours")) * 3600000,
                        ).toISOString(),
                      }),
                    });
                    if (!r.ok) {
                      setError(((await r.json()) as { error: string }).error);
                      return;
                    }
                    setAgentCredential(
                      ((await r.json()) as { credential: string }).credential,
                    );
                    await load();
                  }}
                >
                  <label>
                    Read-only agent identity
                    <input name="agent" required />
                  </label>
                  <label>
                    Bounded mandate
                    <textarea name="mandate" required />
                  </label>
                  <label>
                    Credential hours
                    <input
                      name="hours"
                      type="number"
                      min="1"
                      max="24"
                      defaultValue="1"
                      required
                    />
                  </label>
                  <Button type="submit">
                    Issue investigation-only credential
                  </Button>
                </form>
                {agentCredential && (
                  <p>
                    <strong>Shown once:</strong> <code>{agentCredential}</code>
                  </p>
                )}
              </details>
              <details>
                <summary>Request owner input</summary>
                <form
                  onSubmit={(e) => {
                    e.preventDefault();
                    const f = new FormData(e.currentTarget);
                    void post(`${root}/${i.id}/owner-requests`, {
                      owner_kind: f.get("kind"),
                      owner_id: f.get("owner"),
                      question: f.get("question"),
                    });
                  }}
                >
                  <label>
                    Owner discipline
                    <select name="kind">
                      <option>code</option>
                      <option>service</option>
                      <option>privacy</option>
                      <option>security</option>
                    </select>
                  </label>
                  <label>
                    Owner ID
                    <input name="owner" required />
                  </label>
                  <label>
                    Question
                    <textarea name="question" required />
                  </label>
                  <Button type="submit">Request attributable input</Button>
                </form>
              </details>
            </>
          )}
          {i.owner_requests.map((x) => (
            <p key={x.id}>
              <Badge>{x.status}</Badge> {x.owner_kind} owner {x.owner_id}:{" "}
              {x.question}
            </p>
          ))}
          {i.agent_sessions.map((a) => (
            <p key={a.id}>
              <Badge>{a.status}</Badge> {a.agent_id} · expires{" "}
              {new Date(a.expires_at).toLocaleString()} · guidance{" "}
              {a.guidance.join("; ") || "none"}
              {participants && a.status !== "revoked" && (
                <>
                  {" "}
                  <Button
                    onClick={() =>
                      post(`${root}/${i.id}/agents/${a.id}/controls`, {
                        action: a.status === "paused" ? "resume" : "pause",
                      })
                    }
                  >
                    {a.status === "paused" ? "Resume" : "Pause"}
                  </Button>{" "}
                  <Button
                    onClick={() =>
                      post(`${root}/${i.id}/agents/${a.id}/controls`, {
                        action: "revoke",
                      })
                    }
                  >
                    Revoke
                  </Button>
                </>
              )}
            </p>
          ))}
        </article>
      ))}
    </section>
  );
}
