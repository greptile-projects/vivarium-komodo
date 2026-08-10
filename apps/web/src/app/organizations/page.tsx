"use client";

import Link from "next/link";
import { FormEvent, useCallback, useEffect, useState } from "react";
import { AppShell } from "@/components/app-shell";
import { Badge, Button } from "@/components/ui";

type Member = { user_id: string; role: string; accepted_at?: string };
type Transfer = {
  id: string;
  repository_id: string;
  from_kind: string;
  from_id: string;
  to_kind: string;
  to_id: string;
  state: string;
};
type Organization = {
  id: string;
  slug: string;
  name: string;
  description: string;
  members: Member[];
  transfers: Transfer[];
  teams: { id: string; name: string }[];
  agents: { id: string; name: string; operator_ids: string[] }[];
  role_grants: RoleGrant[];
  access_requests: AccessRequest[];
};
type ResourceRef = { kind: string; id: string; repository_id?: string };
type RoleGrant = { id: string; principal_kind: string; principal_id: string; role: string; resources: ResourceRef[]; exceptions: string[]; reason: string; expires_at: string; revoked_at?: string };
type AccessRequest = RoleGrant & { requested_by_id: string; state: string; grant_id?: string };
type Repository = {
  id: string;
  name: string;
  description: string;
  visibility: string;
};
type Portfolio = {
  repositories: Repository[];
  packages: { id: string; identity: string; version: string }[];
  active_work: { id: string; title: string; repository_id: string }[];
  releases: { id: string; version: string; repository_id: string }[];
  incidents: { id: string; title: string; status: string }[];
};
type InitiativeItem = {
  id: string; title: string; outcome: string; position: number; state: string;
  repository_id: string; source: ResourceRef; depends_on: string[];
  assignee_kind: string; assignee_id: string; contributions: ResourceRef[];
  upcoming_release_ids: string[]; policy_exception_ids: string[];
  blocked_by: string[]; needs_reassignment: boolean; next_decision?: string;
};
type Initiative = { id: string; title: string; outcome: string; state: string; sources: ResourceRef[]; items: InitiativeItem[] };
type Envelope<T> = { items: T[] };
type Session = { user: { id: string } };
type StewardshipMandate = {
  id: string; version: number; title: string; desired_outcomes: string[];
  scopes: { repository_id: string; branches: string[] }[]; trusted_signals: string[];
  exclusions: string[]; budget: { max_hours_per_month: number; max_runs_per_day: number };
  schedule: { starts_at: string; expires_at: string; cadence: string }; agent_id: string;
  allowed_actions: string[]; required_human_decisions: string[]; state: string;
  acceptance?: { operator_id: string; accepted_at: string };
};
type StewardshipPreview = { state: string; authority_created_by_mandate: boolean; mandate_write_authority: boolean; mandate_merge_authority: boolean; note: string; scopes: Record<string, { branches: string[]; effective_policy: unknown[]; existing_agent_grants: RoleGrant[] }> };
type StewardshipOpportunity = {
  id: string; mandate_id: string; mandate_version: number; repository_id: string;
  title: string; summary: string; severity: string; expected_value: string; confidence: number; class: string;
  affected_owner_ids: string[]; affected_revisions: string[]; in_scope_reason: string;
  state: string; rank: number; evidence_stale: boolean; stale_reason?: string; updated_at: string;
  citations: { kind: string; resource_id: string; revision: string; summary: string; observed_at: string }[];
  comments: { id: string; actor_id: string; body: string; created_at: string }[];
  decisions: { action: string; actor_id: string; reason?: string; created_at: string }[];
  work_decisions: { version: number; policy_version: number; mode: string; state: string; risk: string; hours: number; blockers: string[]; actor_id: string; created_at: string }[];
  work?: { proposal_id: string; task_ids: string[]; base_revision: string; promoted_by_id: string };
};
type StewardshipWorkPolicy = { mandate_id: string; mandate_version: number; version: number; rules: { class: string; mode: string; max_risk?: string; max_runs_per_day?: number; max_hours_per_month?: number; priority?: number; min_confidence?: number; required_evidence?: string[] }[] };
type StewardshipReport = {
  mandate_id: string; mandate_version: number; mandate_state: string;
  opportunity_disposition: Record<string, number>; recommendation_decision: Record<string, number>;
  implementation_outcomes: Record<string, number>; verification_results: Record<string, number>; release_results: Record<string, number>;
  resource_use: Record<string, number>; false_positives: number; goal_progress: Record<string, Record<string, number>>;
  notices: { id: string; kind: string; detail: string; action: string; created_at: string }[];
};

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });
  if (!response.ok)
    throw new Error(
      (await response.json().catch(() => ({}))).error ?? "Request failed",
    );
  return response.status === 204 ? (undefined as T) : response.json();
}

function MandateForm({ repositories, agents, mandate, onSubmit }: { repositories: Repository[]; agents: Organization["agents"]; mandate?: StewardshipMandate; onSubmit: (event: FormEvent<HTMLFormElement>) => void }) {
  const scope = (id: string) => mandate?.scopes.find((item) => item.repository_id === id);
  const localTime = (value?: string) => value ? new Date(value).toISOString().slice(0, 16) : "";
  return <form className="stack" onSubmit={onSubmit}>
    <label>Mandate title<input required name="title" defaultValue={mandate?.title} /></label>
    <label>Approved agent<select required name="agent_id" defaultValue={mandate?.agent_id ?? ""}><option value="">Select…</option>{agents.map((agent) => <option key={agent.id} value={agent.id}>{agent.name}</option>)}</select></label>
    <label>Desired outcomes (one per line)<textarea required name="outcomes" defaultValue={mandate?.desired_outcomes.join("\n")} /></label>
    <fieldset><legend>Repository and branch scope</legend>{repositories.map((repository) => <div className="repo-row" key={repository.id}><label><input type="checkbox" name="repository_id" value={repository.id} defaultChecked={Boolean(scope(repository.id))} /> {repository.name}</label><label>Branches<textarea name={`branches:${repository.id}`} defaultValue={scope(repository.id)?.branches.join("\n")} placeholder="main" /></label></div>)}</fieldset>
    <label>Trusted signals (one per line)<textarea required name="signals" defaultValue={mandate?.trusted_signals.join("\n")} placeholder="Required check failure" /></label>
    <label>Exclusions / stop conditions<textarea name="exclusions" defaultValue={mandate?.exclusions.join("\n")} /></label>
    <label>Allowed actions<textarea required name="actions" defaultValue={mandate?.allowed_actions.join("\n")} placeholder="Inspect check evidence&#10;Draft a proposal" /></label>
    <label>Required human decisions<textarea required name="decisions" defaultValue={mandate?.required_human_decisions.join("\n")} placeholder="Merge&#10;Release promotion" /></label>
    <div className="content-grid"><label>Hours / month<input required min="1" max="744" type="number" name="hours" defaultValue={mandate?.budget.max_hours_per_month ?? 20} /></label><label>Runs / day<input required min="1" max="100" type="number" name="runs" defaultValue={mandate?.budget.max_runs_per_day ?? 3} /></label></div>
    <div className="content-grid"><label>Starts<input required type="datetime-local" name="starts_at" defaultValue={localTime(mandate?.schedule.starts_at)} /></label><label>Expires<input required type="datetime-local" name="expires_at" defaultValue={localTime(mandate?.schedule.expires_at)} /></label></div>
    <label>Cadence<select name="cadence" defaultValue={mandate?.schedule.cadence ?? "daily"}><option value="continuous">Continuous</option><option value="daily">Daily</option><option value="weekly">Weekly</option></select></label>
    <Button type="submit">{mandate ? "Create revised version" : "Draft mandate"}</Button>
  </form>;
}

export default function OrganizationsPage() {
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [selected, setSelected] = useState<Organization>();
  const [portfolio, setPortfolio] = useState<Portfolio>();
  const [message, setMessage] = useState("");
  const [userID, setUserID] = useState("");
  const [effective, setEffective] = useState<RoleGrant[]>([]);
  const [initiatives, setInitiatives] = useState<Initiative[]>([]);
  const [mandates, setMandates] = useState<StewardshipMandate[]>([]);
  const [previews, setPreviews] = useState<Record<string, StewardshipPreview>>({});
  const [opportunities, setOpportunities] = useState<StewardshipOpportunity[]>([]);
  const [workPolicies, setWorkPolicies] = useState<StewardshipWorkPolicy[]>([]);
  const [stewardshipReports, setStewardshipReports] = useState<Record<string, StewardshipReport>>({});
  const selectedID = selected?.id;
  const load = useCallback(async () => {
    try {
      const [data, session] = await Promise.all([
        api<Envelope<Organization>>("/organizations"),
        api<Session>("/session"),
      ]);
      setOrganizations(data.items);
      setUserID(session.user.id);
      if (selectedID) {
        const [organization, view, access, initiativeView, stewardship, stewardshipQueue] = await Promise.all([
          api<Organization>(`/organizations/${selectedID}`),
          api<Portfolio>(`/organizations/${selectedID}/portfolio`),
          api<{ items: RoleGrant[] }>(`/organizations/${selectedID}/access/effective`),
          api<{ items: Initiative[] }>(`/organizations/${selectedID}/initiatives`),
          api<{ items: StewardshipMandate[] }>(`/organizations/${selectedID}/stewardship-mandates`),
          api<{ items: StewardshipOpportunity[]; work_policies: StewardshipWorkPolicy[] }>(`/organizations/${selectedID}/stewardship-opportunities`),
        ]);
        setSelected(organization);
        setPortfolio(view);
        setEffective(access.items);
        setInitiatives(initiativeView.items);
        setMandates(stewardship.items);
        setOpportunities(stewardshipQueue.items.map((item) => ({ ...item, work_decisions: item.work_decisions ?? [] })));
        setWorkPolicies(stewardshipQueue.work_policies ?? []);
        const reportEntries = await Promise.all(stewardship.items.map(async (mandate) => [`${mandate.id}:${mandate.version}`, await api<StewardshipReport>(`/organizations/${selectedID}/stewardship-mandates/${mandate.id}/versions/${mandate.version}/report`)] as const));
        setStewardshipReports(Object.fromEntries(reportEntries));
      }
    } catch (error) {
      setMessage(
        error instanceof Error ? error.message : "Unable to load organizations",
      );
    }
  }, [selectedID]);
  useEffect(() => {
    // Loading is the external synchronization this route performs on mount.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);
  async function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const organization = await api<Organization>("/organizations", {
      method: "POST",
      body: JSON.stringify({
        slug: form.get("slug"),
        name: form.get("name"),
        description: form.get("description"),
      }),
    });
    event.currentTarget.reset();
    setSelected(organization);
    setMessage("Organization created.");
  }
  async function invite(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected) return;
    const form = new FormData(event.currentTarget);
    await api(`/organizations/${selected.id}/members`, {
      method: "POST",
      body: JSON.stringify({ handle: form.get("handle") }),
    });
    event.currentTarget.reset();
    setMessage("Invitation recorded. The collaborator must accept it.");
    await load();
  }
  async function createRepository(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected) return;
    const form = new FormData(event.currentTarget);
    await api(`/organizations/${selected.id}/repositories`, {
      method: "POST",
      body: JSON.stringify({
        name: form.get("name"),
        description: form.get("description"),
        visibility: form.get("visibility"),
      }),
    });
    event.currentTarget.reset();
    setMessage("Repository created in the organization.");
    await load();
  }
  async function acceptTransfer(id: string) {
    if (!selected) return;
    await api(
      `/organizations/${selected.id}/repository-transfers/${id}/accept`,
      { method: "POST" },
    );
    setMessage(
      "Ownership transfer accepted without changing repository identity.",
    );
    await load();
  }
  async function requestTransfer(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected) return;
    const form = new FormData(event.currentTarget);
    await api(`/organizations/${selected.id}/repository-transfers`, {
      method: "POST",
      body: JSON.stringify({ repository_id: form.get("repository_id") }),
    });
    event.currentTarget.reset();
    setMessage("Transfer requested. The organization owner must accept control.");
    await load();
  }
  async function acceptMembership(id: string) {
    await api(`/organizations/${id}/members/accept`, { method: "POST" });
    setSelected(await api<Organization>(`/organizations/${id}`));
    setMessage("Organization invitation accepted.");
  }
  async function removeMember(id: string) {
    if (!selected) return;
    await api(`/organizations/${selected.id}/members/${id}`, {
      method: "DELETE",
    });
    setMessage("Member removed from the organization portfolio.");
    await load();
  }
  async function submitAccess(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected) return;
    const form = new FormData(event.currentTarget);
    const repositoryID = String(form.get("repository_id"));
    const principalID = String(form.get("principal_id"));
    const body = {
      principal_kind: selected.agents?.some((agent) => agent.id === principalID) ? "agent" : "team", principal_id: principalID, role: form.get("role"),
      resources: [{ kind: "repository", id: repositoryID }],
      exceptions: String(form.get("exceptions") || "").split(",").map((x) => x.trim()).filter(Boolean),
      reason: form.get("reason"), expires_at: new Date(String(form.get("expires_at"))).toISOString(),
    };
    const owner = selected.members.some((member) => member.user_id === userID && member.role === "owner");
    await api(`/organizations/${selected.id}/${owner ? "access-grants" : "access-requests"}`, { method: "POST", body: JSON.stringify(body) });
    event.currentTarget.reset(); setMessage(owner ? "Scoped authority granted." : "Access request sent for owner review."); await load();
  }
  async function resolveRequest(id: string, decision: "approved" | "denied") {
    if (!selected) return;
    await api(`/organizations/${selected.id}/access-requests/${id}`, { method: "POST", body: JSON.stringify({ decision }) });
    setMessage(`Access request ${decision}.`); await load();
  }
  async function revokeRole(id: string) {
    if (!selected) return;
    await api(`/organizations/${selected.id}/access-grants/${id}`, { method: "DELETE" });
    setMessage("Authority and its derived credentials were revoked."); await load();
  }
  async function createInitiative(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); if (!selected) return;
    const form = new FormData(event.currentTarget);
    await api(`/organizations/${selected.id}/initiatives`, { method: "POST", body: JSON.stringify({
      title: form.get("title"), outcome: form.get("outcome"), sources: [{ kind: form.get("source_kind"), id: form.get("source_id"), repository_id: form.get("repository_id") }],
    }) });
    event.currentTarget.reset(); setMessage("Portfolio initiative created from authoritative work."); await load();
  }
  async function createInitiativeItem(event: FormEvent<HTMLFormElement>, initiativeID: string) {
    event.preventDefault(); if (!selected) return;
    const form = new FormData(event.currentTarget);
    await api(`/organizations/${selected.id}/initiatives/${initiativeID}/items`, { method: "POST", body: JSON.stringify({
      title: form.get("title"), outcome: form.get("outcome"), repository_id: form.get("repository_id"), state: "planned",
      source: { kind: form.get("source_kind"), id: form.get("source_id"), repository_id: form.get("repository_id") },
      assignee_kind: form.get("assignee_kind"), assignee_id: form.get("assignee_id"),
      depends_on: String(form.get("depends_on") || "").split(",").map((x) => x.trim()).filter(Boolean), next_decision: form.get("next_decision"),
    }) });
    event.currentTarget.reset(); setMessage("Cross-repository work connected to the initiative."); await load();
  }
  const lines = (value: FormDataEntryValue | null) => String(value || "").split("\n").map((item) => item.trim()).filter(Boolean);
  async function saveMandate(event: FormEvent<HTMLFormElement>, mandateID?: string) {
    event.preventDefault(); if (!selected) return; const form = new FormData(event.currentTarget);
    const repositoryIDs = form.getAll("repository_id").map(String).filter(Boolean);
    const body = { title: form.get("title"), agent_id: form.get("agent_id"), desired_outcomes: lines(form.get("outcomes")), trusted_signals: lines(form.get("signals")), exclusions: lines(form.get("exclusions")), allowed_actions: lines(form.get("actions")), required_human_decisions: lines(form.get("decisions")), scopes: repositoryIDs.map((repository_id) => ({ repository_id, branches: lines(form.get(`branches:${repository_id}`)) })), budget: { max_hours_per_month: Number(form.get("hours")), max_runs_per_day: Number(form.get("runs")) }, schedule: { starts_at: new Date(String(form.get("starts_at"))).toISOString(), expires_at: new Date(String(form.get("expires_at"))).toISOString(), cadence: form.get("cadence") } };
    await api(`/organizations/${selected.id}/stewardship-mandates${mandateID ? `/${mandateID}/versions` : ""}`, { method: "POST", body: JSON.stringify(body) });
    event.currentTarget.reset(); setMessage(mandateID ? "A new mandate version is awaiting operator acceptance." : "Stewardship mandate drafted for operator acceptance."); await load();
  }
  async function transitionMandate(mandate: StewardshipMandate, action: "accept" | "pause" | "resume" | "revoke") {
    if (!selected) return; await api(`/organizations/${selected.id}/stewardship-mandates/${mandate.id}/versions/${mandate.version}/${action}`, { method: "POST" }); setMessage(`Mandate ${action} recorded.`); await load();
  }
  async function previewMandate(mandate: StewardshipMandate) {
    if (!selected) return; const preview = await api<StewardshipPreview>(`/organizations/${selected.id}/stewardship-mandates/${mandate.id}/versions/${mandate.version}/preview`); setPreviews((current) => ({ ...current, [`${mandate.id}:${mandate.version}`]: preview }));
  }
  async function opportunityDecision(opportunity: StewardshipOpportunity, action: string, detail?: string | number) {
    if (!selected) return;
    const body: Record<string, unknown> = { action };
    if (action === "rank") body.rank = detail;
    else if (action === "snooze") body.snoozed_until = new Date(Date.now() + 7 * 86400000).toISOString();
    else if (detail) body.reason = detail;
    await api(`/organizations/${selected.id}/stewardship-opportunities/${opportunity.id}/decisions`, { method: "POST", body: JSON.stringify(body) });
    setMessage(`Opportunity ${action} recorded.`); await load();
  }
  async function commentOnOpportunity(event: FormEvent<HTMLFormElement>, opportunity: StewardshipOpportunity) {
    event.preventDefault(); if (!selected) return; const form = new FormData(event.currentTarget);
    await api(`/organizations/${selected.id}/stewardship-opportunities/${opportunity.id}/comments`, { method: "POST", body: JSON.stringify({ body: form.get("body") }) });
    event.currentTarget.reset(); setMessage("Challenge added to the shared finding."); await load();
  }
  async function saveWorkPolicy(event: FormEvent<HTMLFormElement>, mandate: StewardshipMandate) {
    event.preventDefault(); if (!selected) return; const form = new FormData(event.currentTarget); const current = workPolicies.filter((policy) => policy.mandate_id === mandate.id && policy.mandate_version === mandate.version).sort((a, b) => b.version - a.version)[0];
    await api(`/organizations/${selected.id}/stewardship-mandates/${mandate.id}/versions/${mandate.version}/work-policy`, { method: "PUT", body: JSON.stringify({ expected_version: current?.version ?? 0, rules: [{ class: form.get("class"), mode: form.get("mode"), max_risk: form.get("risk"), max_runs_per_day: Number(form.get("runs")), max_hours_per_month: Number(form.get("hours")), priority: Number(form.get("priority")), min_confidence: Number(form.get("confidence")) / 100, required_evidence: lines(form.get("evidence")) }] }) });
    setMessage("Opportunity admission policy version recorded."); await load();
  }
  async function admitOpportunity(event: FormEvent<HTMLFormElement>, opportunity: StewardshipOpportunity) {
    event.preventDefault(); if (!selected) return; const form = new FormData(event.currentTarget); const policy = workPolicies.filter((item) => item.mandate_id === opportunity.mandate_id && item.mandate_version === opportunity.mandate_version).sort((a, b) => b.version - a.version)[0];
    await api(`/organizations/${selected.id}/stewardship-opportunities/${opportunity.id}/work-decisions`, { method: "POST", body: JSON.stringify({ mode: form.get("mode"), risk: form.get("risk"), hours: Number(form.get("hours")), expected_decision_version: opportunity.work_decisions?.length ?? 0, expected_policy_version: policy?.version ?? 0 }) });
    setMessage("Governed work decision recorded."); await load();
  }
  async function promoteOpportunity(event: FormEvent<HTMLFormElement>, opportunity: StewardshipOpportunity) {
    event.preventDefault(); if (!selected) return; const form = new FormData(event.currentTarget);
    await api(`/organizations/${selected.id}/stewardship-opportunities/${opportunity.id}/promotion`, { method: "POST", body: JSON.stringify({ title: form.get("proposal_title"), body: opportunity.summary, base_revision: form.get("base_revision"), tasks: [{ title: form.get("task_title"), outcome: form.get("outcome"), owner_kind: form.get("owner_kind"), owner_id: form.get("owner_id"), completion_criteria: lines(form.get("criteria")), risk: form.get("risk"), verification_plan: lines(form.get("verification")), depends_on: [] }] }) });
    setMessage("Opportunity promoted into ordinary proposal work."); await load();
  }
  async function recordStewardshipOutcome(event: FormEvent<HTMLFormElement>, opportunity: StewardshipOpportunity) {
    event.preventDefault(); if (!selected) return; const form = new FormData(event.currentTarget); const mandate = mandates.find((item) => item.id === opportunity.mandate_id && item.version === opportunity.mandate_version); if (!mandate) return;
    const observedAt = new Date().toISOString();
    await api(`/organizations/${selected.id}/stewardship-opportunities/${opportunity.id}/outcomes`, { method: "POST", body: JSON.stringify({ implementation: form.get("implementation"), verification: form.get("verification"), release: form.get("release"), actual_hours: Number(form.get("actual_hours")), runs: Number(form.get("runs")), false_positive: form.get("false_positive") === "on", summary: form.get("summary"), evidence: [{ kind: form.get("evidence_kind"), resource_id: form.get("evidence_id"), repository_id: opportunity.repository_id, revision: form.get("revision"), summary: form.get("evidence_summary"), observed_at: observedAt }], goal_results: mandate.desired_outcomes.map((goal) => ({ goal, state: form.get("goal_state"), evidence: form.get("goal_evidence") })) }) });
    event.currentTarget.reset(); setMessage("Stewardship outcome added to the mandate history."); await load();
  }
  async function recordStewardshipHealth(event: FormEvent<HTMLFormElement>, mandate: StewardshipMandate) {
    event.preventDefault(); if (!selected) return; const form = new FormData(event.currentTarget);
    await api(`/organizations/${selected.id}/stewardship-mandates/${mandate.id}/versions/${mandate.version}/health-events`, { method: "POST", body: JSON.stringify({ kind: form.get("kind"), detail: form.get("detail") }) });
    event.currentTarget.reset(); setMessage("Affected stewardship automation paused with an actionable notice."); await load();
  }

  return (
    <AppShell>
      <section className="page-header">
        <div>
          <p className="eyebrow">Accountable groups</p>
          <h1>Organizations</h1>
          <p>
            Create a durable home for repositories and the evidence around their
            delivery.
          </p>
        </div>
        <Link className="button secondary" href="/">
          Back to workspace
        </Link>
      </section>
      {message && <div className="notice">{message}</div>}
      <div className="content-grid">
        <aside className="panel">
          <h2>Your organizations</h2>
          {organizations.map((org) => {
            const membership = org.members.find(
              (member) => member.user_id === userID,
            );
            return membership && !membership.accepted_at ? (
              <div className="repo-row" key={org.id}>
                <strong>{org.name}</strong>
                <Button size="sm" onClick={() => acceptMembership(org.id)}>
                  Accept invitation
                </Button>
              </div>
            ) : (
              <button
                className="nav-item"
                key={org.id}
                onClick={() => {
                  setSelected(org);
                  setPortfolio(undefined);
                }}
              >
                {org.name}
                <Badge>{org.slug}</Badge>
              </button>
            );
          })}
          <form className="stack" onSubmit={create}>
            <h3>New organization</h3>
            <label>
              Slug
              <input required name="slug" pattern="[a-z0-9-]+" />
            </label>
            <label>
              Name
              <input required name="name" />
            </label>
            <label>
              Description
              <textarea name="description" />
            </label>
            <Button type="submit">Create organization</Button>
          </form>
        </aside>
        <div className="stack">
          {selected ? (
            <>
              <section className="panel">
                <p className="eyebrow">@{selected.slug}</p>
                <h2>{selected.name}</h2>
                <p>{selected.description}</p>
                <div className="stats">
                  <span>
                    <strong>{portfolio?.repositories.length ?? 0}</strong>{" "}
                    repositories
                  </span>
                  <span>
                    <strong>
                      {
                        selected.members.filter((member) => member.accepted_at)
                          .length
                      }
                    </strong>{" "}
                    members
                  </span>
                  <span>
                    <strong>{portfolio?.active_work.length ?? 0}</strong> active
                    requests
                  </span>
                </div>
              </section>
              <section className="panel">
                <h2>Portfolio</h2>
                <div className="repo-list">
                  {portfolio?.repositories.map((repo) => (
                    <Link
                      key={repo.id}
                      href={`/repositories/${repo.id}`}
                      className="repo-row"
                    >
                      <strong>{repo.name}</strong>
                      <span>{repo.description || "No description"}</span>
                      <Badge>{repo.visibility}</Badge>
                    </Link>
                  ))}
                </div>
                <p>
                  {portfolio?.packages.length ?? 0} packages ·{" "}
                  {portfolio?.releases.length ?? 0} releases ·{" "}
                  {portfolio?.incidents.length ?? 0} active incidents
                </p>
              </section>
              <section className="panel">
                <p className="eyebrow">Bounded proactive care</p>
                <h2>Stewardship mandates</h2>
                <p>State what success means, which signals the agent may trust, and where it must stop. A mandate creates accountability—not a credential, write access, or merge authority.</p>
                {mandates.map((mandate) => {
                  const key = `${mandate.id}:${mandate.version}`; const preview = previews[key]; const report = stewardshipReports[key];
                  return <div className="stack" key={key}>
                    <div className="repo-row"><span><strong>{mandate.title}</strong> · version {mandate.version}<br />Agent {selected.agents.find((agent) => agent.id === mandate.agent_id)?.name ?? mandate.agent_id} · {mandate.schedule.cadence} · expires {new Date(mandate.schedule.expires_at).toLocaleString()}<br />{mandate.scopes.map((item) => `${portfolio?.repositories.find((repo) => repo.id === item.repository_id)?.name ?? item.repository_id} (${item.branches.join(", ")})`).join(" · ")}</span><Badge>{mandate.state}</Badge></div>
                    <p><strong>Outcomes:</strong> {mandate.desired_outcomes.join(" · ")}<br /><strong>May:</strong> {mandate.allowed_actions.join(" · ")}<br /><strong>Human decisions:</strong> {mandate.required_human_decisions.join(" · ")}</p>
                    {mandate.acceptance && <p>Accepted by operator <strong>{mandate.acceptance.operator_id}</strong> at {new Date(mandate.acceptance.accepted_at).toLocaleString()}.</p>}
                    <div><Button size="sm" variant="secondary" onClick={() => previewMandate(mandate)}>Preview effective policy</Button>{mandate.state === "pending_acceptance" && <Button size="sm" onClick={() => transitionMandate(mandate, "accept")}>Accept as operator</Button>}{mandate.state === "active" && <Button size="sm" variant="secondary" onClick={() => transitionMandate(mandate, "pause")}>Pause</Button>}{mandate.state === "paused" && <Button size="sm" variant="secondary" onClick={() => transitionMandate(mandate, "resume")}>Resume</Button>}{mandate.state !== "revoked" && mandate.state !== "expired" && <Button size="sm" variant="secondary" onClick={() => transitionMandate(mandate, "revoke")}>Revoke</Button>}</div>
                    {preview && <div className="notice"><strong>No implicit authority:</strong> {preview.note}<br />Mandate write: {String(preview.mandate_write_authority)} · mandate merge: {String(preview.mandate_merge_authority)} · mandate-created authority: {String(preview.authority_created_by_mandate)}<br />{Object.entries(preview.scopes).map(([repository, detail]) => `${portfolio?.repositories.find((repo) => repo.id === repository)?.name ?? repository}: ${detail.effective_policy.length} policy rules, ${detail.existing_agent_grants.length} existing independent grants`).join(" · ")}</div>}
                    {report && <details><summary>Stewardship history and mandate progress</summary><div className="stack"><p><strong>Recommendations:</strong> {Object.entries(report.opportunity_disposition).map(([state, count]) => `${state} ${count}`).join(" · ") || "none"}<br /><strong>Admission:</strong> {Object.entries(report.recommendation_decision).map(([state, count]) => `${state} ${count}`).join(" · ") || "none"}<br /><strong>Implementation:</strong> {Object.entries(report.implementation_outcomes).map(([state, count]) => `${state} ${count}`).join(" · ") || "none"}<br /><strong>Verification:</strong> {Object.entries(report.verification_results).map(([state, count]) => `${state} ${count}`).join(" · ") || "none"}<br /><strong>Release:</strong> {Object.entries(report.release_results).map(([state, count]) => `${state} ${count}`).join(" · ") || "none"}</p><p><strong>Resources:</strong> {report.resource_use.actual_hours ?? 0} actual hours · {report.resource_use.estimated_hours ?? 0} estimated hours · {report.resource_use.runs ?? 0} runs · {report.false_positives} false positives</p>{Object.entries(report.goal_progress).map(([goal, progress]) => <p key={goal}><strong>{goal}:</strong> {Object.entries(progress).map(([state, count]) => `${state} ${count}`).join(" · ")}</p>)}{report.notices.map((notice) => <div className="notice" key={notice.id}><strong>{notice.kind.replaceAll("_", " ")}:</strong> {notice.detail}<br />Next action: {notice.action.replaceAll("_", " ")}</div>)}</div></details>}
                    <details><summary>Tune priorities and evidence rules from history</summary><p>This policy can change ranking and evidence admission only. Scope, actions, budgets, agents, and authority require a new mandate version.</p><form className="stack" onSubmit={(event) => saveWorkPolicy(event, mandate)}><label>Opportunity class<input required name="class" placeholder="check_failure" /></label><div className="content-grid"><label>Admission<select name="mode"><option value="approval">Human approval</option><option value="auto_start">Bounded auto-start</option></select></label><label>Maximum risk<select name="risk"><option>low</option><option>medium</option><option>high</option><option>critical</option></select></label><label>Priority (0–1000)<input name="priority" type="number" min="0" max="1000" defaultValue="100" /></label><label>Minimum confidence %<input name="confidence" type="number" min="0" max="100" defaultValue="80" /></label><label>Runs / day<input name="runs" type="number" min="0" defaultValue="1" /></label><label>Hours / month<input name="hours" type="number" min="0" defaultValue="4" /></label></div><label>Required evidence kinds (one per line)<textarea name="evidence" placeholder="check_run" /></label><Button size="sm" type="submit">Record policy version</Button></form></details>
                    <details><summary>Report an automation safety condition</summary><form className="stack" onSubmit={(event) => recordStewardshipHealth(event, mandate)}><label>Condition<select name="kind"><option value="repeated_failures">Repeated failures</option><option value="inactivity">Inactivity</option><option value="access_revoked">Revoked access</option><option value="anomalous_consumption">Anomalous consumption</option></select></label><label>Actionable detail<textarea required name="detail" /></label><Button size="sm" variant="secondary" type="submit">Pause affected automation</Button></form></details>
                    <details><summary>Revise this mandate</summary><MandateForm repositories={portfolio?.repositories ?? []} agents={selected.agents} mandate={mandate} onSubmit={(event) => saveMandate(event, mandate.id)} /></details>
                  </div>;
                })}
                {mandates.length === 0 && <p>No standing agent responsibility has been defined.</p>}
                <details><summary>New stewardship mandate</summary><MandateForm repositories={portfolio?.repositories ?? []} agents={selected.agents} onSubmit={saveMandate} /></details>
              </section>
              <section className="panel">
                <p className="eyebrow">Proactive attention</p>
                <h2>Stewardship backlog</h2>
                <p>Ranked recommendations remain pinned to the evidence the steward evaluated. Collaborators can challenge the reasoning, and moved evidence is shown as stale until it is evaluated again.</p>
                {opportunities.map((opportunity) => <details key={opportunity.id} open={opportunity.rank === 1 || opportunity.evidence_stale}>
                  <summary><strong>{opportunity.rank ? `#${opportunity.rank} · ` : ""}{opportunity.title}</strong> <Badge>{opportunity.severity}</Badge> <Badge>{opportunity.state}</Badge>{opportunity.evidence_stale && <Badge>stale evidence</Badge>}</summary>
                  <div className="stack">
                    <p>{opportunity.summary}</p>
                    <dl><div><dt>Expected value</dt><dd>{opportunity.expected_value}</dd></div><div><dt>Confidence</dt><dd>{Math.round(opportunity.confidence * 100)}%</dd></div><div><dt>Why in scope</dt><dd>{opportunity.in_scope_reason}</dd></div><div><dt>Affected owners</dt><dd>{opportunity.affected_owner_ids.join(", ")}</dd></div><div><dt>Exact revisions</dt><dd>{opportunity.affected_revisions.join(", ")}</dd></div></dl>
                    {opportunity.evidence_stale && <div className="notice"><strong>Evidence needs reevaluation:</strong> {opportunity.stale_reason}</div>}
                    <div><h3>Supporting citations</h3>{opportunity.citations.map((citation) => <p key={`${citation.kind}:${citation.resource_id}:${citation.revision}`}><strong>{citation.kind} · {citation.resource_id}</strong> at <code>{citation.revision}</code><br />{citation.summary} · observed {new Date(citation.observed_at).toLocaleString()}</p>)}</div>
                    <div><h3>Discussion and decisions</h3>{opportunity.comments.map((comment) => <p key={comment.id}><strong>{comment.actor_id}</strong>: {comment.body}</p>)}{opportunity.decisions.map((decision, index) => <p key={`${decision.created_at}:${index}`}><strong>{decision.actor_id}</strong> marked {decision.action}{decision.reason ? ` — ${decision.reason}` : ""}</p>)}</div>
                    <div><h3>Work admission · {opportunity.class}</h3>{opportunity.work_decisions.map((decision) => <p key={decision.version}><strong>{decision.actor_id}</strong> · {decision.mode} · {decision.state}{decision.blockers.length ? ` — ${decision.blockers.join(", ")}` : ""}</p>)}{opportunity.state === "open" && <form className="content-grid" onSubmit={(event) => admitOpportunity(event, opportunity)}><label>Decision<select name="mode"><option value="approval">Approve</option><option value="auto_start">Evaluate auto-start</option></select></label><label>Risk<select name="risk"><option>low</option><option>medium</option><option>high</option><option>critical</option></select></label><label>Estimated hours<input required name="hours" min="1" type="number" defaultValue="1" /></label><Button size="sm" type="submit">Evaluate before work</Button></form>}{opportunity.state === "accepted" && <form className="stack" onSubmit={(event) => promoteOpportunity(event, opportunity)}><label>Proposal title<input required name="proposal_title" defaultValue={opportunity.title} /></label><label>Current base revision<select required name="base_revision">{opportunity.affected_revisions.map((revision) => <option key={revision}>{revision}</option>)}</select></label><label>First task<input required name="task_title" defaultValue={opportunity.title} /></label><label>Observable outcome<textarea required name="outcome" defaultValue={opportunity.expected_value} /></label><div className="content-grid"><label>Owner type<select name="owner_kind"><option value="human">Human</option><option value="agent">Agent</option></select></label><label>Owner ID<input required name="owner_id" /></label><label>Risk<select name="risk"><option>low</option><option>medium</option><option>high</option><option>critical</option></select></label></div><label>Completion criteria<textarea required name="criteria" /></label><label>Verification plan<textarea required name="verification" /></label><Button size="sm" type="submit">Create linked proposal and task</Button></form>}{opportunity.work && <p>Promoted to <Link href={`/repositories/${opportunity.repository_id}?view=proposals&proposal=${opportunity.work.proposal_id}`}>proposal {opportunity.work.proposal_id}</Link> at <code>{opportunity.work.base_revision}</code>.</p>}</div>
                    <details><summary>Record implementation, verification, and release outcome</summary><form className="stack" onSubmit={(event) => recordStewardshipOutcome(event, opportunity)}><div className="content-grid"><label>Implementation<select name="implementation"><option value="succeeded">Succeeded</option><option value="failed">Failed</option><option value="in_progress">In progress</option><option value="not_started">Not started</option></select></label><label>Verification<select name="verification"><option value="passed">Passed</option><option value="failed">Failed</option><option value="not_run">Not run</option></select></label><label>Release<select name="release"><option value="released">Released</option><option value="failed">Failed</option><option value="rolled_back">Rolled back</option><option value="not_released">Not released</option></select></label><label>Actual hours<input required min="0" name="actual_hours" type="number" defaultValue="0" /></label><label>Runs<input required min="0" name="runs" type="number" defaultValue="0" /></label><label><input name="false_positive" type="checkbox" /> Recommendation was a false positive</label></div><label>Outcome summary<textarea required name="summary" /></label><div className="content-grid"><label>Evidence kind<input required name="evidence_kind" placeholder="check_run" /></label><label>Evidence resource ID<input required name="evidence_id" /></label><label>Exact revision<input required name="revision" defaultValue={opportunity.affected_revisions[0]} /></label></div><label>Evidence summary<textarea required name="evidence_summary" /></label><label>Mandate-goal effect<select name="goal_state"><option value="advanced">Advanced</option><option value="unchanged">Unchanged</option><option value="regressed">Regressed</option></select></label><label>Goal evidence<textarea required name="goal_evidence" /></label><Button size="sm" type="submit">Add accountable outcome</Button></form></details>
                    <div className="button-row"><Button size="sm" variant="secondary" onClick={() => opportunityDecision(opportunity, "rank", 1)}>Rank first</Button><Button size="sm" variant="secondary" onClick={() => opportunityDecision(opportunity, "snooze")}>Snooze 7 days</Button><Button size="sm" variant="secondary" onClick={() => opportunityDecision(opportunity, "dismiss", "Not a current priority")}>Dismiss</Button><Button size="sm" variant="secondary" onClick={() => opportunityDecision(opportunity, "incorrect", "The recommendation is incorrect")}>Mark incorrect</Button>{opportunity.state !== "open" && <Button size="sm" variant="secondary" onClick={() => opportunityDecision(opportunity, "reopen")}>Reopen</Button>}</div>
                    <form className="stack" onSubmit={(event) => commentOnOpportunity(event, opportunity)}><label>Challenge or discuss<textarea required name="body" /></label><Button type="submit" size="sm">Add to discussion</Button></form>
                  </div>
                </details>)}
                {opportunities.length === 0 && <p>No evidence-backed opportunities have been published yet.</p>}
              </section>
              <section className="panel">
                <p className="eyebrow">Shared outcomes</p>
                <h2>Portfolio initiatives</h2>
                <p>Connect existing decisions and delivery evidence across repositories. Current access and ownership are reconciled into blockers, so accountable work cannot silently become orphaned.</p>
                {initiatives.map((initiative) => <div className="stack" key={initiative.id}>
                  <div><h3>{initiative.title}</h3><p>{initiative.outcome}</p><Badge>{initiative.state}</Badge></div>
                  {initiative.items.map((item) => <div className="repo-row" key={item.id}>
                    <span><strong>{item.position}. {item.title}</strong><br />{item.outcome}<br />{item.assignee_kind} · {item.assignee_id} · repository {item.repository_id}<br />Source {item.source.kind}:{item.source.id}{item.upcoming_release_ids?.length ? ` · releases ${item.upcoming_release_ids.join(", ")}` : ""}{item.policy_exception_ids?.length ? ` · exceptions ${item.policy_exception_ids.join(", ")}` : ""}<br />{item.next_decision && <em>Next decision: {item.next_decision}</em>}</span>
                    <span><Badge>{item.needs_reassignment ? "reassign" : item.state}</Badge>{item.blocked_by?.length > 0 && <Badge>blocked · {item.blocked_by.join(", ")}</Badge>}</span>
                  </div>)}
                  <form className="stack" onSubmit={(event) => createInitiativeItem(event, initiative.id)}>
                    <h3>Connect work</h3>
                    <label>Title<input required name="title" /></label><label>Observable outcome<textarea required name="outcome" /></label>
                    <label>Repository<select required name="repository_id"><option value="">Select…</option>{portfolio?.repositories.map((repo) => <option key={repo.id} value={repo.id}>{repo.name}</option>)}</select></label>
                    <label>Work type<select name="source_kind"><option value="proposal_task">Proposal task</option><option value="evolution_task">Evolution task</option><option value="incident_action">Incident action</option><option value="security_repair">Security repair</option></select></label>
                    <label>Source ID<input required name="source_id" /></label><label>Assignee type<select name="assignee_kind"><option value="team">Team</option><option value="human">Human</option><option value="agent">Approved agent</option></select></label>
                    <label>Assignee ID<input required name="assignee_id" /></label><label>Dependency item IDs (comma separated)<input name="depends_on" /></label><label>Next decision<textarea name="next_decision" /></label>
                    <Button type="submit" variant="secondary">Connect item</Button>
                  </form>
                </div>)}
                <form className="stack" onSubmit={createInitiative}>
                  <h3>New initiative</h3><label>Title<input required name="title" /></label><label>Outcome<textarea required name="outcome" /></label>
                  <label>Starting repository<select required name="repository_id"><option value="">Select…</option>{portfolio?.repositories.map((repo) => <option key={repo.id} value={repo.id}>{repo.name}</option>)}</select></label>
                  <label>Starting work<select name="source_kind"><option value="proposal">Proposal</option><option value="evolution">Evolution plan</option><option value="incident">Incident</option><option value="security">Security work</option></select></label><label>Source ID<input required name="source_id" /></label>
                  <Button type="submit">Create initiative</Button>
                </form>
              </section>
              <div className="content-grid">
                <section className="panel">
                  <h2>Members</h2>
                  {selected.members.map((member) => (
                    <div className="repo-row" key={member.user_id}>
                      <span><strong>{member.user_id}</strong> · {member.role} · {member.accepted_at ? "active" : "invited"}</span>
                      {member.role !== "owner" && <Button size="sm" variant="secondary" onClick={() => removeMember(member.user_id)}>Remove</Button>}
                    </div>
                  ))}
                  <form className="stack" onSubmit={invite}>
                    <label>
                      Invite by handle
                      <input required name="handle" />
                    </label>
                    <Button type="submit">Invite member</Button>
                  </form>
                </section>
                <section className="panel">
                  <h2>Create repository</h2>
                  <form className="stack" onSubmit={createRepository}>
                    <label>
                      Name
                      <input required name="name" />
                    </label>
                    <label>
                      Description
                      <textarea name="description" />
                    </label>
                    <label>
                      Visibility
                      <select name="visibility">
                        <option value="private">Private</option>
                        <option value="public">Public</option>
                      </select>
                    </label>
                    <Button type="submit">Create in organization</Button>
                  </form>
                </section>
              </div>
              <section className="panel">
                <h2>Ownership transfers</h2>
                <form className="stack" onSubmit={requestTransfer}>
                  <label>Repository ID<input required name="repository_id" /></label>
                  <Button type="submit" variant="secondary">Transfer into organization</Button>
                </form>
                {selected.transfers.length === 0 && (
                  <p>No ownership changes are waiting.</p>
                )}
                {selected.transfers.map((transfer) => (
                  <div className="repo-row" key={transfer.id}>
                    <span>
                      <strong>{transfer.repository_id}</strong>
                      <br />
                      {transfer.from_kind} → {transfer.to_kind}
                    </span>
                    <Badge>{transfer.state}</Badge>
                    {transfer.state === "pending" && (
                      <Button
                        size="sm"
                        onClick={() => acceptTransfer(transfer.id)}
                      >
                        Accept control
                      </Button>
                    )}
                  </div>
                ))}
              </section>
              <section className="panel">
                <p className="eyebrow">Least privilege</p>
                <h2>Scoped authority</h2>
                <p>Roles apply only to the listed resources until their expiry. Exceptions are explicit denied actions, and every request, decision, credential, and revocation remains in the organization audit trail.</p>
                <h3>Your effective access</h3>
                {effective.length === 0 && <p>No team or approved-agent role currently gives you derived authority.</p>}
                {effective.map((grant) => <div className="repo-row" key={grant.id}><span><strong>{grant.role}</strong> via {grant.principal_kind} {grant.principal_id}<br />{grant.resources.map((resource) => `${resource.kind}:${resource.id}`).join(" · ")} · expires {new Date(grant.expires_at).toLocaleString()}<br />{grant.exceptions.length ? `Denied: ${grant.exceptions.join(", ")}` : "No exceptions"}</span><Badge>effective</Badge></div>)}
                <h3>Active grants</h3>
                {selected.role_grants?.filter((grant) => !grant.revoked_at).map((grant) => <div className="repo-row" key={grant.id}><span><strong>{grant.role}</strong> → {grant.principal_kind} {grant.principal_id}<br />{grant.reason} · {grant.resources.map((resource) => `${resource.kind}:${resource.id}`).join(" · ")}<br />Expires {new Date(grant.expires_at).toLocaleString()}{grant.exceptions.length ? ` · denied ${grant.exceptions.join(", ")}` : ""}</span>{selected.members.some((member) => member.user_id === userID && member.role === "owner") && <Button size="sm" variant="secondary" onClick={() => revokeRole(grant.id)}>Revoke</Button>}</div>)}
                <h3>Requests</h3>
                {selected.access_requests?.map((request) => <div className="repo-row" key={request.id}><span><strong>{request.role}</strong> for {request.principal_kind} {request.principal_id}<br />{request.reason}</span><Badge>{request.state}</Badge>{request.state === "pending" && selected.members.some((member) => member.user_id === userID && member.role === "owner") && <span><Button size="sm" onClick={() => resolveRequest(request.id, "approved")}>Approve</Button> <Button size="sm" variant="secondary" onClick={() => resolveRequest(request.id, "denied")}>Deny</Button></span>}</div>)}
                <form className="stack" onSubmit={submitAccess}>
                  <h3>{selected.members.some((member) => member.user_id === userID && member.role === "owner") ? "Grant authority" : "Request authority"}</h3>
                  <label>Team or agent<select required name="principal_id"><option value="">Select…</option>{selected.teams?.map((team) => <option key={team.id} value={team.id}>Team · {team.name}</option>)}{selected.agents?.map((agent) => <option key={agent.id} value={agent.id}>Agent · {agent.name}</option>)}</select></label>
                  <label>Role<select name="role"><option value="viewer">Viewer</option><option value="contributor">Contributor</option><option value="maintainer">Maintainer</option><option value="operator">Operator</option></select></label>
                  <label>Repository<select required name="repository_id"><option value="">Select…</option>{portfolio?.repositories.map((repo) => <option key={repo.id} value={repo.id}>{repo.name}</option>)}</select></label>
                  <label>Expires at<input required name="expires_at" type="datetime-local" /></label>
                  <label>Explicit denied actions (comma separated)<input name="exceptions" placeholder="deploy:production" /></label>
                  <label>Reason<textarea required name="reason" /></label>
                  <Button type="submit">{selected.members.some((member) => member.user_id === userID && member.role === "owner") ? "Grant scoped role" : "Request access"}</Button>
                </form>
              </section>
            </>
          ) : (
            <section className="empty-state">
              <h2>Select or create an organization</h2>
              <p>
                Its portfolio will connect repositories, packages, active work,
                releases, and incidents without copying their evidence.
              </p>
            </section>
          )}
        </div>
      </div>
    </AppShell>
  );
}
