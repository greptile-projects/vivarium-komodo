"use client";
/* eslint-disable react-hooks/set-state-in-effect */
import { useCallback, useEffect, useState } from "react";
import { Badge, Button } from "@/components/ui";

type Reachability = {
  id: string;
  copy_kind: string;
  reference: string;
  revision?: string;
  object_ids: string[];
  derived_exposures: Array<{ kind: string; reference: string; state: string }>;
  status: string;
  controlled_by?: string;
  summary: string;
  uncertainty?: string;
  citations: Array<{
    kind: string;
    reference: string;
    digest: string;
    access: string;
  }>;
  recorded_by: string;
};
type Rule = {
  id: string;
  kind: string;
  object_ids: string[];
  replacement_digest?: string;
  preserve_authorship: boolean;
  signature_policy: string;
  rationale: string;
  created_by: string;
};
type Candidate = {
  id: string;
  candidate_digest: string;
  refs: Array<{
    reference: string;
    old_revision: string;
    new_revision: string;
  }>;
  commit_map: Array<{
    old_commit: string;
    new_commit: string;
    authorship_preserved: boolean;
    signature_status: string;
  }>;
  changed_object_ids: string[];
  storage_before_bytes: number;
  storage_after_bytes: number;
  rollback_limits: string[];
  collaborator_actions: string[];
  link_impacts: Array<{
    kind: string;
    reference: string;
    status: string;
    action?: string;
  }>;
  unrewritable_resources: string[];
  published: boolean;
};
type Rehearsal = {
  id: string;
  candidate_id: string;
  environment: string;
  status: string;
  checks: Array<{
    domain: string;
    status: string;
    reference: string;
    summary: string;
  }>;
  blockers: Array<{ kind: string; subject: string; detail: string }>;
};
type Publication = {
  id: string;
  candidate_id: string;
  refs: Array<{
    reference: string;
    old_revision: string;
    new_revision: string;
  }>;
  quarantined_object_ids: string[];
  credential_actions: Array<{
    reference: string;
    action: string;
    receipt: string;
  }>;
  pauses: Array<{
    kind: string;
    reference: string;
    status: string;
    guidance: string;
  }>;
  migration_targets: Array<{
    id: string;
    kind: string;
    reference: string;
    owner_id: string;
    audience: string;
    authority: string;
    instructions: string;
    mapping: string;
    status: string;
    receipt?: string;
  }>;
  published_by: string;
  published_at: string;
};
type Remediation = {
  id: string;
  created_by_id: string;
  definition: {
    title: string;
    source: { kind: string; id: string; revision?: string };
    content_description: string;
    reason: string;
    audience: string;
    response_owner_ids: string[];
    objects: Array<{
      id: string;
      repository_id: string;
      kind: string;
      object_id: string;
      path?: string;
      digest?: string;
      match: string;
      reason?: string;
      attributed_to: string;
    }>;
    scope: Array<{ kind: string; reference: string; revision?: string }>;
    discovery_evidence: Array<{
      id: string;
      kind: string;
      reference: string;
      revision?: string;
      digest: string;
      summary: string;
      status: string;
      reason?: string;
      recorded_by: string;
    }>;
    constraints: Array<{
      id: string;
      kind: string;
      reference: string;
      status: string;
      owner_id: string;
      rationale: string;
    }>;
    required_approvals: Array<{
      kind: string;
      owner_id: string;
      required: boolean;
      status: string;
    }>;
  };
  blockers: Array<{
    kind: string;
    subject: string;
    detail: string;
    attributed_to: string;
  }>;
  reachability_map: Reachability[];
  reachability_summary: {
    by_status: Record<string, number>;
    affected_object_ids: string[];
    derived_exposure_count: number;
  };
  rewrite_rules: Rule[];
  rewrite_candidates: Candidate[];
  rewrite_rehearsals: Rehearsal[];
  publications: Publication[];
  updated_at: string;
  created_at: string;
};
const starter = (repo: string) =>
  JSON.stringify(
    {
      source: {
        kind: "security_finding",
        id: "finding-id",
        revision: "exact-finding-revision",
      },
      content_description:
        "Describe the unsafe content by type and location without reproducing it.",
      reason: "Explain why this content must disappear.",
      audience: "response_team",
      response_owner_ids: ["response-owner-id"],
      participant_ids: [],
      objects: [
        {
          id: "match-1",
          repository_id: repo,
          kind: "blob",
          object_id: "exact-object-id",
          digest: "sha256:digest-of-object",
          match: "confirmed",
          attributed_to: "discoverer-id",
        },
      ],
      scope: [
        { kind: "repository", repository_id: repo, reference: repo },
        {
          kind: "ref",
          repository_id: repo,
          reference: "refs/heads/main",
          revision: "exact-revision",
        },
      ],
      discovery_evidence: [
        {
          id: "evidence-1",
          kind: "object_scan",
          reference: "scan-id",
          revision: "scanner-rules-revision",
          digest: "sha256:evidence-digest",
          summary: "State what the scan established without matched bytes.",
          status: "available",
          recorded_by: "discoverer-id",
        },
      ],
      constraints: [
        {
          id: "retention-1",
          kind: "retention",
          reference: "commitment-id",
          status: "conflict",
          owner_id: "commitment-owner-id",
          rationale: "Explain the conflicting promise.",
        },
      ],
      required_approvals: [
        {
          kind: "repository_owner",
          owner_id: "repository-owner-id",
          required: true,
          status: "pending",
        },
      ],
    },
    null,
    2,
  );
export function HistoryRemediations({
  repository,
  actor,
}: {
  repository: string;
  actor: string;
}) {
  const root = `/api/repositories/${repository}/history-remediations`,
    [items, setItems] = useState<Remediation[]>([]),
    [error, setError] = useState("");
  const load = useCallback(async () => {
    const r = await fetch(root);
    if (r.ok) setItems(((await r.json()) as { items: Remediation[] }).items);
  }, [root]);
  useEffect(() => {
    void load();
  }, [load]);
  async function open(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    try {
      const body = {
        title: f.get("title"),
        ...(JSON.parse(String(f.get("definition"))) as object),
      };
      const r = await fetch(root, {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!r.ok) {
        setError(((await r.json()) as { error: string }).error);
        return;
      }
      setError("");
      e.currentTarget.reset();
      await load();
    } catch {
      setError("invalid_history_remediation_json");
    }
  }
  async function map(e: React.FormEvent<HTMLFormElement>, id: string) {
    e.preventDefault();
    try {
      const f = new FormData(e.currentTarget),
        r = await fetch(`${root}/${id}/reachability`, {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: String(f.get("finding")),
        });
      if (!r.ok) {
        setError(((await r.json()) as { error: string }).error);
        return;
      }
      setError("");
      await load();
    } catch {
      setError("invalid_reachability_finding");
    }
  }
  async function append(
    e: React.FormEvent<HTMLFormElement>,
    id: string,
    path: string,
  ) {
    e.preventDefault();
    try {
      const f = new FormData(e.currentTarget),
        r = await fetch(`${root}/${id}/${path}`, {
          method: "POST",
          headers: { "content-type": "application/json" },
          body: String(f.get("payload")),
        });
      if (!r.ok) {
        setError(((await r.json()) as { error: string }).error);
        return;
      }
      setError("");
      await load();
    } catch {
      setError("invalid_rewrite_evidence");
    }
  }
  return (
    <section className="investigation-workspace">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Contain before inspection</p>
          <h2>Restricted history remediation</h2>
          <p>
            Name exact objects, distribution surfaces, responders, approvals,
            and conflicts without copying unsafe payloads into discussion.
          </p>
        </div>
        <Badge>{items.length} restricted workspaces</Badge>
      </div>
      {error && <p className="form-error">{error.replaceAll("_", " ")}</p>}
      {actor && (
        <details className="workspace-card">
          <summary>Open a governed remediation</summary>
          <form className="workspace-form" onSubmit={(e) => void open(e)}>
            <input
              name="title"
              placeholder="Restricted remediation title"
              required
            />
            <textarea
              name="definition"
              defaultValue={starter(repository)}
              rows={28}
              aria-label="History remediation definition as JSON"
              required
            />
            <Button type="submit">Open restricted remediation</Button>
            <small>
              Do not paste secrets, personal data, or matched source bytes.
              Record exact IDs, digests, access status, and audience-safe
              summaries only.
            </small>
          </form>
        </details>
      )}
      {items.map((x) => (
        <article className="workspace-card" key={x.id}>
          <div className="section-heading">
            <div>
              <h3>{x.definition.title}</h3>
              <p>{x.definition.content_description}</p>
            </div>
            <Badge>{x.definition.audience.replaceAll("_", " ")}</Badge>
          </div>
          <p>
            <strong>Source:</strong>{" "}
            {x.definition.source.kind.replaceAll("_", " ")}{" "}
            {x.definition.source.id}
            {x.definition.source.revision && (
              <> @ {x.definition.source.revision}</>
            )}
            <br />
            <strong>Reason:</strong> {x.definition.reason}
            <br />
            <strong>Response owners:</strong>{" "}
            {x.definition.response_owner_ids.join(", ")}
          </p>
          <h4>Exact objects and reach</h4>
          <ul>
            {x.definition.objects.map((o) => (
              <li key={o.id}>
                <Badge>{o.match.replaceAll("_", " ")}</Badge> {o.repository_id}{" "}
                · {o.kind} <code>{o.object_id}</code>
                {o.path && <> · {o.path}</>} — attributed to {o.attributed_to}
              </li>
            ))}
            {x.definition.scope.map((s, i) => (
              <li key={`${s.kind}-${i}`}>
                <Badge>{s.kind}</Badge> {s.reference}
                {s.revision && (
                  <>
                    {" "}
                    @ <code>{s.revision}</code>
                  </>
                )}
              </li>
            ))}
          </ul>
          <h4>Collaboration graph</h4>
          <p>
            {Object.entries(x.reachability_summary.by_status)
              .map(([s, n]) => `${s.replaceAll("_", " ")}: ${n}`)
              .join(" · ") || "No reachability findings yet."}{" "}
            · {x.reachability_summary.derived_exposure_count} derived
            credentials or data references
          </p>
          <ul>
            {x.reachability_map.map((f) => (
              <li key={f.id}>
                <Badge>{f.status.replaceAll("_", " ")}</Badge>{" "}
                <strong>{f.copy_kind.replaceAll("_", " ")}</strong>{" "}
                {f.reference}
                {f.revision && (
                  <>
                    {" "}
                    @ <code>{f.revision}</code>
                  </>
                )}{" "}
                →{" "}
                {f.object_ids.map((id) => (
                  <code key={id}> {id}</code>
                ))}{" "}
                — {f.summary}
                {f.controlled_by && <> · controlled by {f.controlled_by}</>}
                {f.uncertainty && <> · uncertainty: {f.uncertainty}</>} · cited
                by {f.recorded_by}
              </li>
            ))}
          </ul>
          {actor && (
            <details>
              <summary>Add cited reachability or uncertainty</summary>
              <form
                className="workspace-form"
                onSubmit={(e) => void map(e, x.id)}
              >
                <textarea
                  name="finding"
                  rows={16}
                  required
                  aria-label="Payload-free reachability finding as JSON"
                  defaultValue={JSON.stringify(
                    {
                      copy_kind: "branch",
                      reference: "refs/heads/main",
                      repository_id: repository,
                      revision: "exact-revision",
                      object_ids: x.definition.objects.map((o) => o.object_id),
                      derived_exposures: [],
                      status: "suspected",
                      summary:
                        "Describe reachability without reproducing restricted content.",
                      uncertainty: "State what could not be verified.",
                      citations: [
                        {
                          kind: "ref_scan",
                          reference: "scan-id",
                          digest: "sha256:evidence-digest",
                          access: "available",
                        },
                      ],
                    },
                    null,
                    2,
                  )}
                />
                <Button type="submit">Append graph finding</Button>
                <small>
                  Participants and read-only agents may append cited metadata;
                  never paste matched bytes or credential values.
                </small>
              </form>
            </details>
          )}
          <h4>Rewrite plan and compatibility preview</h4>
          <ul>
            {x.rewrite_rules.map((r) => (
              <li key={r.id}>
                <Badge>{r.kind.replaceAll("_", " ")}</Badge>{" "}
                {r.object_ids.join(", ")} ·{" "}
                {r.signature_policy.replaceAll("_", " ")} ·{" "}
                {r.preserve_authorship
                  ? "authorship preserved"
                  : "authorship changes"}{" "}
                — {r.rationale}
              </li>
            ))}
            {x.rewrite_candidates.map((c) => (
              <li key={c.id}>
                <Badge>{c.published ? "published" : "private candidate"}</Badge>{" "}
                <code>{c.candidate_digest}</code> · {c.refs.length} refs,{" "}
                {c.commit_map.length} mapped commits,{" "}
                {c.changed_object_ids.length} changed objects · storage{" "}
                {c.storage_before_bytes} → {c.storage_after_bytes} bytes
                <br />
                Signatures:{" "}
                {c.commit_map
                  .map(
                    (m) =>
                      `${m.old_commit} → ${m.new_commit}: ${m.signature_status}`,
                  )
                  .join(" · ")}
                <br />
                Broken links:{" "}
                {c.link_impacts
                  .filter((l) => l.status !== "preserved")
                  .map((l) => `${l.kind} ${l.reference}: ${l.status}`)
                  .join(" · ") || "none"}
                <br />
                Unrewritable: {c.unrewritable_resources.join(" · ") || "none"}
                <br />
                Actions: {c.collaborator_actions.join(" · ")} · Rollback limits:{" "}
                {c.rollback_limits.join(" · ")}
              </li>
            ))}
            {x.rewrite_rehearsals.map((r) => (
              <li key={r.id}>
                <Badge>{r.status}</Badge> {r.environment} ·{" "}
                {r.checks.map((c) => `${c.domain}: ${c.status}`).join(" · ")}
                {r.blockers.length > 0 && (
                  <>
                    {" "}
                    · blockers:{" "}
                    {r.blockers
                      .map((b) => `${b.subject}: ${b.detail}`)
                      .join(" · ")}
                  </>
                )}
              </li>
            ))}
          </ul>
          {actor && (
            <details>
              <summary>
                Append an immutable rewrite rule, candidate, or rehearsal
              </summary>
              <p>
                Submit one payload-free JSON document to the matching restricted
                endpoint. Candidates are never published by this workspace.
              </p>
              {[
                [
                  "rewrite-rules",
                  {
                    kind: "replace_object",
                    object_ids: x.definition.objects
                      .filter((o) => o.match === "confirmed")
                      .map((o) => o.object_id),
                    replacement_digest: "sha256:sanitized-object",
                    preserve_authorship: true,
                    preserve_timestamps: true,
                    signature_policy: "preserve_if_unchanged",
                    rationale:
                      "Describe the transformation without replacement bytes.",
                  },
                ],
                [
                  "rewrite-candidates",
                  {
                    rule_ids: x.rewrite_rules.map((r) => r.id),
                    refs: x.definition.scope
                      .filter((s) => s.kind === "ref")
                      .map((s) => ({
                        reference: s.reference,
                        old_revision: s.revision,
                        new_revision: "candidate-revision",
                      })),
                    commit_map: [
                      {
                        old_commit: "old-commit",
                        new_commit: "new-commit",
                        authorship_preserved: true,
                        signature_status: "broken",
                      },
                    ],
                    unaffected_content_digest: "sha256:unaffected-content",
                    candidate_digest: "sha256:candidate",
                    changed_object_ids: x.definition.objects.map(
                      (o) => o.object_id,
                    ),
                    storage_before_bytes: 0,
                    storage_after_bytes: 0,
                    rollback_until: "2099-01-01T00:00:00Z",
                    rollback_limits: [
                      "Independent copies require their owners to migrate.",
                    ],
                    collaborator_actions: [
                      "Fetch replacement refs and rebase unpublished work.",
                    ],
                    link_impacts: [],
                    unrewritable_resources: [],
                  },
                ],
                [
                  "rewrite-rehearsals",
                  {
                    candidate_id:
                      x.rewrite_candidates.at(-1)?.id || "candidate-id",
                    environment: "isolated-networkless-workspace",
                    budget_minutes: 30,
                    budget_cost: 1000,
                    checks: [
                      "integrity",
                      "build",
                      "check",
                      "release",
                      "dependency",
                      "clone",
                      "fetch",
                    ].map((domain) => ({
                      domain,
                      status: "passed",
                      reference: `run:${domain}`,
                      digest: `sha256:${domain}`,
                      summary:
                        "Scenario completed against the private candidate.",
                    })),
                    observed_minutes: 0,
                    observed_cost: 0,
                  },
                ],
              ].map(([path, payload]) => (
                <form
                  className="workspace-form"
                  key={String(path)}
                  onSubmit={(e) => void append(e, x.id, String(path))}
                >
                  <strong>{String(path).replaceAll("-", " ")}</strong>
                  <textarea
                    name="payload"
                    rows={12}
                    defaultValue={JSON.stringify(payload, null, 2)}
                    aria-label={`${String(path)} as JSON`}
                    required
                  />
                  <Button type="submit">
                    Append {String(path).replaceAll("-", " ")}
                  </Button>
                </form>
              ))}
            </details>
          )}
          <h4>Payload-free discovery evidence</h4>
          <h4>Published replacement and migration</h4>
          {x.publications.length === 0 ? (
            <p>No replacement refs have been published.</p>
          ) : (
            x.publications.map((p) => (
              <div key={p.id} className="access-list">
                <p>
                  <Badge>published</Badge>{" "}
                  {p.refs
                    .map(
                      (r) =>
                        `${r.reference}: ${r.old_revision} → ${r.new_revision}`,
                    )
                    .join(" · ")}{" "}
                  · quarantined objects: {p.quarantined_object_ids.join(", ")}
                </p>
                <ul>
                  {p.pauses.map((v) => (
                    <li key={`${v.kind}-${v.reference}`}>
                      <Badge>{v.status}</Badge> {v.kind} {v.reference} —{" "}
                      {v.guidance}
                    </li>
                  ))}
                </ul>
                <ul>
                  {p.migration_targets.map((m) => (
                    <li key={m.id}>
                      <Badge>{m.status}</Badge> {m.kind.replaceAll("_", " ")}{" "}
                      {m.reference} · owner {m.owner_id} · {m.mapping} mapping —{" "}
                      {m.instructions}
                      {m.authority === "independent_owner" &&
                        " The coordinator cannot rewrite this target."}
                      {m.receipt && <> · receipt {m.receipt}</>}
                      {m.owner_id === actor && m.status === "pending" && (
                        <form
                          className="workspace-form"
                          onSubmit={(e) =>
                            void append(e, x.id, `migration-targets/${m.id}`)
                          }
                        >
                          <textarea
                            hidden
                            name="payload"
                            defaultValue={JSON.stringify({
                              status: "acknowledged",
                              receipt:
                                "Owner reviewed the mapping and will perform the independent rewrite.",
                            })}
                          />
                          <Button type="submit">Acknowledge mapping</Button>
                        </form>
                      )}
                    </li>
                  ))}
                </ul>
              </div>
            ))
          )}
          {actor &&
            x.publications.length === 0 &&
            x.rewrite_candidates.some((c) => !c.published) && (
              <details>
                <summary>
                  Publish attested replacement refs and containment
                </summary>
                <form
                  className="workspace-form"
                  onSubmit={(e) => void append(e, x.id, "publications")}
                >
                  <textarea
                    name="payload"
                    rows={24}
                    required
                    aria-label="Publication and migration plan as JSON"
                    defaultValue={JSON.stringify(
                      {
                        candidate_id: x.rewrite_candidates.findLast(
                          (c) => !c.published,
                        )?.id,
                        expected_updated_at: x.updated_at,
                        attestation: {
                          digest: x.rewrite_candidates.findLast(
                            (c) => !c.published,
                          )?.candidate_digest,
                          signer_id: "independent-reviewer-id",
                          signature: "ed25519:signature",
                        },
                        quarantined_object_ids: x.rewrite_candidates.findLast(
                          (c) => !c.published,
                        )?.changed_object_ids,
                        credential_actions: [],
                        pauses: [
                          "push",
                          "queue",
                          "session",
                          "workflow",
                          "release",
                        ].map((kind) => ({
                          kind,
                          reference: `${kind}:affected`,
                          status: "paused",
                          guidance:
                            "Fetch replacement refs and follow the mapping before resuming.",
                        })),
                        migration_targets: [
                          {
                            id: "target-1",
                            kind: "local_branch",
                            reference: "branch-or-copy",
                            owner_id: "target-owner-id",
                            audience: "owner",
                            authority: "independent_owner",
                            instructions:
                              "Fetch the replacement refs, preserve unpublished work, then acknowledge or perform the rewrite.",
                            mapping: "redacted",
                            status: "pending",
                          },
                        ],
                      },
                      null,
                      2,
                    )}
                  />
                  <Button type="submit">Atomically publish replacement</Button>
                  <small>
                    Requires current approvals and a passing candidate
                    rehearsal. Independent targets retain their own authority.
                  </small>
                </form>
              </details>
            )}
          <ul>
            {x.definition.discovery_evidence.map((e) => (
              <li key={e.id}>
                <Badge>{e.status}</Badge> <strong>{e.kind}</strong>{" "}
                {e.reference} · <code>{e.digest}</code> — {e.summary} (
                {e.recorded_by})
              </li>
            ))}
          </ul>
          <h4>Governance blockers</h4>
          {x.blockers.length ? (
            <ul>
              {x.blockers.map((b, i) => (
                <li key={i}>
                  <Badge>{b.kind.replaceAll("_", " ")}</Badge>{" "}
                  <strong>{b.subject}</strong> —{" "}
                  {b.detail || "requires resolution"} · {b.attributed_to}
                </li>
              ))}
            </ul>
          ) : (
            <p>No recorded blockers.</p>
          )}
          <p>
            <strong>Required approvals:</strong>{" "}
            {x.definition.required_approvals
              .map((a) => `${a.kind}: ${a.status} (${a.owner_id})`)
              .join(" · ")}
          </p>
          <small>
            Opened by {x.created_by_id} at{" "}
            {new Date(x.created_at).toLocaleString()}. This workspace grants no
            object inspection, Git rewrite, release, package, artifact,
            environment, disclosure, or operational authority.
          </small>
        </article>
      ))}
    </section>
  );
}
