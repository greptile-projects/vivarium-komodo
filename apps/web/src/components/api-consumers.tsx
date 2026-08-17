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
  integration_work?: {
    id: string; kind: string; owner_kind: string; owner_id: string;
    consumer_repository_id: string; consumer_revision: string;
    contract_source_revision: string; sdk_references?: string[];
    example_references?: string[]; sandbox: { synthetic_only: boolean };
  }[];
  verifications?: {
    id: string; pull_request_id: string; candidate_repository_id: string;
    candidate_revision: string; agreement: string; authored_by: string;
    producer_passed: boolean; consumer_passed: boolean;
  }[];
  observations?: {
    id: string; kind: string; audience: string; release_id: string;
    environment: string; operation_id?: string; summary: string;
    contract_version: number;
  }[];
  investigations?: {
    id: string; title: string; status: string; classification: string;
    release_id: string; environment: string;
    invitations?: { agent_id: string; scope: string }[];
    entries?: { id: string; kind: string; body: string; author_id: string; author_kind: string }[];
    reproductions?: { id: string; operation_id: string; inspection: Inspection }[];
    change_routes?: { id: string; defect_owner: string; resource_kind: string; resource_id: string; revision: string }[];
  }[];
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
  async function createWork(e: React.FormEvent<HTMLFormElement>, a: App) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    await send(`/${a.id}/integration-work`, { kind: f.get("kind"), owner_kind: f.get("owner_kind"), owner_id: f.get("owner"), consumer_repository_id: f.get("repository"), consumer_revision: f.get("revision"), resource_id: f.get("resource"), sdk_references: lines(f.get("sdks")), example_references: lines(f.get("examples")), acceptance_criteria: lines(f.get("criteria")) });
  }
  async function verify(e: React.FormEvent<HTMLFormElement>, a: App) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    await send(`/${a.id}/verifications`, { pull_request_id: f.get("pull"), candidate_repository_id: f.get("repository"), candidate_revision: f.get("revision"), results: [{ name: "Producer conformance", kind: "producer_conformance", status: f.get("producer"), coverage: lines(f.get("producer_coverage")), logs: ["sanitized producer scenario result"], cost: 0 }, { name: "Consumer tests", kind: "consumer_test", status: f.get("consumer"), coverage: lines(f.get("consumer_coverage")), logs: ["sanitized consumer test result"], cost: 0 }] });
  }
  async function observe(e: React.FormEvent<HTMLFormElement>, a: App) {
    e.preventDefault(); const f = new FormData(e.currentTarget), end = new Date(), start = new Date(end.getTime() - 60 * 60 * 1000);
    const kind=String(f.get("kind")), value=Number(f.get("value"));
    await send(`/${a.id}/observations`, {kind,audience:f.get("audience"),release_id:f.get("release"),environment:f.get("environment"),operation_id:f.get("operation"),window_start:start.toISOString(),window_end:end.toISOString(),summary:f.get("summary"),inaccessible_evidence:lines(f.get("inaccessible")),...(kind==="availability"?{availability_percent:value}:kind==="latency"?{latency_milliseconds:value}:kind==="quota"?{quota_used:value,quota_limit:a.quota}:kind==="usage"?{usage_count:value}:kind==="schema_conformance"?{schema_conformant:value===1}:{error_code:f.get("error_code")})});
  }
  async function investigate(e: React.FormEvent<HTMLFormElement>, a: App) { e.preventDefault(); const f=new FormData(e.currentTarget); await send(`/${a.id}/investigations`,{title:f.get("title"),observation_ids:lines(f.get("evidence"))}) }
  async function investigationAction(e: React.FormEvent<HTMLFormElement>, a:App, id:string, action:string) { e.preventDefault(); const f=new FormData(e.currentTarget); let body:Record<string,unknown>={}; if(action==="agents")body={agent_id:f.get("agent")}; if(action==="entries")body={kind:f.get("kind"),body:f.get("body"),classification:f.get("classification")}; if(action==="reproductions")body={operation_id:f.get("operation"),failure:f.get("failure"),body:{synthetic:true}}; if(action==="change-work")body={defect_owner:f.get("owner"),resource_kind:f.get("resource_kind"),resource_id:f.get("resource"),repository_id:f.get("repository"),revision:f.get("revision")}; await send(`/${a.id}/investigations/${id}/${action}`,body) }
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
          <details>
            <summary>Create revision-exact integration work</summary>
            <form className="workspace-form" onSubmit={(e) => createWork(e, a)}>
              <select name="kind"><option>task</option><option>session</option><option>workspace</option></select>
              <select name="owner_kind"><option>human</option><option>agent</option></select>
              <input name="owner" placeholder="Human or agent subject" required />
              <input name="repository" placeholder="Consumer repository ID" required />
              <input name="revision" placeholder="Exact consumer commit" required />
              <input name="resource" placeholder="Linked task, session, or workspace ID" />
              <textarea name="sdks" placeholder="Revision-exact SDK references, one per line" />
              <textarea name="examples" placeholder="Revision-exact example references, one per line" />
              <textarea name="criteria" placeholder="Acceptance criteria, one per line" required />
              <Button type="submit">Create integration work</Button>
            </form>
          </details>
          {(a.integration_work?.length || 0) > 0 && <ul>{a.integration_work?.map((w) => <li key={w.id}><Badge>{w.owner_kind}</Badge> {w.kind} for <code>{w.consumer_revision}</code>, contract <code>{w.contract_source_revision}</code> (synthetic sandbox only)</li>)}</ul>}
          <details>
            <summary>Attach pull-request verification</summary>
            <form className="workspace-form" onSubmit={(e) => verify(e, a)}>
              <input name="pull" placeholder="Linked pull request ID" required />
              <input name="repository" placeholder="Candidate repository ID" required />
              <input name="revision" placeholder="Exact candidate commit" required />
              <select name="producer"><option>passed</option><option>failed</option></select>
              <textarea name="producer_coverage" placeholder="Producer scenario coverage" required />
              <select name="consumer"><option>passed</option><option>failed</option></select>
              <textarea name="consumer_coverage" placeholder="Consumer test coverage" required />
              <Button type="submit">Record sanitized verification</Button>
            </form>
          </details>
          {(a.verifications?.length || 0) > 0 && <ul>{a.verifications?.map((v) => <li key={v.id}><Badge>{v.agreement}</Badge> pull {v.pull_request_id} at <code>{v.candidate_revision}</code> — producer {v.producer_passed ? "passed" : "not passed"}, consumer {v.consumer_passed ? "passed" : "not passed"}</li>)}</ul>}
          <details>
            <summary>Share bounded operational evidence</summary>
            <form className="workspace-form" onSubmit={(e)=>observe(e,a)}>
              <select name="kind"><option>availability</option><option>latency</option><option>quota</option><option>error</option><option>schema_conformance</option><option>usage</option></select>
              <select name="audience"><option value="shared">Shared with both owners</option><option value={a.owner_id===actor?"consumer":"producer"}>Keep on my side</option></select>
              <input name="release" placeholder="Exact release ID" required/><select name="environment">{a.approved_environments.map(x=><option key={x}>{x}</option>)}</select>
              <select name="operation">{a.contract_operations.map(x=><option key={x.id}>{x.id}</option>)}</select><input name="value" type="number" step="any" placeholder="Metric value (1 means conformant)"/><input name="error_code" placeholder="Sanitized error code"/>
              <textarea name="summary" placeholder="Sanitized observation; no payloads or credentials" required/><textarea name="inaccessible" placeholder="Private evidence references, one per line"/><Button type="submit">Record evidence</Button>
            </form>
          </details>
          {(a.observations?.length||0)>0&&<><h5>Permitted operational evidence</h5><ul>{a.observations?.map(o=><li key={o.id}><Badge>{o.kind}</Badge> <Badge>{o.audience}</Badge> contract v{o.contract_version} · {o.release_id} · {o.environment}{o.operation_id&&` · ${o.operation_id}`} — {o.summary} <code>{o.id}</code></li>)}</ul><details><summary>Open a shared support investigation</summary><form className="workspace-form" onSubmit={e=>investigate(e,a)}><input name="title" placeholder="Observed failure" required/><textarea name="evidence" placeholder="Visible evidence IDs, one per line" required/><Button type="submit">Open investigation</Button></form></details></>}
          {a.investigations?.map(i=><section className="panel" key={i.id}><p><Badge>{i.status}</Badge> <Badge>{i.classification}</Badge> <strong>{i.title}</strong> · {i.release_id} / {i.environment}</p><details><summary>Invite a read-only diagnostic agent</summary><form className="workspace-form" onSubmit={e=>investigationAction(e,a,i.id,"agents")}><input name="agent" placeholder="Agent subject" required/><Button type="submit">Invite agent</Button></form></details><details><summary>Add a sanitized finding or ownership decision</summary><form className="workspace-form" onSubmit={e=>investigationAction(e,a,i.id,"entries")}><select name="kind"><option>finding</option><option>question</option><option>comment</option><option>decision</option></select><select name="classification"><option value="">No classification</option><option>service</option><option>contract</option><option>client</option><option>environment</option><option>unconfirmed</option></select><textarea name="body" required/><Button type="submit">Add to thread</Button></form></details><details><summary>Reproduce a permitted synthetic request</summary><form className="workspace-form" onSubmit={e=>investigationAction(e,a,i.id,"reproductions")}><select name="operation">{a.contract_operations.map(x=><option key={x.id}>{x.id}</option>)}</select><select name="failure"><option value="">Successful fixture</option>{a.failure_rules.map(x=><option key={x.id}>{x.id}</option>)}</select><Button type="submit">Reproduce without a credential</Button></form></details>{i.status==="confirmed"&&<details><summary>Route confirmed defect to governed work</summary><form className="workspace-form" onSubmit={e=>investigationAction(e,a,i.id,"change-work")}><select name="owner"><option>provider</option><option>consumer</option></select><select name="resource_kind"><option>issue</option><option>proposal</option><option>task</option><option>workspace</option></select><input name="resource" placeholder="Existing governed resource ID" required/><input name="repository" placeholder="Owning repository ID" required/><input name="revision" placeholder="Exact source revision" required/><Button type="submit">Link change work</Button></form></details>}<ul>{i.entries?.map(x=><li key={x.id}><Badge>{x.kind}</Badge> {x.body} — {x.author_kind} {x.author_id}</li>)}{i.reproductions?.map(x=><li key={x.id}><Badge>synthetic reproduction</Badge> {x.operation_id} → {x.inspection.response_status}</li>)}{i.change_routes?.map(x=><li key={x.id}><Badge>{x.defect_owner}</Badge> {x.resource_kind} {x.resource_id} at <code>{x.revision}</code></li>)}</ul></section>)}
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
