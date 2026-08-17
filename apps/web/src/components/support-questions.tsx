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
  answers: Array<{
    id: string; current_revision_id: string;
    revisions: Array<{id:string;revision:number;supersedes_id?:string;author_id:string;author_kind:string;summary:string;instructions:string[];applicable_versions:string[];uncertainty?:string;claims:Array<{id:string;text:string;mode:string;uncertainty?:string;citations:Array<{kind:string;resource_id?:string;revision:string;path?:string;symbol?:string;line_start?:number;line_end?:number;label?:string}>}>}>;
    feedback: Array<{id:string;revision_id:string;claim_id?:string;kind:string;body?:string;actor_id:string}>;
  }>;
};
type Verification = { attempt: { id:string; answer_id:string; answer_revision_id:string; source_revision:string; software_version:string; created_by_id:string; state:string; result?:string; failure_reason?:string; cost_units:number; environment:{name:string;image_digest:string}; events:Array<{sequence:number;type:string;command?:string;stream?:string;message?:string;exit_code?:number}>; artifacts:Array<{path:string;sha256:string;size:number;content:string}> }; stale:boolean; stale_reasons:string[] };
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
    [related, setRelated] = useState<Question["related"]>([]),
    [verifications, setVerifications] = useState<Verification[]>([]);
  const load = useCallback(async () => {
    try {
      const list = await api<{ items: Question[] }>(
        `/repositories/${repository}/support-questions`,
      );
      setItems(list.items);
      if (selected) {
        setCurrent(
          await api(
            `/repositories/${repository}/support-questions/${selected}`,
          ),
        );
        setVerifications((await api<{items:Verification[]}>(`/repositories/${repository}/support-questions/${selected}/verifications`)).items);
      }
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
  async function answer(e: FormEvent<HTMLFormElement>) {
    e.preventDefault(); if(!current)return; const f=new FormData(e.currentTarget), parts=String(f.get("citation")||"").split(" | ");
    try { setCurrent(await api(`/repositories/${repository}/support-questions/${current.id}/answers`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({author_kind:f.get("author_kind"),summary:f.get("summary"),instructions:String(f.get("instructions")).split("\n").filter(Boolean),applicable_versions:String(f.get("versions")).split("\n").filter(Boolean),uncertainty:f.get("uncertainty"),claims:[{text:f.get("claim"),mode:f.get("mode"),uncertainty:f.get("claim_uncertainty"),citations:[{kind:parts[0],resource_id:parts[1],revision:parts[2],path:parts[3],symbol:parts[4],line_start:Number(parts[5]||0),line_end:Number(parts[6]||0),label:parts[7],visibility:current.audience}]}]})})); e.currentTarget.reset(); }
    catch(c){setError(c instanceof Error?c.message:"Could not publish guidance")}
  }
  async function feedback(answerId:string, revisionId:string, claimId:string|undefined, kind:string) {
    if(!current)return; const body=kind==="endorsement"?"":window.prompt(kind==="clarification"?"What context is missing?":"Explain your feedback")||""; if(kind!=="endorsement"&&!body)return;
    setCurrent(await api(`/repositories/${repository}/support-questions/${current.id}/answers/${answerId}/feedback`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({revision_id:revisionId,claim_id:claimId,kind,body})}));
  }
  async function verify(e:FormEvent<HTMLFormElement>,answerId:string,revisionId:string) {
    e.preventDefault(); if(!current)return; const f=new FormData(e.currentTarget), files=f.getAll("inputs") as File[], inputs=[];
    for(const file of files)if(file.size)inputs.push({name:file.name,media_type:file.type||"text/plain",content:await base64(file)});
    const dependencies=Object.fromEntries(String(f.get("dependencies")||"").split("\n").filter(Boolean).map(x=>{const [name,...version]=x.split("=");return [name.trim(),version.join("=").trim()]}));
    try { await api(`/repositories/${repository}/support-questions/${current.id}/verifications`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({answer_id:answerId,answer_revision_id:revisionId,source_revision:f.get("source_revision"),software_version:f.get("software_version"),environment:{name:current.environment,image_digest:f.get("environment_digest"),tools:String(f.get("tools")||"").split("\n").filter(Boolean),resources:{cpu_seconds:Number(f.get("cpu")),memory_mb:Number(f.get("memory")),disk_mb:Number(f.get("disk"))}},dependencies,sanitized_inputs:inputs,artifact_paths:String(f.get("artifacts")||"").split("\n").filter(Boolean),cost_units:Number(f.get("cost")||0)})}); setTimeout(()=>void load(),250); }
    catch(c){setError(c instanceof Error?c.message:"Could not verify answer")}
  }
  async function rerun(id:string){if(!current)return;try{await api(`/repositories/${repository}/support-questions/${current.id}/verifications/${id}/reruns`,{method:"POST",headers:{"Content-Type":"application/json"},body:"{}"});setTimeout(()=>void load(),250)}catch(c){setError(c instanceof Error?c.message:"Could not rerun verification")}}
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
          <h3>Grounded guidance</h3>
          <p className="muted">Claims separate verified evidence from inference and uncertainty. Citations are checked against context visible to this thread.</p>
          {current.answers?.map(a=>{const r=a.revisions.find(x=>x.id===a.current_revision_id)!;return <article className="panel" key={a.id}><p><Badge>revision {r.revision}</Badge> <Badge>{r.author_kind}</Badge> by {r.author_id}</p><h4>{r.summary}</h4><p>Applies to: {r.applicable_versions.join(", ")}</p><ol>{r.instructions.map((x,i)=><li key={i}>{x}</li>)}</ol>{r.uncertainty&&<p><strong>Uncertainty:</strong> {r.uncertainty}</p>}{r.claims.map(c=><section key={c.id}><p><Badge>{c.mode}</Badge> {c.text}</p>{c.uncertainty&&<p className="muted">{c.uncertainty}</p>}<ul>{c.citations.map((x,i)=><li key={i}>{x.kind} · {x.label||x.path||x.resource_id} @ <code>{x.revision}</code>{x.line_start?` lines ${x.line_start}–${x.line_end}`:""}</li>)}</ul>{actor&&<p><Button variant="secondary" onClick={()=>void feedback(a.id,r.id,c.id,"endorsement")}>Endorse</Button> <Button variant="secondary" onClick={()=>void feedback(a.id,r.id,c.id,"challenge")}>Challenge</Button> <Button variant="secondary" onClick={()=>void feedback(a.id,r.id,c.id,"clarification")}>Request clarification</Button></p>}</section>)}{a.feedback.filter(x=>x.revision_id===r.id).map(x=><p key={x.id}><Badge>{x.kind}</Badge> {x.actor_id} {x.body}</p>)}</article>})}
          {!current.answers?.length&&<p>No answer proposed yet.</p>}
          {actor&&<details><summary>Propose grounded guidance</summary><form className="form-stack" onSubmit={answer}><label>Contributor<select name="author_kind"><option value="human">Human</option><option value="agent">Scoped agent</option></select></label><label>Answer summary<textarea name="summary" required/></label><label>Instructions, one per line<textarea name="instructions" required/></label><label>Applicable exact versions, one per line<textarea name="versions" required defaultValue={current.software_version}/></label><label>Claim mode<select name="mode"><option value="verified">Verified by cited evidence</option><option value="inference">Inference</option><option value="uncertainty">Uncertain</option></select></label><label>Claim<textarea name="claim" required/></label><label>Claim uncertainty (required for inference/uncertain)<textarea name="claim_uncertainty"/></label><label>Citation: kind | resource ID | exact revision | path | symbol | start | end | label<input name="citation" required placeholder="source | repository ID | commit | path | symbol | 10 | 20 | handler"/></label><label>Overall uncertainty (required for agents)<textarea name="uncertainty"/></label><Button type="submit">Publish answer revision</Button></form></details>}
        </div>
        <div className="card">
          <h3>Independent answer verification</h3>
          <p className="muted">Run exact guidance in a credential-free, networkless workspace. Only declared sanitized inputs, bounded logs, artifacts, and cost enter the reusable record.</p>
          {current.answers?.map(a=>{const r=a.revisions.find(x=>x.id===a.current_revision_id)!;return actor&&<details key={a.id}><summary>Verify “{r.summary}”</summary><form className="form-stack" onSubmit={e=>void verify(e,a.id,r.id)}><label>Exact source revision<input name="source_revision" required/></label><label>Software version<input name="software_version" required defaultValue={current.software_version}/></label><label>Declared environment<input value={current.environment||""} readOnly/></label><label>Environment image digest<input name="environment_digest" required placeholder="sha256:…"/></label><label>Available tools, one per line<textarea name="tools" defaultValue="sh"/></label><label>Exact dependencies, name=version<textarea name="dependencies"/></label><label>Sanitized inputs<input name="inputs" type="file" multiple accept="text/plain,application/json,application/octet-stream"/></label><label>Artifacts to retain, one relative path per line<textarea name="artifacts"/></label><label>CPU seconds<input name="cpu" type="number" min="1" max="600" defaultValue="30" required/></label><label>Memory MiB<input name="memory" type="number" min="128" max="8192" defaultValue="256" required/></label><label>Disk MiB<input name="disk" type="number" min="128" max="5120" defaultValue="128" required/></label><label>Declared cost units<input name="cost" type="number" min="0" step="any" defaultValue="0"/></label><Button type="submit">Launch bounded verification</Button></form></details>})}
          {verifications.map(v=><article className="panel" key={v.attempt.id}><p><Badge>{v.attempt.state}</Badge> <Badge>{v.stale?"stale":"current evidence"}</Badge> by {v.attempt.created_by_id}</p><p><code>{v.attempt.source_revision}</code> · {v.attempt.software_version} · {v.attempt.environment.name} @ <code>{v.attempt.environment.image_digest}</code> · cost {v.attempt.cost_units}</p>{v.stale_reasons.map(x=><p className="form-error" key={x}>{x.replaceAll("_"," ")}</p>)}{v.attempt.result&&<p>{v.attempt.result}</p>}{v.attempt.failure_reason&&<p className="form-error">{v.attempt.failure_reason}</p>}<details><summary>Commands, logs, outputs, and artifacts</summary>{v.attempt.events.map(x=><pre key={x.sequence}>{x.command||`${x.stream||x.type}: ${x.message||x.exit_code||""}`}</pre>)}{v.attempt.artifacts.map(x=><p key={x.path}><a download={x.path} href={`data:application/octet-stream;base64,${x.content}`}>{x.path}</a> · <code>{x.sha256}</code> · {x.size} bytes</p>)}</details>{actor&&<Button variant="secondary" onClick={()=>void rerun(v.attempt.id)}>Rerun exact record</Button>}</article>)}
          {!verifications.length&&<p>No verification attempts yet.</p>}
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
