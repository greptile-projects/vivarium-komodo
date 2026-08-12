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
type DocumentationTask = {
  id: string;
  collection_id: string;
  title: string;
  path: string;
  revision: string;
  mode: string;
  branch?: string;
  workspace_id?: string;
  creator_id: string;
  origin: { kind: string; resource_id: string };
  evidence: string[];
  events: Array<{
    sequence: number;
    type: string;
    actor_id: string;
    body?: string;
    draft?: string;
    rendered?: string;
    uncertainty?: string;
    citations?: string[];
    references?: Array<{
      path: string;
      start_line: number;
      end_line: number;
      blob_id: string;
      excerpt: string;
    }>;
  }>;
};
type Edition = {
  id: string; collection_id: string; collection_version: number; source_revision: string;
  merge_revision: string; archived: boolean; stable_url: string; published_at: string;
  versions: Array<{label:string;release_id?:string}>; audiences:string[];
  pages: Array<{path:string;blob_id:string;rendered:string}>;
};
type ReaderFeedback = { id:string; collection_id:string; page_path?:string; kind:string; body:string; reporter_id:string; triage?:{kind:string;resource_id:string} };
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
  const [tasks, setTasks] = useState<DocumentationTask[]>([]);
  const [taskID, setTaskID] = useState("");
  const [editions,setEditions]=useState<Edition[]>([]);
  const [feedback,setFeedback]=useState<ReaderFeedback[]>([]);
  const [search,setSearch]=useState("");
  useEffect(() => {
    json<{ items: Collection[] }>(
      `/repositories/${repository}/documentation-collections`,
    )
      .then((x) => setItems(x.items))
      .catch((e) => setError(e.message));
  }, [repository]);
  useEffect(()=>{json<{items:Edition[]}>(`/repositories/${repository}/documentation`).then(x=>setEditions(x.items)).catch(()=>{});},[repository]);
  useEffect(()=>{if(actor)json<{items:ReaderFeedback[]}>(`/repositories/${repository}/documentation-feedback`).then(x=>setFeedback(x.items)).catch(()=>{});},[repository,actor]);
  useEffect(() => {
    json<{ items: DocumentationTask[] }>(
      `/repositories/${repository}/documentation-tasks`,
    )
      .then((x) => setTasks(x.items))
      .catch(() => {});
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
  async function openTask(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!active) return;
    const f = new FormData(event.currentTarget);
    try {
      const value = await json<DocumentationTask>(
        `/repositories/${repository}/documentation-collections/${active.id}/tasks`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            title: f.get("title"),
            path: f.get("path"),
            revision,
            mode: f.get("mode"),
            branch: f.get("branch"),
            origin: {
              kind: f.get("origin_kind"),
              resource_id: f.get("origin_id"),
            },
            evidence: lines(f.get("evidence")),
          }),
        },
      );
      setTasks((x) => [...x, value]);
      setTaskID(value.id);
      event.currentTarget.reset();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not open task");
    }
  }
  async function addEvent(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const task = tasks.find((x) => x.id === taskID);
    if (!task) return;
    const f = new FormData(event.currentTarget),
      type = String(f.get("type"));
    try {
      const value = await json<DocumentationTask>(
        `/repositories/${repository}/documentation-tasks/${task.id}/events`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            type,
            body: f.get("body"),
            draft: type === "draft" ? f.get("body") : "",
            citations: lines(f.get("citations")),
            uncertainty: f.get("uncertainty"),
            references: f.get("reference_path")
              ? [
                  {
                    path: f.get("reference_path"),
                    start_line: Number(f.get("start_line")),
                    end_line: Number(f.get("end_line")),
                    revision: task.revision,
                  },
                ]
              : [],
          }),
        },
      );
      setTasks((x) => x.map((y) => (y.id === value.id ? value : y)));
      event.currentTarget.reset();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not collaborate");
    }
  }
  const activeTask = tasks.find((x) => x.id === taskID);
  const visibleEditions=editions.filter(x=>x.collection_id===active?.id&&(!search||x.pages.some(p=>(p.path+" "+p.rendered).toLowerCase().includes(search.toLowerCase()))));
  async function report(event:FormEvent<HTMLFormElement>){event.preventDefault();const f=new FormData(event.currentTarget),edition=visibleEditions[0];if(!edition)return;try{const value=await json<ReaderFeedback>(`/repositories/${repository}/documentation/${edition.id}/feedback`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({kind:f.get("kind"),page_path:f.get("page_path"),query:f.get("query"),expected_version:f.get("expected_version"),body:f.get("body"),evidence:[]})});setFeedback(x=>[...x,value]);event.currentTarget.reset()}catch(e){setError(e instanceof Error?e.message:"Could not send feedback")}}
  async function triage(item:ReaderFeedback,event:FormEvent<HTMLFormElement>){event.preventDefault();const f=new FormData(event.currentTarget);try{const value=await json<ReaderFeedback>(`/repositories/${repository}/documentation-feedback/${item.id}/triage`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({kind:f.get("kind"),resource_id:f.get("resource_id")})});setFeedback(x=>x.map(y=>y.id===value.id?value:y))}catch(e){setError(e instanceof Error?e.message:"Could not triage")}}
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
      {active&&<section className="card content-stack"><header><p className="eyebrow">Published guidance</p><h3>Reader editions</h3><p>Stable editions preserve the exact reviewed pages and clearly mark versions replaced by a later publication.</p></header><label>Search this collection<input type="search" value={search} onChange={e=>setSearch(e.target.value)} placeholder="Search paths and guidance" /></label>{visibleEditions.length===0?<p>No published edition matches this version or search.</p>:visibleEditions.map(edition=><article className="card" key={edition.id}><Badge tone={edition.archived?"neutral":"accent"}>{edition.archived?"archived edition":"current edition"}</Badge><h3>{edition.versions.map(v=>v.label).join(", ")}</h3><p>For {edition.audiences.join(", ")} · source <code>{edition.source_revision}</code> · merged as <code>{edition.merge_revision}</code></p><a href={edition.stable_url}>Stable link</a>{edition.pages.map(page=><section key={page.path}><h4>{page.path}</h4><div dangerouslySetInnerHTML={{__html:page.rendered}} /></section>)}</article>)}</section>}
      {active&&actor&&visibleEditions[0]&&<section className="card content-stack"><h3>Report a documentation outcome</h3><p>Your report stays attached to this exact edition and page.</p><form className="form-stack" onSubmit={report}><label>Outcome<select name="kind"><option value="page_feedback">Page feedback</option><option value="failed_example">Failed example</option><option value="search_miss">Search miss</option><option value="version_mismatch">Version mismatch</option></select></label><label>Page path<input name="page_path" placeholder={visibleEditions[0].pages[0]?.path} /></label><label>Search query (for a miss)<input name="query" /></label><label>Expected project version<input name="expected_version" /></label><label>What happened<textarea name="body" required /></label><Button type="submit">Send to maintainers</Button></form></section>}
      {active&&actor===owner&&feedback.filter(x=>x.collection_id===active.id).length>0&&<section className="card content-stack"><h3>Reader outcomes</h3>{feedback.filter(x=>x.collection_id===active.id).map(item=><article className="card" key={item.id}><Badge>{item.kind.replaceAll("_"," ")}</Badge><p>{item.body}</p><small>{item.page_path||"Search surface"} · {item.reporter_id}</small>{item.triage?<p>Linked {item.triage.kind.replaceAll("_"," ")} <code>{item.triage.resource_id}</code></p>:<form className="form-stack" onSubmit={e=>triage(item,e)}><label>Accountable workflow<select name="kind"><option value="documentation_task">Documentation task</option><option value="issue">Issue</option><option value="proposal">Proposal</option></select></label><label>Existing resource ID<input name="resource_id" required /></label><Button type="submit">Link and triage</Button></form>}</article>)}</section>}
      {active && actor && (
        <section className="card content-stack">
          <h3>Grounded documentation tasks</h3>
          <p>
            Open a revision-pinned draft from current project evidence.
            Repository permissions govern this surface and any linked workspace.
          </p>
          <form className="form-stack" onSubmit={openTask}>
            <label>
              Task title
              <input name="title" required />
            </label>
            <label>
              Page beneath collection root
              <input name="path" required placeholder="guide.md" />
            </label>
            <label>
              Origin
              <select name="origin_kind">
                <option value="proposal">Proposal</option>
                <option value="issue">Issue</option>
                <option value="pull_request">Pull request</option>
                <option value="release">Release</option>
                <option value="code_investigation">Code investigation</option>
                <option value="stewardship_opportunity">
                  Stewardship opportunity
                </option>
              </select>
            </label>
            <label>
              Origin ID
              <input name="origin_id" required />
            </label>
            <label>
              Evidence notes, one per line
              <textarea name="evidence" />
            </label>
            <label>
              Editing mode
              <select name="mode">
                <option value="workspace">Shared workspace</option>
                <option value="branch">Scoped branch</option>
              </select>
            </label>
            <label>
              Branch (required for branch mode)
              <input name="branch" placeholder="docs/update-guide" />
            </label>
            <Button type="submit">Open grounded task</Button>
          </form>
          {tasks
            .filter((x) => x.collection_id === active.id)
            .map((t) => (
              <button
                key={t.id}
                className={taskID === t.id ? "active" : ""}
                onClick={() => setTaskID(t.id)}
              >
                {t.title} · {t.origin.kind.replaceAll("_", " ")} · {t.mode}
              </button>
            ))}
          {activeTask && (
            <div className="content-stack">
              <h3>{activeTask.title}</h3>
              <p>
                <code>{activeTask.path}</code> at{" "}
                <code>{activeTask.revision}</code>
                {activeTask.workspace_id && (
                  <>
                    {" "}
                    · workspace <code>{activeTask.workspace_id}</code>
                  </>
                )}
              </p>
              {activeTask.events.map((e) => (
                <article className="card" key={e.sequence}>
                  <small>
                    {e.actor_id} · {e.type}
                  </small>
                  {e.rendered ? (
                    <div dangerouslySetInnerHTML={{ __html: e.rendered }} />
                  ) : (
                    <p>{e.body}</p>
                  )}
                  {e.uncertainty && (
                    <p>
                      <strong>Uncertainty:</strong> {e.uncertainty}
                    </p>
                  )}
                  {e.references?.map((r, i) => (
                    <pre className="code-block" key={i}>
                      <code>
                        {r.path}:{r.start_line}-{r.end_line} · {r.blob_id}
                        {"\n"}
                        {r.excerpt}
                      </code>
                    </pre>
                  ))}
                </article>
              ))}
              <form className="form-stack" onSubmit={addEvent}>
                <label>
                  Contribution
                  <select name="type">
                    <option value="draft">Rendered draft</option>
                    <option value="discussion">Discussion</option>
                    <option value="suggestion">
                      Agent or human suggestion
                    </option>
                  </select>
                </label>
                <label>
                  Draft, comment, or suggestion
                  <textarea name="body" required />
                </label>
                <label>
                  Citations, one per line
                  <textarea
                    name="citations"
                    placeholder="Required for suggestions"
                  />
                </label>
                <label>
                  Uncertainty
                  <textarea name="uncertainty" />
                </label>
                <label>
                  Code reference path
                  <input name="reference_path" />
                </label>
                <label>
                  Start line
                  <input
                    name="start_line"
                    type="number"
                    min="1"
                    defaultValue="1"
                  />
                </label>
                <label>
                  End line
                  <input
                    name="end_line"
                    type="number"
                    min="1"
                    defaultValue="1"
                  />
                </label>
                <Button type="submit">Add attributable contribution</Button>
              </form>
            </div>
          )}
        </section>
      )}
    </section>
  );
}
