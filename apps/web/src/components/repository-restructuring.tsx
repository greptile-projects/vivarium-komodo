/* eslint-disable react-hooks/set-state-in-effect */
"use client";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";

type Plan = {
  id: string; title: string; summary: string; creator_id: string; deadline: string;
  sources: { repository_id: string; revision: string; owner_ids: string[]; role: string }[];
  destinations: { id: string; name: string; owner_ids: string[]; visibility: string; default_branch: string; retained_identities: string[] }[];
  mappings: { id: string; source_paths: string[]; destination_id?: string; destination_paths: string[]; history_mode: string; disposition: string; rationale: string }[];
  inventory: { id: string; kind: string; reference: string; revision: string; owner_ids: string[]; access: string; disposition: string; destination_ids: string[]; reason?: string }[];
  success_criteria: string[]; rollback_limits: { latest_time: string; irreversible_after: string; maximum_data_loss: string; required_retentions: string[] };
  blockers: { kind: string; resource_id: string; detail: string }[];
  findings: { id: string; actor_id: string; actor_kind: string; summary: string; impact: string; uncertainty?: string; citations: { reference: string; revision: string }[] }[];
};
const csv=(v:string|undefined)=>(v||"").split(",").map(x=>x.trim()).filter(Boolean);
const lines=(v:FormDataEntryValue|null)=>String(v||"").split("\n").map(x=>x.trim()).filter(Boolean).map(x=>x.split("|").map(y=>y.trim()));

export function RepositoryRestructuring({repository,revision}:{repository:string;revision:string}) {
  const root=`/api/repositories/${repository}/restructuring-plans`,[items,setItems]=useState<Plan[]>([]),[error,setError]=useState("");
  const load=useCallback(async()=>{const r=await fetch(root);if(r.ok)setItems(((await r.json()) as {items:Plan[]}).items)},[root]);
  useEffect(()=>{void load()},[load]);
  async function submit(e:FormEvent<HTMLFormElement>){e.preventDefault();setError("");const f=new FormData(e.currentTarget);
    const sources=lines(f.get("sources")).map(x=>({repository_id:x[0],revision:x[1],owner_ids:csv(x[2]),role:x[3]}));
    const destinations=lines(f.get("destinations")).map(x=>({id:x[0],name:x[1],owner_ids:csv(x[2]),visibility:x[3],default_branch:x[4],retained_identities:csv(x[5])}));
    const mappings=lines(f.get("mappings")).map(x=>({id:x[0],source_repository_id:x[1],source_revision:x[2],source_paths:csv(x[3]),destination_id:x[4],destination_paths:csv(x[5]),history_mode:x[6],disposition:x[7],rationale:x[8]}));
    const inventory=lines(f.get("inventory")).map(x=>({id:x[0],kind:x[1],repository_id:x[2],reference:x[3],revision:x[4],owner_ids:csv(x[5]),access:x[6],disposition:x[7],destination_ids:csv(x[8]),shared_with:csv(x[9]),reason:x[10]}));
    const body={title:f.get("title"),summary:f.get("summary"),sources,destinations,mappings,inventory,deadline:new Date(String(f.get("deadline"))).toISOString(),success_criteria:lines(f.get("criteria")).flat(),rollback_limits:{latest_time:new Date(String(f.get("rollback_time"))).toISOString(),irreversible_after:f.get("irreversible"),maximum_data_loss:f.get("data_loss"),required_retentions:lines(f.get("retentions")).flat()}};
    const r=await fetch(root,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify(body)});if(!r.ok){setError(((await r.json()) as {error:string}).error);return}e.currentTarget.reset();await load();}
  async function finding(e:FormEvent<HTMLFormElement>,plan:string){e.preventDefault();const f=new FormData(e.currentTarget),citation=String(f.get("citation")).split("|").map(x=>x.trim());const r=await fetch(`${root}/${plan}/findings`,{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({actor_kind:f.get("actor_kind"),summary:f.get("finding_summary"),impact:f.get("impact"),affected_item_ids:csv(String(f.get("items"))),uncertainty:f.get("uncertainty"),citations:[{repository_id:citation[0],reference:citation[1],revision:citation[2],path:citation[3]}]})});if(!r.ok){setError(((await r.json()) as {error:string}).error);return}e.currentTarget.reset();await load()}
  return <section className="workspace-stack">
    <header className="workspace-heading"><div><span className="eyebrow">Project boundaries</span><h2>Repository restructuring</h2><p>Review exact moves, retained identities, affected collaboration, ownership, and rollback limits before repository authority changes.</p></div><Badge tone="accent">{items.length} plans</Badge></header>
    <form className="panel form-stack" onSubmit={submit}><h3>Open a restructuring plan</h3>
      <label>Title<input name="title" required /></label><label>Outcome summary<textarea name="summary" required /></label>
      <label>Sources <small>repository | revision | owners comma-separated | role</small><textarea name="sources" required defaultValue={`${repository}|${revision}|owner|primary`} /></label>
      <label>Destinations <small>id | name | owners | public/private/internal | default branch | retained identities</small><textarea name="destinations" required /></label>
      <label>Content and history mappings <small>id | source repository | revision | paths | destination id | destination paths | history mode | move/remain/copy/split/redirect/retire/unresolved | rationale</small><textarea name="mappings" required /></label>
      <label>Affected inventory <small>id | kind | repository | reference | revision | owners | accessible/inaccessible/ambiguous/shared | disposition | destinations | shared with | reason</small><textarea name="inventory" required /></label>
      <div className="form-grid"><label>Deadline<input name="deadline" type="datetime-local" required /></label><label>Rollback until<input name="rollback_time" type="datetime-local" required /></label><label>Irreversible after<input name="irreversible" required /></label><label>Maximum data loss<input name="data_loss" required /></label></div>
      <label>Success criteria <small>one per line</small><textarea name="criteria" required /></label><label>Required retentions <small>one per line</small><textarea name="retentions" required /></label>{error&&<p className="error">{error}</p>}<Button type="submit">Open reviewable plan</Button>
    </form>
    {items.map(p=><article className="panel form-stack" key={p.id}><div className="workspace-heading"><div><h3>{p.title}</h3><p>{p.summary}</p></div><Badge tone={p.blockers.length?"warning":"accent"}>{p.blockers.length} blockers</Badge></div>
      <p><strong>{p.sources.length}</strong> sources → <strong>{p.destinations.length}</strong> destinations · deadline {new Date(p.deadline).toLocaleString()}</p>
      <div className="form-grid">{p.destinations.map(d=><div key={d.id}><strong>{d.name}</strong><p>{d.visibility} · {d.default_branch}</p><small>Owners: {d.owner_ids.join(", ")} · retained: {d.retained_identities.join(", ")||"none"}</small></div>)}</div>
      <h4>What moves and remains</h4>{p.inventory.map(x=><div key={x.id}><Badge tone={x.access==="accessible"?"neutral":"warning"}>{x.access}</Badge> <strong>{x.kind}: {x.reference}</strong> → {x.disposition}{x.destination_ids.length?` (${x.destination_ids.join(", ")})`:""}<small>{x.reason&&` · ${x.reason}`}</small></div>)}
      {p.findings.map(x=><blockquote key={x.id}><strong>{x.actor_kind} · {x.actor_id}</strong>: {x.summary}<br/><small>{x.impact} · cited {x.citations.map(c=>`${c.reference}@${c.revision}`).join(", ")}</small></blockquote>)}
      <form className="form-stack" onSubmit={e=>finding(e,p.id)}><h4>Add a cited impact finding</h4><div className="form-grid"><label>Actor kind<select name="actor_kind"><option value="human">Human</option><option value="read_only_agent">Read-only agent</option></select></label><label>Affected inventory IDs<input name="items" required /></label></div><label>Summary<input name="finding_summary" required /></label><label>Impact<textarea name="impact" required /></label><label>Uncertainty<input name="uncertainty" /></label><label>Citation <small>repository | reference | exact revision | optional path</small><input name="citation" required /></label><Button type="submit" variant="secondary">Add finding</Button></form>
      <small>Rollback until {new Date(p.rollback_limits.latest_time).toLocaleString()}; irreversible after {p.rollback_limits.irreversible_after}. This plan grants no source or destination authority.</small>
    </article>)}
  </section>;
}
