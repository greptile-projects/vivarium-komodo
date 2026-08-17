"use client";
import Link from "next/link";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";
type Evidence = {
  id: string;
  kind: string;
  name: string;
  media_type: string;
  content?: string;
  visibility: string;
};
type Question = {
  id: string;
  author_id: string;
  title: string;
  question: string;
  subject: { kind: string; resource_id?: string; label?: string };
  software_version?: string;
  environment?: string;
  goal: string;
  attempted_steps: string[];
  urgency: string;
  audience: string;
  contact: { preference: string };
  status: string;
  missing_context: string[];
  evidence: Evidence[];
  discussion: Array<{ id: string; author_id: string; body: string }>;
  history: Array<{ sequence: number; type: string; actor_id: string }>;
  related: Array<{
    kind: string;
    resource_id: string;
    title: string;
    status: string;
  }>;
};
async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const r = await fetch(`/api${path}`, init),
    b = await r.json().catch(() => ({}));
  if (!r.ok)
    throw new Error((b as { error?: string }).error || "Request failed");
  return b as T;
}
async function base64(file: File) {
  return await new Promise<string>((resolve, reject) => {
    const r = new FileReader();
    r.onerror = () => reject(r.error);
    r.onload = () => resolve(String(r.result).split(",")[1] || "");
    r.readAsDataURL(file);
  });
}
export function SupportQuestions({
  repository,
  actor,
  selected,
}: {
  repository: string;
  actor: string;
  selected?: string;
}) {
  const [items, setItems] = useState<Question[]>([]),
    [current, setCurrent] = useState<Question>(),
    [creating, setCreating] = useState(false),
    [error, setError] = useState(""),
    [comment, setComment] = useState(""),
    [related, setRelated] = useState<Question["related"]>([]);
  const load = useCallback(async () => {
    try {
      const list = await api<{ items: Question[] }>(
        `/repositories/${repository}/support-questions`,
      );
      setItems(list.items);
      if (selected)
        setCurrent(
          await api(
            `/repositories/${repository}/support-questions/${selected}`,
          ),
        );
    } catch (e) {
      setError(e instanceof Error ? e.message : "Support unavailable");
    }
  }, [repository, selected]);
  useEffect(() => {
    // Support follows the shareable selected-question URL.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);
  async function suggest(q: string) {
    if (q.trim().length < 3) {
      setRelated([]);
      return;
    }
    try {
      const v = await api<{ items: Question["related"] }>(
        `/repositories/${repository}/support-questions/suggestions?q=${encodeURIComponent(q)}`,
      );
      setRelated(v.items);
    } catch {
      setRelated([]);
    }
  }
  async function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget),
      files = f.getAll("evidence") as File[],
      evidence = [];
    for (const file of files)
      if (file.size)
        evidence.push({
          kind: f.get("evidence_kind"),
          name: file.name,
          media_type: file.type || "text/plain",
          content: await base64(file),
          visibility: f.get("evidence_visibility"),
        });
    try {
      const v = await api<Question>(
        `/repositories/${repository}/support-questions`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            title: f.get("title"),
            question: f.get("question"),
            subject: {
              kind: f.get("subject_kind"),
              resource_id: f.get("resource_id"),
              label: f.get("subject_label"),
            },
            software_version: f.get("version"),
            environment: f.get("environment"),
            goal: f.get("goal"),
            attempted_steps: String(f.get("steps"))
              .split("\n")
              .map((x) => x.trim())
              .filter(Boolean),
            urgency: f.get("urgency"),
            audience: f.get("audience"),
            contact: {
              preference: f.get("contact"),
              value: f.get("contact_value"),
            },
            evidence,
          }),
        },
      );
      window.location.assign(
        `/repositories/${repository}?view=support&support=${v.id}`,
      );
    } catch (c) {
      setError(c instanceof Error ? c.message : "Could not ask question");
    }
  }
  async function discuss(e: FormEvent) {
    e.preventDefault();
    if (!current) return;
    setCurrent(
      await api(
        `/repositories/${repository}/support-questions/${current.id}/comments`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ body: comment }),
        },
      ),
    );
    setComment("");
  }
  if (current)
    return (
      <section className="workspace">
        <div className="section-heading">
          <div>
            <p className="eyebrow">
              Developer support · {current.subject.kind}
            </p>
            <h2>{current.title}</h2>
          </div>
          <Link href={`/repositories/${repository}?view=support`}>
            All questions
          </Link>
        </div>
        <div className="card">
          <p>
            <Badge>{current.status}</Badge> <Badge>{current.urgency}</Badge>{" "}
            <Badge>{current.audience}</Badge> Asked by {current.author_id}
          </p>
          <p>{current.question}</p>
          <h3>Goal</h3>
          <p>{current.goal}</p>
          <h3>Software state</h3>
          <p>
            {current.software_version || "Version not provided"} ·{" "}
            {current.environment || "Environment not provided"}
          </p>
          {current.missing_context.length > 0 && (
            <>
              <h3>Missing diagnostic context</h3>
              <ul>
                {current.missing_context.map((x) => (
                  <li key={x}>{x.replaceAll("_", " ")}</li>
                ))}
              </ul>
            </>
          )}
          <h3>Attempted steps</h3>
          {current.attempted_steps.length ? (
            <ol>
              {current.attempted_steps.map((x, i) => (
                <li key={i}>{x}</li>
              ))}
            </ol>
          ) : (
            <p>None provided.</p>
          )}
          <h3>Permitted evidence</h3>
          {current.evidence.length ? (
            current.evidence.map((x) => (
              <p key={x.id}>
                {x.content ? (
                  <a
                    download={x.name}
                    href={`data:${x.media_type};base64,${x.content}`}
                  >
                    {x.name}
                  </a>
                ) : (
                  <>{x.name} · restricted</>
                )}{" "}
                <Badge>{x.visibility}</Badge>
              </p>
            ))
          ) : (
            <p>No evidence attached.</p>
          )}
          <h3>Related questions and issues</h3>
          {current.related.length ? (
            current.related.map((x) => (
              <p key={`${x.kind}-${x.resource_id}`}>
                {x.kind}: {x.title} <Badge>{x.status}</Badge>
              </p>
            ))
          ) : (
            <p>No visible related work found.</p>
          )}
          <p className="muted">
            Contact preference: {current.contact.preference}
          </p>
        </div>
        <div className="card">
          <h3>Discussion</h3>
          {current.discussion.map((x) => (
            <p key={x.id}>
              <strong>{x.author_id}</strong> {x.body}
            </p>
          ))}
          {actor && (
            <form onSubmit={discuss}>
              <label>
                Follow up
                <textarea
                  required
                  value={comment}
                  onChange={(e) => setComment(e.target.value)}
                />
              </label>
              <Button type="submit">Add comment</Button>
            </form>
          )}
          <h3>History</h3>
          {current.history.map((x) => (
            <p key={x.sequence}>
              {x.type} by {x.actor_id}
            </p>
          ))}
        </div>
      </section>
    );
  return (
    <section className="workspace">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Context-rich project help</p>
          <h2>Support</h2>
        </div>
        {actor && (
          <Button onClick={() => setCreating(!creating)}>
            {creating ? "Cancel" : "Ask a question"}
          </Button>
        )}
      </div>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      {creating && (
        <form className="card form-stack" onSubmit={submit}>
          <p className="muted">
            Share only sanitized evidence you are permitted to disclose.
            Maintainer-only evidence never contributes content to public
            suggestions.
          </p>
          <label>
            Title
            <input
              name="title"
              required
              maxLength={200}
              onChange={(e) => void suggest(e.target.value)}
            />
          </label>
          {related.length > 0 && (
            <div>
              <strong>Possibly related</strong>
              {related.map((x) => (
                <p key={`${x.kind}-${x.resource_id}`}>
                  {x.kind}: {x.title}
                </p>
              ))}
            </div>
          )}
          <label>
            Question
            <textarea name="question" required />
          </label>
          <label>
            What are you trying to accomplish?
            <textarea name="goal" required />
          </label>
          <label>
            Subject
            <select name="subject_kind">
              <option value="repository">Repository</option>
              <option value="package">Package</option>
              <option value="release">Release</option>
              <option value="api">API</option>
              <option value="journey">Documented journey</option>
              <option value="error">Error</option>
            </select>
          </label>
          <label>
            Subject resource ID
            <input
              name="resource_id"
              placeholder="Required except for repository"
            />
          </label>
          <label>
            Subject label
            <input name="subject_label" />
          </label>
          <label>
            Software version
            <input
              name="version"
              placeholder="Exact version, release, or commit"
            />
          </label>
          <label>
            Environment
            <textarea
              name="environment"
              placeholder="OS, runtime, SDK, deployment, relevant configuration"
            />
          </label>
          <label>
            What have you tried?
            <textarea name="steps" placeholder="One step per line" />
          </label>
          <label>
            Urgency
            <select name="urgency" defaultValue="normal">
              <option>low</option>
              <option>normal</option>
              <option>high</option>
              <option>urgent</option>
            </select>
          </label>
          <label>
            Audience
            <select name="audience">
              <option value="public">Public</option>
              <option value="repository">Repository participants</option>
            </select>
          </label>
          <label>
            Contact preference
            <select name="contact">
              <option value="thread">This thread</option>
              <option value="none">No contact</option>
              <option value="email">Email</option>
            </select>
          </label>
          <label>
            Email (email preference only)
            <input type="email" name="contact_value" />
          </label>
          <label>
            Evidence type
            <select name="evidence_kind">
              <option value="log">Sanitized log</option>
              <option value="configuration">Sanitized configuration</option>
              <option value="sample_code">Sample code</option>
            </select>
          </label>
          <label>
            Evidence visibility
            <select name="evidence_visibility">
              <option value="audience">Same audience</option>
              <option value="maintainers">Maintainers and me only</option>
            </select>
          </label>
          <label>
            Attach permitted evidence
            <input
              name="evidence"
              type="file"
              multiple
              accept="text/plain,application/json,application/yaml,text/yaml,text/x-go,text/javascript,text/typescript"
            />
          </label>
          <Button type="submit">Ask question</Button>
        </form>
      )}
      <div className="card-list">
        {items.map((x) => (
          <Link
            className="card"
            href={`/repositories/${repository}?view=support&support=${x.id}`}
            key={x.id}
          >
            <p>
              <Badge>{x.status}</Badge> <Badge>{x.urgency}</Badge>{" "}
              <Badge>{x.subject.kind}</Badge>
            </p>
            <h3>{x.title}</h3>
            <p>{x.goal}</p>
            {x.missing_context.length > 0 && (
              <small>Missing: {x.missing_context.join(", ")}</small>
            )}
          </Link>
        ))}
      </div>
    </section>
  );
}
