"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { AppShell } from "@/components/app-shell";
import { Badge, Button } from "@/components/ui";

type PackageVersion = {
  id: string;
  identity: string;
  version: string;
  repository_id: string;
  release_id: string;
  source_commit_id: string;
  sha256: string;
  artifact_size: number;
  documentation?: string;
  documentation_sha256?: string;
  dependencies: Record<string, string>;
  platform: { os: string; arch: string; runtime?: string };
  build_attestation: { run_id: string; build_name: string; command: string };
  lifecycle: string;
  published_at: string;
};

export default function PackagesPage() {
  const [query, setQuery] = useState("");
  const [submitted, setSubmitted] = useState("");
  const [items, setItems] = useState<PackageVersion[]>([]);
  const [selected, setSelected] = useState<PackageVersion>();
  const [error, setError] = useState("");

  useEffect(() => {
    let live = true;
    fetch(`/api/packages?q=${encodeURIComponent(submitted)}`)
      .then(async response => {
        if (!response.ok) throw new Error("Catalog unavailable.");
        return response.json() as Promise<{ items: PackageVersion[] }>;
      })
      .then(result => { if (live) { setItems(result.items); setSelected(current => current && result.items.some(item => item.id === current.id) ? current : result.items[0]); } })
      .catch(cause => { if (live) setError(cause instanceof Error ? cause.message : "Catalog unavailable."); });
    return () => { live = false; };
  }, [submitted]);

  function search(event: FormEvent) {
    event.preventDefault();
    setError("");
    setSubmitted(query.trim());
  }

  return <AppShell>
    <section className="package-catalog">
      <header className="package-catalog-hero">
        <p className="eyebrow">Shared components</p>
        <h1>Choose dependencies with the evidence attached.</h1>
        <p>Search public packages, compare compatibility, read documentation, and trace every byte to reviewed source and a verified release build.</p>
        <form onSubmit={search}><input aria-label="Search packages" value={query} onChange={event => setQuery(event.target.value)} placeholder="Search package names, versions, or documentation"/><Button type="submit">Search</Button></form>
      </header>
      {error && <p role="alert" className="form-error">{error}</p>}
      <div className="package-catalog-layout">
        <section aria-label="Package results" className="package-results">
          <header><h2>Public packages</h2><span>{items.length} versions</span></header>
          {items.length === 0 ? <p className="empty-state">No matching public package versions.</p> : items.map(item => <button key={item.id} className={selected?.id === item.id ? "selected" : ""} onClick={() => setSelected(item)}>
            <span><strong>{item.identity}</strong><Badge tone={item.lifecycle === "active" ? "accent" : "neutral"}>{item.lifecycle}</Badge></span>
            <code>{item.version}</code><small>{item.platform.os} · {item.platform.arch}{item.platform.runtime ? ` · ${item.platform.runtime}` : ""}</small>
          </button>)}
        </section>
        {selected && <article className="package-inspector">
          <header><div><p className="eyebrow">{selected.identity}</p><h2>{selected.version}</h2></div><Badge tone={selected.lifecycle === "active" ? "accent" : "neutral"}>{selected.lifecycle}</Badge></header>
          {selected.lifecycle !== "active" && <p className="package-warning" role="status">This version has a lifecycle warning. Inspect its current state before adopting it.</p>}
          <section><h3>Documentation</h3><p className="package-docs">{selected.documentation || "The publisher has not attached usage documentation to this version."}</p>{selected.documentation_sha256 && <small>Documentation SHA-256 <code>{selected.documentation_sha256}</code></small>}</section>
          <section><h3>Compatibility</h3><dl><div><dt>Operating system</dt><dd>{selected.platform.os}</dd></div><div><dt>Architecture</dt><dd>{selected.platform.arch}</dd></div><div><dt>Runtime</dt><dd>{selected.platform.runtime || "Any declared runtime"}</dd></div></dl></section>
          <section><h3>Dependencies</h3>{Object.keys(selected.dependencies).length === 0 ? <p>None declared.</p> : <dl>{Object.entries(selected.dependencies).map(([name, constraint]) => <div key={name}><dt>{name}</dt><dd>{constraint}</dd></div>)}</dl>}</section>
          <section><h3>Immutable provenance</h3><dl><div><dt>Source</dt><dd><Link href={`/repositories/${selected.repository_id}?ref=${selected.source_commit_id}`}>{selected.source_commit_id}</Link></dd></div><div><dt>Release</dt><dd><Link href={`/repositories/${selected.repository_id}?view=releases&release=${selected.release_id}`}>{selected.release_id}</Link></dd></div><div><dt>Build attempt</dt><dd>{selected.build_attestation.run_id} · {selected.build_attestation.build_name}</dd></div><div><dt>Artifact</dt><dd>{selected.artifact_size.toLocaleString()} bytes · <code>{selected.sha256}</code></dd></div></dl></section>
          <section><h3>Standard client</h3><pre><code>{`npm config set @${selected.identity.split("/")[0].slice(1)}:registry ${typeof window === "undefined" ? "https://kanso.example" : window.location.origin}/api/package-registry\nnpm install ${selected.identity}@${selected.version}`}</code></pre><p>Private installs use a consuming-repository credential that allowlists exact package versions; it grants no publisher or unrelated package access.</p></section>
        </article>}
      </div>
    </section>
  </AppShell>;
}
