/* eslint-disable react-hooks/set-state-in-effect */
"use client";
import { useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";
type Contract = {
  id: string;
  current_version: number;
  versions: { number: number; name: string; version: string }[];
};
type Event = {
  sequence: number;
  type: string;
  actor_id: string;
  reason?: string;
};
type Inspection = {
  sequence: number;
  operation_id: string;
  method: string;
  path: string;
  request_headers: Record<string, string>;
  request_body: Record<string, unknown>;
  response_status: number;
  response_headers: Record<string, string>;
  response_body: Record<string, unknown>;
  failure_rule?: string;
};
type App = {
  id: string;
  owner_id: string;
  pending_owner_id?: string;
  registration: {
    name: string;
    consumer_project: string;
    contract_id: string;
    contract_version: number;
    environments: string[];
    capabilities: string[];
  };
  contract_name: string;
  contract_label: string;
  contract_operations: {
    id: string;
    method: string;
    path: string;
    summary: string;
  }[];
  version: number;
  status: string;
  approved_capabilities: string[];
  approved_environments: string[];
  quota: number;
  used: number;
  credential_state: string;
  credential_expires_at?: string;
  failure_rules: {
    id: string;
    operation_id: string;
    status: number;
    error_code: string;
  }[];
  inspections: Inspection[];
  events: Event[];
};
const lines = (v: FormDataEntryValue | null) =>
  String(v || "")
    .split("\n")
    .map((x) => x.trim())
    .filter(Boolean);
export function APIConsumers({
  repository,
  actor,
}: {
  repository: string;
  actor: string;
}) {
  const root = `/api/repositories/${repository}/api-consumers`,
    [apps, setApps] = useState<App[]>([]),
    [contracts, setContracts] = useState<Contract[]>([]),
    [error, setError] = useState(""),
    [secret, setSecret] = useState("");
  const load = useCallback(async () => {
    if (!actor) return;
    const [r, c] = await Promise.all([
      fetch(root),
      fetch(`/api/repositories/${repository}/api-contracts`),
    ]);
    if (r.ok) setApps(((await r.json()) as { items: App[] }).items);
    if (c.ok) setContracts(((await c.json()) as { items: Contract[] }).items);
  }, [root, repository, actor]);
  useEffect(() => {
    void load();
  }, [load]);
  async function send(path: string, body: unknown) {
    const r = await fetch(root + path, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      }),
      x = (await r.json()) as { error?: string; secret?: string };
    if (!r.ok) {
      setError(x.error || "request_failed");
      return;
    }
    setError("");
    if (x.secret) setSecret(x.secret);
    await load();
  }
  async function register(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget),
      c = contracts.find((x) => x.id === f.get("contract"));
    if (!c) return;
    await send("", {
      name: f.get("name"),
      description: f.get("description"),
      consumer_project: f.get("project"),
      contact: f.get("contact"),
      contract_id: c.id,
      contract_version: Number(f.get("version")),
      environments: lines(f.get("environments")),
      capabilities: lines(f.get("capabilities")),
      credential_lifetime_hours: Number(f.get("lifetime")),
    });
  }
  async function decide(e: React.FormEvent<HTMLFormElement>, a: App) {
    e.preventDefault();
    const f = new FormData(e.currentTarget),
      rules = lines(f.get("failures")).map((x) => {
        const [id, operation, status, code] = x.split("|").map((y) => y.trim());
        return {
          id,
          operation_id: operation,
          status: Number(status),
          error_code: code,
          response: { error: code, synthetic: true },
        };
      });
    await send(`/${a.id}/decision`, {
      expected_version: a.version,
      decision: f.get("decision"),
      capabilities: lines(f.get("capabilities")),
      environments: lines(f.get("environments")),
      quota: Number(f.get("quota")),
      credential_lifetime_hours: Number(f.get("lifetime")),
      synthetic_data: Object.fromEntries(
        a.contract_operations.map((o) => [
          o.id,
          { synthetic: true, operation: o.id, items: [] },
        ]),
      ),
      failure_rules: rules,
      reason: f.get("reason"),
    });
  }
  async function sandbox(e: React.FormEvent<HTMLFormElement>, a: App) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    let body = {};
    try {
      body = JSON.parse(String(f.get("body") || "{}"));
    } catch {
      setError("invalid_json_body");
      return;
    }
    const r = await fetch(`/api/api-sandbox/${a.id}/requests`, {
      method: "POST",
      headers: {
        "content-type": "application/json",
        authorization: `Bearer ${String(f.get("secret"))}`,
      },
      body: JSON.stringify({
        operation_id: f.get("operation"),
        failure: f.get("failure"),
        body,
      }),
    });
    if (!r.ok) setError(((await r.json()) as { error: string }).error);
    else {
      setError("");
      await load();
    }
  }
  if (!actor)
    return (
      <p>
        Sign in with repository read access to register a consumer application.
      </p>
    );
  return (
    <section>
      <div className="section-heading">
        <div>
          <h3>Consumer applications and sandbox</h3>
          <p>
            Prove an integration with synthetic data and application-only
            authority. Credentials grant no Git, repository, deployment,
            production-data, or human-account access.
          </p>
        </div>
        <Badge>{apps.length} applications</Badge>
      </div>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      {secret && (
        <aside className="workspace-card">
          <strong>Copy this one-time credential now</strong>
          <pre>{secret}</pre>
          <p>Only its digest is retained.</p>
          <Button type="button" onClick={() => setSecret("")}>
            I saved it
          </Button>
        </aside>
      )}
      <details>
        <summary>Register a consumer project</summary>
        <form className="workspace-form" onSubmit={register}>
          <input name="name" placeholder="Application name" required />
          <input
            name="project"
            placeholder="Independent project reference"
            required
          />
          <input
            name="contact"
            type="email"
            placeholder="Recovery contact"
            required
          />
          <textarea
            name="description"
            placeholder="Integration purpose"
            required
          />
          <select name="contract">
            {contracts.map((c) => (
              <option key={c.id} value={c.id}>
                {c.versions.at(-1)?.name}
              </option>
            ))}
          </select>
          <input name="version" type="number" min="1" defaultValue="1" />
          <textarea
            name="environments"
            placeholder="Requested environments, one per line"
            required
          />
          <textarea
            name="capabilities"
            placeholder="Requested scopes, one per line"
            required
          />
          <input
            name="lifetime"
            type="number"
            min="1"
            max="2160"
            defaultValue="168"
          />
          <Button type="submit">Request access</Button>
        </form>
      </details>
      {apps.map((a) => (
        <article className="workspace-card" key={a.id}>
          <div className="section-heading">
            <div>
              <h4>{a.registration.name}</h4>
              <p>
                {a.registration.consumer_project} → {a.contract_name}{" "}
                {a.contract_label}
              </p>
            </div>
            <Badge>{a.status}</Badge>
          </div>
          <p>
            <strong>Owner:</strong> {a.owner_id} · <strong>Credential:</strong>{" "}
            {a.credential_state}
            {a.credential_expires_at && (
              <> until {new Date(a.credential_expires_at).toLocaleString()}</>
            )}
          </p>
          <p>
            <strong>Requested:</strong> {a.registration.capabilities.join(", ")}{" "}
            in {a.registration.environments.join(", ")}
          </p>
          {a.status === "pending" && (
            <form className="workspace-form" onSubmit={(e) => decide(e, a)}>
              <select name="decision">
                <option value="approved">approved</option>
                <option value="denied">denied</option>
              </select>
              <textarea
                name="capabilities"
                defaultValue={a.registration.capabilities.join("\n")}
                required
              />
              <textarea
                name="environments"
                defaultValue={a.registration.environments.join("\n")}
                required
              />
              <input
                name="quota"
                type="number"
                min="1"
                max="10000"
                defaultValue="100"
              />
              <input name="lifetime" type="number" min="1" defaultValue="24" />
              <textarea
                name="failures"
                placeholder="rate-limit | operation | 429 | rate_limited"
              />
              <input
                name="reason"
                placeholder="Approval or denial reason"
                required
              />
              <Button type="submit">Record decision</Button>
            </form>
          )}
          {a.status === "approved" && a.owner_id === actor && (
            <aside>
              <p>
                The producer approved {a.approved_capabilities.join(", ")} in {a.approved_environments.join(", ")} with quota {a.quota}. Accepting these narrowed terms issues the one-time credential.
              </p>
              <Button type="button" onClick={() => void send(`/${a.id}/consent`, { expected_version: a.version, reason: "application owner accepted approved sandbox terms" })}>
                Accept terms and issue credential
              </Button>
            </aside>
          )}
          {a.status === "active" && (
            <>
              <p>
                <strong>Sandbox quota:</strong> {a.used}/{a.quota} ·{" "}
                <strong>Approved:</strong> {a.approved_capabilities.join(", ")}
              </p>
              <form className="workspace-form" onSubmit={(e) => sandbox(e, a)}>
                <input
                  name="secret"
                  type="password"
                  placeholder="Credential (never retained by this page)"
                  required
                />
                <select name="operation">
                  {a.contract_operations.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.method} {o.path} — {o.summary}
                    </option>
                  ))}
                </select>
                <select name="failure">
                  <option value="">successful synthetic response</option>
                  {a.failure_rules.map((f) => (
                    <option key={f.id} value={f.id}>
                      {f.id} ({f.status})
                    </option>
                  ))}
                </select>
                <textarea name="body" defaultValue="{}" />
                <Button type="submit">Send sandbox request</Button>
              </form>
              <div className="button-row">
                <Button
                  type="button"
                  onClick={() =>
                    void send(`/${a.id}/credentials`, {
                      reason: "consumer-requested rotation",
                    })
                  }
                >
                  Rotate credential
                </Button>
                <Button
                  type="button"
                  onClick={() =>
                    void send(`/${a.id}/controls`, {
                      action: "report_exposure",
                      reason: "consumer reported possible secret exposure",
                    })
                  }
                >
                  Report exposure
                </Button>
              </div>
            </>
          )}
          {(a.status === "denied" ||
            a.status === "revoked" ||
            a.status === "expired") && (
            <Button
              type="button"
              onClick={() =>
                void send(`/${a.id}/controls`, {
                  action: "reapply",
                  reason: "consumer requested reconsideration",
                })
              }
            >
              Reapply
            </Button>
          )}
          {a.inspections.length > 0 && (
            <details>
              <summary>
                Request and response inspection ({a.inspections.length})
              </summary>
              {a.inspections.map((x) => (
                <div key={x.sequence}>
                  <code>
                    {x.method} {x.path} → {x.response_status}
                  </code>
                  <pre>
                    {JSON.stringify(
                      {
                        request: {
                          headers: x.request_headers,
                          body: x.request_body,
                        },
                        response: {
                          headers: x.response_headers,
                          body: x.response_body,
                        },
                      },
                      null,
                      2,
                    )}
                  </pre>
                </div>
              ))}
            </details>
          )}
          <details>
            <summary>Attributable history ({a.events.length})</summary>
            <ol>
              {a.events.map((x) => (
                <li key={x.sequence}>
                  {x.type} by {x.actor_id}
                  {x.reason && <> — {x.reason}</>}
                </li>
              ))}
            </ol>
          </details>
        </article>
      ))}
    </section>
  );
}
