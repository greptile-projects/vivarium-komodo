"use client";

import Link from "next/link";
import { use, useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Badge, Button } from "@/components/ui";
import { Book, Branch, Check, ChevronRight, Clock, Code, Copy, File, Folder } from "@/components/icons";

type Repository = { id:string; name:string; description:string; visibility:"public"|"private"; empty:boolean; git_url:string; updated_at:string };
type BranchRecord = { name:string; commit_id:string; is_default:boolean };
type Branches = { items:BranchRecord[]; default_branch:string };
type Commit = { id:string; parent_ids:string[]; author:string; email:string; authored_at?:string; message:string };
type TreeEntry = { name:string; path:string; type:"tree"|"blob"|"commit"; mode:number; object_id:string; size?:number };
type Tree = { revision:string; commit_id:string; path:string; tree_id:string; entries:TreeEntry[] };
type Blob = { revision:string; commit_id:string; path:string; object_id:string; size:number; binary:boolean; truncated:boolean; content:string };
type Commits = { items:Commit[]; revision:string; commit_id:string; total_count:number };

async function get<T>(path:string):Promise<T> { const response = await fetch(`/api${path}`); if (!response.ok) throw new Error(response.status === 401 ? "Sign in to view this private repository." : "This repository state could not be found."); return response.json(); }
const short = (id:string) => id.slice(0,7);
const summary = (message:string) => message.split("\n")[0] || "Untitled commit";
const formatSize = (size:number) => size < 1024 ? `${size} B` : `${(size/1024).toFixed(1)} KB`;

export default function RepositoryPage({ params, searchParams }: { params:Promise<{id:string}>; searchParams:Promise<{ref?:string;path?:string;view?:string}> }) {
  const { id } = use(params); const query = use(searchParams); const router = useRouter();
  const revision = query.ref ?? ""; const path = query.path ?? ""; const view = query.view === "commits" ? "commits" : "code";
  const [repository,setRepository] = useState<Repository>(); const [branches,setBranches] = useState<Branches>();
  const [tree,setTree] = useState<Tree>(); const [blob,setBlob] = useState<Blob>(); const [commits,setCommits] = useState<Commits>();
  const [error,setError] = useState(""); const [copied,setCopied] = useState(false);
  const ref = revision || branches?.default_branch || "";

  const load = useCallback(async () => {
    setError("");
    try {
      const [repo,branchData] = await Promise.all([get<Repository>(`/repositories/${id}`),get<Branches>(`/repositories/${id}/branches`)]);
      setRepository(repo); setBranches(branchData);
      const selected = revision || branchData.default_branch;
      if (repo.empty || !selected) return;
      if (view === "commits") { setCommits(await get<Commits>(`/repositories/${id}/commits?ref=${encodeURIComponent(selected)}&per_page=100`)); setTree(undefined); setBlob(undefined); }
      else if (path) {
        try { setBlob(await get<Blob>(`/repositories/${id}/blob?ref=${encodeURIComponent(selected)}&path=${encodeURIComponent(path)}`)); setTree(undefined); }
        catch { setTree(await get<Tree>(`/repositories/${id}/tree?ref=${encodeURIComponent(selected)}&path=${encodeURIComponent(path)}`)); setBlob(undefined); }
        setCommits(undefined);
      } else { setTree(await get<Tree>(`/repositories/${id}/tree?ref=${encodeURIComponent(selected)}`)); setBlob(undefined); setCommits(undefined); }
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Repository unavailable."); }
  },[id,path,revision,view]);
  useEffect(() => {
    // Repository state follows the shareable revision/path URL.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  },[load]);

  function navigate(next:{ref?:string;path?:string;view?:string}) { const values = new URLSearchParams(); const nextRef = next.ref ?? ref; if (nextRef) values.set("ref",nextRef); const nextPath = next.path ?? path; if (nextPath) values.set("path",nextPath); const nextView = next.view ?? view; if (nextView !== "code") values.set("view",nextView); router.push(`/repositories/${id}?${values}`); }
  const clone = repository ? `${process.env.NEXT_PUBLIC_GIT_ORIGIN ?? "http://localhost:8080"}${repository.git_url}` : "";
  async function copyClone() { await navigator.clipboard.writeText(clone); setCopied(true); setTimeout(()=>setCopied(false),1600); }

  if (error) return <RepositoryFrame><section className="repository-error panel"><Book/><h1>Repository unavailable</h1><p>{error}</p><Link href="/">Return to workspace</Link></section></RepositoryFrame>;
  if (!repository || !branches) return <RepositoryFrame><div className="repository-loading">Reading repository state…</div></RepositoryFrame>;
  const contextCommit = tree?.commit_id ?? blob?.commit_id ?? commits?.commit_id;
  return <RepositoryFrame>
    <header className="repository-heading"><div className="repository-title"><span className="repo-icon"><Book/></span><div><div className="repository-owner"><Link href="/">Workspace</Link><ChevronRight size={13}/><strong>{repository.name}</strong><Badge>{repository.visibility === "public" ? "Public" : "Private"}</Badge></div><h1>{repository.name}</h1><p>{repository.description || "No description has been added yet."}</p></div></div><div className="clone-control"><code>{clone}</code><Button variant="secondary" size="sm" onClick={copyClone}>{copied?<Check size={14}/>:<Copy size={14}/>} {copied?"Copied":"Clone"}</Button></div></header>
    <nav className="repository-tabs" aria-label="Repository"><button className={view === "code"?"active":""} onClick={()=>navigate({view:"code",path:""})}><Code size={15}/>Code</button><button className={view === "commits"?"active":""} onClick={()=>navigate({view:"commits",path:""})}><Clock size={15}/>Commits</button></nav>
    {repository.empty ? <section className="empty-repository panel"><Branch/><h2>This repository is ready for its first push</h2><p>Clone it, add project files and a README, then push the <code>{branches.default_branch}</code> branch.</p><pre><code>{`git clone ${clone}\ncd ${repository.name}\ngit push origin ${branches.default_branch}`}</code></pre></section> : <>
      <div className="revision-bar panel"><label><Branch size={15}/><span className="sr-only">Branch</span><select value={ref} onChange={event=>navigate({ref:event.target.value,path:""})}>{branches.items.map(branch=><option key={branch.name}>{branch.name}</option>)}</select></label><div><span>Viewing</span><strong>{ref}</strong>{contextCommit&&<><span>at</span><code>{short(contextCommit)}</code></>}</div><span className="revision-note">Branch and revision stay attached to every path.</span></div>
      {view === "commits" && commits ? <CommitList commits={commits.items} repository={id} refName={ref}/> : blob ? <BlobView blob={blob} onCrumb={next=>navigate({path:next})}/> : tree ? <TreeView tree={tree} onPath={next=>navigate({path:next})}/> : <div className="repository-loading">Loading revision…</div>}
    </>}
  </RepositoryFrame>;
}

function RepositoryFrame({children}:{children:React.ReactNode}) { return <div className="repository-page"><a className="skip-link" href="#main-content">Skip to repository content</a><header className="repository-topbar"><Link className="brand" href="/"><span className="brand-mark">K</span><span>Kanso</span></Link><Link href="/" className="back-workspace">Your workspace</Link></header><main id="main-content" className="repository-main">{children}</main></div>; }
function Breadcrumbs({path,onCrumb}:{path:string;onCrumb:(path:string)=>void}) { const parts=path.split("/").filter(Boolean); return <nav className="breadcrumbs" aria-label="File path"><button onClick={()=>onCrumb("")}>root</button>{parts.map((part,index)=><span key={`${part}-${index}`}><ChevronRight size={12}/><button onClick={()=>onCrumb(parts.slice(0,index+1).join("/"))}>{part}</button></span>)}</nav>; }
function TreeView({tree,onPath}:{tree:Tree;onPath:(path:string)=>void}) { return <section className="file-browser panel"><div className="browser-header"><Breadcrumbs path={tree.path} onCrumb={onPath}/><code>{tree.entries.length} items · {short(tree.tree_id)}</code></div>{tree.entries.length ? <div className="file-list">{tree.entries.map(entry=><button key={entry.path} onClick={()=>onPath(entry.path)}><span className={entry.type === "tree"?"folder-icon":"file-icon"}>{entry.type === "tree"?<Folder size={16}/>:<File size={16}/>}</span><strong>{entry.name}</strong><span>{entry.type === "tree"?"Directory":formatSize(entry.size??0)}</span><code>{short(entry.object_id)}</code></button>)}</div>:<p className="empty-directory">This directory is empty.</p>}</section>; }
function BlobView({blob,onCrumb}:{blob:Blob;onCrumb:(path:string)=>void}) { const lines=blob.content.split("\n"); return <section className="blob-browser"><div className="browser-header panel"><Breadcrumbs path={blob.path} onCrumb={onCrumb}/><span>{formatSize(blob.size)} · <code>{short(blob.object_id)}</code></span></div>{blob.binary?<div className="binary-file panel"><File/><h2>Binary file</h2><p>This file cannot be displayed, but its identity and size are preserved above.</p></div>:<div className="code-view panel">{lines.map((line,index)=><div className="code-line" key={index}><span>{index+1}</span><code>{line || " "}</code></div>)}{blob.truncated&&<p className="truncated">Preview limited to the first 1 MB.</p>}</div>}</section>; }
function CommitList({commits,repository,refName}:{commits:Commit[];repository:string;refName:string}) { return <section className="commit-list panel"><div className="browser-header"><strong>{commits.length} commits on {refName}</strong><span>Newest reachable work first</span></div>{commits.map(commit=><article key={commit.id}><span className="commit-node"><Clock size={14}/></span><div><h2>{summary(commit.message)}</h2><p>{commit.author || "Unknown author"}{commit.authored_at&&<> committed <time dateTime={commit.authored_at}>{new Date(commit.authored_at).toLocaleString()}</time></>}</p></div><Link href={`/repositories/${repository}?ref=${commit.id}`}><code>{short(commit.id)}</code></Link></article>)}</section>; }
