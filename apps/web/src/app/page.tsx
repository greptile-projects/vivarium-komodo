"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  ArrowRight,
  Bell,
  Book,
  Branch,
  Check,
  Copy,
  Key,
  Plus,
  Search,
  Shield,
  Sparkles,
  Trash,
  User,
} from "@/components/icons";
import { Badge, Button } from "@/components/ui";
import { WorkspaceShell } from "@/components/app-shell";

type UserRecord = { id: string; handle: string; display_name: string };
type Grant = {
  id: string;
  name: string;
  kind: "web" | "api" | "git";
  scopes: string[];
  created_at: string;
  expires_at: string;
  last_used_at?: string;
  revoked_at?: string;
};
type Repository = {
  id: string;
  owner_id: string;
  name: string;
  description: string;
  visibility: "private" | "public";
  empty: boolean;
  git_url: string;
  updated_at: string;
  upstream_repository_id?: string;
};
type Session = { user: UserRecord; access: Grant };
type Envelope<T> = { items: T[]; total_count: number };
type ExtensionRecord = { id:string; name:string; description:string; owner_id:string; operator_contact:string; capabilities:string[]; callback:{url:string;verified_at?:string}; actions:{url:string;verified_at?:string}; requested_permissions:string[]; event_types:string[]; rotation_policy:{interval_days:number;overlap_hours:number;contact_on_failure:boolean}; status:string };
type FederationPeer = { instance:string; discovery_url:string; status:string; trust:string; identity_changed:boolean; last_error?:string; document?:{version:number;capabilities:string[];operators:{name:string;contact:string}[];keys:{id:string;status:string}[];actors:{subject:string;display_name:string}[]} };
type FederatedRepository = { reference:string; instance:string; repository_id:string; status:string; followed:boolean; revision?:string; fetched_at?:string; stale:boolean; visibility_changed:boolean; last_error?:string };
type InboxItem = {
  id: string;
  classification: "review" | "response" | "awareness";
  repository_name: string;
  actor_handle: string;
  title: string;
  summary: string;
  href: string;
  created_at: string;
};
type SecurityEvidence = {
  id: string;
  title: string;
  kind: string;
  description: string;
};
type SecurityLink = {
  id: string;
  kind: string;
  repository_id: string;
  resource_id?: string;
  revision?: string;
  label: string;
  details?: string;
  actor_id: string;
  created_at: string;
};
type SecurityReport = {
  id: string;
  title: string;
  summary: string;
  reporter_id: string;
  contact: { channel: string; value: string };
  affected_repositories: { repository_id: string; versions?: string[] }[];
  evidence: SecurityEvidence[];
  resource_links: SecurityLink[];
  findings: {
    id: string;
    type: string;
    body: string;
    actor_id: string;
    evidence_ids: string[];
    created_at: string;
  }[];
  impact_matrix: {
    id: string;
    repository_id: string;
    version: string;
    environment: string;
    state: string;
    rationale: string;
    actor_id: string;
    evidence_ids: string[];
    updated_at: string;
  }[];
  repairs: {
    id: string;
    repository_id: string;
    version: string;
    outcome: string;
    base_revision: string;
    state: string;
    verification?: {
      revision: string;
      state: string;
      gates: { kind: string; name: string; attempt_id: string; state: string }[];
      approvals: { actor_id: string; decision: string; summary: string }[];
      integration_entry_id?: string;
      integration_commit_id?: string;
      release_attestations: { release_id: string; version: string; artifact_id: string; artifact_sha256: string }[];
      remaining_gaps: string[];
    };
  }[];
  investigations: {
    id: string;
    agent: string;
    mandate: string;
    state: string;
    evidence_ids: string[];
    records: {
      sequence: number;
      type: string;
      actor_id: string;
      body: string;
      uncertainty?: string;
      created_at: string;
    }[];
  }[];
  severity: string;
  embargo_state: string;
  response_team: {
    user_id: string;
    invited_by_id: string;
    invited_at: string;
  }[];
  messages: {
    id: string;
    author_id: string;
    body: string;
    created_at: string;
  }[];
  audit_log: {
    sequence: number;
    type: string;
    actor_id: string;
    subject_id?: string;
    detail?: string;
    created_at: string;
  }[];
  created_at: string;
  updated_at: string;
};

const errorCopy: Record<string, string> = {
  handle_taken: "That handle is already in use.",
  invalid_credentials: "That handle and password do not match.",
  invalid_password: "Use a password between 12 and 72 characters.",
  invalid_profile: "Check your display name and handle.",
  invalid_repository:
    "Use lowercase letters, numbers, dots, underscores, or hyphens for the repository name.",
  name_taken: "You already have a repository with that name.",
  invalid_grant: "Choose a valid grant type, lifetime, and permission set.",
};

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: "unavailable" }));
    throw new Error(
      errorCopy[body.error] ?? "Something went wrong. Please try again.",
    );
  }
  if (response.status === 204) return undefined as T;
  return response.json();
}

export default function Home() {
  const [session, setSession] = useState<Session | null | undefined>(undefined);
  const [repositories, setRepositories] = useState<Repository[]>([]);
  const [publicRepositories, setPublicRepositories] = useState<Repository[]>(
    [],
  );
  const [grants, setGrants] = useState<Grant[]>([]);
  const [inbox, setInbox] = useState<InboxItem[]>([]);
  const [securityReports, setSecurityReports] = useState<SecurityReport[]>([]);
  const [view, setView] = useState<
    "workspace" | "repositories" | "inbox" | "security" | "access"
  >("workspace");

  const loadWorkspace = useCallback(async () => {
    try {
      const current = await api<Session>("/session");
      setSession(current);
      const [repoData, publicData, grantData, inboxData, securityData] =
        await Promise.all([
          api<Envelope<Repository>>(
            "/repositories?affiliation=all&per_page=100",
          ),
          api<Envelope<Repository>>("/repositories/public?per_page=100"),
          api<Envelope<Grant>>("/access-grants?per_page=100"),
          api<Envelope<InboxItem>>("/inbox?per_page=100"),
          api<Envelope<SecurityReport>>("/security-reports?per_page=100"),
        ]);
      setRepositories(repoData.items);
      setPublicRepositories(publicData.items);
      setGrants(grantData.items);
      setInbox(inboxData.items);
      setSecurityReports(securityData.items);
    } catch {
      setSession(null);
      setRepositories([]);
      setPublicRepositories([]);
      setGrants([]);
      setInbox([]);
      setSecurityReports([]);
    }
  }, []);

  useEffect(() => {
    // Session discovery intentionally begins after hydration; the cookie is HttpOnly.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadWorkspace();
  }, [loadWorkspace]);

  if (session === undefined)
    return (
      <div className="splash">
        <span className="brand-mark">K</span>
        <p>Opening your workspace…</p>
      </div>
    );
  if (!session) return <Onboarding onAuthenticated={loadWorkspace} />;

  return (
    <Dashboard
      session={session}
      repositories={repositories}
      publicRepositories={publicRepositories}
      grants={grants}
      inbox={inbox}
      securityReports={securityReports}
      view={view}
      setView={setView}
      refresh={loadWorkspace}
      onSignedOut={() => {
        setSession(null);
        setRepositories([]);
        setGrants([]);
        setInbox([]);
      }}
    />
  );
}

function Onboarding({
  onAuthenticated,
}: {
  onAuthenticated: () => Promise<void>;
}) {
  const [mode, setMode] = useState<"create" | "signin">("create");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError("");
    const data = new FormData(event.currentTarget);
    const handle = String(data.get("handle"));
    const password = String(data.get("password"));
    try {
      if (mode === "create")
        await api("/users", {
          method: "POST",
          body: JSON.stringify({
            handle,
            display_name: data.get("display_name"),
            password,
          }),
        });
      await api("/sessions", {
        method: "POST",
        body: JSON.stringify({ handle, password }),
      });
      await onAuthenticated();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Something went wrong.",
      );
      setPending(false);
    }
  }

  return (
    <main className="onboarding-shell" id="main-content">
      <section className="onboarding-story" aria-labelledby="onboarding-title">
        <Link className="brand onboarding-brand" href="/">
          <span className="brand-mark">K</span>
          <span>Kanso</span>
        </Link>
        <div className="story-copy">
          <div className="eyebrow">
            <Sparkles size={14} /> Build in the open, together
          </div>
          <h1 id="onboarding-title">
            Make a place for the work that comes next.
          </h1>
          <p>
            Create your account, start a repository, and invite another
            perspective. Your workspace is the beginning—not the destination.
          </p>
          <ol className="journey">
            <li className="active">
              <span>1</span>
              <div>
                <strong>Join Kanso</strong>
                <small>Choose the identity your collaborators will see.</small>
              </div>
            </li>
            <li>
              <span>2</span>
              <div>
                <strong>Start a repository</strong>
                <small>Give an idea a durable home.</small>
              </div>
            </li>
            <li>
              <span>3</span>
              <div>
                <strong>Build together</strong>
                <small>Share access and publish a branch.</small>
              </div>
            </li>
          </ol>
        </div>
        <p className="story-note">A calm place to build software together.</p>
      </section>
      <section
        className="auth-side"
        aria-label={mode === "create" ? "Create an account" : "Sign in"}
      >
        <div className="auth-card">
          <div className="auth-tabs" role="tablist">
            <button
              role="tab"
              aria-selected={mode === "create"}
              onClick={() => {
                setMode("create");
                setError("");
              }}
            >
              Create account
            </button>
            <button
              role="tab"
              aria-selected={mode === "signin"}
              onClick={() => {
                setMode("signin");
                setError("");
              }}
            >
              Sign in
            </button>
          </div>
          <div className="auth-heading">
            <span className="auth-icon">
              <User />
            </span>
            <h2>{mode === "create" ? "Welcome to Kanso" : "Welcome back"}</h2>
            <p>
              {mode === "create"
                ? "Your first repository is only a minute away."
                : "Continue building with your collaborators."}
            </p>
          </div>
          <form className="form-stack" onSubmit={submit}>
            {mode === "create" && (
              <label>
                Display name
                <input
                  name="display_name"
                  autoComplete="name"
                  required
                  maxLength={80}
                  placeholder="Ada Lovelace"
                />
              </label>
            )}
            <label>
              Handle
              <div className="input-prefix">
                <span>@</span>
                <input
                  name="handle"
                  autoComplete="username"
                  required
                  minLength={2}
                  maxLength={39}
                  pattern="[A-Za-z0-9-]+"
                  placeholder="ada"
                />
              </div>
            </label>
            <label>
              Password
              <input
                name="password"
                type="password"
                autoComplete={
                  mode === "create" ? "new-password" : "current-password"
                }
                required
                minLength={12}
                maxLength={72}
                placeholder="At least 12 characters"
              />
              {mode === "create" && (
                <small>12–72 characters. Keep it unique to Kanso.</small>
              )}
            </label>
            {error && (
              <p className="form-error" role="alert">
                {error}
              </p>
            )}
            <Button type="submit" disabled={pending}>
              {pending ? (
                "Opening workspace…"
              ) : mode === "create" ? (
                <>
                  Create account <ArrowRight size={16} />
                </>
              ) : (
                <>
                  Sign in <ArrowRight size={16} />
                </>
              )}
            </Button>
          </form>
        </div>
      </section>
    </main>
  );
}

function Dashboard({
  session,
  repositories,
  publicRepositories,
  grants,
  inbox,
  securityReports,
  view,
  setView,
  refresh,
  onSignedOut,
}: {
  session: Session;
  repositories: Repository[];
  publicRepositories: Repository[];
  grants: Grant[];
  inbox: InboxItem[];
  securityReports: SecurityReport[];
  view: "workspace" | "repositories" | "inbox" | "security" | "access";
  setView: (
    view: "workspace" | "repositories" | "inbox" | "security" | "access",
  ) => void;
  refresh: () => Promise<void>;
  onSignedOut: () => void;
}) {
  const [showCreate, setShowCreate] = useState(false);
  const [query, setQuery] = useState("");
  const initials = session.user.display_name
    .split(/\s+/)
    .map((part) => part[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();
  const filtered = useMemo(
    () =>
      repositories.filter((repo) =>
        `${repo.name} ${repo.description}`
          .toLowerCase()
          .includes(query.toLowerCase()),
      ),
    [repositories, query],
  );
  const discoverable = useMemo(() => {
    const joined = new Set(repositories.map((repo) => repo.id));
    return publicRepositories.filter(
      (repo) =>
        !joined.has(repo.id) &&
        `${repo.name} ${repo.description}`
          .toLowerCase()
          .includes(query.toLowerCase()),
    );
  }, [publicRepositories, repositories, query]);
  async function signOut() {
    await api("/session", { method: "DELETE" }).catch(() => undefined);
    onSignedOut();
  }

  return (
    <WorkspaceShell
      displayName={session.user.display_name}
      handle={session.user.handle}
      initials={initials}
      repositoryCount={repositories.length}
      inboxCount={inbox.length}
      view={view}
      query={query}
      onQuery={setQuery}
      onView={setView}
      onCreate={() => setShowCreate(true)}
      onSignOut={signOut}
    >
      {view === "access" ? (
        <Access grants={grants} refresh={refresh} />
      ) : view === "inbox" ? (
        <Inbox items={inbox} refresh={refresh} />
      ) : view === "security" ? (
        <SecurityReports
          actor={session.user.id}
          repositories={[
            ...repositories,
            ...publicRepositories.filter(
              (item) =>
                !repositories.some((repository) => repository.id === item.id),
            ),
          ]}
          items={securityReports}
          refresh={refresh}
        />
      ) : (
        <Repositories
          user={session.user}
          repositories={filtered}
          discoverable={discoverable}
          total={repositories.length}
          searching={Boolean(query)}
          showCreate={showCreate}
          setShowCreate={setShowCreate}
          refresh={refresh}
          full={view === "repositories"}
        />
      )}
    </WorkspaceShell>
  );
}

function SecurityReports({
  actor,
  repositories,
  items,
  refresh,
}: {
  actor: string;
  repositories: Repository[];
  items: SecurityReport[];
  refresh: () => Promise<void>;
}) {
  const [selected, setSelected] = useState("");
  const [report, setReport] = useState<SecurityReport | null>(null);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [workerCredential, setWorkerCredential] = useState("");
  const open = useCallback(async (id: string) => {
    setSelected(id);
    setError("");
    try {
      setReport(await api<SecurityReport>(`/security-reports/${id}`));
    } catch (cause) {
      setReport(null);
      setError(cause instanceof Error ? cause.message : "Report unavailable.");
    }
  }, []);
  const maintainer = Boolean(
    report?.affected_repositories.some((affected) =>
      repositories.some(
        (repository) =>
          repository.id === affected.repository_id &&
          repository.owner_id === actor,
      ),
    ),
  );
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const data = new FormData(event.currentTarget);
    const repositoryIDs = data.getAll("repository").map(String);
    const versions = String(data.get("versions"))
      .split("\n")
      .map((value) => value.trim())
      .filter(Boolean);
    try {
      const item = await api<SecurityReport>("/security-reports", {
        method: "POST",
        body: JSON.stringify({
          title: data.get("title"),
          summary: data.get("summary"),
          contact: { channel: data.get("channel"), value: data.get("contact") },
          affected_repositories: repositoryIDs.map((repository_id) => ({
            repository_id,
            versions,
          })),
          evidence: [
            {
              title: data.get("evidence_title"),
              kind: data.get("evidence_kind"),
              description: data.get("evidence"),
            },
          ],
        }),
      });
      setCreating(false);
      await refresh();
      await open(item.id);
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Report could not be created.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function mutate(path: string, method: string, body?: unknown) {
    if (!report) return;
    setBusy(true);
    setError("");
    try {
      const updated = await api<SecurityReport>(
        `/security-reports/${report.id}${path}`,
        { method, body: body === undefined ? undefined : JSON.stringify(body) },
      );
      setReport(updated);
      await refresh();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Change was not accepted.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function delegate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!report) return;
    setBusy(true);
    const form = event.currentTarget;
    const data = new FormData(form);
    try {
      const result = await api<{
        report: SecurityReport;
        worker_credential: string;
      }>(`/security-reports/${report.id}/investigations`, {
        method: "POST",
        body: JSON.stringify({
          agent: "codex",
          mandate: data.get("mandate"),
          evidence_ids: data.getAll("evidence"),
        }),
      });
      setReport(result.report);
      setWorkerCredential(result.worker_credential);
      form.reset();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Investigation could not be delegated.",
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="security-workspace">
      <div className="eyebrow">
        <Shield size={14} /> Confidential coordination
      </div>
      <div className="page-heading">
        <div>
          <h1>Security reports</h1>
          <p>
            A private, audited channel for suspected vulnerabilities. Reports
            never enter repository activity, search, or notifications.
          </p>
        </div>
        <Button onClick={() => setCreating((value) => !value)}>
          <Plus size={16} />
          Report vulnerability
        </Button>
      </div>
      {creating && (
        <form className="panel security-form" onSubmit={create}>
          <label>
            Report title
            <input name="title" required maxLength={200} />
          </label>
          <label>
            Safe contact channel
            <select name="channel">
              <option value="email">Email</option>
              <option value="signal">Signal</option>
              <option value="matrix">Matrix</option>
              <option value="other">Other</option>
            </select>
          </label>
          <label>
            Contact address
            <input name="contact" required maxLength={500} />
          </label>
          <fieldset>
            <legend>Affected repositories</legend>
            {repositories.map((repository) => (
              <label key={repository.id}>
                <input
                  type="checkbox"
                  name="repository"
                  value={repository.id}
                />
                {repository.name}
              </label>
            ))}
          </fieldset>
          <label className="wide">
            Affected versions{" "}
            <small>
              One version or range per line; applied to the selected
              repositories.
            </small>
            <textarea name="versions" required rows={3} />
          </label>
          <label className="wide">
            What you observed
            <textarea name="summary" required rows={5} />
          </label>
          <label>
            Evidence type
            <select name="evidence_kind">
              <option value="reproduction">Reproduction</option>
              <option value="description">Description</option>
              <option value="log">Redacted log</option>
              <option value="artifact">Artifact reference</option>
              <option value="reference">External reference</option>
            </select>
          </label>
          <label>
            Evidence title
            <input name="evidence_title" required maxLength={300} />
          </label>
          <label className="wide">
            Evidence details
            <textarea name="evidence" required rows={5} />
          </label>
          {error && <p className="form-error wide">{error}</p>}
          <div className="form-actions wide">
            <Button
              variant="secondary"
              type="button"
              onClick={() => setCreating(false)}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={busy}>
              Submit privately
            </Button>
          </div>
        </form>
      )}
      <div className="security-layout">
        <aside className="panel security-list">
          {items.map((item) => (
            <button
              className={selected === item.id ? "active" : ""}
              onClick={() => void open(item.id)}
              key={item.id}
            >
              <span>
                <strong>{item.title}</strong>
                <small>{new Date(item.updated_at).toLocaleString()}</small>
              </span>
              <Badge
                tone={item.embargo_state === "active" ? "accent" : "neutral"}
              >
                {item.severity} · {item.embargo_state}
              </Badge>
            </button>
          ))}
          {!items.length && <p>No private reports are available to you.</p>}
        </aside>
        {report && (
          <article className="panel security-detail">
            <header>
              <div>
                <p className="eyebrow">
                  Private report · {report.id.slice(0, 8)}
                </p>
                <h2>{report.title}</h2>
              </div>
              <span>
                <Badge tone="accent">{report.severity}</Badge>{" "}
                <Badge>{report.embargo_state}</Badge>
              </span>
            </header>
            <section>
              <p className="security-summary">{report.summary}</p>
              <dl>
                <div>
                  <dt>Reporter contact</dt>
                  <dd>
                    {report.contact.channel}: {report.contact.value}
                  </dd>
                </div>
                <div>
                  <dt>Affected scope</dt>
                  <dd>
                    {report.affected_repositories.map((item) => (
                      <span key={item.repository_id}>
                        {repositories.find(
                          (repo) => repo.id === item.repository_id,
                        )?.name ?? item.repository_id}
                        : {item.versions?.join(", ")}
                      </span>
                    ))}
                  </dd>
                </div>
              </dl>
            </section>
            {maintainer && (
              <section className="security-controls">
                <label>
                  Severity
                  <select
                    value={report.severity}
                    onChange={(event) =>
                      void mutate("/triage", "PATCH", {
                        severity: event.target.value,
                      })
                    }
                  >
                    {["unknown", "low", "medium", "high", "critical"].map(
                      (value) => (
                        <option key={value}>{value}</option>
                      ),
                    )}
                  </select>
                </label>
                <label>
                  Embargo
                  <select
                    value={report.embargo_state}
                    onChange={(event) =>
                      void mutate("/triage", "PATCH", {
                        embargo_state: event.target.value,
                      })
                    }
                  >
                    <option value="requested">Requested</option>
                    <option value="active">Active</option>
                    <option value="lifted">Lifted</option>
                  </select>
                </label>
                <form
                  onSubmit={(event) => {
                    event.preventDefault();
                    const form = event.currentTarget;
                    void mutate("/team", "POST", {
                      user_id: new FormData(form).get("user_id"),
                    }).then(() => form.reset());
                  }}
                >
                  <label>
                    Invite responder by user ID
                    <input name="user_id" required />
                  </label>
                  <Button type="submit" disabled={busy}>
                    Invite
                  </Button>
                </form>
              </section>
            )}
            <section className="security-section">
              <h3>Submitted evidence</h3>
              {report.evidence.map((item, index) => (
                <article key={item.id || `${item.title}-${index}`}>
                  <Badge>{item.kind}</Badge>
                  <strong>{item.title}</strong>
                  <p>{item.description}</p>
                </article>
              ))}
            </section>
            <section className="security-section">
              <h3>Vulnerability evidence graph</h3>
              {report.resource_links?.map((item) => (
                <article key={item.id}>
                  <Badge>{item.kind.replaceAll("_", " ")}</Badge>
                  <strong>{item.label}</strong>
                  <p>
                    {item.repository_id} · {item.revision || item.resource_id}
                  </p>
                  <small>{item.details}</small>
                </article>
              ))}
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  const form = event.currentTarget;
                  const data = new FormData(form);
                  void mutate("/resources", "POST", {
                    kind: data.get("kind"),
                    repository_id: data.get("repository_id"),
                    resource_id: data.get("resource_id"),
                    revision: data.get("revision"),
                    label: data.get("label"),
                    details: data.get("details"),
                  }).then(() => form.reset());
                }}
              >
                <select name="kind">
                  {[
                    "commit",
                    "dependency",
                    "build",
                    "release_artifact",
                    "deployment",
                    "version_line",
                  ].map((value) => (
                    <option key={value} value={value}>
                      {value.replaceAll("_", " ")}
                    </option>
                  ))}
                </select>
                <select name="repository_id">
                  {report.affected_repositories.map((item) => (
                    <option key={item.repository_id}>
                      {item.repository_id}
                    </option>
                  ))}
                </select>
                <input
                  name="resource_id"
                  placeholder="Build, artifact, dependency, or deployment ID"
                />
                <input
                  name="revision"
                  placeholder="Exact commit or version line"
                />
                <input
                  name="label"
                  required
                  placeholder="What this evidence establishes"
                />
                <textarea
                  name="details"
                  rows={2}
                  placeholder="Provenance and relevant detail"
                />
                <Button type="submit" disabled={busy}>
                  Connect evidence
                </Button>
              </form>
            </section>
            <section className="security-section">
              <h3>Hypotheses and conclusions</h3>
              {report.findings?.map((item) => (
                <article key={item.id}>
                  <Badge
                    tone={item.type === "conclusion" ? "accent" : "neutral"}
                  >
                    {item.type}
                  </Badge>
                  <strong>{item.actor_id}</strong>
                  <p>{item.body}</p>
                </article>
              ))}
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  const form = event.currentTarget;
                  const data = new FormData(form);
                  void mutate("/findings", "POST", {
                    type: data.get("type"),
                    body: data.get("body"),
                    evidence_ids: data.getAll("evidence"),
                  }).then(() => form.reset());
                }}
              >
                <select name="type">
                  <option>hypothesis</option>
                  <option>conclusion</option>
                </select>
                <textarea name="body" required rows={3} />
                {[...report.evidence, ...(report.resource_links || [])].map(
                  (item) => (
                    <label key={item.id}>
                      <input type="checkbox" name="evidence" value={item.id} />
                      {"title" in item ? item.title : item.label}
                    </label>
                  ),
                )}
                <Button type="submit" disabled={busy}>
                  Record assessment
                </Button>
              </form>
            </section>
            <section className="security-section">
              <h3>Version × environment impact</h3>
              {report.impact_matrix?.map((item) => (
                <article key={item.id}>
                  <Badge
                    tone={
                        item.state === "confirmed" || item.state === "fixed"
                          ? "accent"
                          : "neutral"
                    }
                  >
                    {item.state}
                  </Badge>
                  <strong>
                    {item.version} · {item.environment}
                  </strong>
                  <p>{item.rationale}</p>
                </article>
              ))}
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  const form = event.currentTarget;
                  const data = new FormData(form);
                  void mutate("/impact", "PUT", {
                    repository_id: data.get("repository_id"),
                    version: data.get("version"),
                    environment: data.get("environment"),
                    state: data.get("state"),
                    rationale: data.get("rationale"),
                    evidence_ids: data.getAll("evidence"),
                  }).then(() => form.reset());
                }}
              >
                <select name="repository_id">
                  {report.affected_repositories.map((item) => (
                    <option key={item.repository_id}>
                      {item.repository_id}
                    </option>
                  ))}
                </select>
                <input
                  name="version"
                  required
                  placeholder="Supported version or line"
                />
                <input name="environment" required placeholder="Environment" />
                <select name="state">
                  {["confirmed", "suspected", "unaffected", "fixed"].map(
                    (value) => (
                      <option key={value}>{value}</option>
                    ),
                  )}
                </select>
                <textarea name="rationale" required rows={2} />
                {[...report.evidence, ...(report.resource_links || [])].map(
                  (item) => (
                    <label key={item.id}>
                      <input type="checkbox" name="evidence" value={item.id} />
                      {"title" in item ? item.title : item.label}
                    </label>
                  ),
                )}
                <Button type="submit" disabled={busy}>
                  Update impact
                </Button>
              </form>
            </section>
            <section className="security-section">
              <h3>Repair coverage and release evidence</h3>
              {report.repairs?.length ? report.repairs.map((repair) => {
                const verification = repair.verification;
                return <article key={repair.id}>
                  <Badge tone={verification?.state === "attested" ? "accent" : "neutral"}>{verification?.state ?? "not verified"}</Badge>
                  <strong>{repair.version} · {repair.repository_id}</strong>
                  <p>{repair.outcome}</p>
                  {verification ? <>
                    <p>Exact candidate <code>{verification.revision}</code></p>
                    <ul>{verification.gates.map((gate) => <li key={gate.attempt_id}><span>{gate.kind.replaceAll("_", " ")} · {gate.name}</span><Badge tone={gate.state === "passed" ? "accent" : "neutral"}>{gate.state}</Badge></li>)}</ul>
                    <p>Approvals: {verification.approvals.length ? verification.approvals.map((approval) => `${approval.actor_id} (${approval.decision})`).join(", ") : "None"}</p>
                    {verification.integration_entry_id && <p>Protected integration <code>{verification.integration_entry_id}</code> · <code>{verification.integration_commit_id}</code></p>}
                    {verification.release_attestations.map((artifact) => <p key={`${artifact.release_id}:${artifact.artifact_id}`}>Release {artifact.version} · artifact <code>{artifact.artifact_id}</code> · SHA-256 <code>{artifact.artifact_sha256}</code></p>)}
                    <p>Remaining gaps: {verification.remaining_gaps.length ? verification.remaining_gaps.join("; ") : "None recorded"}</p>
                  </> : <p>No exact-candidate verification has started.</p>}
                </article>;
              }) : <p>No repair lines have been defined.</p>}
            </section>
            <section className="security-section">
              <h3>Read-only agent investigations</h3>
              {workerCredential && (
                <div className="token-reveal" role="status">
                  <strong>Copy the one-time worker credential now</strong>
                  <code>{workerCredential}</code>
                </div>
              )}
              {report.investigations?.map((item) => (
                <article key={item.id}>
                  <Badge>{item.state}</Badge>
                  <strong>{item.agent}</strong>
                  <p>{item.mandate}</p>
                  {item.records.map((record) => (
                    <p key={record.sequence}>
                      {record.actor_id}: {record.body}
                      {record.uncertainty &&
                        ` — uncertainty: ${record.uncertainty}`}
                    </p>
                  ))}
                </article>
              ))}
              <form onSubmit={delegate}>
                <textarea
                  name="mandate"
                  required
                  rows={3}
                  placeholder="Bounded investigation mandate"
                />
                {[...report.evidence, ...(report.resource_links || [])].map(
                  (item) => (
                    <label key={item.id}>
                      <input type="checkbox" name="evidence" value={item.id} />
                      {"title" in item ? item.title : item.label}
                    </label>
                  ),
                )}
                <Button type="submit" disabled={busy}>
                  Delegate selected evidence
                </Button>
              </form>
            </section>
            <section className="security-section">
              <h3>Bounded response team</h3>
              {report.response_team.length ? (
                <ul>
                  {report.response_team.map((member) => (
                    <li key={member.user_id}>
                      <code>{member.user_id}</code>
                      {maintainer && (
                        <button
                          onClick={() =>
                            void mutate(`/team/${member.user_id}`, "DELETE")
                          }
                        >
                          Revoke access
                        </button>
                      )}
                    </li>
                  ))}
                </ul>
              ) : (
                <p>No additional responders have access.</p>
              )}
            </section>
            <section className="security-section">
              <h3>Private conversation</h3>
              <ol>
                {report.messages.map((message) => (
                  <li key={message.id}>
                    <p>{message.body}</p>
                    <small>
                      {message.author_id === actor ? "You" : message.author_id}{" "}
                      · {new Date(message.created_at).toLocaleString()}
                    </small>
                  </li>
                ))}
              </ol>
              <form
                onSubmit={(event) => {
                  event.preventDefault();
                  const form = event.currentTarget;
                  void mutate("/messages", "POST", {
                    body: new FormData(form).get("body"),
                  }).then(() => form.reset());
                }}
              >
                <textarea
                  name="body"
                  required
                  rows={3}
                  placeholder="Share a private update with the reporter and response team"
                />
                <Button type="submit" disabled={busy}>
                  Send privately
                </Button>
              </form>
            </section>
            <details className="security-audit">
              <summary>Access audit · {report.audit_log.length} events</summary>
              <ol>
                {[...report.audit_log].reverse().map((event) => (
                  <li key={event.sequence}>
                    <strong>{event.type.replaceAll(".", " ")}</strong>
                    <span>
                      {event.actor_id}
                      {event.subject_id && ` → ${event.subject_id}`}
                    </span>
                    <time>{new Date(event.created_at).toLocaleString()}</time>
                  </li>
                ))}
              </ol>
            </details>
          </article>
        )}
      </div>
      {error && !creating && <p className="form-error">{error}</p>}
    </div>
  );
}

function Inbox({
  items,
  refresh,
}: {
  items: InboxItem[];
  refresh: () => Promise<void>;
}) {
  const [classification, setClassification] = useState<
    "all" | InboxItem["classification"]
  >("all");
  const visible =
    classification === "all"
      ? items
      : items.filter((item) => item.classification === classification);
  async function clear(id: string) {
    await api(`/inbox/${id}`, { method: "DELETE" });
    await refresh();
  }
  return (
    <>
      <div className="eyebrow">
        <Bell size={14} /> Attention
      </div>
      <div className="page-heading">
        <div>
          <h1>Inbox</h1>
          <p>
            Decisions, responses, and changes worth knowing about—together in
            one place.
          </p>
        </div>
      </div>
      <div className="inbox-filters" aria-label="Filter inbox">
        {(["all", "review", "response", "awareness"] as const).map((value) => (
          <button
            key={value}
            className={classification === value ? "active" : ""}
            aria-pressed={classification === value}
            onClick={() => setClassification(value)}
          >
            {value[0].toUpperCase() + value.slice(1)}
            <span>
              {value === "all"
                ? items.length
                : items.filter((item) => item.classification === value).length}
            </span>
          </button>
        ))}
      </div>
      {visible.length ? (
        <section className="panel inbox-list" aria-label="Inbox items">
          {visible.map((item) => (
            <article className="inbox-item" key={item.id}>
              <span className={`inbox-kind ${item.classification}`}>
                {item.classification}
              </span>
              <div>
                <Link href={item.href}>
                  <h2>{item.title}</h2>
                </Link>
                <p>{item.summary}</p>
                <small>
                  @{item.actor_handle} · {item.repository_name} ·{" "}
                  {new Date(item.created_at).toLocaleDateString()}
                </small>
              </div>
              <div className="inbox-actions">
                <Link className="button secondary sm" href={item.href}>
                  Open
                </Link>
                <button onClick={() => clear(item.id)}>Clear</button>
              </div>
            </article>
          ))}
        </section>
      ) : (
        <section className="first-step panel">
          <span className="empty-icon">
            <Check size={22} />
          </span>
          <h2>Nothing needs your attention</h2>
          <p>
            {classification === "all"
              ? "You’re caught up."
              : `No ${classification} items are waiting.`}
          </p>
        </section>
      )}
    </>
  );
}

function Repositories({
  user,
  repositories,
  discoverable,
  total,
  searching,
  showCreate,
  setShowCreate,
  refresh,
  full,
}: {
  user: UserRecord;
  repositories: Repository[];
  discoverable: Repository[];
  total: number;
  searching: boolean;
  showCreate: boolean;
  setShowCreate: (show: boolean) => void;
  refresh: () => Promise<void>;
  full: boolean;
}) {
  return (
    <>
      <div className="eyebrow">
        <Sparkles size={14} />
        {total ? "Your workspace" : "One step from collaboration"}
      </div>
      <div className="page-heading">
        <div>
          <h1>
            {total
              ? `${full ? "Repositories" : "Welcome back"}, ${user.display_name.split(" ")[0]}.`
              : `Welcome, ${user.display_name.split(" ")[0]}.`}
          </h1>
          <p>
            {total
              ? "Find a project or make room for the next idea."
              : "Create your first repository to give your work a home."}
          </p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          <Plus size={16} />
          New repository
        </Button>
      </div>
      {showCreate && (
        <CreateRepository
          close={() => setShowCreate(false)}
          refresh={refresh}
        />
      )}
      {repositories.length ? (
        <section aria-labelledby="repository-heading">
          <div className="section-heading">
            <div>
              <h2 id="repository-heading">
                {searching ? "Matching repositories" : "Your repositories"}
              </h2>
              <p>
                {repositories.length} owned or shared{" "}
                {repositories.length === 1 ? "project" : "projects"} ready for
                collaboration
              </p>
            </div>
          </div>
          <div className="repo-grid">
            {repositories.map((repo) => (
              <RepositoryTile key={repo.id} repository={repo} actor={user.id} />
            ))}
          </div>
        </section>
      ) : total ? (
        <EmptySearch />
      ) : (
        <FirstRepository onCreate={() => setShowCreate(true)} />
      )}{" "}
      {discoverable.length > 0 && (
        <section
          className="discovery-section"
          aria-labelledby="discovery-heading"
        >
          <div className="section-heading">
            <div>
              <h2 id="discovery-heading">Discover public projects</h2>
              <p>
                Explore a project, fork it, and propose a change without waiting
                for an invitation.
              </p>
            </div>
          </div>
          <div className="repo-grid">
            {discoverable.map((repo) => (
              <RepositoryTile key={repo.id} repository={repo} actor={user.id} />
            ))}
          </div>
        </section>
      )}
    </>
  );
}

function FirstRepository({ onCreate }: { onCreate: () => void }) {
  return (
    <section className="first-step panel">
      <span className="empty-icon">
        <Branch size={22} />
      </span>
      <Badge tone="accent">Next step</Badge>
      <h2>Start something worth sharing</h2>
      <p>
        A repository holds your code, decisions, and the conversation around
        both. Create one now; it starts private.
      </p>
      <Button onClick={onCreate}>
        <Plus size={16} />
        Create your first repository
      </Button>
      <div className="next-up">
        <Check size={16} />
        <span>
          <strong>After that</strong> Clone it with Git, publish a branch, and
          invite a contributor.
        </span>
      </div>
    </section>
  );
}
function EmptySearch() {
  return (
    <div className="first-step panel">
      <span className="empty-icon">
        <Search />
      </span>
      <h2>No repositories match</h2>
      <p>Try another name or description.</p>
    </div>
  );
}

function CreateRepository({
  close,
  refresh,
}: {
  close: () => void;
  refresh: () => Promise<void>;
}) {
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError("");
    const data = new FormData(event.currentTarget);
    try {
      await api("/repositories", {
        method: "POST",
        body: JSON.stringify({
          name: data.get("name"),
          description: data.get("description"),
          visibility: data.get("visibility"),
        }),
      });
      await refresh();
      close();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Could not create repository.",
      );
      setPending(false);
    }
  }
  return (
    <section className="create-panel panel" aria-labelledby="create-title">
      <div className="create-copy">
        <span className="repo-icon">
          <Book />
        </span>
        <div>
          <h2 id="create-title">Create a repository</h2>
          <p>
            Give the project a clear name. You can invite collaborators as soon
            as it exists.
          </p>
        </div>
      </div>
      <form className="repository-form" onSubmit={submit}>
        <label>
          Repository name
          <input
            name="name"
            required
            pattern="[a-z0-9._-]+"
            maxLength={100}
            placeholder="project-name"
          />
        </label>
        <label>
          Description <span>Optional</span>
          <textarea
            name="description"
            maxLength={280}
            placeholder="What are you building together?"
          />
        </label>
        <fieldset>
          <legend>Visibility</legend>
          <label className="radio-card">
            <input
              type="radio"
              name="visibility"
              value="private"
              defaultChecked
            />
            <Shield size={17} />
            <span>
              <strong>Private</strong>
              <small>Only you and invited contributors</small>
            </span>
          </label>
          <label className="radio-card">
            <input type="radio" name="visibility" value="public" />
            <Book size={17} />
            <span>
              <strong>Public</strong>
              <small>Anyone can view and clone</small>
            </span>
          </label>
        </fieldset>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <div className="form-actions">
          <Button type="button" variant="secondary" onClick={close}>
            Cancel
          </Button>
          <Button type="submit" disabled={pending}>
            {pending ? "Creating…" : "Create repository"}
          </Button>
        </div>
      </form>
    </section>
  );
}

function RepositoryTile({
  repository,
  actor,
}: {
  repository: Repository;
  actor: string;
}) {
  const [copied, setCopied] = useState(false);
  const clone = `${process.env.NEXT_PUBLIC_GIT_ORIGIN ?? "http://localhost:8080"}${repository.git_url}`;
  async function copy() {
    await navigator.clipboard.writeText(clone);
    setCopied(true);
    setTimeout(() => setCopied(false), 1800);
  }
  return (
    <article className="repo-card panel">
      <div className="repo-card-top">
        <span className="repo-icon">
          <Book size={18} />
        </span>
        <span className="repo-badges">
          <Badge>
            {repository.visibility === "private" ? "Private" : "Public"}
          </Badge>
          {repository.upstream_repository_id && (
            <Badge tone="accent">Fork</Badge>
          )}
          {repository.owner_id !== actor && (
            <Badge tone="accent">Contributing</Badge>
          )}
        </span>
      </div>
      <Link href={`/repositories/${repository.id}`}>
        <h3>{repository.name}</h3>
      </Link>
      <p>{repository.description || "No description yet."}</p>
      <div className="clone-row">
        <code>{clone}</code>
        <button
          onClick={copy}
          aria-label={`Copy clone URL for ${repository.name}`}
        >
          {copied ? <Check size={15} /> : <Copy size={15} />}
        </button>
      </div>
      <div className="repo-meta">
        <span>
          <Branch size={13} />
          main
        </span>
        <span>{repository.empty ? "Ready for first push" : "Active"}</span>
      </div>
    </article>
  );
}

function Access({
  grants,
  refresh,
}: {
  grants: Grant[];
  refresh: () => Promise<void>;
}) {
  const [showCreate, setShowCreate] = useState(false);
  const [token, setToken] = useState("");
  const [extensions, setExtensions] = useState<ExtensionRecord[]>([]);
  const [extensionError, setExtensionError] = useState("");
  const [peers, setPeers] = useState<FederationPeer[]>([]);
  const [remoteRepositories, setRemoteRepositories] = useState<FederatedRepository[]>([]);
  const [federationError, setFederationError] = useState("");
  useEffect(() => { void api<Envelope<ExtensionRecord>>("/extensions").then(value => setExtensions(value.items)); }, []);
  const loadPeers = () => api<Envelope<FederationPeer>>("/federation/peers").then(value => setPeers(value.items));
  const loadRemoteRepositories = () => api<Envelope<FederatedRepository>>("/federation/repositories").then(value => setRemoteRepositories(value.items));
  useEffect(() => { void loadPeers(); }, []);
  useEffect(() => { void loadRemoteRepositories(); }, []);
  const active = grants.filter(
    (grant) => !grant.revoked_at && new Date(grant.expires_at) > new Date(),
  );
  async function revoke(id: string) {
    await api(`/access-grants/${id}`, { method: "DELETE" });
    await refresh();
  }
  return (
    <>
      <div className="eyebrow">
        <Shield size={14} /> Security & tools
      </div>
      <div className="page-heading">
        <div>
          <h1>Active access</h1>
          <p>Review the sessions and credentials that can act as you.</p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          <Plus size={16} />
          New credential
        </Button>
      </div>
      {token && (
        <div className="token-reveal panel" role="status">
          <div>
            <strong>Copy your new credential now</strong>
            <p>For your security, Kanso won’t show it again.</p>
          </div>
          <code>{token}</code>
          <Button
            variant="secondary"
            onClick={() => navigator.clipboard.writeText(token)}
          >
            <Copy size={15} />
            Copy
          </Button>
          <button
            className="dismiss"
            onClick={() => setToken("")}
            aria-label="Dismiss credential"
          >
            ×
          </button>
        </div>
      )}
      {showCreate && (
        <CreateGrant
          close={() => setShowCreate(false)}
          created={(value) => {
            setToken(value);
            setShowCreate(false);
            void refresh();
          }}
        />
      )}
      <section className="panel access-list" aria-labelledby="access-heading">
        <div className="panel-title">
          <span id="access-heading">Credentials</span>
          <Badge tone="accent">{active.length} active</Badge>
        </div>
        {active.map((grant) => (
          <div className="grant-row" key={grant.id}>
            <span className="grant-icon">
              {grant.kind === "web" ? <User size={17} /> : <Key size={17} />}
            </span>
            <div>
              <strong>{grant.name}</strong>
              <p>
                {grant.kind === "web"
                  ? "Browser session"
                  : `${grant.kind.toUpperCase()} credential`}{" "}
                · Expires {new Date(grant.expires_at).toLocaleDateString()}
              </p>
              <small>{grant.scopes.join(" · ")}</small>
            </div>
            {grant.id ===
            grants.find((item) => item.kind === "web" && !item.revoked_at)
              ?.id ? (
              <Badge>Current</Badge>
            ) : (
              <button
                className="danger-button"
                onClick={() => revoke(grant.id)}
              >
                <Trash size={14} />
                Revoke
              </button>
            )}
          </div>
        ))}
      </section>
      <section className="panel access-list" aria-labelledby="extensions-heading">
        <div className="panel-title"><span id="extensions-heading">Extensions</span><Badge tone="accent">{extensions.filter(item => item.status === "verified").length} verified</Badge></div>
        <p>External collaborators use their own attributable identity. Registration never lets one act as you, another user, or an agent.</p>
        {extensions.map(item => <div className="grant-row" key={item.id}><span className="grant-icon"><Sparkles size={17}/></span><div><strong>{item.name}</strong><p>{item.operator_contact} · {item.status.replaceAll("_", " ")}</p><small>{item.requested_permissions.join(" · ")} · rotates every {item.rotation_policy.interval_days} days</small></div><Badge tone={item.status === "verified" ? "accent" : "neutral"}>{item.status}</Badge></div>)}
        <form className="repository-form compact" onSubmit={async event => { event.preventDefault(); const form=event.currentTarget;const data=new FormData(form);setExtensionError("");try{await api("/extensions",{method:"POST",body:JSON.stringify({name:data.get("name"),description:data.get("description"),operator_contact:data.get("contact"),capabilities:String(data.get("capabilities")).split(",").map(x=>x.trim()).filter(Boolean),callback_url:data.get("callback"),action_url:data.get("actions"),requested_permissions:data.getAll("permissions"),event_types:data.getAll("events"),rotation_policy:{interval_days:Number(data.get("interval")),overlap_hours:24,contact_on_failure:true}})});const value=await api<Envelope<ExtensionRecord>>("/extensions");setExtensions(value.items);form.reset()}catch(cause){setExtensionError(cause instanceof Error?cause.message:"Could not register extension.")}}}>
          <label>Name<input name="name" required maxLength={100} placeholder="Build observer"/></label><label>Operator contact<input name="contact" type="email" required placeholder="integrations@example.com"/></label><label className="wide">Description<textarea name="description" rows={2}/></label><label>Callback endpoint<input name="callback" type="url" pattern="https://.*" required placeholder="https://example.com/events"/></label><label>Action endpoint<input name="actions" type="url" pattern="https://.*" required placeholder="https://example.com/actions"/></label><label>Capabilities<input name="capabilities" required placeholder="Report checks, annotate changes"/></label><label>Rotation interval (days)<input name="interval" type="number" min="1" max="365" defaultValue="30" required/></label>
          <fieldset><legend>Requested permissions</legend>{["metadata:read","contents:read","issues:read","issues:write","pull_requests:read","pull_requests:write","checks:write"].map(x=><label key={x}><input type="checkbox" name="permissions" value={x}/>{x}</label>)}</fieldset>
          <fieldset><legend>Supported events</legend>{["repository.created","push","issue.opened","issue.updated","pull_request.opened","pull_request.updated","check.requested"].map(x=><label key={x}><input type="checkbox" name="events" value={x}/>{x}</label>)}</fieldset>
          {extensionError&&<p className="form-error wide">{extensionError}</p>}<div className="form-actions wide"><Button type="submit">Register extension</Button></div>
        </form>
      </section>
      <section className="panel access-list" aria-labelledby="federation-heading">
        <div className="panel-title"><span id="federation-heading">Federation identities</span><Badge tone="accent">{peers.filter(peer=>peer.trust==="trusted").length} trusted</Badge></div>
        <p>Discover signed instance capabilities, operators, keys, and deliberately published collaborators. Trust is local, revocable, and never grants remote administration or reveals private membership.</p>
        {peers.map(peer=><div className="grant-row" key={peer.discovery_url}><span className="grant-icon"><User size={17}/></span><div><strong>{peer.instance||peer.discovery_url}</strong><p>{peer.status} · {peer.trust}{peer.identity_changed?" · unchained identity change":""}</p><small>{peer.last_error||peer.document?.capabilities.join(" · ")||"No verified document"}</small>{peer.document?.actors.map(actor=><div key={actor.subject}><code>{actor.subject}</code> · {actor.display_name}</div>)}</div><div className="form-actions">{peer.status==="reachable"&&peer.trust!=="trusted"&&!peer.identity_changed&&<Button size="sm" variant="secondary" onClick={async()=>{await api("/federation/peer-trust",{method:"POST",body:JSON.stringify({instance:peer.instance,action:"trust"})});await loadPeers()}}>Trust</Button>}{peer.trust==="trusted"&&<Button size="sm" variant="secondary" onClick={async()=>{await api("/federation/peer-trust",{method:"POST",body:JSON.stringify({instance:peer.instance,action:"revoke"})});await loadPeers()}}>Revoke</Button>}</div></div>)}
        <form className="repository-form compact" onSubmit={async event=>{event.preventDefault();const form=event.currentTarget;const data=new FormData(form);setFederationError("");try{await api("/federation/peers/discoveries",{method:"POST",body:JSON.stringify({url:data.get("url")})});await loadPeers();form.reset()}catch(cause){setFederationError(cause instanceof Error?cause.message:"Discovery failed")}}}><label className="wide">Peer discovery URL<input name="url" type="url" required placeholder="https://community.example/.well-known/komodo-federation"/></label>{federationError&&<p className="form-error wide">{federationError}</p>}<div className="form-actions wide"><Button type="submit">Discover instance</Button></div></form>
      </section>
      <section className="panel access-list" aria-labelledby="federated-repositories-heading">
        <div className="panel-title"><span id="federated-repositories-heading">Federated repositories</span><Badge tone="accent">{remoteRepositories.filter(item=>item.followed).length} followed</Badge></div>
        <p>Resolve a public project through a trusted peer. Cached context remains visibly remote, revision-bound, and read-only.</p>
        {remoteRepositories.map(item=><div className="grant-row" key={item.reference}><span className="grant-icon"><Book size={17}/></span><div><Link href={`/federation/repositories?ref=${encodeURIComponent(item.reference)}`}><strong>{item.reference}</strong></Link><p>{item.status}{item.stale?" · stale cache":""}{item.visibility_changed?" · visibility changed":""}</p><small>{item.last_error||item.revision||"No verified snapshot"}</small></div>{item.followed&&<Badge tone="accent">Following</Badge>}</div>)}
        <form className="repository-form compact" onSubmit={async event=>{event.preventDefault();const form=event.currentTarget;const data=new FormData(form);setFederationError("");try{await api("/federation/repositories/resolutions",{method:"POST",body:JSON.stringify({reference:data.get("reference"),follow:data.get("follow")==="on"})});await loadRemoteRepositories();form.reset()}catch(cause){setFederationError(cause instanceof Error?cause.message:"Resolution failed")}}}><label className="wide">Repository reference<input name="reference" required placeholder="repository:repo_id@https://community.example"/></label><label><input name="follow" type="checkbox" defaultChecked/> Follow updates locally</label>{federationError&&<p className="form-error wide">{federationError}</p>}<div className="form-actions wide"><Button type="submit">Resolve repository</Button></div></form>
      </section>
    </>
  );
}

function CreateGrant({
  close,
  created,
}: {
  close: () => void;
  created: (token: string) => void;
}) {
  const [kind, setKind] = useState<"git" | "api">("git");
  const [error, setError] = useState("");
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const scopes =
      kind === "git"
        ? ["git:read", "git:write"]
        : ["profile:read", "repository:read", "repository:write"];
    try {
      const grant = await api<Grant & { token: string }>("/access-grants", {
        method: "POST",
        body: JSON.stringify({
          name: data.get("name"),
          kind,
          scopes,
          expires_in_hours: Number(data.get("expires_in_hours")),
        }),
      });
      created(grant.token);
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Could not create credential.",
      );
    }
  }
  return (
    <section className="create-panel panel">
      <div className="create-copy">
        <span className="repo-icon">
          <Key />
        </span>
        <div>
          <h2>Create a credential</h2>
          <p>
            Use a short-lived Git credential to clone and push, or an API
            credential for tools.
          </p>
        </div>
      </div>
      <form className="repository-form compact" onSubmit={submit}>
        <label>
          Name
          <input name="name" required maxLength={80} placeholder="My laptop" />
        </label>
        <fieldset>
          <legend>Kind</legend>
          <label className="radio-card">
            <input
              type="radio"
              checked={kind === "git"}
              onChange={() => setKind("git")}
            />
            <Branch size={17} />
            <span>
              <strong>Git</strong>
              <small>Clone, fetch, and push</small>
            </span>
          </label>
          <label className="radio-card">
            <input
              type="radio"
              checked={kind === "api"}
              onChange={() => setKind("api")}
            />
            <Key size={17} />
            <span>
              <strong>API</strong>
              <small>Use scripts and developer tools</small>
            </span>
          </label>
        </fieldset>
        <label>
          Lifetime
          <select
            name="expires_in_hours"
            defaultValue={kind === "git" ? 720 : 2160}
            key={kind}
          >
            <option value="24">1 day</option>
            <option value="168">7 days</option>
            <option value={kind === "git" ? "720" : "2160"}>
              {kind === "git" ? "30 days" : "90 days"}
            </option>
          </select>
        </label>
        {error && <p className="form-error">{error}</p>}
        <div className="form-actions">
          <Button type="button" variant="secondary" onClick={close}>
            Cancel
          </Button>
          <Button type="submit">Create credential</Button>
        </div>
      </form>
    </section>
  );
}
