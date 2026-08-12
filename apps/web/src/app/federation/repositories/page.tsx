"use client";

import { Suspense, useCallback, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Badge, Button } from "@/components/ui";

type Remote = { reference:string; instance:string; source_url:string; status:string; followed:boolean; revision?:string; signature?:string; key_id?:string; fetched_at?:string; last_checked_at:string; last_error?:string; stale:boolean; visibility_changed:boolean; snapshot?:Snapshot };
type Snapshot = { repository:{id:string;name:string;description:string;visibility:string;owner_id:string}; revision:string; published_at:string; capabilities:string[]; unsupported_capabilities:string[]; branches:Array<{name:string;commit_id:string;is_default:boolean}>; releases:Array<{id:string;version:string;commit_id:string;notes:string}>; contributor_pathway?:{current_version:number;versions:Array<{number:number;goals:string[];prerequisites:string[];supported_setup:string[]}>}; issues:Array<{id:string;title:string;severity:string;status:string;reporter_id:string;updated_at:string}>; contribution_opportunities:Array<{id:string;title:string;risk:string;revision:string;required_skills:string[];expected_outcome:string}>; activity:Array<{id:string;type:string;actor_id:string;created_at:string;resource:{type:string;id:string}}> };

async function api<T>(path:string, init?:RequestInit):Promise<T>{const response=await fetch(`/api${path}`,{...init,headers:{"Content-Type":"application/json",...init?.headers}});if(!response.ok)throw new Error((await response.json().catch(()=>({error:response.statusText}))).error);return response.json()}

function FederatedRepositoryReader(){
  const params=useSearchParams(); const reference=params.get("ref")||""; const [remote,setRemote]=useState<Remote>(); const [error,setError]=useState("");
  const load=useCallback(()=>api<Remote>(`/federation/repositories/resolutions?reference=${encodeURIComponent(reference)}`).then(setRemote).catch(cause=>setError(cause instanceof Error?cause.message:"Unable to load remote project")),[reference]);
  useEffect(()=>{if(reference)void load()},[reference,load]);
  if(error)return <main className="panel"><h1>Remote project unavailable</h1><p>{error}</p></main>;
  if(!remote)return <main className="panel"><p>Loading verified remote context…</p></main>;
  const snapshot=remote.snapshot;
  return <main>
    <div className="eyebrow">Federated collaboration target</div>
    <div className="page-heading"><div><h1>{snapshot?.repository.name||remote.reference}</h1><p>{snapshot?.repository.description||remote.last_error||"No accessible snapshot is available."}</p></div><Button variant="secondary" onClick={async()=>{await api("/federation/repositories/resolutions",{method:"POST",body:JSON.stringify({reference,follow:remote.followed})});await load()}}>Check authoritative source</Button></div>
    <section className="panel"><div className="panel-title"><span>Remote authority</span><Badge tone={remote.status==="current"?"accent":"neutral"}>{remote.status}</Badge></div><p><a href={remote.source_url} rel="noreferrer">{remote.instance}</a> is authoritative. This home-instance view cannot administer or mutate the project.</p><p><code>{remote.revision||"No verified revision"}</code></p><small>Signed by {remote.key_id||"unknown key"} · checked {new Date(remote.last_checked_at).toLocaleString()}{remote.fetched_at?` · snapshot ${new Date(remote.fetched_at).toLocaleString()}`:""}</small>{(remote.stale||remote.visibility_changed||remote.last_error)&&<p className="form-error">{remote.visibility_changed?"Remote visibility changed. ":""}{remote.stale?"Showing the last verified stale snapshot. ":""}{remote.last_error}</p>}</section>
    {snapshot&&<>
      <section className="panel"><div className="panel-title"><span>Capabilities</span><Badge>{snapshot.capabilities.length} available</Badge></div><p>{snapshot.capabilities.join(" · ")}</p><small>Unsupported here: {snapshot.unsupported_capabilities.join(" · ")}</small></section>
      <section className="panel"><div className="panel-title"><span>Branches and releases</span><Badge>{snapshot.revision.slice(0,12)}</Badge></div>{snapshot.branches.map(branch=><p key={branch.name}><strong>{branch.name}</strong>{branch.is_default?" (default)":""} · <code>{branch.commit_id}</code></p>)}{snapshot.releases.map(release=><div key={release.id}><strong>{release.version}</strong><p>{release.notes} · <code>{release.commit_id}</code></p></div>)}</section>
      <section className="panel"><div className="panel-title"><span>Contributor guidance</span></div>{snapshot.contributor_pathway?.versions.slice(-1).map(version=><div key={version.number}><p>{version.goals.join(" · ")}</p><strong>Prerequisites</strong><p>{version.prerequisites.join(" · ")}</p><strong>Supported setup</strong><p>{version.supported_setup.join(" · ")}</p></div>)||<p>The remote does not publish contributor guidance.</p>}</section>
      <section className="panel"><div className="panel-title"><span>Useful work</span><Badge tone="accent">{snapshot.contribution_opportunities.length} opportunities</Badge></div>{snapshot.contribution_opportunities.map(item=><div key={item.id}><strong>{item.title}</strong><p>{item.expected_outcome}</p><small>{item.risk} risk · {item.required_skills.join(" · ")} · revision {item.revision}</small></div>)}{snapshot.issues.map(issue=><div key={issue.id}><strong>{issue.title}</strong><p>{issue.status} · {issue.severity} · reported by {issue.reporter_id}</p></div>)}</section>
      <section className="panel"><div className="panel-title"><span>Attributable activity</span><Badge>{snapshot.activity.length} events</Badge></div>{snapshot.activity.map(event=><p key={event.id}><strong>{event.actor_id}</strong> {event.type} {event.resource.type}:{event.resource.id} · {new Date(event.created_at).toLocaleString()}</p>)}</section>
    </>}
  </main>
}

export default function FederatedRepositoryPage(){return <Suspense fallback={<main className="panel"><p>Loading verified remote context…</p></main>}><FederatedRepositoryReader/></Suspense>}
