"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowRight, Book, Branch, Check, Copy, Key, Plus, Search, Shield, Sparkles, Trash, User } from "@/components/icons";
import { Badge, Button } from "@/components/ui";
import { WorkspaceShell } from "@/components/app-shell";

type UserRecord = { id: string; handle: string; display_name: string };
type Grant = { id: string; name: string; kind: "web" | "api" | "git"; scopes: string[]; created_at: string; expires_at: string; last_used_at?: string; revoked_at?: string };
type Repository = { id: string; owner_id: string; name: string; description: string; visibility: "private" | "public"; empty: boolean; git_url: string; updated_at: string };
type Session = { user: UserRecord; access: Grant };
type Envelope<T> = { items: T[]; total_count: number };

const errorCopy: Record<string, string> = {
  handle_taken: "That handle is already in use.", invalid_credentials: "That handle and password do not match.",
  invalid_password: "Use a password between 12 and 72 characters.", invalid_profile: "Check your display name and handle.",
  invalid_repository: "Use lowercase letters, numbers, dots, underscores, or hyphens for the repository name.",
  name_taken: "You already have a repository with that name.", invalid_grant: "Choose a valid grant type, lifetime, and permission set.",
};

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api${path}`, { ...init, headers: { "Content-Type": "application/json", ...init?.headers } });
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: "unavailable" }));
    throw new Error(errorCopy[body.error] ?? "Something went wrong. Please try again.");
  }
  if (response.status === 204) return undefined as T;
  return response.json();
}

export default function Home() {
  const [session, setSession] = useState<Session | null | undefined>(undefined);
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [grants, setGrants] = useState<Grant[]>([]);
  const [view, setView] = useState<"workspace" | "repositories" | "access">("workspace");

  const loadWorkspace = useCallback(async () => {
    try {
      const current = await api<Session>("/session");
      setSession(current);
      const [repoData, grantData] = await Promise.all([api<Envelope<Repository>>("/repositories?affiliation=all&per_page=100"), api<Envelope<Grant>>("/access-grants?per_page=100")]);
      setRepositories(repoData.items); setGrants(grantData.items);
    } catch { setSession(null); setRepositories([]); setGrants([]); }
  }, []);

  useEffect(() => {
    // Session discovery intentionally begins after hydration; the cookie is HttpOnly.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadWorkspace();
  }, [loadWorkspace]);

  if (session === undefined) return <div className="splash"><span className="brand-mark">K</span><p>Opening your workspace…</p></div>;
  if (!session) return <Onboarding onAuthenticated={loadWorkspace} />;

  return <Dashboard session={session} repositories={repositories} grants={grants} view={view} setView={setView}
    refresh={loadWorkspace} onSignedOut={() => { setSession(null); setRepositories([]); setGrants([]); }} />;
}

function Onboarding({ onAuthenticated }: { onAuthenticated: () => Promise<void> }) {
  const [mode, setMode] = useState<"create" | "signin">("create");
  const [error, setError] = useState(""); const [pending, setPending] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setPending(true); setError("");
    const data = new FormData(event.currentTarget); const handle = String(data.get("handle")); const password = String(data.get("password"));
    try {
      if (mode === "create") await api("/users", { method: "POST", body: JSON.stringify({ handle, display_name: data.get("display_name"), password }) });
      await api("/sessions", { method: "POST", body: JSON.stringify({ handle, password }) });
      await onAuthenticated();
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Something went wrong."); setPending(false); }
  }

  return <main className="onboarding-shell" id="main-content">
    <section className="onboarding-story" aria-labelledby="onboarding-title">
      <Link className="brand onboarding-brand" href="/"><span className="brand-mark">K</span><span>Kanso</span></Link>
      <div className="story-copy"><div className="eyebrow"><Sparkles size={14}/> Build in the open, together</div>
        <h1 id="onboarding-title">Make a place for the work that comes next.</h1>
        <p>Create your account, start a repository, and invite another perspective. Your workspace is the beginning—not the destination.</p>
        <ol className="journey"><li className="active"><span>1</span><div><strong>Join Kanso</strong><small>Choose the identity your collaborators will see.</small></div></li><li><span>2</span><div><strong>Start a repository</strong><small>Give an idea a durable home.</small></div></li><li><span>3</span><div><strong>Build together</strong><small>Share access and publish a branch.</small></div></li></ol>
      </div><p className="story-note">A calm place to build software together.</p>
    </section>
    <section className="auth-side" aria-label={mode === "create" ? "Create an account" : "Sign in"}>
      <div className="auth-card"><div className="auth-tabs" role="tablist"><button role="tab" aria-selected={mode === "create"} onClick={() => { setMode("create"); setError(""); }}>Create account</button><button role="tab" aria-selected={mode === "signin"} onClick={() => { setMode("signin"); setError(""); }}>Sign in</button></div>
        <div className="auth-heading"><span className="auth-icon"><User/></span><h2>{mode === "create" ? "Welcome to Kanso" : "Welcome back"}</h2><p>{mode === "create" ? "Your first repository is only a minute away." : "Continue building with your collaborators."}</p></div>
        <form className="form-stack" onSubmit={submit}>
          {mode === "create" && <label>Display name<input name="display_name" autoComplete="name" required maxLength={80} placeholder="Ada Lovelace"/></label>}
          <label>Handle<div className="input-prefix"><span>@</span><input name="handle" autoComplete="username" required minLength={2} maxLength={39} pattern="[A-Za-z0-9-]+" placeholder="ada"/></div></label>
          <label>Password<input name="password" type="password" autoComplete={mode === "create" ? "new-password" : "current-password"} required minLength={12} maxLength={72} placeholder="At least 12 characters"/>{mode === "create" && <small>12–72 characters. Keep it unique to Kanso.</small>}</label>
          {error && <p className="form-error" role="alert">{error}</p>}
          <Button type="submit" disabled={pending}>{pending ? "Opening workspace…" : mode === "create" ? <>Create account <ArrowRight size={16}/></> : <>Sign in <ArrowRight size={16}/></>}</Button>
        </form>
      </div>
    </section>
  </main>;
}

function Dashboard({ session, repositories, grants, view, setView, refresh, onSignedOut }: { session: Session; repositories: Repository[]; grants: Grant[]; view: "workspace"|"repositories"|"access"; setView: (view: "workspace"|"repositories"|"access") => void; refresh: () => Promise<void>; onSignedOut: () => void }) {
  const [showCreate, setShowCreate] = useState(false); const [query, setQuery] = useState("");
  const initials = session.user.display_name.split(/\s+/).map(part => part[0]).join("").slice(0, 2).toUpperCase();
  const filtered = useMemo(() => repositories.filter(repo => `${repo.name} ${repo.description}`.toLowerCase().includes(query.toLowerCase())), [repositories, query]);
  async function signOut() { await api("/session", { method: "DELETE" }).catch(() => undefined); onSignedOut(); }

  return <WorkspaceShell displayName={session.user.display_name} handle={session.user.handle} initials={initials} repositoryCount={repositories.length} view={view} query={query} onQuery={setQuery} onView={setView} onCreate={() => setShowCreate(true)} onSignOut={signOut}>
    {view === "access" ? <Access grants={grants} refresh={refresh}/> : <Repositories user={session.user} repositories={filtered} total={repositories.length} searching={Boolean(query)} showCreate={showCreate} setShowCreate={setShowCreate} refresh={refresh} full={view === "repositories"}/>}
  </WorkspaceShell>;
}

function Repositories({ user, repositories, total, searching, showCreate, setShowCreate, refresh, full }: { user: UserRecord; repositories: Repository[]; total: number; searching: boolean; showCreate: boolean; setShowCreate: (show: boolean) => void; refresh: () => Promise<void>; full: boolean }) {
  return <><div className="eyebrow"><Sparkles size={14}/>{total ? "Your workspace" : "One step from collaboration"}</div><div className="page-heading"><div><h1>{total ? `${full ? "Repositories" : "Welcome back"}, ${user.display_name.split(" ")[0]}.` : `Welcome, ${user.display_name.split(" ")[0]}.`}</h1><p>{total ? "Find a project or make room for the next idea." : "Create your first repository to give your work a home."}</p></div><Button onClick={() => setShowCreate(true)}><Plus size={16}/>New repository</Button></div>
    {showCreate && <CreateRepository close={() => setShowCreate(false)} refresh={refresh}/>}
    {repositories.length ? <section aria-labelledby="repository-heading"><div className="section-heading"><div><h2 id="repository-heading">{searching ? "Matching repositories" : "Your repositories"}</h2><p>{repositories.length} owned or shared {repositories.length === 1 ? "project" : "projects"} ready for collaboration</p></div></div><div className="repo-grid">{repositories.map(repo => <RepositoryTile key={repo.id} repository={repo} actor={user.id}/>)}</div></section> : total ? <EmptySearch/> : <FirstRepository onCreate={() => setShowCreate(true)}/>}</>;
}

function FirstRepository({ onCreate }: { onCreate: () => void }) { return <section className="first-step panel"><span className="empty-icon"><Branch size={22}/></span><Badge tone="accent">Next step</Badge><h2>Start something worth sharing</h2><p>A repository holds your code, decisions, and the conversation around both. Create one now; it starts private.</p><Button onClick={onCreate}><Plus size={16}/>Create your first repository</Button><div className="next-up"><Check size={16}/><span><strong>After that</strong> Clone it with Git, publish a branch, and invite a contributor.</span></div></section>; }
function EmptySearch() { return <div className="first-step panel"><span className="empty-icon"><Search/></span><h2>No repositories match</h2><p>Try another name or description.</p></div>; }

function CreateRepository({ close, refresh }: { close: () => void; refresh: () => Promise<void> }) {
  const [error, setError] = useState(""); const [pending, setPending] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); setPending(true); setError(""); const data = new FormData(event.currentTarget); try { await api("/repositories", { method: "POST", body: JSON.stringify({ name: data.get("name"), description: data.get("description"), visibility: data.get("visibility") }) }); await refresh(); close(); } catch (cause) { setError(cause instanceof Error ? cause.message : "Could not create repository."); setPending(false); } }
  return <section className="create-panel panel" aria-labelledby="create-title"><div className="create-copy"><span className="repo-icon"><Book/></span><div><h2 id="create-title">Create a repository</h2><p>Give the project a clear name. You can invite collaborators as soon as it exists.</p></div></div><form className="repository-form" onSubmit={submit}><label>Repository name<input name="name" required pattern="[a-z0-9._-]+" maxLength={100} placeholder="project-name"/></label><label>Description <span>Optional</span><textarea name="description" maxLength={280} placeholder="What are you building together?"/></label><fieldset><legend>Visibility</legend><label className="radio-card"><input type="radio" name="visibility" value="private" defaultChecked/><Shield size={17}/><span><strong>Private</strong><small>Only you and invited contributors</small></span></label><label className="radio-card"><input type="radio" name="visibility" value="public"/><Book size={17}/><span><strong>Public</strong><small>Anyone can view and clone</small></span></label></fieldset>{error && <p className="form-error" role="alert">{error}</p>}<div className="form-actions"><Button type="button" variant="secondary" onClick={close}>Cancel</Button><Button type="submit" disabled={pending}>{pending ? "Creating…" : "Create repository"}</Button></div></form></section>;
}

function RepositoryTile({ repository, actor }: { repository: Repository; actor: string }) {
  const [copied, setCopied] = useState(false); const clone = `${process.env.NEXT_PUBLIC_GIT_ORIGIN ?? "http://localhost:8080"}${repository.git_url}`;
  async function copy() { await navigator.clipboard.writeText(clone); setCopied(true); setTimeout(() => setCopied(false), 1800); }
  return <article className="repo-card panel"><div className="repo-card-top"><span className="repo-icon"><Book size={18}/></span><span className="repo-badges"><Badge>{repository.visibility === "private" ? "Private" : "Public"}</Badge>{repository.owner_id !== actor && <Badge tone="accent">Contributing</Badge>}</span></div><Link href={`/repositories/${repository.id}`}><h3>{repository.name}</h3></Link><p>{repository.description || "No description yet."}</p><div className="clone-row"><code>{clone}</code><button onClick={copy} aria-label={`Copy clone URL for ${repository.name}`}>{copied ? <Check size={15}/> : <Copy size={15}/>}</button></div><div className="repo-meta"><span><Branch size={13}/>main</span><span>{repository.empty ? "Ready for first push" : "Active"}</span></div></article>;
}

function Access({ grants, refresh }: { grants: Grant[]; refresh: () => Promise<void> }) {
  const [showCreate, setShowCreate] = useState(false); const [token, setToken] = useState("");
  const active = grants.filter(grant => !grant.revoked_at && new Date(grant.expires_at) > new Date());
  async function revoke(id: string) { await api(`/access-grants/${id}`, { method: "DELETE" }); await refresh(); }
  return <><div className="eyebrow"><Shield size={14}/> Security & tools</div><div className="page-heading"><div><h1>Active access</h1><p>Review the sessions and credentials that can act as you.</p></div><Button onClick={() => setShowCreate(true)}><Plus size={16}/>New credential</Button></div>
    {token && <div className="token-reveal panel" role="status"><div><strong>Copy your new credential now</strong><p>For your security, Kanso won’t show it again.</p></div><code>{token}</code><Button variant="secondary" onClick={() => navigator.clipboard.writeText(token)}><Copy size={15}/>Copy</Button><button className="dismiss" onClick={() => setToken("")} aria-label="Dismiss credential">×</button></div>}
    {showCreate && <CreateGrant close={() => setShowCreate(false)} created={value => { setToken(value); setShowCreate(false); void refresh(); }}/>}
    <section className="panel access-list" aria-labelledby="access-heading"><div className="panel-title"><span id="access-heading">Credentials</span><Badge tone="accent">{active.length} active</Badge></div>{active.map(grant => <div className="grant-row" key={grant.id}><span className="grant-icon">{grant.kind === "web" ? <User size={17}/> : <Key size={17}/>}</span><div><strong>{grant.name}</strong><p>{grant.kind === "web" ? "Browser session" : `${grant.kind.toUpperCase()} credential`} · Expires {new Date(grant.expires_at).toLocaleDateString()}</p><small>{grant.scopes.join(" · ")}</small></div>{grant.id === grants.find(item => item.kind === "web" && !item.revoked_at)?.id ? <Badge>Current</Badge> : <button className="danger-button" onClick={() => revoke(grant.id)}><Trash size={14}/>Revoke</button>}</div>)}</section></>;
}

function CreateGrant({ close, created }: { close: () => void; created: (token: string) => void }) {
  const [kind, setKind] = useState<"git"|"api">("git"); const [error, setError] = useState("");
  async function submit(event: FormEvent<HTMLFormElement>) { event.preventDefault(); const data = new FormData(event.currentTarget); const scopes = kind === "git" ? ["git:read", "git:write"] : ["profile:read", "repository:read", "repository:write"]; try { const grant = await api<Grant & { token: string }>("/access-grants", { method: "POST", body: JSON.stringify({ name: data.get("name"), kind, scopes, expires_in_hours: Number(data.get("expires_in_hours")) }) }); created(grant.token); } catch (cause) { setError(cause instanceof Error ? cause.message : "Could not create credential."); } }
  return <section className="create-panel panel"><div className="create-copy"><span className="repo-icon"><Key/></span><div><h2>Create a credential</h2><p>Use a short-lived Git credential to clone and push, or an API credential for tools.</p></div></div><form className="repository-form compact" onSubmit={submit}><label>Name<input name="name" required maxLength={80} placeholder="My laptop"/></label><fieldset><legend>Kind</legend><label className="radio-card"><input type="radio" checked={kind === "git"} onChange={() => setKind("git")}/><Branch size={17}/><span><strong>Git</strong><small>Clone, fetch, and push</small></span></label><label className="radio-card"><input type="radio" checked={kind === "api"} onChange={() => setKind("api")}/><Key size={17}/><span><strong>API</strong><small>Use scripts and developer tools</small></span></label></fieldset><label>Lifetime<select name="expires_in_hours" defaultValue={kind === "git" ? 720 : 2160} key={kind}><option value="24">1 day</option><option value="168">7 days</option><option value={kind === "git" ? "720" : "2160"}>{kind === "git" ? "30 days" : "90 days"}</option></select></label>{error && <p className="form-error">{error}</p>}<div className="form-actions"><Button type="button" variant="secondary" onClick={close}>Cancel</Button><Button type="submit">Create credential</Button></div></form></section>;
}
