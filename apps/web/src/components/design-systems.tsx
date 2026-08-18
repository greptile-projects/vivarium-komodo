/* eslint-disable react-hooks/set-state-in-effect */
"use client";

import { useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";

type Named = { name: string };
type Version = {
  number: number; name: string; description: string; source_revision: string; definition_path: string; release_revision: string; change_reason: string; author_id: string; published_at: string;
  tokens: (Named & { category: string; value: string; description: string })[];
  components: (Named & { purpose: string; usage: string; do_not_use?: string; props: string[] })[];
  interaction_patterns: (Named & { trigger: string; behavior: string; feedback: string; keyboard: string })[];
  content_rules: (Named & { guidance: string; example: string; avoid?: string })[];
  responsive_rules: (Named & { minimum_width: number; maximum_width?: number; behavior: string })[];
  themes: (Named & { purpose: string; token_overrides: Record<string, string> })[];
  examples: (Named & { subject: string; markup: string; theme: string; locale: string; viewport: string; description: string })[];
  accessibility_constraints: { subject: string; requirement: string; evidence?: string }[];
  localization_constraints: { subject: string; requirement: string; evidence?: string }[];
  owner_ids: string[]; adoption_policy: { required: boolean; consumers: string[]; exceptions: string; review_cadence: string };
  consumers: { name: string; implementation_revision: string; release_revision?: string; adopted_version: number; status: string; notes?: string }[];
  provenance: { kind: string; reference: string; revision?: string; rationale: string }[];
};
type System = { id: string; current_version: number; versions: Version[]; gaps: { kind: string; subject: string; detail: string; version: number }[] };
type Catalog = { items: System[]; conflicts: { kind: string; name: string; systems: string[]; values: string[] }[] };
const starter = JSON.stringify({
  tokens: [{ name: "color.action", category: "color", value: "#0969da", description: "Primary actions" }],
  components: [{ name: "Button", purpose: "Start an action", usage: "Use one primary action per region", do_not_use: "Navigation", props: ["label", "disabled"] }],
  interaction_patterns: [{ name: "Submit", trigger: "Form submit", behavior: "Disable during save", feedback: "Announce success or error", keyboard: "Enter submits" }],
  content_rules: [{ name: "Action labels", guidance: "Lead with a verb", example: "Save changes", avoid: "Submit" }],
  responsive_rules: [{ name: "compact", minimum_width: 0, maximum_width: 639, behavior: "Stack actions" }],
  themes: [{ name: "light", purpose: "Default", token_overrides: {} }],
  examples: [{ name: "Save action", subject: "Button", markup: "<button>Save changes</button>", theme: "light", locale: "en", viewport: "compact", description: "Primary save action" }],
  accessibility_constraints: [{ subject: "Button", requirement: "Visible focus and accessible name", evidence: "button accessibility check" }],
  localization_constraints: [{ subject: "Action labels", requirement: "Allow 200% text expansion" }],
  owner_ids: ["design-team"], adoption_policy: { required: true, consumers: ["web"], exceptions: "Owner-reviewed exception", review_cadence: "Each release" },
  consumers: [{ name: "web", implementation_revision: "commit", release_revision: "release", adopted_version: 1, status: "current" }],
  provenance: [{ kind: "pull_request", reference: "pull-id", revision: "commit", rationale: "Reviewed implementation" }]
}, null, 2);
function decisions(v: Version) { return JSON.stringify({ tokens: v.tokens, components: v.components, interaction_patterns: v.interaction_patterns, content_rules: v.content_rules, responsive_rules: v.responsive_rules, themes: v.themes, examples: v.examples, accessibility_constraints: v.accessibility_constraints, localization_constraints: v.localization_constraints, owner_ids: v.owner_ids, adoption_policy: v.adoption_policy, consumers: v.consumers, provenance: v.provenance }, null, 2); }

export function DesignSystems({ repository, actor }: { repository: string; actor: string }) {
  const root = `/api/repositories/${repository}/design-systems`;
  const [catalog, setCatalog] = useState<Catalog>({ items: [], conflicts: [] });
  const [error, setError] = useState("");
  const load = useCallback(async () => { const r = await fetch(root); if (r.ok) setCatalog(await r.json() as Catalog); }, [root]);
  useEffect(() => { void load(); }, [load]);
  async function publish(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); const form = event.currentTarget; const data = new FormData(form);
    try {
      const decisions = JSON.parse(String(data.get("decisions"))) as object;
      const body = { ...decisions, name: data.get("name"), description: data.get("description"), source_revision: data.get("source_revision"), definition_path: data.get("definition_path"), release_revision: data.get("release_revision"), change_reason: data.get("change_reason") };
      const r = await fetch(root, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
      if (!r.ok) { setError((await r.json() as { error: string }).error); return; }
      setError(""); form.reset(); await load();
    } catch { setError("invalid_design_definition_json"); }
  }
  async function revise(event: React.FormEvent<HTMLFormElement>, system: System) {
    event.preventDefault(); const data = new FormData(event.currentTarget);
    try {
      const body = { ...JSON.parse(String(data.get("decisions"))) as object, expected_version: system.current_version, name: data.get("name"), description: data.get("description"), source_revision: data.get("source_revision"), definition_path: data.get("definition_path"), release_revision: data.get("release_revision"), change_reason: data.get("change_reason") };
      const r = await fetch(`${root}/${system.id}/versions`, { method: "POST", headers: { "content-type": "application/json" }, body: JSON.stringify(body) });
      if (!r.ok) { setError((await r.json() as { error: string }).error); return; } setError(""); await load();
    } catch { setError("invalid_design_definition_json"); }
  }
  return <section className="investigation-workspace">
    <div className="section-heading"><div><p className="eyebrow">Governed product language</p><h2>Design systems</h2><p>Reviewed visual, interaction, content, responsive, accessibility, and localization decisions—without hiding disagreement or adoption gaps.</p></div><Badge>{catalog.items.length} systems</Badge></div>
    {catalog.conflicts.length > 0 && <aside className="workspace-card" aria-label="Conflicting definitions"><h3>Conflicting current definitions</h3><p>These decisions differ across published systems; choose deliberately instead of treating either value as canonical.</p><ul>{catalog.conflicts.map((c) => <li key={`${c.kind}-${c.name}`}><Badge>conflict</Badge> <strong>{c.kind} {c.name}</strong>: {c.values.join(" ↔ ")}</li>)}</ul></aside>}
    {error && <p className="form-error" role="alert">{error}</p>}
    {actor && <details className="workspace-card"><summary>Publish a reviewed design system</summary><form className="workspace-form" onSubmit={publish}><input name="name" placeholder="System name" required /><textarea name="description" placeholder="Purpose and scope" required /><input name="source_revision" placeholder="Exact reviewed source commit" required /><input name="definition_path" placeholder="ui/design-system.json" required /><input name="release_revision" placeholder="Exact release revision" required /><textarea name="decisions" defaultValue={starter} rows={24} aria-label="Structured design decisions as JSON" required /><input name="change_reason" placeholder="Why these decisions changed" required /><Button type="submit">Publish immutable version</Button></form></details>}
    {catalog.items.map((system) => { const v = system.versions[system.versions.length - 1]; return <article className="workspace-card" key={system.id}>
      <div className="section-heading"><div><h3>{v.name}</h3><p>{v.description}</p></div><Badge>version {v.number}</Badge></div>
      <p><strong>Implementation:</strong> <code>{v.source_revision}</code> at <code>{v.definition_path}</code> · <strong>release:</strong> <code>{v.release_revision}</code></p><p><strong>Owners:</strong> {v.owner_ids.length ? v.owner_ids.join(", ") : "Missing"} · <strong>Adoption:</strong> {v.adoption_policy.required ? "required" : "optional"}, reviewed {v.adoption_policy.review_cadence || "without a declared cadence"}</p>
      {system.gaps.length > 0 && <><h4>Explicit gaps</h4><ul>{system.gaps.map((g, i) => <li key={i}><Badge>{g.kind}</Badge> <strong>{g.subject}</strong> — {g.detail}</li>)}</ul></>}
      <h4>Rendered examples</h4><div className="workspace-grid">{v.examples.map((x) => <figure key={x.name}><iframe sandbox="" title={`${x.name}, ${x.theme} theme, ${x.locale} locale, ${x.viewport} viewport`} srcDoc={`<!doctype html><meta charset="utf-8"><style>body{font:16px system-ui;padding:16px;color:#24292f}button{font:inherit;padding:.5rem .75rem}</style>${x.markup}`} /><figcaption><strong>{x.name}</strong> — {x.description} <Badge>{x.theme}</Badge> <Badge>{x.locale}</Badge> <Badge>{x.viewport}</Badge></figcaption></figure>)}</div>
      <h4>Tokens and themes</h4><ul>{v.tokens.map((x) => <li key={x.name}><code>{x.name}: {x.value}</code> — {x.description}</li>)}{v.themes.map((x) => <li key={x.name}><strong>{x.name}</strong> — {x.purpose}</li>)}</ul>
      <h4>Components, interactions, and content</h4><ul>{v.components.map((x) => <li key={x.name}><strong>{x.name}:</strong> {x.usage}{x.do_not_use && <>; avoid for {x.do_not_use}</>}</li>)}{v.interaction_patterns.map((x) => <li key={x.name}><strong>{x.name}:</strong> {x.trigger} → {x.behavior}; {x.feedback}{x.keyboard && <>; keyboard: {x.keyboard}</>}</li>)}{v.content_rules.map((x) => <li key={x.name}><strong>{x.name}:</strong> {x.guidance} (“{x.example}”){x.avoid && <>; avoid “{x.avoid}”</>}</li>)}</ul>
      <h4>Responsive behavior and constraints</h4><ul>{v.responsive_rules.map((x) => <li key={x.name}><strong>{x.name}</strong> ({x.minimum_width}px{x.maximum_width ? `–${x.maximum_width}px` : "+"}): {x.behavior}</li>)}{v.accessibility_constraints.map((x, i) => <li key={`a${i}`}><Badge>accessibility</Badge> {x.subject}: {x.requirement}{x.evidence && <> — evidence: {x.evidence}</>}</li>)}{v.localization_constraints.map((x, i) => <li key={`l${i}`}><Badge>localization</Badge> {x.subject}: {x.requirement}</li>)}</ul>
      <h4>Adoption and provenance</h4><ul>{v.consumers.map((x) => <li key={x.name}><strong>{x.name}</strong> <Badge>{x.status}</Badge> implements design v{x.adopted_version} at <code>{x.implementation_revision}</code>{x.notes && <> — {x.notes}</>}</li>)}{v.provenance.map((x, i) => <li key={i}>{x.kind}: <code>{x.reference}</code>{x.revision && <> at <code>{x.revision}</code></>} — {x.rationale}</li>)}</ul>
      <details><summary>History and rationale ({system.versions.length} versions)</summary><ol>{system.versions.map((x) => <li key={x.number}>v{x.number}, {x.change_reason}, by {x.author_id} at <time dateTime={x.published_at}>{new Date(x.published_at).toLocaleString()}</time>; source <code>{x.source_revision}</code></li>)}</ol></details>
      {actor && <details><summary>Publish a new version</summary><form className="workspace-form" onSubmit={(event) => void revise(event, system)}><input name="name" defaultValue={v.name} required /><textarea name="description" defaultValue={v.description} required /><input name="source_revision" defaultValue={v.source_revision} aria-label="Exact reviewed source commit" required /><input name="definition_path" defaultValue={v.definition_path} required /><input name="release_revision" defaultValue={v.release_revision} required /><textarea name="decisions" defaultValue={decisions(v)} rows={20} required /><input name="change_reason" placeholder="Why this version changed" required /><Button type="submit">Publish version {v.number + 1}</Button></form></details>}
    </article>; })}
  </section>;
}
