"use client";
import { useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";

type Release = { id: string; version: string; commit_id: string };
type Bundle = {
  id: string; release_version: string; revision: string; audience: string; trust_status: string;
  artifacts: { id: string; name: string; sha256: string; size: number }[];
  components: { kind: string; name: string; version?: string; license: string; origin: string; dependencies: string[] }[];
  licenses: string[]; notices: string[];
  omissions: { subject: string; reason: string; impact: string }[];
  verification: { algorithm: string; public_key: string; payload_sha256: string; instructions: string[] };
  trust_notices: { id: string; kind: string; subject: string; detail: string; action: string; campaign_id?: string }[];
};

export function ProvenanceBundles({ repository, owner }: { repository: string; owner: boolean }) {
  const [releases, setReleases] = useState<Release[]>([]);
  const [items, setItems] = useState<Bundle[]>([]);
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    const response = await fetch(`/api/repositories/${repository}/releases?per_page=100`);
    if (!response.ok) return;
    const result = (await response.json()) as { items: Release[] };
    setReleases(result.items);
    const lists = await Promise.all(result.items.map(async (release) => {
      const r = await fetch(`/api/repositories/${repository}/releases/${release.id}/provenance-bundles`);
      return r.ok ? ((await r.json()) as { items: Bundle[] }).items : [];
    }));
    setItems(lists.flat());
  }, [repository]);
  useEffect(() => {
    // The release catalog is durable server state selected by repository.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);
  async function publish(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    let body: unknown;
    try { body = JSON.parse(String(form.get("definition"))); } catch { setError("Invalid bundle JSON."); return; }
    const response = await fetch(`/api/repositories/${repository}/releases/${form.get("release")}/provenance-bundles`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
    if (!response.ok) { setError(((await response.json()) as { error: string }).error.replaceAll("_", " ")); return; }
    setError(""); await load();
  }
  return <section className="panel">
    <p className="eyebrow">Exact artifacts → signed contents and terms → durable trust notices</p>
    <div className="section-heading"><div><h2>Release provenance bundles</h2><p>Consumers can download, verify, and compare signed SBOM evidence without project access. Later trust changes add warnings without changing the original signature.</p></div><Badge>{items.length} bundles</Badge></div>
    {error && <p role="alert" className="form-error">{error}</p>}
    {owner && releases.length > 0 && <details><summary>Publish an authorized release bundle</summary><form className="workspace-form" onSubmit={publish}>
      <label>Release<select name="release">{releases.map(x => <option key={x.id} value={x.id}>{x.version} · {x.commit_id.slice(0, 12)}</option>)}</select></label>
      <label>Audience-ready bundle definition<textarea name="definition" rows={18} required defaultValue={'{\n  "audience": "public",\n  "graph_id": "",\n  "assessment_id": "",\n  "artifacts": [],\n  "components": [],\n  "licenses": [],\n  "notices": [],\n  "source_attestations": [],\n  "build_attestations": [],\n  "omissions": []\n}'} /></label>
      <Button type="submit">Sign and publish immutable bundle</Button>
    </form></details>}
    {items.map(x => <article className="workspace-card" key={x.id}>
      <div className="section-heading"><div><h3>{x.release_version} · <code>{x.revision.slice(0, 12)}</code></h3><p>Bundle <code>{x.id}</code> · {x.audience} · {x.verification.algorithm}</p></div><Badge>{x.trust_status.replaceAll("_", " ")}</Badge></div>
      <p><strong>Signed payload:</strong> <code>{x.verification.payload_sha256}</code><br/><strong>Public key:</strong> <code>{x.verification.public_key.slice(0, 20)}…</code></p>
      <h4>Artifacts and SBOM</h4>
      {x.artifacts.map(a => <p key={a.id}><strong>{a.name}</strong> · {a.size} bytes · <code>sha256:{a.sha256}</code></p>)}
      {x.components.map((c, i) => <p key={`${c.kind}-${c.name}-${i}`}><Badge>{c.kind}</Badge> <strong>{c.name}{c.version && ` ${c.version}`}</strong> · {c.license} · {c.origin}<br/>Dependencies: {c.dependencies.join(", ") || "none declared"}</p>)}
      <p><strong>Licenses:</strong> {x.licenses.join(", ") || "none"} · <strong>notices:</strong> {x.notices.join(" · ") || "none"}</p>
      {x.omissions.map((o, i) => <p className="form-error" key={i}><strong>Declared omission — {o.subject}:</strong> {o.reason} · {o.impact}</p>)}
      {x.trust_notices.map(n => <p className="form-error" key={n.id}><strong>{n.kind.replaceAll("_", " ")} — {n.subject}:</strong> {n.detail} Next: {n.action}{n.campaign_id && ` · campaign ${n.campaign_id}`}</p>)}
      <details><summary>Independent verification instructions</summary><ol>{x.verification.instructions.map(v => <li key={v}>{v}</li>)}</ol><p>Public API: <code>/provenance-bundles/{x.id}</code> · verification: <code>/provenance-bundles/{x.id}/verify</code></p></details>
    </article>)}
  </section>;
}
