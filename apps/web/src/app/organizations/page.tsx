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
};
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
type Envelope<T> = { items: T[] };
type Session = { user: { id: string } };

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

export default function OrganizationsPage() {
  const [organizations, setOrganizations] = useState<Organization[]>([]);
  const [selected, setSelected] = useState<Organization>();
  const [portfolio, setPortfolio] = useState<Portfolio>();
  const [message, setMessage] = useState("");
  const [userID, setUserID] = useState("");
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
        const [organization, view] = await Promise.all([
          api<Organization>(`/organizations/${selectedID}`),
          api<Portfolio>(`/organizations/${selectedID}/portfolio`),
        ]);
        setSelected(organization);
        setPortfolio(view);
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
