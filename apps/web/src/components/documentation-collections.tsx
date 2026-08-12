"use client";

import { FormEvent, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";

type Version = {
  number: number;
  name: string;
  description: string;
  versions: Array<{
    label: string;
    source_revision: string;
    release_id?: string;
  }>;
  owner_ids: string[];
  audiences: string[];
  policy: { renderer: string; publication: string };
  author_id: string;
  change_reason: string;
  created_at: string;
};
type Collection = {
  id: string;
  current_version: number;
  history: Version[];
  pages: Array<{
    path: string;
    blob_id: string;
    content: string;
    author: string;
  }>;
  findings: Array<{ code: string; detail: string; path?: string }>;
  healthy: boolean;
};
async function json<T>(url: string, init?: RequestInit) {
  const response = await fetch(`/api${url}`, init);
  const body = await response.json();
  if (!response.ok) throw new Error(body.error || "Request failed");
  return body as T;
}
const lines = (value: FormDataEntryValue | null) =>
  String(value || "")
    .split("\n")
    .map((x) => x.trim())
    .filter(Boolean);

export function DocumentationCollections({
  repository,
  actor,
  owner,
  revision,
}: {
  repository: string;
  actor: string;
  owner: string;
  revision: string;
}) {
  const [items, setItems] = useState<Collection[]>([]);
  const [selected, setSelected] = useState("");
  const [error, setError] = useState("");
  useEffect(() => {
    json<{ items: Collection[] }>(
      `/repositories/${repository}/documentation-collections`,
    )
      .then((x) => setItems(x.items))
      .catch((e) => setError(e.message));
  }, [repository]);
  const active = items.find((x) => x.id === (selected || items[0]?.id));
  const current = active?.history.at(-1);
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    try {
      const value = await json<Collection>(
        `/repositories/${repository}/documentation-collections`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            expected_version: 0,
            name: form.get("name"),
            description: form.get("description"),
            root_path: form.get("root_path"),
            entry_paths: lines(form.get("entry_paths")),
            versions: [
              {
                label: form.get("version_label"),
                source_revision: revision,
                release_id: form.get("release_id"),
              },
            ],
            owner_ids: lines(form.get("owners")),
            audiences: lines(form.get("audiences")),
            policy: {
              navigation: "path",
              renderer: form.get("renderer"),
              publication: "maintainer_reviewed",
            },
            links: [],
            change_reason: form.get("reason"),
          }),
        },
      );
      setItems((x) => [...x, value]);
      setSelected(value.id);
      event.currentTarget.reset();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not publish");
    }
  }
  return (
    <section className="content-stack">
      <header className="section-heading">
        <div>
          <p className="eyebrow">Reviewed repository source</p>
          <h2>Documentation</h2>
          <p>
            See which project version each page explains, who owns it, and which
            reviewed source produced it.
          </p>
        </div>
      </header>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      {actor === owner && (
        <form className="card form-stack" onSubmit={create}>
          <h3>Define a collection</h3>
          <label>
            Name
            <input name="name" required />
          </label>
          <label>
            Description
            <textarea name="description" />
          </label>
          <label>
            Repository root path
            <input name="root_path" placeholder="docs" required />
          </label>
          <label>
            Pages beneath root, one per line
            <textarea name="entry_paths" placeholder="README.md" required />
          </label>
          <label>
            Version label
            <input name="version_label" placeholder="current" required />
          </label>
          <label>
            Release ID (optional)
            <input name="release_id" />
          </label>
          <label>
            Owner IDs, one per line
            <textarea name="owners" defaultValue={owner} />
          </label>
          <label>
            Audiences, one per line
            <textarea name="audiences" defaultValue="developers" required />
          </label>
          <label>
            Renderer
            <select name="renderer">
              <option value="markdown">Markdown</option>
              <option value="plain_text">Plain text</option>
            </select>
          </label>
          <label>
            Change reason
            <input name="reason" required />
          </label>
          <Button type="submit" disabled={!revision}>
            Publish reviewed collection
          </Button>
        </form>
      )}
      <div className="card">
        <h3>Collections</h3>
        {items.length === 0 ? (
          <p>No repository-backed documentation has been published.</p>
        ) : (
          items.map((c) => (
            <button
              key={c.id}
              className={active?.id === c.id ? "active" : ""}
              onClick={() => setSelected(c.id)}
            >
              {c.history.at(-1)?.name} · v{c.current_version} ·{" "}
              {c.healthy ? "healthy" : `${c.findings.length} finding(s)`}
            </button>
          ))
        )}
      </div>
      {active && current && (
        <article className="card content-stack">
          <header>
          <Badge tone={active.healthy ? "accent" : "neutral"}>
              {active.healthy ? "source healthy" : "attention required"}
            </Badge>
            <h2>{current.name}</h2>
            <p>{current.description}</p>
          </header>
          <dl>
            <div>
              <dt>Explains</dt>
              <dd>
                {current.versions.map((x) => (
                  <span key={x.label}>
                    {x.label}
                    {x.release_id ? ` · release ${x.release_id}` : ""}
                    <br />
                    <code>{x.source_revision}</code>
                  </span>
                ))}
              </dd>
            </div>
            <div>
              <dt>Maintained by</dt>
              <dd>{current.owner_ids.join(", ") || "No owner"}</dd>
            </div>
            <div>
              <dt>Reviewed contract</dt>
              <dd>
                Revision {current.number}, authored by {current.author_id} ·{" "}
                {current.change_reason}
              </dd>
            </div>
            <div>
              <dt>Audience and policy</dt>
              <dd>
                {current.audiences.join(", ")} · {current.policy.renderer} ·{" "}
                {current.policy.publication.replaceAll("_", " ")}
              </dd>
            </div>
          </dl>
          {active.findings.map((f, i) => (
            <p className="form-error" key={i}>
              <strong>{f.code.replaceAll("_", " ")}:</strong> {f.detail}{" "}
              {f.path && <code>{f.path}</code>}
            </p>
          ))}
          {active.pages.map((page) => (
            <section key={page.path}>
              <h3>{page.path}</h3>
              <p>
                Source blob <code>{page.blob_id}</code> · commit author{" "}
                {page.author}
              </p>
              <pre className="code-block">
                <code>{page.content}</code>
              </pre>
            </section>
          ))}
          {active.history.length > 1 && (
            <details>
              <summary>
                Collection history ({active.history.length} revisions)
              </summary>
              {active.history.map((h) => (
                <p key={h.number}>
                  v{h.number} · {h.author_id} · {h.change_reason}
                </p>
              ))}
            </details>
          )}
        </article>
      )}
    </section>
  );
}
