"use client";

import Link from "next/link";
import { use, useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Badge, Button } from "@/components/ui";
import { GroundedQuestions } from "@/components/grounded-questions";
import { CollaborativeInvestigations } from "@/components/collaborative-investigations";
import { ImpactAssessments } from "@/components/impact-assessments";
import { TechnicalDecisions } from "@/components/technical-decisions";
import { DeliveryTeams } from "@/components/delivery-teams";
import { Issues } from "@/components/issues";
import { ContributorPathway } from "@/components/contributor-pathway";
import { ContributionOpportunities } from "@/components/contribution-opportunities";
import { DocumentationCollections } from "@/components/documentation-collections";
import { Extensions } from "@/components/extensions";
import { GovernanceCharter } from "@/components/governance-charter";
import { PerformanceGoals } from "@/components/performance-goals";
import { AccessibilityCommitments } from "@/components/accessibility-commitments";
import { AccessibilityBarriers } from "@/components/accessibility-barriers";
import { AccessibilityAssessments } from "@/components/accessibility-assessments";
import { ProjectFunds } from "@/components/project-funds";
import { ProductExperiments } from "@/components/product-experiments";
import { ProductFeedback } from "@/components/product-feedback";
import { DataCommitments } from "@/components/data-commitments";
import { DataFlows } from "@/components/data-flows";
import { LocalePlans } from "@/components/locale-plans";
import { TranslationWork } from "@/components/translation-work";
import { LocalizationVerification } from "@/components/localization-verification";
import { PrivacyAssessments } from "@/components/privacy-assessments";
import {
  Book,
  Branch,
  Check,
  ChevronRight,
  Clock,
  Code,
  Copy,
  Edit,
  File,
  Folder,
  GitPullRequest,
  Lightbulb,
  MessageCircle,
  Plus,
  Sparkles,
  Trash,
  User,
} from "@/components/icons";

type Repository = {
  id: string;
  owner_id: string;
  name: string;
  description: string;
  visibility: "public" | "private";
  empty: boolean;
  git_url: string;
  updated_at: string;
  upstream_repository_id?: string;
  upstream_api_url?: string;
  collaborator_ids?: string[];
};
type BranchRecord = { name: string; commit_id: string; is_default: boolean };
type Branches = { items: BranchRecord[]; default_branch: string };
type Commit = {
  id: string;
  parent_ids: string[];
  author: string;
  email: string;
  authored_at?: string;
  message: string;
};
type TreeEntry = {
  name: string;
  path: string;
  type: "tree" | "blob" | "commit";
  mode: number;
  object_id: string;
  size?: number;
};
type Tree = {
  revision: string;
  commit_id: string;
  path: string;
  tree_id: string;
  entries: TreeEntry[];
};
type Blob = {
  revision: string;
  commit_id: string;
  path: string;
  object_id: string;
  size: number;
  binary: boolean;
  truncated: boolean;
  content: string;
};
type DevelopmentWorkspace = {
  id: string;
  repository_id: string;
  revision: string;
  creator_id: string;
  source_context: { type: string; id?: string; parent_id?: string };
  effective_access: { actor_id: string; permission: string };
  definition: {
    version: number;
    tools: Array<{ name: string; version: string }>;
    dependencies: string[];
    setup: string[];
    ports?: Array<{ number: number; label: string; path: string }>;
    resources: {
      cpu_seconds: number;
      memory_mb: number;
      disk_mb: number;
      setup_timeout_seconds: number;
    };
  };
  definition_digest: string;
  state:
    "setting_up" | "ready" | "failed" | "suspended" | "stopped" | "expired";
  created_at: string;
  updated_at: string;
  effective_policy: {
    cpu_seconds: number;
    memory_mb: number;
    disk_mb: number;
    network: string;
    idle_minutes: number;
    retention_days: number;
    expiry_notice_hours: number;
    sharing: string;
    agent_execution: boolean;
  };
  expires_at?: string;
  expiry_announced_at?: string;
  stopped_at?: string;
  rebuild_required: boolean;
  rebuild_reasons?: string[];
  consumption?: Array<{
    actor_id: string;
    kind: string;
    quantity: number;
    unit: string;
    recorded_at: string;
  }>;
  setup_evidence: Array<{
    sequence: number;
    type: string;
    stream?: string;
    command?: string;
    message?: string;
    exit_code?: number;
  }>;
  activity: Array<{
    sequence: number;
    type: string;
    kind?: string;
    surface?: string;
    path?: string;
    actor_id?: string;
    target_id?: string;
    message?: string;
    exit_code?: number;
  }>;
  changes: Array<{
    sequence: number;
    path: string;
    actor_id: string;
    digest?: string;
    deleted?: boolean;
  }>;
  presence: Array<{
    actor_id: string;
    surface: string;
    path?: string;
    expires_at: string;
  }>;
  controls: Array<{
    id: string;
    subject_id: string;
    subject_kind: string;
    mode: string;
    scopes: string[];
    state: string;
    granted_by: string;
    version: number;
  }>;
  checkpoints: Array<{
    id: string;
    parent_id?: string;
    creator_id: string;
    base_revision: string;
    definition_digest: string;
    summary: string;
    created_at: string;
    reproducibility: {
      dependencies?: string[];
      commands?: string[];
      notes?: string;
    };
    changes: Array<{
      path: string;
      operation: "add" | "modify" | "delete";
      base_digest?: string;
      digest?: string;
      patch?: string;
      binary?: boolean;
      size?: number;
    }>;
    status: {
      reproducible: boolean;
      diverged: boolean;
      conflicts: string[];
      missing_dependencies: string[];
      reasons: string[];
    };
  }>;
};
type WorkspaceFile = {
  path: string;
  directory: boolean;
  size?: number;
  content?: string;
  binary?: boolean;
  digest?: string;
};
type WorkspaceMatch = { path: string; line: number; text: string };
type Commits = {
  items: Commit[];
  revision: string;
  commit_id: string;
  total_count: number;
};
type KnowledgeMap = {
  repository_id: string;
  revision: string;
  commit_id: string;
  analysis: {
    status: "complete" | "incomplete";
    stale: boolean;
    analyzed_at: string;
    files_analyzed: number;
    files_skipped: number;
    reasons: string[];
  };
  symbols: Array<{
    name: string;
    kind: string;
    language: string;
    definition: { path: string; line: number };
    references: Array<{ path: string; line: number }>;
    callers: Array<{ path: string; line: number }>;
    tests: Array<{ path: string; line: number }>;
    owner?: Commit;
  }>;
  dependencies: Array<{
    id: string;
    repository_id: string;
    provider_repository_id: string;
    interface_name: string;
    constraint: string;
    commit_id: string;
    state: string;
    evidence_path?: string;
  }>;
};
type Proposal = {
  id: string;
  repository_id: string;
  author_id: string;
  title: string;
  body: string;
  state: "open" | "closed";
  created_at: string;
  updated_at: string;
  closed_at?: string;
  closed_by_id?: string;
};
type ProposalComment = {
  id: string;
  proposal_id: string;
  author_id: string;
  body: string;
  created_at: string;
};
type ProposalList = { items: Proposal[]; total_count: number };
type CommentList = { items: ProposalComment[]; total_count: number };
type ProposalTask = {
  id: string;
  proposal_id: string;
  title: string;
  outcome: string;
  position: number;
  status:
    | "planned"
    | "in_progress"
    | "completed"
    | "canceled"
    | "draft"
    | "review"
    | "merged"
    | "closed"
    | "superseded";
  depends_on: string[];
  discussion_comment_ids: string[];
  created_by_id: string;
  updated_by_id: string;
  created_at: string;
  updated_at: string;
  ready: boolean;
  completion_criteria?: string[];
  reasoning_context?: { kind: string; opportunity_id?: string };
  assignment?: {
    id: string;
    kind: "human" | "agent";
    assignee_id: string;
    mandate: string;
    repository_id: string;
    base_revision: string;
    permissions: string[];
    credential_issued: boolean;
    assigned_by_id: string;
    assigned_at: string;
    session_id?: string;
    working_branch?: string;
  };
  contributions?: Array<{
    pull_request_id: string;
    session_id?: string;
    source_commit_id: string;
    target_commit_id: string;
    status: "draft" | "review" | "merged" | "closed" | "superseded";
    published_by_id: string;
    published_at: string;
  }>;
};
type ProposalPlanEvent = {
  id: string;
  actor_id: string;
  action: string;
  task: ProposalTask;
  created_at: string;
};
type ProposalPlan = {
  proposal_id: string;
  tasks: ProposalTask[];
  history: ProposalPlanEvent[];
};
type PullRequest = {
  id: string;
  repository_id: string;
  source_repository_id: string;
  proposal_id?: string;
  author_id: string;
  title: string;
  body: string;
  source_branch: string;
  target_branch: string;
  source_commit_id: string;
  target_commit_id: string;
  draft: boolean;
  status: "open" | "merged";
  created_at: string;
  updated_at: string;
  merged_at?: string;
  merged_by_id?: string;
  merge_commit_id?: string;
  reasoning_context?: { kind: string; opportunity_id?: string };
  delivery_evidence?: {
    reasoning: string;
    commands: string[];
    residual_risks: string[];
    completion_criteria: Array<{ criterion: string; status: string; evidence?: string }>;
    recorded_by_id: string;
    recorded_at: string;
  };
};
type PullRequestCommit = {
  id: string;
  parent_ids: string[];
  message: string;
  author: string;
  committer: string;
};
type PullRequestFile = {
  path: string;
  status: "added" | "modified" | "deleted";
  old_object_id?: string;
  new_object_id?: string;
  old_mode?: string;
  new_mode?: string;
  additions: number;
  deletions: number;
  binary: boolean;
  patch?: string;
};
type PullRequestComment = {
  id: string;
  pull_request_id: string;
  author_id: string;
  body: string;
  created_at: string;
};
type PullRequestList = { items: PullRequest[]; total_count: number };
type PullRequestCommitList = {
  items: PullRequestCommit[];
  total_count: number;
};
type PullRequestFileList = { items: PullRequestFile[]; total_count: number };
type PullRequestCommentList = {
  items: PullRequestComment[];
  total_count: number;
};
type PullRequestReview = {
  pull_request_id: string;
  reviewer_id: string;
  decision: "approve" | "request_changes";
  commit_id: string;
  submitted_at: string;
  updated_at: string;
  stale: boolean;
};
type PullRequestReviewList = {
  items: PullRequestReview[];
  total_count: number;
};
type FederatedPullEvent = { id:string; pull_reference:string; source_instance:string; actor_subject:string; kind:"discussion"|"review"|"revision"|"check"|"preview"|"requested_changes"|"closure"; revision:string; body?:string; state?:string; audience:"public"|"participants"; evidence?:Record<string,string>; occurred_at:string; verification:string; current:boolean };
type FederatedPullEventList = { items:FederatedPullEvent[]; total_count:number; authoritative_local_revision:string };
type ChangeSessionEvent = {
  id: string;
  type: string;
  actor_id: string;
  run_id?: string;
  initiator_id?: string;
  agent?: string;
  revision_id?: string;
  metadata?: Record<string, string>;
  created_at: string;
};
type RunPublication = {
  summary: string;
  commit_ids: string[];
  changed_files: string[];
  checks: string[];
  concerns: string[];
  source_commit_id: string;
  published_at: string;
};
type AgentRun = {
  id: string;
  initiator_id: string;
  agent: string;
  instructions: string;
  revision_id: string;
  context_paths: string[];
  working_branch: string;
  credential_grant_id: string;
  credential_expires_at: string;
  credential_revoked_at?: string;
  state: "queued" | "running" | "paused" | "succeeded" | "failed" | "canceled";
  publication?: RunPublication;
  created_at: string;
};
type ChangeSession = {
  id: string;
  repository_id: string;
  pull_request_id: string;
  initiator_id: string;
  source_commit_id: string;
  check_failure?: {
    run_id: string;
    commit_id: string;
    name: string;
    command: string;
    working_directory?: string;
    timeout_seconds: number;
    environment?: Record<string, string>;
    declared_artifacts?: string[];
    logs: { sequence: number; stream: "stdout" | "stderr"; message: string }[];
    artifacts: CheckArtifact[];
    exit_code: number;
    timed_out: boolean;
    error?: string;
  };
  state: "awaiting_instructions" | "delegated";
  created_at: string;
  updated_at: string;
  events?: ChangeSessionEvent[];
  runs?: AgentRun[];
};
type ChangeSessionList = { items: ChangeSession[]; total_count: number };
type CheckArtifact = {
  id: string;
  path: string;
  size: number;
  sha256: string;
  media_type: string;
};
type CheckEvent = {
  sequence: number;
  type: "status" | "log" | "command" | "artifact";
  timestamp: string;
  status?: CheckRun["state"];
  stream?: "stdout" | "stderr";
  message?: string;
  actor_id?: string;
  outcome?: { exit_code: number; timed_out: boolean };
  artifact?: CheckArtifact;
};
type CheckRun = {
  id: string;
  commit_id: string;
  definition: { name: string; command: string };
  state: "queued" | "running" | "succeeded" | "failed" | "canceled";
  triggered_by_id?: string;
  retry_of_id?: string;
  canceled_by_id?: string;
  created_at: string;
  started_at?: string;
  completed_at?: string;
  events: CheckEvent[];
};
type CheckRunList = { items: CheckRun[]; total_count: number };
type Preview = {
  id: string;
  revision: string;
  creator_id: string;
  state: "setting_up" | "ready" | "failed" | "stopped" | "expired";
  url?: string;
  stale: boolean;
  failure?: string;
  created_at: string;
  expires_at: string;
  build_attestation: { definition_digest: string; configuration_digest: string };
  definition: { version: number; configuration?: string[]; audience: { network: string; data: string; identity: string; actions: string[] }; resources: { cpu_seconds: number; memory_mb: number; disk_mb: number; lifetime_minutes: number } };
  invitations: Array<{ id: string; user_id: string; role: "view" | "test" | "feedback"; source_kind: string; source_id?: string; invited_by_id: string; expires_at: string; revoked_at?: string }>;
  access_events: Array<{ sequence: number; type: string; actor_id: string; invitation_id: string; created_at: string }>;
  events: Array<{ sequence: number; type: string; state?: string; stream?: string; message?: string; command?: string; exit_code?: number }>;
  findings: PreviewFinding[];
};
type PreviewFinding = { id:string; author_id:string; route:string; revision:string; title:string; description:string; reproduction_steps:string[]; classification:string; status:string; duplicate_of?:string; related_finding_ids?:string[]; evidence:Array<{id:string;kind:string;name:string;media_type:string;size:number;sha256:string;redacted:boolean;audience:string}>; comments:Array<{id:string;author_id:string;body:string;created_at:string}>; history:Array<{sequence:number;type:string;actor_id:string;value?:string;created_at:string}>; work?:{kind:string;proposal_id:string;task_id:string;change_session_id?:string;workspace_id?:string;owner_kind:string;owner_id:string;acceptance_criteria:string[];evidence_ids:string[]}; repairs?:Array<{revision:string;commit_ids:string[];commands?:string[];checks?:string[];author_ids:string[];preview_id:string;published_by_id:string;published_at:string}>; created_at:string; updated_at:string };
type PreviewList = { items: Preview[] };
type ReadinessBranch = {
  name: string;
  exists: boolean;
  commit_id?: string;
  snapshot_commit_id: string;
  matches_pull_request: boolean;
};
type PullRequestReadiness = {
  ready: boolean;
  can_merge: boolean;
  has_conflicts: boolean | null;
  source_branch: ReadinessBranch;
  target_branch: ReadinessBranch;
  reviews: {
    required_owner_approvals: number;
    current_owner_approvals: number;
    current_change_requests: number;
    stale_reviews: number;
  };
  checks: {
    target_branch: string;
    commit_id: string;
    satisfied: boolean;
    requirements: {
      name: string;
      status:
        "missing" | "pending" | "failed" | "canceled" | "stale" | "succeeded";
      run_id?: string;
      commit_id?: string;
    }[];
  };
  blockers: { code: string; message: string }[];
};
type IntegrationQueuePolicy = {
  branch: string;
  enabled: boolean;
  concurrency: number;
  failure_behavior: "pause" | "remove";
  required_checks: string[] | null;
  required_owner_approvals: number;
};
type IntegrationQueueEntry = {
  id: string;
  pull_request_id: string;
  source_commit_id: string;
  target_commit_id: string;
  candidate_commit_id: string;
  candidate_tree_id: string;
  required_checks: string[];
  position: number;
  state: "verifying" | "blocked" | "passed" | "paused" | "removed" | "merged";
  reason?: string;
  completed_at?: string;
  next_action: string;
  blocker?: string;
  enqueued_by_id: string;
  events: {
    action: string;
    actor_id?: string;
    from?: number;
    to?: number;
    created_at: string;
  }[];
  attempt_history: {
    generation: number;
    target_commit_id: string;
    candidate_commit_id: string;
    created_at: string;
    checks: PullRequestReadiness["checks"];
  }[];
  created_at: string;
  checks: PullRequestReadiness["checks"];
};
type IntegrationQueueEntries = {
  items: IntegrationQueueEntry[];
  total_count: number;
  policy: IntegrationQueuePolicy;
};
type ReleaseCandidate = {
  id: string;
  repository_id: string;
  version: string;
  notes: string;
  commit_id: string;
  prior_release_id?: string;
  prior_commit_id?: string;
  status: "candidate";
  created_by_id: string;
  created_at: string;
  pull_requests: Array<{
    id: string;
    title: string;
    author_id: string;
    merge_commit_id: string;
  }>;
  proposal_ids: string[];
  task_ids: string[];
  contributor_ids: string[];
};
type ReleaseList = { items: ReleaseCandidate[]; total_count: number };
type PackageVersion = {
  id: string;
  identity: string;
  name: string;
  version: string;
  repository_id: string;
  release_id: string;
  source_commit_id: string;
  artifact_id: string;
  artifact_path: string;
  artifact_media_type: string;
  artifact_size: number;
  sha256: string;
  build_attestation: {
    run_id: string;
    build_name: string;
    command: string;
    completed_at: string;
  };
  platform: { os: string; arch: string; runtime?: string };
  dependencies: Record<string, string>;
  publisher_id: string;
  documentation?: string;
  documentation_sha256?: string;
  visibility: "public" | "private";
  lifecycle: "active";
  published_at: string;
};
type PackageList = { items: PackageVersion[]; total_count: number };
type RelationshipGraph = {
  repository_id: string;
  can_write: boolean;
  nodes: Array<{
    repository_id: string;
    name: string;
    owner_id: string;
    visibility: string;
  }>;
  interfaces: Array<{
    id: string;
    repository_id: string;
    name: string;
    version: string;
    commit_id: string;
    release_id: string;
    schema_path?: string;
    published_by_id: string;
    published_at: string;
  }>;
  edges: Array<{
    dependency: {
      id: string;
      repository_id: string;
      commit_id: string;
      release_id?: string;
      provider_repository_id: string;
      interface_name: string;
      constraint: string;
      declared_by_id: string;
      declared_at: string;
    };
    provider?: RelationshipGraph["interfaces"][number];
    status: "resolved" | "stale" | "unresolved";
    reasons: string[];
    consumer_release?: ReleaseCandidate;
    provider_release?: ReleaseCandidate;
    consumer_environments: Array<{
      id: string;
      name: string;
      deployment_id: string;
      state: string;
      release_id: string;
      commit_id: string;
    }>;
    provider_environments: Array<{
      id: string;
      name: string;
      deployment_id: string;
      state: string;
      release_id: string;
      commit_id: string;
    }>;
  }>;
  summary: {
    repositories: number;
    relationships: number;
    resolved: number;
    stale: number;
    unresolved: number;
  };
};
type EvolutionPlan = {
  id: string;
  repository_id: string;
  interface_name: string;
  source_kind: "proposal" | "pull_request";
  source_id: string;
  candidate_commit_id: string;
  candidate_schema_path: string;
  candidate_schema_sha256: string;
  predecessor: RelationshipGraph["interfaces"][number];
  predecessor_schema_sha256: string;
  affected_consumers: Array<{
    dependency_id: string;
    repository_id: string;
    owner_id: string;
    commit_id: string;
    constraint: string;
  }>;
  strategy: string;
  changes: Array<{
    id: string;
    classification: "breaking" | "compatible" | "behavioral" | "unknown";
    area: string;
    summary: string;
    rationale: string;
    actor_id: string;
  }>;
  steps: Array<{
    id: string;
    position: number;
    owner_id: string;
    summary: string;
    depends_on?: string;
  }>;
  exceptions: Array<{
    id: string;
    consumer_repository_id?: string;
    reason: string;
    expires_at?: string;
    actor_id: string;
  }>;
  tasks: Array<{
    id: string;
    target_repository_id: string;
    status: string;
    work?: { pull_request_id?: string; head_revision: string };
  }>;
  verifications: Array<{
    id: string;
    revisions: Array<{
      repository_id: string;
      commit_id: string;
      task_id: string;
      pull_request_id: string;
      dependency_id?: string;
    }>;
    run_ids: string[];
    triggered_by_id: string;
    created_at: string;
  }>;
  rollout?: {
    verification_id: string;
    state: "awaiting_approval" | "active" | "paused" | "blocked" | "completed";
    updated_at: string;
    phases: Array<{
      id: string;
      position: number;
      name: string;
      compatibility_gates: string[];
      state:
        "pending" | "ready" | "running" | "paused" | "blocked" | "completed";
      approvals: Array<{
        repository_id: string;
        actor_id: string;
        decision: "approve" | "reject";
        note?: string;
      }>;
      steps: Array<{
        id: string;
        repository_id: string;
        kind: "queue" | "release" | "deployment" | "rollback" | "repair";
        resource_id?: string;
        state: string;
      }>;
      outcomes: Array<{
        step_id: string;
        repository_id: string;
        kind: string;
        resource_id: string;
        state: string;
        actor_id: string;
        note?: string;
        created_at: string;
      }>;
    }>;
  };
  acknowledgements: Array<{
    actor_id: string;
    decision: "acknowledge" | "request_changes";
    note: string;
    owner_for_repository_ids: string[];
    created_at: string;
  }>;
  findings: Array<{
    id: string;
    kind: string;
    body: string;
    uncertainty?: string;
    repository_ids: string[];
    actor_id: string;
    created_at: string;
  }>;
  analyses: Array<{
    id: string;
    agent: string;
    mandate: string;
    repository_ids: string[];
    state: string;
    initiated_by_id: string;
    credential_expires_at: string;
  }>;
  created_by_id: string;
  updated_at: string;
};
type EvolutionList = { items: EvolutionPlan[]; total_count: number };
type EvolutionVerificationDetail = {
  verification: EvolutionPlan["verifications"][number];
  current: boolean;
  attested: boolean;
  runs: Array<{
    id: string;
    state: string;
    definition: { name: string; kind?: string };
    events: Array<{
      sequence: number;
      type: string;
      stream?: string;
      message?: string;
      artifact?: { id: string; path: string; sha256: string };
    }>;
  }>;
};
type ReleaseBuild = {
  id: string;
  commit_id: string;
  state: "queued" | "running" | "succeeded" | "failed" | "canceled";
  triggered_by_id?: string;
  retry_of_id?: string;
  created_at: string;
  completed_at?: string;
  definition: {
    name: string;
    command: string;
    working_directory?: string;
    dependencies?: string[];
  };
  events: Array<{
    sequence: number;
    type: string;
    stream?: string;
    message?: string;
    artifact?: {
      id: string;
      path: string;
      size: number;
      sha256: string;
      media_type: string;
    };
  }>;
};
type ReleaseAttestation = {
  release_id: string;
  repository_id: string;
  source_commit_id: string;
  created_by_id: string;
  verified: boolean;
  attempts: ReleaseBuild[];
};
type DeliveryEnvironment = {
  id: string;
  name: string;
  position: number;
  command: string;
  configuration: Record<string, string>;
  secret_names: string[];
  required_approvals: number;
  concurrency: number;
  updated_by_id: string;
};
type Deployment = {
  id: string;
  environment_id: string;
  release_id: string;
  build_run_id: string;
  artifact_id: string;
  artifact_path: string;
  artifact_sha256: string;
  source_commit_id: string;
  state:
    | "pending"
    | "queued"
    | "running"
    | "paused"
    | "succeeded"
    | "failed"
    | "canceled";
  initiated_by_id: string;
  approvals: Array<{ actor_id: string; created_at: string }>;
  events: Array<{
    sequence: number;
    type: string;
    state?: string;
    actor_id?: string;
    stream?: string;
    message?: string;
    stage?: string;
    signal?: string;
    outcome?: string;
    created_at: string;
  }>;
  current_stage?: string;
  decision_by_id?: string;
  decision_reason?: string;
  recovery_of_id?: string;
  recovery_action?: string;
  created_at: string;
};
type EnvironmentList = { items: DeliveryEnvironment[]; total_count: number };
type DeploymentList = { items: Deployment[]; total_count: number };
type IncidentEvent = {
  sequence: number;
  type: string;
  actor_id: string;
  status?: string;
  severity?: string;
  audience?: "participants" | "public";
  message?: string;
  roles?: Record<string, string>;
  update_sequence?: number;
  created_at: string;
};
type IncidentEvidence = {
  id: string;
  kind:
    | "logs"
    | "health_signal"
    | "deployment"
    | "release"
    | "commit"
    | "pull_request"
    | "incident";
  repository_id: string;
  resource_id: string;
  event_sequence?: number;
  start_at?: string;
  end_at?: string;
  title: string;
  audience: "participants" | "public";
  attached_by_id: string;
  captured_at: string;
};
type IncidentFinding = {
  id: string;
  kind: "observation" | "hypothesis" | "query" | "conclusion";
  body: string;
  query?: string;
  evidence_ids: string[];
  audience: "participants" | "public";
  author_id: string;
  created_at: string;
};
type IncidentMitigation = {
  id: string;
  kind: "pause_rollout" | "restore_release" | "emergency_repair";
  title: string;
  description: string;
  environment_id: string;
  deployment_id?: string;
  evidence_ids: string[];
  proposed_by_id: string;
  state: string;
  comments: Array<{
    id: string;
    actor_id: string;
    body: string;
    created_at: string;
  }>;
  decisions: Array<{
    decision: string;
    actor_id: string;
    reason: string;
    override: boolean;
    created_at: string;
  }>;
  attempts: Array<{
    sequence: number;
    actor_id: string;
    outcome: string;
    resource_type?: string;
    resource_id?: string;
    message: string;
    created_at: string;
  }>;
  recovery_criteria: Array<{
    name: string;
    deployment_id: string;
    event_sequence: number;
    outcome: string;
    verified_by_id?: string;
    verified_at?: string;
  }>;
  created_at: string;
  updated_at: string;
};
type IncidentResolution = {
  impact_summary: string;
  timeline_summary: string;
  contributing_factors: string[];
  conclusion_ids: string[];
  corrective_work: Array<{
    proposal_id: string;
    task_id: string;
    owner_id: string;
    due_at?: string;
    state: string;
    invalidated: boolean;
    pull_request_ids?: string[];
    release_ids?: string[];
    deployment_ids?: string[];
    overdue: boolean;
  }>;
  resolved_by_id: string;
  resolved_at: string;
};
type Incident = {
  id: string;
  repository_id: string;
  title: string;
  summary: string;
  severity: "critical" | "high" | "medium" | "low";
  status:
    "declared" | "investigating" | "mitigating" | "monitoring" | "resolved";
  declared_by_id: string;
  roles: Record<string, string>;
  affected: Array<{ repository_id: string; environment_id: string }>;
  source_signal?: {
    deployment_id: string;
    event_sequence: number;
    stage?: string;
    signal?: string;
    outcome?: string;
  };
  followers: string[];
  acknowledgements: Array<{
    actor_id: string;
    update_sequence: number;
    created_at: string;
  }>;
  evidence: IncidentEvidence[];
  findings: IncidentFinding[];
  mitigations: IncidentMitigation[];
  resolution?: IncidentResolution;
  timeline: IncidentEvent[];
  created_at: string;
  updated_at: string;
};
type IncidentList = { items: Incident[]; total_count: number };
type UserRecord = { id: string; handle: string; display_name: string };
type Collaborator = {
  user_id: string;
  handle: string;
  display_name: string;
  role: "contributor";
};
type CollaboratorList = { items: Collaborator[]; total_count: number };

async function get<T>(path: string): Promise<T> {
  const response = await fetch(`/api${path}`);
  if (!response.ok)
    throw new Error(
      response.status === 401
        ? "Sign in to view this private repository."
        : "This repository state could not be found.",
    );
  return response.json();
}
async function send<T>(
  path: string,
  method: string,
  body?: unknown,
): Promise<T> {
  const response = await fetch(`/api${path}`, {
    method,
    headers:
      body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) {
    const result = (await response.json().catch(() => ({}))) as {
      error?: string;
    };
    throw new Error(
      response.status === 401
        ? "Sign in with repository access to continue."
        : result.error === "invalid_proposal"
          ? "Choose an available proposal or add a title of 200 characters or fewer."
          : result.error === "invalid_branches"
            ? "Choose two different, available branches."
            : result.error === "invalid_pull_request"
              ? "Add a title and valid branch details."
              : result.error === "invalid_comment"
                ? "Write a comment before posting."
                : result.error === "pull_request_not_ready"
                  ? "Resolve the listed blockers before merging."
                  : result.error === "source_branch_unavailable"
                    ? "The candidate branch is no longer available."
                    : result.error === "name_taken"
                      ? "You already have a repository with that name."
                      : result.error === "fork_branch_diverged"
                        ? "This branch contains independent work and cannot be fast-forwarded. Merge upstream in a local clone instead."
                        : result.error === "upstream_branch_not_found"
                          ? "The upstream repository does not publish this branch."
                          : "This action is not available to your account.",
    );
  }
  return response.status === 204 ? (undefined as T) : response.json();
}
const short = (id: string) => id.slice(0, 7);
const summary = (message: string) =>
  message.split("\n")[0] || "Untitled commit";
const formatSize = (size: number) =>
  size < 1024 ? `${size} B` : `${(size / 1024).toFixed(1)} KB`;
const userRequests = new Map<string, Promise<UserRecord>>();
function Actor({ id, compact = false }: { id: string; compact?: boolean }) {
  const [user, setUser] = useState<UserRecord>();
  useEffect(() => {
    let active = true;
    let request = userRequests.get(id);
    if (!request) {
      request = get<UserRecord>(`/users/${id}`);
      userRequests.set(id, request);
    }
    request
      .then((value) => {
        if (active) setUser(value);
      })
      .catch(() => undefined);
    return () => {
      active = false;
    };
  }, [id]);
  return user ? (
    <span title={id}>
      {compact ? (
        `@${user.handle}`
      ) : (
        <>
          {user.display_name} <small>@{user.handle}</small>
        </>
      )}
    </span>
  ) : (
    <code>{short(id)}</code>
  );
}

export default function RepositoryPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{
    ref?: string;
    path?: string;
    view?: string;
    proposal?: string;
    state?: string;
    q?: string;
    pull?: string;
    section?: string;
    release?: string;
    incident?: string;
    workspace?: string;
    symbol?: string;
    investigation?: string;
    conversation?: string;
    assessment?: string;
    decision?: string;
    team?: string;
    goal?: string;
    issue?: string;
    feedback?: string;
  }>;
}) {
  const { id } = use(params);
  const query = use(searchParams);
  const router = useRouter();
  const revision = query.ref ?? "";
  const path = query.path ?? "";
  const view =
    query.view === "commits"
      ? "commits"
      : query.view === "proposals"
        ? "proposals"
        : query.view === "pulls"
          ? "pulls"
          : query.view === "queue"
            ? "queue"
            : query.view === "relationships"
              ? "relationships"
              : query.view === "releases"
                ? "releases"
                : query.view === "incidents"
                  ? "incidents"
                  : query.view === "workspaces"
                    ? "workspaces"
                    : query.view === "intelligence"
                      ? "intelligence"
                      : query.view === "investigations"
                        ? "investigations"
                        : query.view === "impact"
                          ? "impact"
                          : query.view === "decisions"
                            ? "decisions"
                          : query.view === "teams"
                            ? "teams"
                            : query.view === "funds"
                              ? "funds"
                            : query.view === "performance"
                              ? "performance"
                            : query.view === "accessibility"
                              ? "accessibility"
                            : query.view === "privacy"
                              ? "privacy"
                            : query.view === "locales"
                              ? "locales"
                            : query.view === "experiments"
                              ? "experiments"
                            : query.view === "feedback"
                              ? "feedback"
                            : query.view === "contribute"
                              ? "contribute"
                              : query.view === "documentation"
                                ? "documentation"
                              : query.view === "governance"
                                ? "governance"
                              : query.view === "extensions"
                                ? "extensions"
                              : query.view === "issues"
                                ? "issues"
                      : query.view === "people"
                        ? "people"
                        : "code";
  const [repository, setRepository] = useState<Repository>();
  const [upstream, setUpstream] = useState<Repository>();
  const [branches, setBranches] = useState<Branches>();
  const [tree, setTree] = useState<Tree>();
  const [blob, setBlob] = useState<Blob>();
  const [commits, setCommits] = useState<Commits>();
  const [error, setError] = useState("");
  const [copied, setCopied] = useState(false);
  const [actor, setActor] = useState("");
  const [showFork, setShowFork] = useState(false);
  const [showUpstreamPull, setShowUpstreamPull] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [actionError, setActionError] = useState("");
  const ref = revision || branches?.default_branch || "";

  const load = useCallback(async () => {
    setError("");
    try {
      const [repo, branchData, currentActor] = await Promise.all([
        get<Repository>(`/repositories/${id}`),
        get<Branches>(`/repositories/${id}/branches`),
        fetch("/api/session")
          .then(async (response) =>
            response.ok
              ? ((await response.json()) as { user: UserRecord }).user.id
              : "",
          )
          .catch(() => ""),
      ]);
      setActor(currentActor);
      setRepository(repo);
      setUpstream(
        repo.upstream_repository_id
          ? await get<Repository>(
              `/repositories/${repo.upstream_repository_id}`,
            ).catch(() => undefined)
          : undefined,
      );
      setBranches(branchData);
      const selected = revision || branchData.default_branch;
      if (
        view === "proposals" ||
        view === "pulls" ||
        view === "queue" ||
        view === "relationships" ||
        view === "releases" ||
        view === "incidents" ||
        view === "workspaces" ||
        view === "intelligence" ||
        view === "investigations" ||
        view === "decisions" ||
        view === "teams" ||
        view === "funds" ||
        view === "performance" ||
        view === "accessibility" ||
        view === "privacy" ||
        view === "locales" ||
        view === "experiments" ||
        view === "feedback" ||
        view === "issues" ||
        view === "documentation" ||
        view === "people" ||
        repo.empty ||
        !selected
      ) {
        setTree(undefined);
        setBlob(undefined);
        setCommits(undefined);
        return;
      }
      if (view === "commits") {
        setCommits(
          await get<Commits>(
            `/repositories/${id}/commits?ref=${encodeURIComponent(selected)}&per_page=100`,
          ),
        );
        setTree(undefined);
        setBlob(undefined);
      } else if (path) {
        try {
          setBlob(
            await get<Blob>(
              `/repositories/${id}/blob?ref=${encodeURIComponent(selected)}&path=${encodeURIComponent(path)}`,
            ),
          );
          setTree(undefined);
        } catch {
          setTree(
            await get<Tree>(
              `/repositories/${id}/tree?ref=${encodeURIComponent(selected)}&path=${encodeURIComponent(path)}`,
            ),
          );
          setBlob(undefined);
        }
        setCommits(undefined);
      } else {
        setTree(
          await get<Tree>(
            `/repositories/${id}/tree?ref=${encodeURIComponent(selected)}`,
          ),
        );
        setBlob(undefined);
        setCommits(undefined);
      }
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Repository unavailable.",
      );
    }
  }, [id, path, revision, view]);
  useEffect(() => {
    // Repository state follows the shareable revision/path URL.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  function navigate(next: { ref?: string; path?: string; view?: string }) {
    const values = new URLSearchParams();
    const nextView = next.view ?? view;
    if (
      nextView !== "proposals" &&
      nextView !== "pulls" &&
      nextView !== "queue" &&
      nextView !== "relationships" &&
      nextView !== "releases" &&
      nextView !== "workspaces" &&
      nextView !== "teams" &&
      nextView !== "issues" &&
      nextView !== "people"
    ) {
      const nextRef = next.ref ?? ref;
      if (nextRef) values.set("ref", nextRef);
      const nextPath = next.path ?? path;
      if (nextPath) values.set("path", nextPath);
    }
    if (nextView !== "code") values.set("view", nextView);
    router.push(`/repositories/${id}?${values}`);
  }
  const clone = repository
    ? `${process.env.NEXT_PUBLIC_GIT_ORIGIN ?? "http://localhost:8080"}${repository.git_url}`
    : "";
  async function copyClone() {
    await navigator.clipboard.writeText(clone);
    setCopied(true);
    setTimeout(() => setCopied(false), 1600);
  }
  async function synchronize() {
    if (!ref) return;
    setSyncing(true);
    setActionError("");
    try {
      await send(`/repositories/${id}/sync`, "POST", { branch: ref });
      await load();
    } catch (cause) {
      setActionError(
        cause instanceof Error
          ? cause.message
          : "Could not synchronize this branch.",
      );
    } finally {
      setSyncing(false);
    }
  }

  if (error)
    return (
      <RepositoryFrame>
        <section className="repository-error panel">
          <Book />
          <h1>Repository unavailable</h1>
          <p>{error}</p>
          <Link href="/">Return to workspace</Link>
        </section>
      </RepositoryFrame>
    );
  if (!repository || !branches)
    return (
      <RepositoryFrame>
        <div className="repository-loading">Reading repository state…</div>
      </RepositoryFrame>
    );
  const contextCommit =
    tree?.commit_id ?? blob?.commit_id ?? commits?.commit_id;
  return (
    <RepositoryFrame>
      <header className="repository-heading">
        <div className="repository-title">
          <span className="repo-icon">
            <Book />
          </span>
          <div>
            <div className="repository-owner">
              <Link href="/">Workspace</Link>
              <ChevronRight size={13} />
              <strong>{repository.name}</strong>
              <Badge>
                {repository.visibility === "public" ? "Public" : "Private"}
              </Badge>
            </div>
            <h1>{repository.name}</h1>
            <p>
              {repository.description || "No description has been added yet."}
            </p>
          </div>
        </div>
        <div className="repository-actions">
          <div className="clone-control">
            <code>{clone}</code>
            <Button variant="secondary" size="sm" onClick={copyClone}>
              {copied ? <Check size={14} /> : <Copy size={14} />}{" "}
              {copied ? "Copied" : "Clone"}
            </Button>
          </div>
          {actor && actor !== repository.owner_id && (
            <Button size="sm" onClick={() => setShowFork(true)}>
              <Branch size={14} />
              Fork
            </Button>
          )}
        </div>
      </header>
      {repository.upstream_repository_id && (
        <div className="fork-lineage panel">
          <Branch size={15} />
          <span>
            Forked from{" "}
            <Link href={`/repositories/${repository.upstream_repository_id}`}>
              {upstream?.name ?? short(repository.upstream_repository_id)}
            </Link>
          </span>
          {actor === repository.owner_id && (
            <>
              <Button
                size="sm"
                onClick={() => setShowUpstreamPull((value) => !value)}
              >
                <GitPullRequest size={14} />
                Contribute upstream
              </Button>
              {ref && (
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={syncing}
                  onClick={() => void synchronize()}
                >
                  {syncing ? "Synchronizing…" : `Sync ${ref}`}
                </Button>
              )}
            </>
          )}
        </div>
      )}
      {showUpstreamPull && repository.upstream_repository_id && (
        <UpstreamPullRequestForm
          fork={repository}
          upstream={upstream}
          branches={branches.items}
          close={() => setShowUpstreamPull(false)}
          created={(item) =>
            router.push(
              `/repositories/${repository.upstream_repository_id}?view=pulls&pull=${item.id}`,
            )
          }
        />
      )}
      {showFork && (
        <ForkRepository
          repository={repository}
          close={() => setShowFork(false)}
          created={(fork) => router.push(`/repositories/${fork.id}`)}
        />
      )}
      {actionError && (
        <p className="form-error fork-action-error" role="alert">
          {actionError}
        </p>
      )}
      <nav className="repository-tabs" aria-label="Repository">
        <button
          className={view === "feedback" ? "active" : ""}
          onClick={() => navigate({ view: "feedback", path: "" })}
        >
          <MessageCircle size={15} />
          Feedback
        </button>
        <button
          className={view === "experiments" ? "active" : ""}
          onClick={() => navigate({ view: "experiments", path: "" })}
        >
          <Sparkles size={15} />
          Experiments
        </button>
        <button
          className={view === "governance" ? "active" : ""}
          onClick={() => navigate({ view: "governance", path: "" })}
        >
          <User size={15} />
          Governance
        </button>
        <button
          className={view === "extensions" ? "active" : ""}
          onClick={() => navigate({ view: "extensions", path: "" })}
        >
          <Sparkles size={15} />
          Extensions
        </button>
        <button
          className={view === "documentation" ? "active" : ""}
          onClick={() => navigate({ view: "documentation", ref, path: "" })}
        >
          <Book size={15} />
          Docs
        </button>
        <button
          className={view === "contribute" ? "active" : ""}
          onClick={() => navigate({ view: "contribute", ref, path: "" })}
        >
          <Book size={15} />
          Contribute
        </button>
        <button
          className={view === "issues" ? "active" : ""}
          onClick={() => navigate({ view: "issues", path: "" })}
        >
          <MessageCircle size={15} />
          Issues
        </button>
        <button
          className={view === "teams" ? "active" : ""}
          onClick={() => navigate({ view: "teams", path: "" })}
        >
          <User size={15} />
          Delivery teams
        </button>
        <button
          className={view === "funds" ? "active" : ""}
          onClick={() => navigate({ view: "funds", path: "" })}
        >
          <Sparkles size={15} />
          Funds
        </button>
        <button
          className={view === "performance" ? "active" : ""}
          onClick={() => navigate({ view: "performance", path: "" })}
        >
          <Clock size={15} />
          Performance
        </button>
        <button
          className={view === "accessibility" ? "active" : ""}
          onClick={() => navigate({ view: "accessibility", path: "" })}
        >
          <User size={15} />
          Accessibility
        </button>
        <button
          className={view === "privacy" ? "active" : ""}
          onClick={() => navigate({ view: "privacy", path: "" })}
        >
          <User size={15} />
          Privacy
        </button>
        <button
          className={view === "locales" ? "active" : ""}
          onClick={() => navigate({ view: "locales", path: "" })}
        >
          <Book size={15} />
          Locales
        </button>
        <button
          className={view === "decisions" ? "active" : ""}
          onClick={() => navigate({ view: "decisions", path: "" })}
        >
          <Lightbulb size={15} />
          Decisions
        </button>
        <button
          className={view === "impact" ? "active" : ""}
          onClick={() => navigate({ view: "impact", ref, path: "" })}
        >
          <Sparkles size={15} />
          Impact
        </button>
        <button
          className={view === "investigations" ? "active" : ""}
          onClick={() => navigate({ view: "investigations", ref, path: "" })}
        >
          <MessageCircle size={15} />
          Investigations
        </button>
        <button
          className={view === "intelligence" ? "active" : ""}
          onClick={() => navigate({ view: "intelligence", ref, path: "" })}
        >
          <Sparkles size={15} />
          Knowledge map
        </button>
        <button
          className={view === "workspaces" ? "active" : ""}
          onClick={() => navigate({ view: "workspaces", path: "" })}
        >
          <Code size={15} />
          Workspaces
        </button>
        <button
          className={view === "incidents" ? "active" : ""}
          onClick={() => navigate({ view: "incidents", path: "" })}
        >
          <MessageCircle size={15} />
          Incidents
        </button>
        <button
          className={view === "code" ? "active" : ""}
          onClick={() => navigate({ view: "code", path: "" })}
        >
          <Code size={15} />
          Code
        </button>
        <button
          className={view === "queue" ? "active" : ""}
          onClick={() =>
            navigate({ view: "queue", ref: branches.default_branch, path: "" })
          }
        >
          <Branch size={15} />
          Integration queue
        </button>
        <button
          className={view === "relationships" ? "active" : ""}
          onClick={() => navigate({ view: "relationships", path: "" })}
        >
          <Book size={15} />
          Relationships
        </button>
        <button
          className={view === "releases" ? "active" : ""}
          onClick={() => navigate({ view: "releases", path: "" })}
        >
          <Sparkles size={15} />
          Releases
        </button>
        <button
          className={view === "commits" ? "active" : ""}
          onClick={() => navigate({ view: "commits", path: "" })}
        >
          <Clock size={15} />
          Commits
        </button>
        <button
          className={view === "proposals" ? "active" : ""}
          onClick={() => navigate({ view: "proposals", path: "" })}
        >
          <Lightbulb size={15} />
          Proposals
        </button>
        <button
          className={view === "pulls" ? "active" : ""}
          onClick={() => navigate({ view: "pulls", path: "" })}
        >
          <GitPullRequest size={15} />
          Pull requests
        </button>
        {actor === repository.owner_id && (
          <button
            className={view === "people" ? "active" : ""}
            onClick={() => navigate({ view: "people", path: "" })}
          >
            <User size={15} />
            People
          </button>
        )}
      </nav>
      {view === "governance" ? (
        <GovernanceCharter scope="repositories" id={repository.id} canManage={actor===repository.owner_id} />
      ) : view === "extensions" ? (
        <Extensions repository={repository.id} />
      ) : view === "issues" ? (
        <Issues repository={repository.id} actor={actor} owner={repository.owner_id} selected={query.issue} />
      ) : view === "documentation" ? (
        <DocumentationCollections repository={repository.id} actor={actor} owner={repository.owner_id} revision={ref} />
      ) : view === "contribute" ? (
        <><ContributorPathway repository={repository.id} actor={actor} owner={repository.owner_id} revision={ref} /><ContributionOpportunities repository={repository.id} actor={actor} owner={repository.owner_id} /></>
      ) : view === "proposals" ? (
        <ProposalWorkspace
          repository={repository}
          branches={branches.items}
          owner={repository.owner_id}
          selected={query.proposal}
          initialState={query.state}
          initialQuery={query.q}
        />
      ) : view === "teams" ? (
        <DeliveryTeams repository={repository.id} actor={actor} selected={query.team} />
      ) : view === "performance" ? (
        <PerformanceGoals repository={repository.id} actor={actor} />
      ) : view === "accessibility" ? (
        <><AccessibilityAssessments repository={repository.id} actor={actor} /><AccessibilityCommitments repository={repository.id} actor={actor} /><AccessibilityBarriers repository={repository.id} actor={actor} /></>
      ) : view === "privacy" ? (
        <><DataFlows repository={repository.id} /><DataCommitments repository={repository.id} actor={actor} /></>
      ) : view === "locales" ? (
        <><LocalePlans repository={repository.id} actor={actor} /><TranslationWork repository={repository.id} /><LocalizationVerification repository={repository.id} /></>
      ) : view === "funds" ? (
        <ProjectFunds repository={repository.id} actor={actor} />
      ) : view === "experiments" ? (
        <ProductExperiments repository={repository.id} actor={actor} />
      ) : view === "feedback" ? (
        <ProductFeedback repository={repository.id} actor={actor} selected={query.feedback} />
      ) : view === "decisions" ? (
        <TechnicalDecisions repository={repository} actor={actor} selected={query.decision} />
      ) : view === "pulls" ? (
        <PullRequestWorkspace
          repository={id}
          branches={branches.items}
          selected={query.pull}
          section={query.section}
        />
      ) : view === "queue" ? (
        <IntegrationQueueWorkspace
          repository={repository}
          branches={branches.items}
          initialBranch={revision || branches.default_branch}
          actor={actor}
          onBranch={(branch) =>
            navigate({ view: "queue", ref: branch, path: "" })
          }
        />
      ) : view === "relationships" ? (
        <RelationshipWorkspace repository={repository} actor={actor} />
      ) : view === "releases" ? (
        <ReleaseWorkspace
          repository={repository}
          branches={branches.items}
          actor={actor}
          selected={query.release}
        />
      ) : view === "incidents" ? (
        <IncidentWorkspace
          repository={repository}
          actor={actor}
          selected={query.incident}
        />
      ) : view === "workspaces" ? (
        <DevelopmentWorkspaces
          repository={repository}
          branches={branches.items}
          actor={actor}
          selected={query.workspace}
        />
      ) : view === "intelligence" ? (
        <CodeIntelligenceWorkspace
          repository={repository}
          branches={branches.items}
          initialRevision={revision || branches.default_branch}
          initialQuery={query.q}
          selectedSymbol={query.symbol}
        />
      ) : view === "investigations" ? (
        <CollaborativeInvestigations
          repository={repository.id}
          revision={ref}
          selected={query.investigation}
          conversation={query.conversation}
        />
      ) : view === "impact" ? (
        <ImpactAssessments
          repository={repository.id}
          revision={ref}
          selected={query.assessment}
          actor={actor}
        />
      ) : view === "people" && actor === repository.owner_id ? (
        <CollaboratorWorkspace repository={id} />
      ) : repository.empty ? (
        <section className="empty-repository panel">
          <Branch />
          <h2>This repository is ready for its first push</h2>
          <p>
            Clone it, add project files and a README, then push the{" "}
            <code>{branches.default_branch}</code> branch.
          </p>
          <pre>
            <code>{`git clone ${clone}\ncd ${repository.name}\ngit push origin ${branches.default_branch}`}</code>
          </pre>
        </section>
      ) : (
        <>
          <div className="revision-bar panel">
            <label>
              <Branch size={15} />
              <span className="sr-only">Branch</span>
              <select
                value={ref}
                onChange={(event) =>
                  navigate({ ref: event.target.value, path: "" })
                }
              >
                {branches.items.map((branch) => (
                  <option key={branch.name}>{branch.name}</option>
                ))}
              </select>
            </label>
            <div>
              <span>Viewing</span>
              <strong>{ref}</strong>
              {contextCommit && (
                <>
                  <span>at</span>
                  <code>{short(contextCommit)}</code>
                </>
              )}
            </div>
            <span className="revision-note">
              Branch and revision stay attached to every path.
            </span>
          </div>
          {view === "commits" && commits ? (
            <CommitList commits={commits.items} repository={id} refName={ref} />
          ) : blob ? (
            <BlobView
              blob={blob}
              onCrumb={(next) => navigate({ path: next })}
            />
          ) : tree ? (
            <TreeView tree={tree} onPath={(next) => navigate({ path: next })} />
          ) : (
            <div className="repository-loading">Loading revision…</div>
          )}
        </>
      )}
      {!repository.empty && actor && <GroundedQuestions repository={repository.id} revision={ref} context={query.workspace?{type:"workspace",id:query.workspace}:query.incident?{type:"incident",id:query.incident}:query.pull?{type:"pull_request",id:query.pull}:query.proposal?{type:"proposal",id:query.proposal}:path?{type:"file",path}:{type:"repository"}} />}
    </RepositoryFrame>
  );
}

function ForkRepository({
  repository,
  close,
  created,
}: {
  repository: Repository;
  close: () => void;
  created: (fork: Repository) => void;
}) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError("");
    const data = new FormData(event.currentTarget);
    try {
      created(
        await send<Repository>(`/repositories/${repository.id}/forks`, "POST", {
          name: data.get("name"),
          visibility: data.get("visibility"),
        }),
      );
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Could not create this fork.",
      );
      setPending(false);
    }
  }
  return (
    <section className="fork-panel panel" aria-labelledby="fork-title">
      <div>
        <span className="repo-icon">
          <Branch />
        </span>
        <div>
          <h2 id="fork-title">Fork {repository.name}</h2>
          <p>
            Create an independently owned copy for experiments and
            contributions.
          </p>
        </div>
      </div>
      <form className="repository-form" onSubmit={submit}>
        <label>
          Fork name
          <input
            name="name"
            required
            pattern="[a-z0-9._-]+"
            maxLength={100}
            defaultValue={repository.name}
          />
        </label>
        <label>
          Visibility
          <select name="visibility" defaultValue="private">
            <option value="private">Private</option>
            <option value="public">Public</option>
          </select>
        </label>
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
            {pending ? "Forking…" : "Create fork"}
          </Button>
        </div>
      </form>
    </section>
  );
}

function UpstreamPullRequestForm({
  fork,
  upstream,
  branches,
  close,
  created,
}: {
  fork: Repository;
  upstream?: Repository;
  branches: BranchRecord[];
  close: () => void;
  created: (pull: PullRequest) => void;
}) {
  const [targets, setTargets] = useState<BranchRecord[]>([]);
  const [source, setSource] = useState(
    branches.find((branch) => !branch.is_default)?.name ??
      branches[0]?.name ??
      "",
  );
  const [target, setTarget] = useState("");
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    if (!fork.upstream_repository_id) return;
    get<Branches>(`/repositories/${fork.upstream_repository_id}/branches`)
      .then((result) => {
        setTargets(result.items);
        setTarget(result.default_branch || result.items[0]?.name || "");
      })
      .catch(() => setError("Could not read upstream branches."));
  }, [fork.upstream_repository_id]);
  return (
    <form
      className="pull-form panel"
      onSubmit={async (event) => {
        event.preventDefault();
        setBusy(true);
        setError("");
        try {
          created(
            await send<PullRequest>(
              `/repositories/${fork.upstream_repository_id}/pull-requests`,
              "POST",
              {
                title,
                body,
                source_repository_id: fork.id,
                source_branch: source,
                target_branch: target,
              },
            ),
          );
        } catch (cause) {
          setError(
            cause instanceof Error
              ? cause.message
              : "Unable to open the upstream pull request.",
          );
          setBusy(false);
        }
      }}
    >
      <div className="pull-form-intro">
        <GitPullRequest />
        <span>
          <strong>Contribute to {upstream?.name ?? "upstream"}</strong>
          <small>
            Your fork branch and both exact revisions will be recorded for
            upstream review.
          </small>
        </span>
      </div>
      <label>
        Title
        <input
          required
          maxLength={200}
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="Summarize the contribution"
        />
      </label>
      <label className="pull-body">
        Description
        <textarea
          maxLength={65536}
          value={body}
          onChange={(event) => setBody(event.target.value)}
          placeholder="Explain the change and where feedback would help."
        />
      </label>
      <div className="branch-compare">
        <label>
          Fork branch
          <select
            value={source}
            onChange={(event) => setSource(event.target.value)}
          >
            {branches.map((branch) => (
              <option key={branch.name}>{branch.name}</option>
            ))}
          </select>
        </label>
        <span>into</span>
        <label>
          Upstream branch
          <select
            value={target}
            onChange={(event) => setTarget(event.target.value)}
          >
            {targets.map((branch) => (
              <option key={branch.name}>{branch.name}</option>
            ))}
          </select>
        </label>
      </div>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <div className="form-actions">
        <Button type="button" variant="secondary" onClick={close}>
          Cancel
        </Button>
        <Button type="submit" disabled={busy || !source || !target}>
          {busy ? "Opening…" : "Open upstream pull request"}
        </Button>
      </div>
    </form>
  );
}

function RepositoryFrame({ children }: { children: React.ReactNode }) {
  return (
    <div className="repository-page">
      <a className="skip-link" href="#main-content">
        Skip to repository content
      </a>
      <header className="repository-topbar">
        <Link className="brand" href="/">
          <span className="brand-mark">K</span>
          <span>Kanso</span>
        </Link>
        <Link href="/" className="back-workspace">
          Your workspace
        </Link>
      </header>
      <main id="main-content" className="repository-main">
        {children}
      </main>
    </div>
  );
}
function Breadcrumbs({
  path,
  onCrumb,
}: {
  path: string;
  onCrumb: (path: string) => void;
}) {
  const parts = path.split("/").filter(Boolean);
  return (
    <nav className="breadcrumbs" aria-label="File path">
      <button onClick={() => onCrumb("")}>root</button>
      {parts.map((part, index) => (
        <span key={`${part}-${index}`}>
          <ChevronRight size={12} />
          <button onClick={() => onCrumb(parts.slice(0, index + 1).join("/"))}>
            {part}
          </button>
        </span>
      ))}
    </nav>
  );
}
function TreeView({
  tree,
  onPath,
}: {
  tree: Tree;
  onPath: (path: string) => void;
}) {
  return (
    <section className="file-browser panel">
      <div className="browser-header">
        <Breadcrumbs path={tree.path} onCrumb={onPath} />
        <code>
          {tree.entries.length} items · {short(tree.tree_id)}
        </code>
      </div>
      {tree.entries.length ? (
        <div className="file-list">
          {tree.entries.map((entry) => (
            <button key={entry.path} onClick={() => onPath(entry.path)}>
              <span
                className={entry.type === "tree" ? "folder-icon" : "file-icon"}
              >
                {entry.type === "tree" ? (
                  <Folder size={16} />
                ) : (
                  <File size={16} />
                )}
              </span>
              <strong>{entry.name}</strong>
              <span>
                {entry.type === "tree"
                  ? "Directory"
                  : formatSize(entry.size ?? 0)}
              </span>
              <code>{short(entry.object_id)}</code>
            </button>
          ))}
        </div>
      ) : (
        <p className="empty-directory">This directory is empty.</p>
      )}
    </section>
  );
}
function CodeIntelligenceWorkspace({
  repository,
  branches,
  initialRevision,
  initialQuery,
  selectedSymbol,
}: {
  repository: Repository;
  branches: BranchRecord[];
  initialRevision: string;
  initialQuery?: string;
  selectedSymbol?: string;
}) {
  const router = useRouter();
  const [query, setQuery] = useState(initialQuery ?? "");
  const [map, setMap] = useState<KnowledgeMap>();
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    setError("");
    try {
      const params = new URLSearchParams({ ref: initialRevision });
      if (initialQuery) params.set("q", initialQuery);
      if (selectedSymbol) params.set("symbol", selectedSymbol);
      setMap(
        await get<KnowledgeMap>(
          `/repositories/${repository.id}/code-intelligence?${params}`,
        ),
      );
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Knowledge map unavailable.",
      );
    }
  }, [initialQuery, initialRevision, repository.id, selectedSymbol]);
  useEffect(() => {
    // Knowledge map state follows the shareable revision/query URL.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);
  const open = (values: { ref?: string; q?: string; symbol?: string }) => {
    const params = new URLSearchParams({
      view: "intelligence",
      ref: values.ref ?? initialRevision,
    });
    if (values.q) params.set("q", values.q);
    if (values.symbol) params.set("symbol", values.symbol);
    router.push(`/repositories/${repository.id}?${params}`);
  };
  const source = (location: { path: string; line: number }) =>
    `/repositories/${repository.id}?ref=${encodeURIComponent(map?.commit_id ?? initialRevision)}&path=${encodeURIComponent(location.path)}#L${location.line}`;
  const inspected = selectedSymbol
    ? map?.symbols.find((item) => item.name === selectedSymbol)
    : undefined;
  return (
    <section className="knowledge-map">
      <header className="proposal-toolbar">
        <div>
          <p className="eyebrow">Revision-aware evidence</p>
          <h2>Knowledge map</h2>
          <p>
            Find definitions, usages, tests, recent owners, and declared
            repository relationships without leaving this exact snapshot.
          </p>
        </div>
      </header>
      <div className="revision-bar panel">
        <label>
          Revision
          <select
            value={initialRevision}
            onChange={(event) =>
              open({ ref: event.target.value, q: initialQuery })
            }
          >
            {branches.map((branch) => (
              <option key={branch.name} value={branch.name}>
                {branch.name} · {short(branch.commit_id)}
              </option>
            ))}
          </select>
        </label>
        {map && (
          <span>
            Analyzing <code>{short(map.commit_id)}</code> ·{" "}
            {map.analysis.files_analyzed} files
          </span>
        )}
      </div>
      <form
        className="panel proposal-toolbar"
        onSubmit={(event) => {
          event.preventDefault();
          open({ q: query });
        }}
      >
        <label>
          <span className="sr-only">Search symbols and paths</span>
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search symbols and paths"
          />
        </label>
        <Button size="sm">Search</Button>
      </form>
      {error && (
        <p role="alert" className="form-error">
          {error}
        </p>
      )}
      {map && map.analysis.status !== "complete" && (
        <div className="panel form-error">
          <strong>Analysis is incomplete.</strong>{" "}
          {map.analysis.reasons
            .map((reason) => reason.replaceAll("_", " "))
            .join(" · ")}
          . Results remain pinned to <code>{short(map.commit_id)}</code>.
        </div>
      )}
      {map && map.analysis.stale && (
        <div className="panel form-error">
          This analysis is stale. Choose the current branch revision to refresh
          it.
        </div>
      )}
      <div className="workspace-runtime-layout">
        <aside className="workspace-runtime-list panel">
          <h3>Symbols</h3>
          {map?.symbols.map((symbol) => (
            <Link
              className={symbol.name === selectedSymbol ? "active" : ""}
              key={`${symbol.definition.path}:${symbol.definition.line}:${symbol.name}`}
              href={`/repositories/${repository.id}?view=intelligence&ref=${encodeURIComponent(map.commit_id)}&symbol=${encodeURIComponent(symbol.name)}`}
            >
              <span>
                <strong>{symbol.name}</strong>
                <Badge>{symbol.kind}</Badge>
              </span>
              <small>
                {symbol.definition.path}:{symbol.definition.line}
              </small>
            </Link>
          ))}
          {map && !map.symbols.length && <p>No matching symbols found.</p>}
        </aside>
        <div>
          {inspected ? (
            <article className="panel">
              <header>
                <div>
                  <p className="eyebrow">
                    {inspected.language} {inspected.kind}
                  </p>
                  <h3>{inspected.name}</h3>
                </div>
              </header>
              <p>
                Defined at{" "}
                <Link href={source(inspected.definition)}>
                  {inspected.definition.path}:{inspected.definition.line}
                </Link>
              </p>
              {inspected.owner && (
                <p>
                  Most recently changed by{" "}
                  <strong>{inspected.owner.author}</strong> in{" "}
                  <Link
                    href={`/repositories/${repository.id}?view=commits&ref=${map?.commit_id}`}
                  >
                    {short(inspected.owner.id)}
                  </Link>
                  {inspected.owner.authored_at
                    ? ` on ${new Date(inspected.owner.authored_at).toLocaleDateString()}`
                    : ""}
                  .
                </p>
              )}
              <EvidenceList
                title="Callers"
                items={inspected.callers}
                source={source}
              />
              <EvidenceList
                title="References"
                items={inspected.references}
                source={source}
              />
              <EvidenceList
                title="Tests"
                items={inspected.tests}
                source={source}
              />
            </article>
          ) : (
            <div className="panel">
              <h3>Select a symbol</h3>
              <p>
                Inspect its exact definition, callers, references, test
                evidence, and most recent file owner.
              </p>
            </div>
          )}
          <article className="panel">
            <h3>Declared cross-repository dependencies</h3>
            {map?.dependencies.length ? (
              <ul>
                {map.dependencies.map((dependency) => (
                  <li key={dependency.id}>
                    <Badge>{dependency.state}</Badge>{" "}
                    <Link
                      href={`/repositories/${dependency.provider_repository_id}?view=intelligence`}
                    >
                      {dependency.interface_name}
                    </Link>{" "}
                    <code>{dependency.constraint}</code>
                    {dependency.evidence_path && (
                      <>
                        {" "}
                        · evidence <code>{dependency.evidence_path}</code>
                      </>
                    )}
                  </li>
                ))}
              </ul>
            ) : (
              <p>No readable dependencies are declared for this revision.</p>
            )}
          </article>
        </div>
      </div>
    </section>
  );
}

function EvidenceList({
  title,
  items,
  source,
}: {
  title: string;
  items: Array<{ path: string; line: number }>;
  source: (item: { path: string; line: number }) => string;
}) {
  return (
    <section>
      <h4>
        {title} <Badge>{items.length}</Badge>
      </h4>
      {items.length ? (
        <ul>
          {items.slice(0, 100).map((item, index) => (
            <li key={`${item.path}:${item.line}:${index}`}>
              <Link href={source(item)}>
                {item.path}:{item.line}
              </Link>
            </li>
          ))}
        </ul>
      ) : (
        <p className="muted">
          No {title.toLowerCase()} found in the analyzed languages.
        </p>
      )}
    </section>
  );
}

function BlobView({
  blob,
  onCrumb,
}: {
  blob: Blob;
  onCrumb: (path: string) => void;
}) {
  const lines = blob.content.split("\n");
  return (
    <section className="blob-browser">
      <div className="browser-header panel">
        <Breadcrumbs path={blob.path} onCrumb={onCrumb} />
        <span>
          {formatSize(blob.size)} · <code>{short(blob.object_id)}</code>
        </span>
      </div>
      {blob.binary ? (
        <div className="binary-file panel">
          <File />
          <h2>Binary file</h2>
          <p>
            This file cannot be displayed, but its identity and size are
            preserved above.
          </p>
        </div>
      ) : (
        <div className="code-view panel">
          {lines.map((line, index) => (
            <div className="code-line" id={`L${index + 1}`} key={index}>
              <span>{index + 1}</span>
              <code>{line || " "}</code>
            </div>
          ))}
          {blob.truncated && (
            <p className="truncated">Preview limited to the first 1 MB.</p>
          )}
        </div>
      )}
    </section>
  );
}
function CommitList({
  commits,
  repository,
  refName,
}: {
  commits: Commit[];
  repository: string;
  refName: string;
}) {
  return (
    <section className="commit-list panel">
      <div className="browser-header">
        <strong>
          {commits.length} commits on {refName}
        </strong>
        <span>Newest reachable work first</span>
      </div>
      {commits.map((commit) => (
        <article key={commit.id}>
          <span className="commit-node">
            <Clock size={14} />
          </span>
          <div>
            <h2>{summary(commit.message)}</h2>
            <p>
              {commit.author || "Unknown author"}
              {commit.authored_at && (
                <>
                  {" "}
                  committed{" "}
                  <time dateTime={commit.authored_at}>
                    {new Date(commit.authored_at).toLocaleString()}
                  </time>
                </>
              )}
            </p>
          </div>
          <Link href={`/repositories/${repository}?ref=${commit.id}`}>
            <code>{short(commit.id)}</code>
          </Link>
        </article>
      ))}
    </section>
  );
}

function DevelopmentWorkspaces({
  repository,
  branches,
  actor,
  selected,
}: {
  repository: Repository;
  branches: BranchRecord[];
  actor: string;
  selected?: string;
}) {
  const [items, setItems] = useState<DevelopmentWorkspace[]>([]);
  const [revision, setRevision] = useState(
    branches.find((branch) => branch.is_default)?.commit_id ?? "",
  );
  const [contextType, setContextType] = useState("repository");
  const [contextID, setContextID] = useState("");
  const [parentID, setParentID] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [files, setFiles] = useState<WorkspaceFile[]>([]);
  const [filePath, setFilePath] = useState("");
  const [content, setContent] = useState("");
  const [fileDigest, setFileDigest] = useState("");
  const [search, setSearch] = useState("");
  const [matches, setMatches] = useState<WorkspaceMatch[]>([]);
  const [command, setCommand] = useState("");
  const [terminal, setTerminal] = useState("");
  const [message, setMessage] = useState("");
  const [controlSubject, setControlSubject] = useState("");
  const [checkpointSummary, setCheckpointSummary] = useState("");
  const [checkpointPaths, setCheckpointPaths] = useState("");
  const [checkpointDependencies, setCheckpointDependencies] = useState("");
  const [checkpointCommands, setCheckpointCommands] = useState("");
  const [checkpointParent, setCheckpointParent] = useState("");
  const [policy, setPolicy] =
    useState<DevelopmentWorkspace["effective_policy"]>();
  const load = useCallback(async () => {
    try {
      setItems(
        (
          await get<{ items: DevelopmentWorkspace[] }>(
            `/repositories/${repository.id}/workspaces`,
          )
        ).items,
      );
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Workspaces unavailable.",
      );
    }
  }, [repository.id]);
  useEffect(() => {
    // Remote workspace lifecycle is intentionally refreshed while setup runs.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
    const timer = setInterval(() => void load(), 2500);
    return () => clearInterval(timer);
  }, [load]);
  useEffect(() => {
    get<DevelopmentWorkspace["effective_policy"]>(
      `/repositories/${repository.id}/workspace-policy`,
    )
      .then(setPolicy)
      .catch(() => undefined);
  }, [repository.id]);
  const inspected = items.find((item) => item.id === selected) ?? items[0];
  const presenceWorkspaceID = inspected?.id;
  const presenceWorkspaceState = inspected?.state;
  useEffect(() => {
    if (!presenceWorkspaceID || presenceWorkspaceState !== "ready" || !actor)
      return;
    const heartbeat = () =>
      send(
        `/repositories/${repository.id}/workspaces/${presenceWorkspaceID}/presence`,
        "POST",
        { surface: filePath ? "files" : "discussion", path: filePath },
      )
        .then(load)
        .catch(() => undefined);
    void heartbeat();
    const timer = setInterval(heartbeat, 30000);
    return () => clearInterval(timer);
  }, [
    presenceWorkspaceID,
    presenceWorkspaceState,
    actor,
    filePath,
    repository.id,
    load,
  ]);
  async function control(
    item: DevelopmentWorkspace,
    action: "suspend" | "resume",
  ) {
    setBusy(true);
    setError("");
    try {
      await send(
        `/repositories/${repository.id}/workspaces/${item.id}/${action}`,
        "POST",
        {},
      );
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : `Unable to ${action}.`);
    } finally {
      setBusy(false);
    }
  }
  async function browse(item: DevelopmentWorkspace, path = "") {
    try {
      const result = await get<{ items: WorkspaceFile[] }>(
        `/repositories/${repository.id}/workspaces/${item.id}/files?path=${encodeURIComponent(path)}`,
      );
      setFiles(result.items);
      if (result.items.length === 1 && !result.items[0].directory) {
        setFilePath(result.items[0].path);
        setContent(result.items[0].content ?? "");
        setFileDigest(result.items[0].digest ?? "");
      }
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Unable to browse workspace files.",
      );
    }
  }
  return (
    <section className="development-workspaces">
      <header className="proposal-toolbar">
        <div>
          <p className="eyebrow">Exact-revision environments</p>
          <h2>Development workspaces</h2>
          <p>
            Enter the repository-defined project state with retained setup
            evidence.
          </p>
        </div>
      </header>
      {actor === repository.owner_id && policy && (
        <form
          className="panel workspace-policy"
          onSubmit={async (e) => {
            e.preventDefault();
            setBusy(true);
            try {
              setPolicy(
                await send(
                  `/repositories/${repository.id}/workspace-policy`,
                  "PUT",
                  policy,
                ),
              );
            } catch (cause) {
              setError(
                cause instanceof Error
                  ? cause.message
                  : "Unable to update workspace policy.",
              );
            } finally {
              setBusy(false);
            }
          }}
        >
          <h3>Workspace governance</h3>
          <p>
            Bound every new environment; running work retains its captured
            policy and evidence.
          </p>
          <label>
            CPU seconds
            <input
              type="number"
              min="1"
              max="3600"
              value={policy.cpu_seconds}
              onChange={(e) =>
                setPolicy({ ...policy, cpu_seconds: Number(e.target.value) })
              }
            />
          </label>
          <label>
            Memory MB
            <input
              type="number"
              min="128"
              max="32768"
              value={policy.memory_mb}
              onChange={(e) =>
                setPolicy({ ...policy, memory_mb: Number(e.target.value) })
              }
            />
          </label>
          <label>
            Disk MB
            <input
              type="number"
              min="128"
              max="102400"
              value={policy.disk_mb}
              onChange={(e) =>
                setPolicy({ ...policy, disk_mb: Number(e.target.value) })
              }
            />
          </label>
          <label>
            Idle minutes
            <input
              type="number"
              min="5"
              value={policy.idle_minutes}
              onChange={(e) =>
                setPolicy({ ...policy, idle_minutes: Number(e.target.value) })
              }
            />
          </label>
          <label>
            Retention days
            <input
              type="number"
              min="1"
              value={policy.retention_days}
              onChange={(e) =>
                setPolicy({ ...policy, retention_days: Number(e.target.value) })
              }
            />
          </label>
          <label>
            Sharing
            <select
              value={policy.sharing}
              onChange={(e) =>
                setPolicy({ ...policy, sharing: e.target.value })
              }
            >
              <option value="private">Creator only</option>
              <option value="participants">Repository participants</option>
            </select>
          </label>
          <label>
            <input
              type="checkbox"
              checked={policy.agent_execution}
              onChange={(e) =>
                setPolicy({ ...policy, agent_execution: e.target.checked })
              }
            />{" "}
            Approved agent execution
          </label>
          <Button disabled={busy}>Save policy</Button>
        </form>
      )}
      {actor && (
        <form
          className="workspace-launch panel"
          onSubmit={async (event) => {
            event.preventDefault();
            setBusy(true);
            setError("");
            try {
              await send(`/repositories/${repository.id}/workspaces`, "POST", {
                revision,
                source_context: {
                  type: contextType,
                  ...(contextID ? { id: contextID } : {}),
                  ...(parentID ? { parent_id: parentID } : {}),
                },
              });
              await load();
            } catch (cause) {
              setError(
                cause instanceof Error
                  ? cause.message
                  : "Unable to launch workspace.",
              );
            } finally {
              setBusy(false);
            }
          }}
        >
          <label>
            Exact revision
            <input
              required
              list="workspace-revisions"
              value={revision}
              onChange={(event) => setRevision(event.target.value)}
            />
            <datalist id="workspace-revisions">
              {branches.map((branch) => (
                <option key={branch.name} value={branch.commit_id}>
                  {branch.name}
                </option>
              ))}
            </datalist>
          </label>
          <label>
            Shared context
            <select
              value={contextType}
              onChange={(event) => setContextType(event.target.value)}
            >
              <option value="repository">Repository</option>
              <option value="proposal_task">Proposal task</option>
              <option value="pull_request">Pull request</option>
              <option value="incident_repair">Incident repair</option>
            </select>
          </label>
          {contextType !== "repository" && (
            <label>
              {contextType === "proposal_task"
                ? "Task ID"
                : contextType === "incident_repair"
                  ? "Mitigation ID"
                  : "Pull request ID"}
              <input
                required
                value={contextID}
                onChange={(event) => setContextID(event.target.value)}
              />
            </label>
          )}
          {(contextType === "proposal_task" ||
            contextType === "incident_repair") && (
            <label>
              {contextType === "proposal_task" ? "Proposal ID" : "Incident ID"}
              <input
                required
                value={parentID}
                onChange={(event) => setParentID(event.target.value)}
              />
            </label>
          )}
          <Button type="submit" disabled={busy || !revision}>
            {busy ? "Launching…" : "Launch workspace"}
          </Button>
        </form>
      )}
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <div className="workspace-runtime-layout">
        <aside className="workspace-runtime-list panel">
          {items.map((item) => (
            <Link
              key={item.id}
              className={item.id === inspected?.id ? "active" : ""}
              href={`/repositories/${repository.id}?view=workspaces&workspace=${item.id}`}
            >
              <span>
                <strong>{item.source_context.type.replaceAll("_", " ")}</strong>
                <Badge>{item.state}</Badge>
              </span>
              <code>{short(item.revision)}</code>
              <small>
                by <Actor id={item.creator_id} />
              </small>
            </Link>
          ))}
          {!items.length && <p>No environments have been launched yet.</p>}
        </aside>
        {inspected && (
          <article className="workspace-runtime panel">
            <header>
              <div>
                <p className="eyebrow">
                  Foundation {short(inspected.definition_digest)}
                </p>
                <h3>
                  {inspected.source_context.type.replaceAll("_", " ")} workspace
                </h3>
              </div>
              <Badge>{inspected.state}</Badge>
            </header>
            <dl>
              <div>
                <dt>Revision</dt>
                <dd>
                  <code>{inspected.revision}</code>
                </dd>
              </div>
              <div>
                <dt>Effective access</dt>
                <dd>
                  {inspected.effective_access.permission} ·{" "}
                  <Actor id={inspected.effective_access.actor_id} />
                </dd>
              </div>
              <div>
                <dt>Resources</dt>
                <dd>
                  {inspected.definition.resources.memory_mb} MB memory ·{" "}
                  {inspected.definition.resources.disk_mb} MB disk ·{" "}
                  {inspected.definition.resources.cpu_seconds}s CPU
                </dd>
              </div>
              <div>
                <dt>Tools</dt>
                <dd>
                  {inspected.definition.tools
                    .map((tool) => `${tool.name} ${tool.version}`)
                    .join(", ") || "System runtime"}
                </dd>
              </div>
            </dl>
            <div className="workspace-controls">
              {inspected.state === "ready" && (
                <Button
                  disabled={busy}
                  variant="secondary"
                  onClick={() => void control(inspected, "suspend")}
                >
                  Suspend
                </Button>
              )}
              {inspected.state === "suspended" && (
                <Button
                  disabled={busy}
                  onClick={() => void control(inspected, "resume")}
                >
                  Resume retained foundation
                </Button>
              )}
              {actor === repository.owner_id &&
                (inspected.state === "ready" ||
                  inspected.state === "suspended") && (
                  <>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={async () => {
                        await send(
                          `/repositories/${repository.id}/workspaces/${inspected.id}/expiry`,
                          "POST",
                          {
                            expires_at: new Date(
                              Date.now() +
                                Math.max(
                                  inspected.effective_policy
                                    .expiry_notice_hours,
                                  24,
                                ) *
                                  3600000,
                            ).toISOString(),
                          },
                        );
                        await load();
                      }}
                    >
                      Announce expiry
                    </Button>
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={async () => {
                        await send(
                          `/repositories/${repository.id}/workspaces/${inspected.id}/stop`,
                          "POST",
                          {
                            reason: "stopped by repository owner",
                            expire: false,
                          },
                        );
                        await load();
                      }}
                    >
                      Stop environment
                    </Button>
                  </>
                )}
              {inspected.expires_at && inspected.state === "ready" && (
                <a
                  href={`/api/repositories/${repository.id}/workspaces/${inspected.id}/export`}
                >
                  Export unpublished work
                </a>
              )}
            </div>
            <p>
              Policy: {inspected.effective_policy.memory_mb} MB memory ·{" "}
              {inspected.effective_policy.disk_mb} MB disk · network{" "}
              {inspected.effective_policy.network} ·{" "}
              {inspected.effective_policy.sharing} sharing · agents{" "}
              {inspected.effective_policy.agent_execution
                ? "allowed"
                : "blocked"}
              .
            </p>
            {inspected.expires_at && (
              <p className="form-error">
                Expiry announced for{" "}
                {new Date(inspected.expires_at).toLocaleString()}. Export or
                checkpoint unpublished work before then.
              </p>
            )}
            {inspected.rebuild_required && (
              <p className="form-error">
                Rebuild required: {inspected.rebuild_reasons?.join(", ")}
              </p>
            )}
            {inspected.consumption?.length ? (
              <p>
                Attributed consumption:{" "}
                {inspected.consumption.map((c) => (
                  <span key={`${c.actor_id}-${c.recorded_at}`}>
                    <Actor id={c.actor_id} compact /> {c.quantity} {c.unit}{" "}
                    {c.kind};{" "}
                  </span>
                ))}
              </p>
            ) : null}
            {inspected.state === "ready" && (
              <section className="workspace-collaboration">
                <h4>Live collaboration</h4>
                <div className="workspace-presence">
                  {inspected.presence?.map((presence) => (
                    <span key={presence.actor_id}>
                      <Actor id={presence.actor_id} compact /> ·{" "}
                      {presence.surface}
                      {presence.path ? ` / ${presence.path}` : ""}
                    </span>
                  ))}
                  {!inspected.presence?.length && (
                    <span>No collaborators connected.</span>
                  )}
                </div>
                <form
                  onSubmit={async (e) => {
                    e.preventDefault();
                    await send(
                      `/repositories/${repository.id}/workspaces/${inspected.id}/messages`,
                      "POST",
                      { message },
                    );
                    setMessage("");
                    await load();
                  }}
                >
                  <label>
                    Discuss the running work
                    <input
                      value={message}
                      onChange={(e) => setMessage(e.target.value)}
                    />
                  </label>
                  <Button size="sm" disabled={!message}>
                    Send
                  </Button>
                </form>
                <form
                  onSubmit={async (e) => {
                    e.preventDefault();
                    await send(
                      `/repositories/${repository.id}/workspaces/${inspected.id}/controls`,
                      "POST",
                      {
                        subject_id: controlSubject,
                        subject_kind: "human",
                        mode: "edit",
                        scopes: ["files", "preview"],
                      },
                    );
                    setControlSubject("");
                    await load();
                  }}
                >
                  <label>
                    Grant collaborator edit control
                    <input
                      placeholder="User ID"
                      value={controlSubject}
                      onChange={(e) => setControlSubject(e.target.value)}
                    />
                  </label>
                  <Button size="sm" disabled={!controlSubject}>
                    Grant
                  </Button>
                </form>
                {inspected.controls?.map((grant) => (
                  <div className="workspace-control" key={grant.id}>
                    <span>
                      <Actor id={grant.subject_id} compact /> · {grant.mode}{" "}
                      {grant.scopes.join(", ")} · <Badge>{grant.state}</Badge>
                    </span>
                    {grant.state !== "revoked" && (
                      <span>
                        {grant.state === "paused" ? (
                          <Button
                            size="sm"
                            onClick={async () => {
                              await send(
                                `/repositories/${repository.id}/workspaces/${inspected.id}/controls/${grant.id}/interventions`,
                                "POST",
                                { action: "resume", version: grant.version },
                              );
                              await load();
                            }}
                          >
                            Resume
                          </Button>
                        ) : (
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={async () => {
                              await send(
                                `/repositories/${repository.id}/workspaces/${inspected.id}/controls/${grant.id}/interventions`,
                                "POST",
                                { action: "pause", version: grant.version },
                              );
                              await load();
                            }}
                          >
                            Pause
                          </Button>
                        )}
                        <Button
                          size="sm"
                          variant="secondary"
                          onClick={async () => {
                            await send(
                              `/repositories/${repository.id}/workspaces/${inspected.id}/controls/${grant.id}/interventions`,
                              "POST",
                              { action: "revoke", version: grant.version },
                            );
                            await load();
                          }}
                        >
                          Revoke
                        </Button>
                      </span>
                    )}
                  </div>
                ))}
                <ol className="workspace-activity">
                  {inspected.activity
                    ?.slice(-20)
                    .reverse()
                    .map((entry) => (
                      <li key={entry.sequence}>
                        <Badge>{entry.kind ?? entry.type}</Badge>{" "}
                        {entry.actor_id && (
                          <Actor id={entry.actor_id} compact />
                        )}{" "}
                        {entry.type.replaceAll("_", " ")}
                        {entry.path ? ` · ${entry.path}` : ""}
                        {entry.message ? ` — ${entry.message}` : ""}
                      </li>
                    ))}
                </ol>
              </section>
            )}
            <section>
              <h4>Setup evidence</h4>
              <pre>
                {inspected.setup_evidence
                  .map(
                    (event) =>
                      `${event.sequence}. ${event.command ?? event.stream ?? event.type}${event.message ? ` — ${event.message}` : ""}${event.exit_code !== undefined ? ` (exit ${event.exit_code})` : ""}`,
                  )
                  .join("\n")}
              </pre>
            </section>
            <section className="workspace-checkpoints">
              <header>
                <div>
                  <p className="eyebrow">Durable unfinished work</p>
                  <h4>Checkpoints</h4>
                </div>
              </header>
              {inspected.state === "ready" && actor && (
                <form
                  onSubmit={async (event) => {
                    event.preventDefault();
                    setBusy(true);
                    setError("");
                    try {
                      await send(
                        `/repositories/${repository.id}/workspaces/${inspected.id}/checkpoints`,
                        "POST",
                        {
                          summary: checkpointSummary,
                          paths: checkpointPaths
                            .split(",")
                            .map((value) => value.trim())
                            .filter(Boolean),
                          parent_id: checkpointParent,
                          reproducibility: {
                            dependencies: checkpointDependencies
                              .split(",")
                              .map((value) => value.trim())
                              .filter(Boolean),
                            commands: checkpointCommands
                              .split("\n")
                              .map((value) => value.trim())
                              .filter(Boolean),
                          },
                        },
                      );
                      setCheckpointSummary("");
                      setCheckpointPaths("");
                      await load();
                    } catch (cause) {
                      setError(
                        cause instanceof Error
                          ? cause.message
                          : "Unable to create checkpoint.",
                      );
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  <label>
                    What should a collaborator recover?
                    <input
                      required
                      value={checkpointSummary}
                      onChange={(event) =>
                        setCheckpointSummary(event.target.value)
                      }
                    />
                  </label>
                  <label>
                    Repository paths to capture
                    <input
                      required
                      placeholder="src/parser.go, tests/parser_test.go"
                      value={checkpointPaths}
                      onChange={(event) =>
                        setCheckpointPaths(event.target.value)
                      }
                    />
                  </label>
                  <label>
                    Required dependencies
                    <input
                      placeholder="go modules, postgres"
                      value={checkpointDependencies}
                      onChange={(event) =>
                        setCheckpointDependencies(event.target.value)
                      }
                    />
                  </label>
                  <label>
                    Reproduction commands
                    <textarea
                      required
                      placeholder="go test ./..."
                      value={checkpointCommands}
                      onChange={(event) =>
                        setCheckpointCommands(event.target.value)
                      }
                    />
                  </label>
                  <label>
                    Branch from checkpoint
                    <select
                      value={checkpointParent}
                      onChange={(event) =>
                        setCheckpointParent(event.target.value)
                      }
                    >
                      <option value="">Workspace base</option>
                      {inspected.checkpoints?.map((checkpoint) => (
                        <option key={checkpoint.id} value={checkpoint.id}>
                          {short(checkpoint.id)} · {checkpoint.summary}
                        </option>
                      ))}
                    </select>
                  </label>
                  <Button
                    size="sm"
                    disabled={
                      busy ||
                      !checkpointSummary ||
                      !checkpointPaths ||
                      !checkpointCommands
                    }
                  >
                    Create attributed checkpoint
                  </Button>
                  <small>
                    Only declared repository paths are captured. Credential
                    paths, secret-like content, setup outputs, and private
                    terminal data are rejected or excluded.
                  </small>
                </form>
              )}
              <ol>
                {inspected.checkpoints
                  ?.slice()
                  .reverse()
                  .map((checkpoint) => (
                    <li key={checkpoint.id}>
                      <article className="panel">
                        <header>
                          <div>
                            <strong>{checkpoint.summary}</strong>
                            <small>
                              {" "}
                              by <Actor
                                id={checkpoint.creator_id}
                                compact
                              /> ·{" "}
                              {new Date(checkpoint.created_at).toLocaleString()}
                            </small>
                          </div>
                          <Badge>
                            {checkpoint.status.reproducible
                              ? "reproducible"
                              : "attention needed"}
                          </Badge>
                        </header>
                        <p>
                          <code>{short(checkpoint.id)}</code> from{" "}
                          <code>{short(checkpoint.base_revision)}</code>
                          {checkpoint.parent_id ? (
                            <>
                              {" "}
                              · follows{" "}
                              <code>{short(checkpoint.parent_id)}</code>
                            </>
                          ) : null}
                        </p>
                        {checkpoint.status.reasons?.map((reason) => (
                          <p className="form-error" key={reason}>
                            {reason}
                          </p>
                        ))}
                        {checkpoint.status.missing_dependencies?.length > 0 && (
                          <p>
                            Missing:{" "}
                            {checkpoint.status.missing_dependencies.join(", ")}
                          </p>
                        )}
                        <ul>
                          {checkpoint.changes.map((change) => (
                            <li key={change.path}>
                              <Badge>{change.operation}</Badge>{" "}
                              <code>{change.path}</code>
                              {change.binary ? " · binary" : null}
                              {change.patch && <pre>{change.patch}</pre>}
                            </li>
                          ))}
                        </ul>
                        {inspected.state === "ready" && actor && (
                          <Button
                            size="sm"
                            variant="secondary"
                            onClick={async () => {
                              setBusy(true);
                              try {
                                await send(
                                  `/repositories/${repository.id}/workspaces/${inspected.id}/checkpoints/${checkpoint.id}/restore`,
                                  "POST",
                                  {},
                                );
                                await load();
                                if (filePath) await browse(inspected, filePath);
                              } catch (cause) {
                                setError(
                                  cause instanceof Error
                                    ? cause.message
                                    : "Checkpoint conflicts with current work.",
                                );
                              } finally {
                                setBusy(false);
                              }
                            }}
                          >
                            Restore safely
                          </Button>
                        )}
                      </article>
                    </li>
                  ))}
              </ol>
              {!inspected.checkpoints?.length && (
                <p>No checkpoint evidence yet.</p>
              )}
            </section>
            {inspected.state === "ready" && (
              <section className="workspace-ide">
                <nav aria-label="Workspace tools">
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => void browse(inspected)}
                  >
                    Files
                  </Button>
                  {inspected.definition.ports?.map((port) => (
                    <a
                      key={port.number}
                      target="workspace-preview"
                      href={`/api/repositories/${repository.id}/workspaces/${inspected.id}/preview/${port.number}/index.html`}
                    >
                      {port.label} :{port.number}
                    </a>
                  ))}
                </nav>
                <div className="workspace-file-tools">
                  <label>
                    Search files
                    <input
                      value={search}
                      onChange={(event) => setSearch(event.target.value)}
                    />
                  </label>
                  <Button
                    size="sm"
                    onClick={async () => {
                      const result = await get<{ items: WorkspaceMatch[] }>(
                        `/repositories/${repository.id}/workspaces/${inspected.id}/search?q=${encodeURIComponent(search)}`,
                      );
                      setMatches(result.items);
                    }}
                  >
                    Search
                  </Button>
                </div>
                {matches.length > 0 && (
                  <ul className="workspace-search-results">
                    {matches.map((match) => (
                      <li key={`${match.path}:${match.line}`}>
                        <button
                          onClick={() => void browse(inspected, match.path)}
                        >
                          <code>
                            {match.path}:{match.line}
                          </code>{" "}
                          {match.text}
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
                <div className="workspace-editor-layout">
                  <aside>
                    {files.map((file) => (
                      <button
                        key={file.path}
                        onClick={() => void browse(inspected, file.path)}
                      >
                        {file.directory ? "▸" : "·"} {file.path}
                      </button>
                    ))}
                  </aside>
                  <div>
                    <label>
                      Editor
                      <input
                        value={filePath}
                        onChange={(event) => setFilePath(event.target.value)}
                      />
                    </label>
                    <textarea
                      aria-label="File contents"
                      value={content}
                      onChange={(event) => setContent(event.target.value)}
                    />
                    <Button
                      size="sm"
                      disabled={!filePath}
                      onClick={async () => {
                        const updated = await send<DevelopmentWorkspace>(
                          `/repositories/${repository.id}/workspaces/${inspected.id}/files`,
                          "PUT",
                          { path: filePath, content, base_digest: fileDigest },
                        );
                        setFileDigest(updated.changes.at(-1)?.digest ?? "");
                        await load();
                      }}
                    >
                      Save file
                    </Button>
                  </div>
                </div>
                <form
                  className="workspace-terminal"
                  onSubmit={async (event) => {
                    event.preventDefault();
                    setBusy(true);
                    try {
                      const response = await send<{
                        result: {
                          stdout: string;
                          stderr: string;
                          exit_code: number;
                        };
                      }>(
                        `/repositories/${repository.id}/workspaces/${inspected.id}/commands`,
                        "POST",
                        { command },
                      );
                      setTerminal(
                        `$ ${command}\n${response.result.stdout}${response.result.stderr}\n[exit ${response.result.exit_code}]`,
                      );
                      setCommand("");
                      await load();
                    } catch (cause) {
                      setError(
                        cause instanceof Error
                          ? cause.message
                          : "Command failed.",
                      );
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  <label>
                    Terminal command
                    <input
                      value={command}
                      onChange={(event) => setCommand(event.target.value)}
                      placeholder="bun test"
                    />
                  </label>
                  <Button size="sm" disabled={busy || !command}>
                    Run
                  </Button>
                  <pre>
                    {terminal || "Commands run without network or credentials."}
                  </pre>
                </form>
                {inspected.definition.ports?.length ? (
                  <iframe
                    title="Workspace application preview"
                    name="workspace-preview"
                    sandbox="allow-scripts"
                  />
                ) : (
                  <p className="muted">
                    Declare a static preview port and path in
                    .komodo/workspaces.json.
                  </p>
                )}
              </section>
            )}
          </article>
        )}
      </div>
    </section>
  );
}

function CollaboratorWorkspace({ repository }: { repository: string }) {
  const [items, setItems] = useState<Collaborator[]>([]);
  const [handle, setHandle] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const load = useCallback(
    async () =>
      setItems(
        (
          await get<CollaboratorList>(
            `/repositories/${repository}/collaborators?per_page=100`,
          )
        ).items,
      ),
    [repository],
  );
  useEffect(() => {
    // Collaborator membership is loaded when the owner opens this workspace.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);
  async function invite(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const user = await get<UserRecord>(
        `/users/by-handle/${encodeURIComponent(handle.replace(/^@/, ""))}`,
      );
      await send(
        `/repositories/${repository}/collaborators/${user.id}`,
        "PUT",
        {},
      );
      setHandle("");
      await load();
    } catch {
      setError("No account with that handle could be invited.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="people-workspace">
      <header className="proposal-toolbar">
        <div>
          <p className="eyebrow">
            <User size={14} />
            Repository access
          </p>
          <h2>People</h2>
          <p>
            Invite a newcomer by their Kanso handle. They will find this
            repository in their workspace and can publish candidate branches
            with Git.
          </p>
        </div>
      </header>
      <form className="invite-form panel" onSubmit={invite}>
        <label>
          Contributor handle
          <div className="input-prefix">
            <span>@</span>
            <input
              required
              value={handle}
              onChange={(event) => setHandle(event.target.value)}
              placeholder="collaborator"
            />
          </div>
        </label>
        <Button type="submit" disabled={busy}>
          <Plus size={14} />
          {busy ? "Inviting…" : "Invite contributor"}
        </Button>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
      </form>
      <section
        className="people-list panel"
        aria-label="Repository contributors"
      >
        <div className="panel-title">
          <span>Contributors</span>
          <Badge tone="accent">{items.length}</Badge>
        </div>
        {items.map((item) => (
          <article key={item.user_id}>
            <span className="avatar sm">
              {item.display_name.slice(0, 2).toUpperCase()}
            </span>
            <div>
              <strong>{item.display_name}</strong>
              <small>
                @{item.handle} · can propose and publish candidate branches
              </small>
            </div>
            <button
              className="danger-button"
              onClick={async () => {
                await send(
                  `/repositories/${repository}/collaborators/${item.user_id}`,
                  "DELETE",
                );
                await load();
              }}
            >
              <Trash size={13} />
              Remove
            </button>
          </article>
        ))}
        {!items.length && (
          <p className="no-comments">
            No contributors yet. Invite the person who should publish the first
            candidate branch.
          </p>
        )}
      </section>
    </section>
  );
}

function RelationshipWorkspace({
  repository,
  actor,
}: {
  repository: Repository;
  actor: string;
}) {
  const [graph, setGraph] = useState<RelationshipGraph>();
  const [releases, setReleases] = useState<ReleaseCandidate[]>([]);
  const [plans, setPlans] = useState<EvolutionPlan[]>([]);
  const [selectedPlan, setSelectedPlan] = useState("");
  const [verificationDetails, setVerificationDetails] = useState<
    Record<string, EvolutionVerificationDetail>
  >({});
  const [creating, setCreating] = useState<
    "interface" | "dependency" | "evolution" | ""
  >("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const authorized = Boolean(actor && graph?.can_write);
  const load = useCallback(async () => {
    try {
      const [nextGraph, nextReleases, nextPlans] = await Promise.all([
        get<RelationshipGraph>(`/repositories/${repository.id}/relationships`),
        get<ReleaseList>(
          `/repositories/${repository.id}/releases?per_page=100`,
        ),
        get<EvolutionList>(`/repositories/${repository.id}/evolution-plans`),
      ]);
      setGraph(nextGraph);
      setReleases(nextReleases.items);
      setPlans(nextPlans.items);
      setSelectedPlan((current) => current || nextPlans.items[0]?.id || "");
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Relationship evidence is unavailable.",
      );
    }
  }, [repository.id]);
  useEffect(() => {
    // Relationship state follows the selected repository.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);
  async function submitInterface(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const data = new FormData(event.currentTarget);
    try {
      await send(`/repositories/${repository.id}/interfaces`, "POST", {
        name: data.get("name"),
        version: data.get("version"),
        release_id: data.get("release"),
        schema_path: data.get("schema"),
      });
      setCreating("");
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Interface could not be published.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function submitDependency(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const data = new FormData(event.currentTarget);
    const release = String(data.get("release") || "");
    try {
      await send(`/repositories/${repository.id}/dependencies`, "POST", {
        provider_repository_id: data.get("provider"),
        interface_name: data.get("name"),
        constraint: data.get("constraint"),
        release_id: release || undefined,
        commit_id: release ? undefined : data.get("commit"),
      });
      setCreating("");
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Dependency could not be declared.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function submitEvolution(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    const data = new FormData(event.currentTarget);
    try {
      const plan = await send<EvolutionPlan>(
        `/repositories/${repository.id}/evolution-plans`,
        "POST",
        {
          interface_name: data.get("name"),
          source_kind: data.get("source_kind"),
          source_id: data.get("source_id"),
          candidate_commit_id: data.get("commit"),
          candidate_schema_path: data.get("schema"),
        },
      );
      setCreating("");
      await load();
      setSelectedPlan(plan.id);
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Evolution plan could not be created.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function saveContract(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const plan = plans.find((item) => item.id === selectedPlan);
    if (!plan) return;
    setBusy(true);
    const data = new FormData(event.currentTarget);
    const steps = String(data.get("steps") || "")
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean);
    try {
      await send(
        `/repositories/${repository.id}/evolution-plans/${plan.id}/contract`,
        "PUT",
        {
          strategy: data.get("strategy"),
          changes: [
            {
              classification: data.get("classification"),
              area: data.get("area"),
              summary: data.get("change"),
              rationale: data.get("rationale"),
            },
          ],
          steps: steps.map((line) => {
            const at = line.indexOf(":");
            return {
              owner_id: at > 0 ? line.slice(0, at).trim() : repository.owner_id,
              summary: at > 0 ? line.slice(at + 1).trim() : line,
            };
          }),
          exceptions: String(data.get("exceptions") || "")
            .split("\n")
            .map((reason) => reason.trim())
            .filter(Boolean)
            .map((reason) => ({ reason })),
        },
      );
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Migration contract could not be saved.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function acknowledge(
    plan: EvolutionPlan,
    decision: "acknowledge" | "request_changes",
  ) {
    try {
      await send(
        `/repositories/${repository.id}/evolution-plans/${plan.id}/acknowledgements`,
        "POST",
        {
          decision,
          note:
            decision === "acknowledge"
              ? "Reviewed blast radius and migration contract."
              : "Migration contract needs revision.",
        },
      );
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Acknowledgement was not accepted.",
      );
    }
  }
  async function delegate(plan: EvolutionPlan) {
    try {
      const result = await send<{ worker_credential: string }>(
        `/repositories/${repository.id}/evolution-plans/${plan.id}/analyses`,
        "POST",
        {
          agent: "codex",
          mandate:
            "Analyze selected interface and consumer snapshots; record evidence, risks, and uncertainty.",
          repository_ids: [
            repository.id,
            ...plan.affected_consumers.map((item) => item.repository_id),
          ],
        },
      );
      setNotice(
        `Read-only credential (shown once): ${result.worker_credential}`,
      );
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Analysis could not be delegated.",
      );
    }
  }
  async function verifyEvolution(plan: EvolutionPlan) {
    const tasks = plan.tasks.filter((task) => task.work?.pull_request_id);
    if (tasks.length < 2) {
      setError(
        "Publish provider and consumer task pull requests before verification.",
      );
      return;
    }
    setBusy(true);
    try {
      await send(
        `/repositories/${repository.id}/evolution-plans/${plan.id}/verifications`,
        "POST",
        {
          task_ids: [...tasks]
            .sort(
              (a, b) =>
                Number(b.target_repository_id === repository.id) -
                Number(a.target_repository_id === repository.id),
            )
            .map((task) => task.id),
        },
      );
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Compatibility verification could not start.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function inspectVerification(plan: EvolutionPlan, id: string) {
    try {
      const detail = await get<EvolutionVerificationDetail>(
        `/repositories/${repository.id}/evolution-plans/${plan.id}/verifications/${id}`,
      );
      setVerificationDetails((current) => ({ ...current, [id]: detail }));
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Verification evidence is unavailable.",
      );
    }
  }
  async function configureRollout(plan: EvolutionPlan) {
    const attested = [...plan.verifications]
      .reverse()
      .find((attempt) => verificationDetails[attempt.id]?.attested);
    if (!attested) {
      setError(
        "Inspect a current passing compatibility matrix before defining rollout phases.",
      );
      return;
    }
    const consumers = plan.affected_consumers.map((item) => item.repository_id);
    try {
      await send(
        `/repositories/${repository.id}/evolution-plans/${plan.id}/rollout`,
        "PUT",
        {
          verification_id: attested.id,
          phases: [
            ...consumers.map((id) => ({
              name: `Make ${names.get(id) ?? short(id)} compatible`,
              compatibility_gates: ["contract", "integration"],
              steps: ["queue", "release", "deployment"].map((kind) => ({
                repository_id: id,
                kind,
              })),
            })),
            {
              name: "Provider cutover",
              compatibility_gates: ["contract", "integration"],
              steps: ["queue", "release", "deployment"].map((kind) => ({
                repository_id: repository.id,
                kind,
              })),
            },
          ],
        },
      );
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Rollout could not be configured.",
      );
    }
  }
  async function decidePhase(
    plan: EvolutionPlan,
    phase: NonNullable<EvolutionPlan["rollout"]>["phases"][number],
    repositoryID: string,
    decision: "approve" | "reject",
  ) {
    try {
      await send(
        `/repositories/${repository.id}/evolution-plans/${plan.id}/rollout/phases/${phase.id}/approvals`,
        "POST",
        {
          repository_id: repositoryID,
          decision,
          note:
            decision === "approve"
              ? "Repository window and authority confirmed."
              : "Repository is not ready to participate.",
        },
      );
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Rollout decision was not accepted.",
      );
    }
  }
  async function linkOutcome(
    plan: EvolutionPlan,
    phaseID: string,
    step: NonNullable<
      EvolutionPlan["rollout"]
    >["phases"][number]["steps"][number],
  ) {
    const resource = window.prompt(
      `Link the existing ${step.kind} resource ID for ${names.get(step.repository_id) ?? short(step.repository_id)}:`,
    );
    if (!resource) return;
    try {
      await send(
        `/repositories/${repository.id}/evolution-plans/${plan.id}/rollout/phases/${phaseID}/steps/${step.id}/outcomes`,
        "POST",
        {
          resource_id: resource,
          note: "Linked from the established workflow; state derived by the platform.",
        },
      );
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Outcome could not be linked.",
      );
    }
  }
  const names = new Map(
    graph?.nodes.map((node) => [node.repository_id, node.name]),
  );
  return (
    <section className="relationship-workspace">
      <header className="release-heading">
        <div>
          <p className="eyebrow">Cross-repository evidence</p>
          <h2>Interface relationships</h2>
          <p>
            Inspect released contracts, then agree on blast radius and migration
            before incompatible code merges.
          </p>
        </div>
        {authorized && (
          <div className="relationship-actions">
            <Button
              variant="secondary"
              onClick={() =>
                setCreating(creating === "dependency" ? "" : "dependency")
              }
            >
              <Plus size={14} />
              Declare dependency
            </Button>
            <Button
              variant="secondary"
              onClick={() =>
                setCreating(creating === "evolution" ? "" : "evolution")
              }
            >
              <Plus size={14} />
              Plan evolution
            </Button>
            <Button
              onClick={() =>
                setCreating(creating === "interface" ? "" : "interface")
              }
            >
              <Plus size={14} />
              Publish interface
            </Button>
          </div>
        )}
      </header>
      {creating === "interface" && (
        <form className="panel relationship-form" onSubmit={submitInterface}>
          <label>
            Interface name
            <input name="name" required placeholder="payments-api" />
          </label>
          <label>
            Semantic version
            <input
              name="version"
              required
              placeholder="1.2.0"
              pattern="v?[0-9]+\.[0-9]+\.[0-9]+"
            />
          </label>
          <label>
            Release
            <select name="release" required>
              <option value="">Select exact release…</option>
              {releases.map((item) => (
                <option value={item.id} key={item.id}>
                  {item.version} · {short(item.commit_id)}
                </option>
              ))}
            </select>
          </label>
          <label>
            Schema path (optional)
            <input name="schema" placeholder="api/openapi.yaml" />
          </label>
          <div className="release-form-actions">
            <Button disabled={busy}>
              {busy ? "Publishing…" : "Publish version"}
            </Button>
          </div>
        </form>
      )}
      {creating === "dependency" && (
        <form className="panel relationship-form" onSubmit={submitDependency}>
          <label>
            Provider repository ID
            <input name="provider" required />
          </label>
          <label>
            Interface name
            <input name="name" required placeholder="payments-api" />
          </label>
          <label>
            Compatibility constraint
            <input name="constraint" required placeholder="^1.2.0" />
          </label>
          <label>
            Consumer release
            <select name="release">
              <option value="">Unreleased exact commit</option>
              {releases.map((item) => (
                <option value={item.id} key={item.id}>
                  {item.version} · {short(item.commit_id)}
                </option>
              ))}
            </select>
          </label>
          <label>
            Exact commit when unreleased
            <input name="commit" placeholder="40-character object ID" />
          </label>
          <div className="release-form-actions">
            <Button disabled={busy}>
              {busy ? "Declaring…" : "Declare dependency"}
            </Button>
          </div>
        </form>
      )}
      {creating === "evolution" && (
        <form className="panel relationship-form" onSubmit={submitEvolution}>
          <label>
            Published interface
            <input name="name" required placeholder="payments-api" />
          </label>
          <label>
            Provider source
            <select name="source_kind">
              <option value="proposal">Proposal</option>
              <option value="pull_request">Pull request</option>
            </select>
          </label>
          <label>
            Proposal or pull request ID
            <input name="source_id" required />
          </label>
          <label>
            Candidate commit (proposal)
            <input name="commit" placeholder="Exact candidate revision" />
          </label>
          <label>
            Candidate schema path
            <input name="schema" required placeholder="api/openapi.yaml" />
          </label>
          <div className="release-form-actions">
            <Button disabled={busy}>
              {busy ? "Comparing…" : "Create evolution plan"}
            </Button>
          </div>
        </form>
      )}
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      {notice && (
        <p className="panel evolution-credential" role="status">
          {notice}
        </p>
      )}
      {graph && (
        <>
          <div className="relationship-summary">
            {(
              [
                "repositories",
                "relationships",
                "resolved",
                "stale",
                "unresolved",
              ] as const
            ).map((key) => (
              <article className="panel" key={key}>
                <strong>{graph.summary[key]}</strong>
                <span>{key}</span>
              </article>
            ))}
          </div>
          <div className="relationship-graph">
            {graph.edges.map((edge) => (
              <article
                className={`panel relationship-edge ${edge.status}`}
                key={edge.dependency.id}
              >
                <header>
                  <Badge
                    tone={edge.status === "resolved" ? "accent" : "neutral"}
                  >
                    {edge.status}
                  </Badge>
                  <span>
                    <Link
                      href={`/repositories/${edge.dependency.repository_id}?view=relationships`}
                    >
                      {names.get(edge.dependency.repository_id) ??
                        short(edge.dependency.repository_id)}
                    </Link>
                    <small>
                      owned by{" "}
                      <Actor
                        id={
                          graph.nodes.find(
                            (node) =>
                              node.repository_id ===
                              edge.dependency.repository_id,
                          )?.owner_id ?? ""
                        }
                      />
                    </small>
                  </span>
                  <ChevronRight />
                  <span>
                    <Link
                      href={`/repositories/${edge.dependency.provider_repository_id}?view=relationships`}
                    >
                      {names.get(edge.dependency.provider_repository_id) ??
                        short(edge.dependency.provider_repository_id)}
                    </Link>
                    <small>
                      owned by{" "}
                      <Actor
                        id={
                          graph.nodes.find(
                            (node) =>
                              node.repository_id ===
                              edge.dependency.provider_repository_id,
                          )?.owner_id ?? ""
                        }
                      />
                    </small>
                  </span>
                </header>
                <div className="relationship-contract">
                  <strong>
                    {edge.dependency.interface_name}{" "}
                    <code>{edge.dependency.constraint}</code>
                  </strong>
                  <span>
                    consumer <code>{short(edge.dependency.commit_id)}</code>
                    {edge.consumer_release && (
                      <> · release {edge.consumer_release.version}</>
                    )}
                  </span>
                  <span>
                    {edge.provider ? (
                      <>
                        resolved to <b>{edge.provider.version}</b> at{" "}
                        <code>{short(edge.provider.commit_id)}</code>
                      </>
                    ) : (
                      "No compatible published version"
                    )}
                  </span>
                </div>
                {edge.reasons.length > 0 && (
                  <ul>
                    {edge.reasons.map((reason) => (
                      <li key={reason}>{reason.replaceAll("_", " ")}</li>
                    ))}
                  </ul>
                )}
                <div className="relationship-environments">
                  <span>
                    <b>Consumer environments</b>
                    {edge.consumer_environments.length ? (
                      edge.consumer_environments.map((env) => (
                        <small key={env.id}>
                          {env.name || short(env.id)} · {env.state} ·{" "}
                          {short(env.commit_id)}
                        </small>
                      ))
                    ) : (
                      <small>No deployment evidence</small>
                    )}
                  </span>
                  <span>
                    <b>Provider environments</b>
                    {edge.provider_environments.length ? (
                      edge.provider_environments.map((env) => (
                        <small key={env.id}>
                          {env.name || short(env.id)} · {env.state} ·{" "}
                          {short(env.commit_id)}
                        </small>
                      ))
                    ) : (
                      <small>No deployment evidence</small>
                    )}
                  </span>
                </div>
                <footer>
                  Declared by <Actor id={edge.dependency.declared_by_id} /> ·{" "}
                  {new Date(edge.dependency.declared_at).toLocaleString()}
                </footer>
              </article>
            ))}
            {!graph.edges.length && (
              <article className="panel release-empty">
                <Book />
                <h3>No relationships declared</h3>
                <p>
                  Publish an interface from a release or declare this
                  repository&apos;s dependency on another owner&apos;s contract.
                </p>
              </article>
            )}
          </div>
        </>
      )}
      {plans.length > 0 && (
        <section className="evolution-plans">
          <header>
            <div>
              <p className="eyebrow">Shared migration decisions</p>
              <h3>Evolution plans</h3>
            </div>
            <select
              value={selectedPlan}
              onChange={(event) => setSelectedPlan(event.target.value)}
            >
              {plans.map((plan) => (
                <option value={plan.id} key={plan.id}>
                  {plan.interface_name} · {plan.source_kind.replace("_", " ")}
                </option>
              ))}
            </select>
          </header>
          {plans
            .filter((plan) => plan.id === selectedPlan)
            .map((plan) => (
              <article className="panel evolution-plan" key={plan.id}>
                <div className="evolution-comparison">
                  <span>
                    <small>Released predecessor</small>
                    <strong>{plan.predecessor.version}</strong>
                    <code>{short(plan.predecessor.commit_id)}</code>
                    <small>
                      {plan.predecessor.schema_path} ·{" "}
                      {short(plan.predecessor_schema_sha256)}
                    </small>
                  </span>
                  <ChevronRight />
                  <span>
                    <small>
                      Candidate from {plan.source_kind.replace("_", " ")}
                    </small>
                    <strong>{plan.interface_name}</strong>
                    <code>{short(plan.candidate_commit_id)}</code>
                    <small>
                      {plan.candidate_schema_path} ·{" "}
                      {short(plan.candidate_schema_sha256)}
                    </small>
                  </span>
                </div>
                <div className="evolution-blast">
                  <h4>Affected consumers and owners</h4>
                  {plan.affected_consumers.map((consumer) => (
                    <span key={consumer.dependency_id}>
                      <Link
                        href={`/repositories/${consumer.repository_id}?view=relationships`}
                      >
                        {names.get(consumer.repository_id) ??
                          short(consumer.repository_id)}
                      </Link>
                      <Actor id={consumer.owner_id} />
                      <code>{consumer.constraint}</code>
                    </span>
                  ))}
                  {!plan.affected_consumers.length && (
                    <p>No declared consumers are affected.</p>
                  )}
                </div>
                {authorized && (
                  <form
                    className="evolution-contract"
                    key={plan.updated_at}
                    onSubmit={saveContract}
                  >
                    <label>
                      Migration strategy
                      <textarea
                        name="strategy"
                        required
                        defaultValue={plan.strategy}
                        placeholder="Describe rollout, dual-support window, and completion criteria."
                      />
                    </label>
                    <div>
                      <label>
                        Classification
                        <select
                          name="classification"
                          defaultValue={
                            plan.changes[0]?.classification ?? "breaking"
                          }
                        >
                          <option value="breaking">Breaking</option>
                          <option value="compatible">Compatible</option>
                          <option value="behavioral">Behavioral</option>
                          <option value="unknown">Unknown</option>
                        </select>
                      </label>
                      <label>
                        Area
                        <input
                          name="area"
                          required
                          defaultValue={plan.changes[0]?.area}
                          placeholder="request schema"
                        />
                      </label>
                    </div>
                    <label>
                      Change summary
                      <input
                        name="change"
                        required
                        defaultValue={plan.changes[0]?.summary}
                      />
                    </label>
                    <label>
                      Rationale
                      <textarea
                        name="rationale"
                        defaultValue={plan.changes[0]?.rationale}
                      />
                    </label>
                    <label>
                      Ordered steps{" "}
                      <small>one per line as owner-id: outcome</small>
                      <textarea
                        name="steps"
                        defaultValue={plan.steps
                          .map((step) => `${step.owner_id}: ${step.summary}`)
                          .join("\n")}
                      />
                    </label>
                    <label>
                      Exceptions <small>one reason per line</small>
                      <textarea
                        name="exceptions"
                        defaultValue={plan.exceptions
                          .map((item) => item.reason)
                          .join("\n")}
                      />
                    </label>
                    <div className="relationship-actions">
                      <Button disabled={busy}>Save migration contract</Button>
                      <Button
                        type="button"
                        variant="secondary"
                        onClick={() => void delegate(plan)}
                      >
                        <Sparkles size={14} />
                        Delegate read-only analysis
                      </Button>
                    </div>
                  </form>
                )}
                <div className="evolution-decisions">
                  <h4>Agreement</h4>
                  <div className="relationship-actions">
                    {actor && (
                      <>
                        <Button
                          variant="secondary"
                          onClick={() => void acknowledge(plan, "acknowledge")}
                        >
                          <Check size={14} />
                          Acknowledge
                        </Button>
                        <Button
                          variant="secondary"
                          onClick={() =>
                            void acknowledge(plan, "request_changes")
                          }
                        >
                          Request changes
                        </Button>
                      </>
                    )}
                  </div>
                  {plan.acknowledgements.map((item) => (
                    <p key={item.actor_id}>
                      <Actor id={item.actor_id} />{" "}
                      <b>{item.decision.replace("_", " ")}</b> for{" "}
                      {item.owner_for_repository_ids.length} owned repository
                      {item.owner_for_repository_ids.length === 1
                        ? ""
                        : "ies"}: {item.note}
                    </p>
                  ))}
                </div>
                {plan.findings.length > 0 && (
                  <div className="evolution-findings">
                    <h4>Agent findings and uncertainty</h4>
                    {plan.findings.map((item) => (
                      <blockquote key={item.id}>
                        <b>{item.kind}</b> · {item.body}
                        {item.uncertainty && (
                          <small>Uncertainty: {item.uncertainty}</small>
                        )}
                        <footer>
                          {item.actor_id} ·{" "}
                          {item.repository_ids.map(short).join(", ")}
                        </footer>
                      </blockquote>
                    ))}
                  </div>
                )}
                <div className="evolution-findings">
                  <header className="release-heading">
                    <div>
                      <h4>Compatibility matrix</h4>
                      <p>
                        Evidence stays bound to exact linked pull request
                        revisions.
                      </p>
                    </div>
                    {authorized && (
                      <Button
                        type="button"
                        disabled={busy}
                        onClick={() => void verifyEvolution(plan)}
                      >
                        <Check size={14} />
                        Verify current combination
                      </Button>
                    )}
                  </header>
                  {plan.verifications.map((attempt) => {
                    const detail = verificationDetails[attempt.id];
                    return (
                      <article className="panel" key={attempt.id}>
                        <header>
                          <Badge tone={detail?.attested ? "accent" : "neutral"}>
                            {detail
                              ? detail.attested
                                ? "attested"
                                : detail.current
                                  ? "not passing"
                                  : "superseded"
                              : "inspect evidence"}
                          </Badge>
                          <span>
                            {attempt.revisions.length} repositories ·{" "}
                            {attempt.run_ids.length} checks
                          </span>
                          <Button
                            type="button"
                            variant="secondary"
                            onClick={() =>
                              void inspectVerification(plan, attempt.id)
                            }
                          >
                            Refresh evidence
                          </Button>
                        </header>
                        <div className="relationship-contract">
                          {attempt.revisions.map((revision) => (
                            <span key={revision.repository_id}>
                              <b>
                                {names.get(revision.repository_id) ??
                                  short(revision.repository_id)}
                              </b>{" "}
                              <code>{short(revision.commit_id)}</code> · pull{" "}
                              <code>{short(revision.pull_request_id)}</code>
                            </span>
                          ))}
                        </div>
                        {detail?.runs.map((run) => (
                          <div key={run.id}>
                            <Badge
                              tone={
                                run.state === "succeeded" ? "accent" : "neutral"
                              }
                            >
                              {run.state}
                            </Badge>{" "}
                            <b>
                              {run.definition.kind} · {run.definition.name}
                            </b>
                            {run.events
                              .filter((event) => event.type === "log")
                              .map((event) => (
                                <pre key={event.sequence}>
                                  {event.stream}: {event.message}
                                </pre>
                              ))}
                            {run.events
                              .flatMap((event) =>
                                event.artifact ? [event.artifact] : [],
                              )
                              .map((artifact) => (
                                <a
                                  key={artifact.id}
                                  href={`/api/repositories/${repository.id}/evolution-plans/${plan.id}/verifications/${attempt.id}/runs/${run.id}/artifacts/${artifact.id}`}
                                >
                                  {artifact.path} · {short(artifact.sha256)}
                                </a>
                              ))}
                          </div>
                        ))}
                      </article>
                    );
                  })}
                  {!plan.verifications.length && (
                    <p>
                      No exact cross-repository combination has been tested yet.
                    </p>
                  )}
                </div>
                <div className="evolution-rollout">
                  <header className="release-heading">
                    <div>
                      <h4>Governed rollout</h4>
                      <p>
                        Owner-approved phases connect compatibility evidence to
                        existing queues, releases, environments, and recovery.
                      </p>
                    </div>
                    {!plan.rollout && repository.owner_id === actor && (
                      <Button
                        type="button"
                        disabled={busy}
                        onClick={() => void configureRollout(plan)}
                      >
                        Define rollout phases
                      </Button>
                    )}
                    {plan.rollout && (
                      <Badge
                        tone={
                          plan.rollout.state === "completed"
                            ? "accent"
                            : "neutral"
                        }
                      >
                        {plan.rollout.state.replace("_", " ")}
                      </Badge>
                    )}
                  </header>
                  {plan.rollout?.phases.map((phase) => {
                    const owned = [
                      ...new Set(
                        phase.steps
                          .filter((step) =>
                            step.repository_id === repository.id
                              ? repository.owner_id === actor
                              : plan.affected_consumers.some(
                                  (item) =>
                                    item.repository_id === step.repository_id &&
                                    item.owner_id === actor,
                                ),
                          )
                          .map((step) => step.repository_id),
                      ),
                    ];
                    const next = phase.steps.find(
                      (step) =>
                        step.state === "pending" || step.state === "failed",
                    );
                    return (
                      <article className="panel rollout-phase" key={phase.id}>
                        <header>
                          <span>
                            <Badge
                              tone={
                                phase.state === "completed"
                                  ? "accent"
                                  : "neutral"
                              }
                            >
                              {phase.state}
                            </Badge>{" "}
                            <b>
                              {phase.position}. {phase.name}
                            </b>
                          </span>
                          <small>
                            Gates: {phase.compatibility_gates.join(", ")}
                          </small>
                        </header>
                        <div className="rollout-owners">
                          {[
                            ...new Set(
                              phase.steps.map((step) => step.repository_id),
                            ),
                          ].map((id) => {
                            const approval = phase.approvals.find(
                              (item) => item.repository_id === id,
                            );
                            return (
                              <span key={id}>
                                <b>{names.get(id) ?? short(id)}</b>
                                <Badge
                                  tone={
                                    approval?.decision === "approve"
                                      ? "accent"
                                      : "neutral"
                                  }
                                >
                                  {approval?.decision ??
                                    "owner approval needed"}
                                </Badge>
                                {owned.includes(id) && (
                                  <>
                                    <Button
                                      type="button"
                                      variant="secondary"
                                      onClick={() =>
                                        void decidePhase(
                                          plan,
                                          phase,
                                          id,
                                          "approve",
                                        )
                                      }
                                    >
                                      Approve
                                    </Button>
                                    <Button
                                      type="button"
                                      variant="secondary"
                                      onClick={() =>
                                        void decidePhase(
                                          plan,
                                          phase,
                                          id,
                                          "reject",
                                        )
                                      }
                                    >
                                      Pause
                                    </Button>
                                  </>
                                )}
                              </span>
                            );
                          })}
                        </div>
                        <div className="rollout-steps">
                          {phase.steps.map((step) => (
                            <span key={step.id}>
                              <Badge
                                tone={
                                  step.state === "succeeded" ||
                                  step.state === "rolled_back"
                                    ? "accent"
                                    : "neutral"
                                }
                              >
                                {step.state}
                              </Badge>
                              <b>{step.kind}</b>
                              <small>
                                {names.get(step.repository_id) ??
                                  short(step.repository_id)}
                              </small>
                              {step.resource_id && (
                                <code>{short(step.resource_id)}</code>
                              )}
                              {owned.includes(step.repository_id) &&
                                (phase.state === "ready" ||
                                  phase.state === "running" ||
                                  phase.state === "paused") && (
                                  <Button
                                    type="button"
                                    variant="secondary"
                                    onClick={() =>
                                      void linkOutcome(plan, phase.id, step)
                                    }
                                  >
                                    Link outcome
                                  </Button>
                                )}
                            </span>
                          ))}
                        </div>
                        {next && (
                          <p className="rollout-next">
                            <b>Next:</b>{" "}
                            {phase.state === "paused"
                              ? `Recover ${names.get(next.repository_id) ?? short(next.repository_id)} through rollback or agent repair.`
                              : phase.state === "pending" ||
                                  phase.state === "blocked"
                                ? "Waiting for repository owners and prior phases."
                                : `Execute ${next.kind} for ${names.get(next.repository_id) ?? short(next.repository_id)}.`}
                          </p>
                        )}
                        {phase.outcomes.length > 0 && (
                          <details>
                            <summary>
                              Retained outcomes ({phase.outcomes.length})
                            </summary>
                            {phase.outcomes.map((outcome, index) => (
                              <p key={`${outcome.step_id}-${index}`}>
                                <Actor id={outcome.actor_id} /> recorded{" "}
                                <b>
                                  {outcome.kind} {outcome.state}
                                </b>{" "}
                                · <code>{short(outcome.resource_id)}</code>
                                {outcome.note && ` · ${outcome.note}`}
                              </p>
                            ))}
                          </details>
                        )}
                      </article>
                    );
                  })}
                  {!plan.rollout && (
                    <p>
                      No rollout is defined. A passing current matrix is
                      required before the provider owner can sequence
                      participation.
                    </p>
                  )}
                </div>
              </article>
            ))}
        </section>
      )}
    </section>
  );
}

function ReleaseWorkspace({
  repository,
  branches,
  actor,
  selected,
}: {
  repository: Repository;
  branches: BranchRecord[];
  actor: string;
  selected?: string;
}) {
  const [items, setItems] = useState<ReleaseCandidate[]>([]);
  const [current, setCurrent] = useState<ReleaseCandidate>();
  const [creating, setCreating] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [attestation, setAttestation] = useState<ReleaseAttestation>();
  const [packages, setPackages] = useState<PackageVersion[]>([]);
  const [publishingPackage, setPublishingPackage] = useState(false);
  const [rerunning, setRerunning] = useState("");
  const [environments, setEnvironments] = useState<DeliveryEnvironment[]>([]);
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [deliveryBusy, setDeliveryBusy] = useState("");
  const authorized =
    actor === repository.owner_id ||
    repository.collaborator_ids?.includes(actor);
  const load = useCallback(async () => {
    try {
      const result = await get<ReleaseList>(
        `/repositories/${repository.id}/releases?per_page=100`,
      );
      setItems(result.items);
      setCurrent(
        result.items.find((item) => item.id === selected) ?? result.items[0],
      );
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Releases unavailable.",
      );
    }
  }, [repository.id, selected]);
  useEffect(() => {
    // Release state follows the shareable selected-release URL.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);
  const loadAttestation = useCallback(async () => {
    if (!current) {
      setAttestation(undefined);
      return;
    }
    try {
      setAttestation(
        await get<ReleaseAttestation>(
          `/repositories/${repository.id}/releases/${current.id}/attestation`,
        ),
      );
    } catch {
      setError("Build attestation unavailable.");
    }
  }, [current, repository.id]);
  useEffect(() => {
    // Build attempts are durable server state; poll only while work is active.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadAttestation();
    if (
      !attestation?.attempts.some(
        (run) => run.state === "queued" || run.state === "running",
      )
    )
      return;
    const timer = window.setInterval(() => void loadAttestation(), 1500);
    return () => window.clearInterval(timer);
  }, [loadAttestation, attestation?.attempts]);
  const loadPackages = useCallback(async () => {
    try {
      const result = await get<PackageList>(
        `/repositories/${repository.id}/packages?per_page=100`,
      );
      setPackages(result.items);
    } catch {
      setError("Published packages unavailable.");
    }
  }, [repository.id]);
  useEffect(() => {
    // Package versions are immutable durable records.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadPackages();
  }, [loadPackages]);
  const loadDelivery = useCallback(async () => {
    try {
      const [environmentData, deploymentData] = await Promise.all([
        get<EnvironmentList>(`/repositories/${repository.id}/environments`),
        get<DeploymentList>(`/repositories/${repository.id}/deployments`),
      ]);
      setEnvironments(environmentData.items);
      setDeployments(deploymentData.items);
    } catch {
      setError("Delivery state unavailable.");
    }
  }, [repository.id]);
  useEffect(() => {
    // Delivery records are durable server state and may advance asynchronously.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadDelivery();
    const timer = window.setInterval(() => void loadDelivery(), 2000);
    return () => window.clearInterval(timer);
  }, [loadDelivery]);
  async function rerun(run: ReleaseBuild) {
    setRerunning(run.id);
    setError("");
    try {
      await send(
        `/repositories/${repository.id}/releases/${current?.id}/builds/${run.id}/rerun`,
        "POST",
        {},
      );
      await loadAttestation();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Could not rerun build.",
      );
    } finally {
      setRerunning("");
    }
  }
  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setPending(true);
    setError("");
    const data = new FormData(event.currentTarget);
    try {
      const item = await send<ReleaseCandidate>(
        `/repositories/${repository.id}/releases`,
        "POST",
        {
          version: data.get("version"),
          notes: data.get("notes"),
          commit_id: data.get("commit"),
          prior_release_id: data.get("prior") || "",
        },
      );
      setCreating(false);
      await load();
      setCurrent(item);
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Could not define release.",
      );
    } finally {
      setPending(false);
    }
  }
  async function saveEnvironment(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setDeliveryBusy("environment");
    setError("");
    const data = new FormData(event.currentTarget);
    const configuration = Object.fromEntries(
      String(data.get("configuration") || "")
        .split("\n")
        .filter(Boolean)
        .map((line) => {
          const at = line.indexOf("=");
          return at < 1
            ? [line.trim(), ""]
            : [line.slice(0, at).trim(), line.slice(at + 1)];
        }),
    );
    const secrets = Object.fromEntries(
      String(data.get("secrets") || "")
        .split("\n")
        .filter(Boolean)
        .map((line) => {
          const at = line.indexOf("=");
          return at < 1
            ? [line.trim(), ""]
            : [line.slice(0, at).trim(), line.slice(at + 1)];
        }),
    );
    try {
      await send(`/repositories/${repository.id}/environments`, "POST", {
        name: data.get("name"),
        position: Number(data.get("position")),
        command: data.get("command"),
        configuration,
        secrets,
        required_approvals: Number(data.get("approvals")),
        concurrency: Number(data.get("concurrency")),
      });
      event.currentTarget.reset();
      await loadDelivery();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Could not save environment.",
      );
    } finally {
      setDeliveryBusy("");
    }
  }
  async function publishPackage(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!current) return;
    setPublishingPackage(true);
    setError("");
    const data = new FormData(event.currentTarget);
    const [buildRunID, artifactID] = String(data.get("artifact")).split(":");
    const dependencies = Object.fromEntries(
      String(data.get("dependencies") || "")
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean)
        .map((line) => {
          const at = line.lastIndexOf("@");
          return at > 0 ? [line.slice(0, at), line.slice(at + 1)] : [line, "*"];
        }),
    );
    try {
      await send(`/repositories/${repository.id}/packages`, "POST", {
        name: data.get("name"),
        version: data.get("version"),
        release_id: current.id,
        build_run_id: buildRunID,
        artifact_id: artifactID,
        platform: {
          os: data.get("os"),
          arch: data.get("arch"),
          runtime: data.get("runtime"),
        },
        dependencies,
        documentation: data.get("documentation"),
        visibility: data.get("visibility"),
      });
      event.currentTarget.reset();
      await loadPackages();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Package publication failed.",
      );
    } finally {
      setPublishingPackage(false);
    }
  }
  async function promote(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!current) return;
    setDeliveryBusy("promote");
    const data = new FormData(event.currentTarget);
    const [run, artifact] = String(data.get("artifact")).split(":");
    try {
      await send(`/repositories/${repository.id}/deployments`, "POST", {
        environment_id: data.get("environment"),
        release_id: current.id,
        build_run_id: run,
        artifact_id: artifact,
      });
      await loadDelivery();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Promotion was not accepted.",
      );
    } finally {
      setDeliveryBusy("");
    }
  }
  async function approve(deployment: Deployment) {
    setDeliveryBusy(deployment.id);
    try {
      await send(
        `/repositories/${repository.id}/deployments/${deployment.id}/approvals`,
        "POST",
        {},
      );
      await loadDelivery();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Approval was not accepted.",
      );
    } finally {
      setDeliveryBusy("");
    }
  }
  async function control(
    deployment: Deployment,
    action: "pause" | "resume" | "cancel" | "fail",
  ) {
    setDeliveryBusy(deployment.id);
    try {
      await send(
        `/repositories/${repository.id}/deployments/${deployment.id}/control`,
        "POST",
        {
          action,
          reason:
            action === "fail"
              ? "Collaborator marked the observed rollout unhealthy."
              : `Collaborator requested rollout ${action}.`,
        },
      );
      await loadDelivery();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Rollout decision was not accepted.",
      );
    } finally {
      setDeliveryBusy("");
    }
  }
  async function recover(
    deployment: Deployment,
    action: "rollback" | "repair",
  ) {
    setDeliveryBusy(`${deployment.id}:${action}`);
    setError("");
    try {
      const result = await send<{ pull_request?: { id: string } }>(
        `/repositories/${repository.id}/deployments/${deployment.id}/recovery`,
        "POST",
        { action, context_paths: [".komodo"] },
      );
      if (result.pull_request) {
        window.location.assign(
          `/repositories/${repository.id}?view=pulls&pull=${result.pull_request.id}&section=sessions`,
        );
        return;
      }
      await loadDelivery();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Recovery could not be started.",
      );
    } finally {
      setDeliveryBusy("");
    }
  }
  return (
    <div className="release-workspace">
      <header className="release-heading">
        <div>
          <p className="eyebrow">Immutable delivery candidates</p>
          <h2>Releases</h2>
          <p>
            Capture exactly what will ship and the integrated work it contains
            before build or promotion begins.
          </p>
        </div>
        {authorized && (
          <Button onClick={() => setCreating((value) => !value)}>
            <Plus size={14} />
            Define release
          </Button>
        )}
      </header>
      {creating && (
        <form className="panel release-form" onSubmit={submit}>
          <label>
            Version
            <input
              name="version"
              required
              maxLength={100}
              placeholder="v1.0.0"
            />
          </label>
          <label>
            Exact repository state
            <select
              name="commit"
              required
              defaultValue={
                branches.find((branch) => branch.is_default)?.commit_id ??
                branches[0]?.commit_id
              }
            >
              {branches.map((branch) => (
                <option value={branch.commit_id} key={branch.name}>
                  {branch.name} · {short(branch.commit_id)}
                </option>
              ))}
            </select>
          </label>
          <label>
            Compare since
            <select name="prior" defaultValue="">
              <option value="">Beginning of history</option>
              {items.map((item) => (
                <option value={item.id} key={item.id}>
                  {item.version} · {short(item.commit_id)}
                </option>
              ))}
            </select>
          </label>
          <label className="release-notes">
            Release notes
            <textarea
              name="notes"
              rows={5}
              maxLength={65536}
              placeholder="Explain why this state is ready to deliver and what participants should know."
            />
          </label>
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          <div className="release-form-actions">
            <Button
              variant="secondary"
              type="button"
              onClick={() => setCreating(false)}
            >
              Cancel
            </Button>
            <Button disabled={pending}>
              {pending ? "Capturing…" : "Capture candidate"}
            </Button>
          </div>
        </form>
      )}
      {!creating && error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      {items.length === 0 && !creating ? (
        <section className="panel release-empty">
          <Sparkles />
          <h3>No release candidates yet</h3>
          <p>Select an exact branch tip and capture the work that led to it.</p>
        </section>
      ) : (
        <div className="release-layout">
          <aside className="panel release-list">
            {items.map((item) => (
              <Link
                className={current?.id === item.id ? "active" : ""}
                href={`/repositories/${repository.id}?view=releases&release=${item.id}`}
                key={item.id}
              >
                <span>
                  <strong>{item.version}</strong>
                  <small>{new Date(item.created_at).toLocaleString()}</small>
                </span>
                <Badge tone="accent">{item.status}</Badge>
                <code>{short(item.commit_id)}</code>
              </Link>
            ))}
          </aside>
          {current && (
            <section className="panel release-detail">
              <header>
                <div>
                  <p className="eyebrow">Release candidate</p>
                  <h2>{current.version}</h2>
                </div>
                <Badge tone="accent">{current.status}</Badge>
              </header>
              <div className="release-source">
                <span>
                  <small>Exact source commit</small>
                  <code>{current.commit_id}</code>
                </span>
                <span>
                  <small>Defined by</small>
                  <strong>
                    <Actor id={current.created_by_id} />
                  </strong>
                </span>
                <span>
                  <small>Captured</small>
                  <strong>
                    {new Date(current.created_at).toLocaleString()}
                  </strong>
                </span>
                {current.prior_release_id && (
                  <span>
                    <small>Changes since</small>
                    <Link
                      href={`/repositories/${repository.id}?view=releases&release=${current.prior_release_id}`}
                    >
                      {items.find(
                        (item) => item.id === current.prior_release_id,
                      )?.version ?? short(current.prior_release_id)}
                    </Link>
                  </span>
                )}
              </div>
              <article className="release-copy">
                <h3>Release notes</h3>
                <p>{current.notes || "No release notes were supplied."}</p>
              </article>
              <article className="release-inclusions release-attestation">
                <header>
                  <div>
                    <h3>Build attestation</h3>
                    <p>
                      Repository-defined commands executed against{" "}
                      <code>{short(current.commit_id)}</code> in an isolated
                      workspace.
                    </p>
                  </div>
                  <Badge tone={attestation?.verified ? "accent" : "neutral"}>
                    {attestation?.verified ? "verified" : "not verified"}
                  </Badge>
                </header>
                {!attestation ? (
                  <p>Loading retained build evidence…</p>
                ) : (
                  attestation.attempts.map((run) => {
                    const artifacts = run.events.flatMap((event) =>
                      event.artifact ? [event.artifact] : [],
                    );
                    const logs = run.events.filter(
                      (event) => event.type === "log",
                    );
                    return (
                      <details
                        className={`release-build ${run.state}`}
                        key={run.id}
                        open={run.state === "failed"}
                      >
                        <summary>
                          <span>
                            <strong>{run.definition.name}</strong>
                            <small>
                              {run.retry_of_id ? "Rerun" : "Initial attempt"} ·
                              by{" "}
                              <Actor
                                id={
                                  run.triggered_by_id ||
                                  attestation.created_by_id
                                }
                              />
                            </small>
                          </span>
                          <Badge
                            tone={
                              run.state === "succeeded" ? "accent" : "neutral"
                            }
                          >
                            {run.state}
                          </Badge>
                        </summary>
                        <div className="release-build-body">
                          <dl>
                            <div>
                              <dt>Command</dt>
                              <dd>
                                <code>{run.definition.command}</code>
                              </dd>
                            </div>
                            <div>
                              <dt>Dependencies</dt>
                              <dd>
                                {run.definition.dependencies?.join(", ") ||
                                  "None"}
                              </dd>
                            </div>
                            <div>
                              <dt>Source</dt>
                              <dd>
                                <code>{run.commit_id}</code>
                              </dd>
                            </div>
                          </dl>
                          {logs.length > 0 && (
                            <pre
                              aria-label={`${run.definition.name} build logs`}
                            >
                              {logs.map((event) => event.message).join("")}
                            </pre>
                          )}
                          {artifacts.map((artifact) => (
                            <a
                              className="release-artifact"
                              href={`/api/repositories/${repository.id}/releases/${current.id}/builds/${run.id}/artifacts/${artifact.id}`}
                              key={artifact.id}
                            >
                              <span>
                                <strong>{artifact.path}</strong>
                                <small>{artifact.size} bytes · SHA-256</small>
                              </span>
                              <code>{artifact.sha256}</code>
                            </a>
                          ))}
                          {authorized &&
                            (run.state === "failed" ||
                              run.state === "canceled" ||
                              run.state === "succeeded") && (
                              <Button
                                variant="secondary"
                                disabled={rerunning === run.id}
                                onClick={() => void rerun(run)}
                              >
                                {rerunning === run.id
                                  ? "Queuing…"
                                  : "Rerun exact attempt"}
                              </Button>
                            )}
                        </div>
                      </details>
                    );
                  })
                )}
              </article>
              <article className="release-inclusions package-publication">
                <header>
                  <div>
                    <h3>Reusable packages</h3>
                    <p>
                      Publish verified build bytes with immutable source,
                      checksum, platform, dependency, and publisher provenance.
                    </p>
                  </div>
                  <Badge tone="accent">
                    {
                      packages.filter((item) => item.release_id === current.id)
                        .length
                    }{" "}
                    published
                  </Badge>
                </header>
                {repository.owner_id === actor && attestation?.verified && (
                  <details>
                    <summary>Publish verified artifact</summary>
                    <form className="package-form" onSubmit={publishPackage}>
                      <label>
                        Package name
                        <input
                          name="name"
                          required
                          placeholder="sdk"
                          pattern="[a-z0-9][a-z0-9._-]*"
                        />
                      </label>
                      <label>
                        Semantic version
                        <input name="version" required placeholder="1.0.0" />
                      </label>
                      <label>
                        Verified artifact
                        <select name="artifact" required defaultValue="">
                          <option value="" disabled>
                            Select build artifact…
                          </option>
                          {attestation.attempts.flatMap((run) =>
                            run.state === "succeeded"
                              ? run.events.flatMap((event) =>
                                  event.artifact
                                    ? [
                                        <option
                                          value={`${run.id}:${event.artifact.id}`}
                                          key={`${run.id}:${event.artifact.id}`}
                                        >
                                          {run.definition.name} ·{" "}
                                          {event.artifact.path}
                                        </option>,
                                      ]
                                    : [],
                                )
                              : [],
                          )}
                        </select>
                      </label>
                      <label>
                        Operating system
                        <input name="os" required placeholder="linux" />
                      </label>
                      <label>
                        Architecture
                        <input name="arch" required placeholder="amd64" />
                      </label>
                      <label>
                        Runtime
                        <input name="runtime" placeholder="go1.24" />
                      </label>
                      <label>
                        Visibility
                        <select
                          name="visibility"
                          defaultValue={repository.visibility}
                        >
                          <option value="private">Private</option>
                          {repository.visibility === "public" && (
                            <option value="public">Public</option>
                          )}
                        </select>
                      </label>
                      <label className="package-dependencies">
                        Dependencies
                        <textarea
                          name="dependencies"
                          rows={3}
                          placeholder={
                            "@owner/core@^1.0.0\n@owner/types@~2.1.0"
                          }
                        />
                      </label>
                      <label className="package-dependencies">
                        Documentation
                        <textarea
                          name="documentation"
                          rows={5}
                          placeholder="Install and usage guidance for collaborators choosing this version."
                        />
                      </label>
                      <div className="release-form-actions">
                        <Button disabled={publishingPackage}>
                          {publishingPackage
                            ? "Publishing…"
                            : "Publish immutable version"}
                        </Button>
                      </div>
                    </form>
                  </details>
                )}
                <div className="package-versions">
                  {packages
                    .filter((item) => item.release_id === current.id)
                    .map((item) => (
                      <article key={item.id}>
                        <header>
                          <strong>
                            {item.identity}@{item.version}
                          </strong>
                          <Badge tone="accent">{item.lifecycle}</Badge>
                        </header>
                        <dl>
                          <div>
                            <dt>Artifact</dt>
                            <dd>
                              <a
                                href={`/api/repositories/${repository.id}/packages/${item.id}/artifact`}
                              >
                                {item.artifact_path}
                              </a>
                            </dd>
                          </div>
                          <div>
                            <dt>SHA-256</dt>
                            <dd>
                              <code>{item.sha256}</code>
                            </dd>
                          </div>
                          <div>
                            <dt>Source</dt>
                            <dd>
                              <code>{item.source_commit_id}</code>
                            </dd>
                          </div>
                          <div>
                            <dt>Build</dt>
                            <dd>
                              {item.build_attestation.build_name} ·{" "}
                              <code>
                                {short(item.build_attestation.run_id)}
                              </code>
                            </dd>
                          </div>
                          <div>
                            <dt>Platform</dt>
                            <dd>
                              {item.platform.os}/{item.platform.arch}
                              {item.platform.runtime
                                ? ` · ${item.platform.runtime}`
                                : ""}
                            </dd>
                          </div>
                          <div>
                            <dt>Publisher</dt>
                            <dd>
                              <Actor id={item.publisher_id} /> ·{" "}
                              {item.visibility}
                            </dd>
                          </div>
                        </dl>
                        {Object.keys(item.dependencies).length > 0 && (
                          <small>
                            Depends on{" "}
                            {Object.entries(item.dependencies)
                              .map(
                                ([name, constraint]) => `${name}@${constraint}`,
                              )
                              .join(", ")}
                          </small>
                        )}
                      </article>
                    ))}
                </div>
              </article>
              <article className="release-inclusions delivery">
                <header>
                  <div>
                    <h3>Governed delivery</h3>
                    <p>
                      Promote one verified artifact through ordered shared
                      environments.
                    </p>
                  </div>
                </header>
                {repository.owner_id === actor && (
                  <details>
                    <summary>Define environment policy</summary>
                    <form
                      className="delivery-policy"
                      onSubmit={saveEnvironment}
                    >
                      <label>
                        Name
                        <input name="name" required placeholder="Production" />
                      </label>
                      <label>
                        Order
                        <input
                          name="position"
                          type="number"
                          min="1"
                          defaultValue={environments.length + 1}
                          required
                        />
                      </label>
                      <label>
                        Required approvals
                        <input
                          name="approvals"
                          type="number"
                          min="0"
                          max="20"
                          defaultValue="1"
                        />
                      </label>
                      <label>
                        Concurrency
                        <input
                          name="concurrency"
                          type="number"
                          min="1"
                          max="20"
                          defaultValue="1"
                        />
                      </label>
                      <label className="wide">
                        Deployment command
                        <input
                          name="command"
                          required
                          placeholder="$KOMODO_ARTIFACT_PATH --deploy"
                        />
                      </label>
                      <label className="wide">
                        Scoped configuration
                        <textarea
                          name="configuration"
                          rows={3}
                          placeholder={"REGION=us-east-1\nSERVICE=api"}
                        />
                      </label>
                      <label className="wide">
                        Protected credentials
                        <textarea
                          name="secrets"
                          rows={3}
                          placeholder={"DEPLOY_TOKEN=…"}
                        />
                        <small>
                          Values are encrypted at rest, injected only for the
                          command, and never returned.
                        </small>
                      </label>
                      <Button disabled={deliveryBusy === "environment"}>
                        {deliveryBusy === "environment"
                          ? "Saving…"
                          : "Save environment"}
                      </Button>
                    </form>
                  </details>
                )}
                {environments.length === 0 ? (
                  <p>No delivery environments are configured.</p>
                ) : (
                  <>
                    <div className="environment-chain">
                      {environments.map((environment) => (
                        <span key={environment.id}>
                          <strong>
                            {environment.position}. {environment.name}
                          </strong>
                          <small>
                            {environment.required_approvals} approvals ·{" "}
                            {environment.concurrency} at once ·{" "}
                            {environment.secret_names.length} protected
                          </small>
                        </span>
                      ))}
                    </div>
                    {authorized && attestation?.verified && (
                      <form className="promotion-form" onSubmit={promote}>
                        <label>
                          Environment
                          <select name="environment">
                            {environments.map((environment) => (
                              <option
                                value={environment.id}
                                key={environment.id}
                              >
                                {environment.position}. {environment.name}
                              </option>
                            ))}
                          </select>
                        </label>
                        <label>
                          Exact artifact
                          <select name="artifact" required>
                            {attestation.attempts
                              .filter((run) => run.state === "succeeded")
                              .flatMap((run) =>
                                run.events.flatMap((event) =>
                                  event.artifact
                                    ? [
                                        <option
                                          value={`${run.id}:${event.artifact.id}`}
                                          key={`${run.id}:${event.artifact.id}`}
                                        >
                                          {event.artifact.path} ·{" "}
                                          {short(event.artifact.sha256)}
                                        </option>,
                                      ]
                                    : [],
                                ),
                              )}
                          </select>
                        </label>
                        <Button disabled={deliveryBusy === "promote"}>
                          {deliveryBusy === "promote"
                            ? "Requesting…"
                            : "Request promotion"}
                        </Button>
                      </form>
                    )}
                  </>
                )}
                {deployments
                  .filter((deployment) => deployment.release_id === current.id)
                  .map((deployment) => {
                    const environment = environments.find(
                      (item) => item.id === deployment.environment_id,
                    );
                    const logs = deployment.events.filter(
                      (event) => event.type === "log",
                    );
                    const active = [
                      "pending",
                      "queued",
                      "running",
                      "paused",
                    ].includes(deployment.state);
                    return (
                      <details
                        id={`deployment-${deployment.id}`}
                        className="deployment-record"
                        open={active || deployment.state === "failed"}
                        key={deployment.id}
                      >
                        <summary>
                          <span>
                            <strong>
                              {environment?.name ?? "Environment"}
                              {deployment.current_stage
                                ? ` · ${deployment.current_stage}`
                                : ""}
                            </strong>
                            <small>
                              {deployment.artifact_path} · initiated by{" "}
                              <Actor id={deployment.initiated_by_id} />
                            </small>
                          </span>
                          <Badge
                            tone={
                              deployment.state === "succeeded"
                                ? "accent"
                                : "neutral"
                            }
                          >
                            {deployment.state}
                          </Badge>
                        </summary>
                        <div>
                          <p>
                            Affected revision{" "}
                            <code>{deployment.source_commit_id}</code>
                          </p>
                          <p>
                            Artifact <code>{deployment.artifact_sha256}</code>
                          </p>
                          {deployment.recovery_of_id && (
                            <p>
                              {deployment.recovery_action === "rollback"
                                ? "Known-good rollback for"
                                : "Recovery for"}{" "}
                              <a
                                href={`#deployment-${deployment.recovery_of_id}`}
                              >
                                <code>{short(deployment.recovery_of_id)}</code>
                              </a>
                            </p>
                          )}
                          <p>
                            Approvals:{" "}
                            {deployment.approvals.length
                              ? deployment.approvals.map((approval) => (
                                  <Actor
                                    id={approval.actor_id}
                                    key={approval.actor_id}
                                  />
                                ))
                              : "None yet"}
                          </p>
                          {deployment.state === "pending" &&
                            deployment.initiated_by_id !== actor &&
                            authorized && (
                              <Button
                                variant="secondary"
                                disabled={deliveryBusy === deployment.id}
                                onClick={() => void approve(deployment)}
                              >
                                Approve exact artifact
                              </Button>
                            )}
                          {authorized && active && (
                            <div className="release-actions">
                              {deployment.state === "running" && (
                                <Button
                                  variant="secondary"
                                  disabled={deliveryBusy === deployment.id}
                                  onClick={() =>
                                    void control(deployment, "pause")
                                  }
                                >
                                  Pause rollout
                                </Button>
                              )}
                              {deployment.state === "paused" && (
                                <Button
                                  variant="secondary"
                                  disabled={deliveryBusy === deployment.id}
                                  onClick={() =>
                                    void control(deployment, "resume")
                                  }
                                >
                                  Resume rollout
                                </Button>
                              )}
                              <Button
                                variant="secondary"
                                disabled={deliveryBusy === deployment.id}
                                onClick={() =>
                                  void control(deployment, "cancel")
                                }
                              >
                                Cancel
                              </Button>
                              {(deployment.state === "running" ||
                                deployment.state === "paused") && (
                                <Button
                                  variant="danger"
                                  disabled={deliveryBusy === deployment.id}
                                  onClick={() =>
                                    void control(deployment, "fail")
                                  }
                                >
                                  Mark unsuccessful
                                </Button>
                              )}
                            </div>
                          )}
                          {authorized && deployment.state === "failed" && (
                            <div
                              className="release-actions"
                              aria-label="Deployment recovery"
                            >
                              <Button
                                variant="danger"
                                disabled={deliveryBusy.startsWith(
                                  deployment.id,
                                )}
                                onClick={() =>
                                  void recover(deployment, "rollback")
                                }
                              >
                                Restore last known good
                              </Button>
                              <Button
                                variant="secondary"
                                disabled={deliveryBusy.startsWith(
                                  deployment.id,
                                )}
                                onClick={() =>
                                  void recover(deployment, "repair")
                                }
                              >
                                Open agent repair
                              </Button>
                            </div>
                          )}
                          {deployment.decision_by_id && (
                            <p>
                              Latest decision by{" "}
                              <Actor id={deployment.decision_by_id} />
                              {deployment.decision_reason
                                ? `: ${deployment.decision_reason}`
                                : ""}
                            </p>
                          )}
                          {logs.length > 0 && (
                            <pre aria-label="Deployment logs">
                              {logs.map((event) => event.message).join("")}
                            </pre>
                          )}
                          <ol className="deployment-timeline">
                            {deployment.events
                              .filter((event) => event.type !== "log")
                              .map((event) => (
                                <li key={event.sequence}>
                                  <strong>
                                    {event.type}
                                    {event.stage ? ` · ${event.stage}` : ""}
                                    {event.signal ? ` · ${event.signal}` : ""}
                                    {event.outcome ? ` · ${event.outcome}` : ""}
                                    {event.state ? ` · ${event.state}` : ""}
                                  </strong>
                                  {event.actor_id && (
                                    <>
                                      {" "}
                                      by <Actor id={event.actor_id} />
                                    </>
                                  )}
                                  {event.message && (
                                    <span>{event.message}</span>
                                  )}
                                  <small>
                                    {new Date(
                                      event.created_at,
                                    ).toLocaleString()}
                                  </small>
                                </li>
                              ))}
                          </ol>
                        </div>
                      </details>
                    );
                  })}
              </article>
              <article className="release-inclusions">
                <h3>Included collaboration</h3>
                <div className="release-counts">
                  <span>
                    <strong>{current.pull_requests.length}</strong> pull
                    requests
                  </span>
                  <span>
                    <strong>{current.proposal_ids.length}</strong> proposals
                  </span>
                  <span>
                    <strong>{current.task_ids.length}</strong> tasks
                  </span>
                  <span>
                    <strong>{current.contributor_ids.length}</strong>{" "}
                    contributors
                  </span>
                </div>
                {current.pull_requests.length > 0 ? (
                  current.pull_requests.map((pull) => (
                    <div className="release-pull" key={pull.id}>
                      <GitPullRequest size={15} />
                      <span>
                        <Link
                          href={`/repositories/${repository.id}?view=pulls&pull=${pull.id}`}
                        >
                          {pull.title}
                        </Link>
                        <small>
                          by <Actor id={pull.author_id} /> · merge{" "}
                          <code>{short(pull.merge_commit_id)}</code>
                        </small>
                      </span>
                    </div>
                  ))
                ) : (
                  <p>No merged pull requests fall within this history range.</p>
                )}
                {(current.proposal_ids.length > 0 ||
                  current.task_ids.length > 0) && (
                  <small className="release-links">
                    Proposal links:{" "}
                    {current.proposal_ids.map((id, index) => (
                      <span key={id}>
                        {index > 0 ? ", " : ""}
                        <Link
                          href={`/repositories/${repository.id}?view=proposals&proposal=${id}`}
                        >
                          {short(id)}
                        </Link>
                      </span>
                    ))}
                    {current.task_ids.length > 0 &&
                      ` · ${current.task_ids.length} linked delivery ${current.task_ids.length === 1 ? "task" : "tasks"}`}
                  </small>
                )}
              </article>
            </section>
          )}
        </div>
      )}
    </div>
  );
}

function IncidentWorkspace({
  repository,
  actor,
  selected,
}: {
  repository: Repository;
  actor: string;
  selected?: string;
}) {
  const [items, setItems] = useState<Incident[]>([]);
  const [environments, setEnvironments] = useState<DeliveryEnvironment[]>([]);
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [baseRevision, setBaseRevision] = useState("");
  const [creating, setCreating] = useState(false);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const current = items.find((item) => item.id === selected) ?? items[0];
  const authorized = Boolean(
    actor &&
    (actor === repository.owner_id ||
      repository.collaborator_ids?.includes(actor)),
  );
  const responders = [
    repository.owner_id,
    ...(repository.collaborator_ids ?? []),
  ];
  const load = useCallback(async () => {
    try {
      const [incidentData, environmentData, deploymentData, branches] =
        await Promise.all([
          get<IncidentList>(`/repositories/${repository.id}/incidents`),
          get<EnvironmentList>(`/repositories/${repository.id}/environments`),
          get<DeploymentList>(`/repositories/${repository.id}/deployments`),
          get<Branches>(`/repositories/${repository.id}/branches`),
        ]);
      setItems(incidentData.items);
      setEnvironments(environmentData.items);
      setDeployments(deploymentData.items);
      setBaseRevision(
        branches.items.find((branch) => branch.is_default)?.commit_id ?? "",
      );
    } catch {
      setError("Incident state unavailable.");
    }
  }, [repository.id]);
  useEffect(() => {
    // Incident state follows the shareable selected-incident URL.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);
  async function declare(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy("create");
    setError("");
    const data = new FormData(event.currentTarget);
    const environment = String(data.get("environment"));
    const deploymentID = String(data.get("deployment"));
    const deployment = deployments.find((item) => item.id === deploymentID);
    const signal = deployment?.events.findLast(
      (item) =>
        item.signal && ["failed", "unhealthy"].includes(item.outcome ?? ""),
    );
    const additional = String(data.get("affected") ?? "")
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => {
        const [repository_id, environment_id = ""] = line.split(":");
        return { repository_id, environment_id };
      });
    try {
      const item = await send<Incident>(
        `/repositories/${repository.id}/incidents`,
        "POST",
        {
          title: data.get("title"),
          summary: data.get("summary"),
          severity: data.get("severity"),
          roles: {
            commander: data.get("commander"),
            operations: data.get("operations") || undefined,
            communications: data.get("communications") || undefined,
          },
          affected: [
            { repository_id: repository.id, environment_id: environment },
            ...additional,
          ],
          source_signal:
            deployment && signal
              ? {
                  repository_id: repository.id,
                  deployment_id: deployment.id,
                  event_sequence: signal.sequence,
                }
              : undefined,
        },
      );
      window.location.assign(
        `/repositories/${repository.id}?view=incidents&incident=${item.id}`,
      );
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Incident could not be declared.",
      );
    } finally {
      setBusy("");
    }
  }
  async function mutate(path: string, method: string, body?: unknown) {
    if (!current) return;
    setBusy(path);
    setError("");
    try {
      await send(
        `/repositories/${repository.id}/incidents/${current.id}${path}`,
        method,
        body,
      );
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Incident change was not accepted.",
      );
    } finally {
      setBusy("");
    }
  }
  async function attachEvidence(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const kind = String(data.get("kind"));
    const start = String(data.get("start_at") || "");
    const end = String(data.get("end_at") || "");
    await mutate("/evidence", "POST", {
      kind,
      repository_id: repository.id,
      resource_id: data.get("resource_id"),
      event_sequence: Number(data.get("event_sequence")) || undefined,
      start_at: start ? new Date(start).toISOString() : undefined,
      end_at: end ? new Date(end).toISOString() : undefined,
      title: data.get("title"),
      audience: data.get("audience"),
    });
    form.reset();
  }
  async function addFinding(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    await mutate("/findings", "POST", {
      kind: data.get("kind"),
      body: data.get("body"),
      query: data.get("query"),
      audience: data.get("audience"),
      evidence_ids: data.getAll("evidence"),
    });
    form.reset();
  }
  async function proposeMitigation(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const deployment = deployments.find(
      (item) => item.id === data.get("deployment"),
    );
    if (!deployment) {
      setError("Choose the affected deployment.");
      return;
    }
    await mutate("/mitigations", "POST", {
      kind: data.get("kind"),
      title: data.get("title"),
      description: data.get("description"),
      repository_id: repository.id,
      environment_id: deployment.environment_id,
      deployment_id: deployment.id,
      evidence_ids: data.getAll("evidence"),
      recovery_criteria: [{ name: data.get("criterion") }],
    });
    form.reset();
  }
  async function executeMitigation(item: IncidentMitigation) {
    if (!item.deployment_id) return;
    setBusy(item.id);
    setError("");
    try {
      if (item.kind === "pause_rollout") {
        await send(
          `/repositories/${repository.id}/incidents/${current!.id}/mitigations/${item.id}/execution`,
          "POST",
          {
            outcome: "started",
            message: "Authorized responder paused the governed rollout",
          },
        );
      } else {
        const recovery = await send<{
          deployment?: Deployment;
          pull_request?: { id: string };
        }>(
          `/repositories/${repository.id}/deployments/${item.deployment_id}/recovery`,
          "POST",
          {
            action: item.kind === "restore_release" ? "rollback" : "repair",
            instructions: item.description,
            context_paths: [],
          },
        );
        const resourceType =
          item.kind === "restore_release" ? "deployment" : "pull_request";
        const resourceID = recovery.deployment?.id ?? recovery.pull_request?.id;
        await send(
          `/repositories/${repository.id}/incidents/${current!.id}/mitigations/${item.id}/execution`,
          "POST",
          {
            outcome: "started",
            resource_type: resourceType,
            resource_id: resourceID,
            message:
              item.kind === "restore_release"
                ? "Governed attested rollback started"
                : "Draft emergency repair entered the ordinary change workflow",
          },
        );
      }
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Mitigation execution was not accepted.",
      );
    } finally {
      setBusy("");
    }
  }
  async function verifyMitigation(item: IncidentMitigation) {
    const candidate = deployments.find(
      (deployment) =>
        deployment.environment_id === item.environment_id &&
        deployment.events.some(
          (event) =>
            event.signal && ["healthy", "passed"].includes(event.outcome ?? ""),
        ),
    );
    const signal = candidate?.events.findLast(
      (event) =>
        event.signal && ["healthy", "passed"].includes(event.outcome ?? ""),
    );
    if (!candidate || !signal) {
      setError(
        "No successful health signal is available for this environment yet.",
      );
      return;
    }
    await mutate(`/mitigations/${item.id}/verification`, "POST", {
      results: item.recovery_criteria.map((criterion) => ({
        name: criterion.name,
        deployment_id: candidate.id,
        event_sequence: signal.sequence,
      })),
    });
  }
  async function resolve(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const due = String(data.get("due_at") || "");
    await mutate("/resolution", "POST", {
      impact_summary: data.get("impact"),
      timeline_summary: data.get("timeline"),
      contributing_factors: String(data.get("factors"))
        .split("\n")
        .map((value) => value.trim())
        .filter(Boolean),
      conclusion_ids: data.getAll("conclusion"),
      proposal_title: data.get("proposal_title"),
      proposal_body: data.get("proposal_body"),
      commitments: [
        {
          title: data.get("task_title"),
          outcome: data.get("outcome"),
          owner_id: data.get("owner"),
          kind: "human",
          mandate: data.get("mandate"),
          base_revision: baseRevision,
          due_at: due ? new Date(due).toISOString() : undefined,
        },
      ],
    });
    form.reset();
  }
  const latestUpdate = current?.timeline
    .filter((event) => event.type === "update")
    .at(-1);
  const acknowledged =
    latestUpdate &&
    current?.acknowledgements.some(
      (item) =>
        item.actor_id === actor &&
        item.update_sequence === latestUpdate.sequence,
    );
  return (
    <div className="incident-workspace">
      <header className="release-heading">
        <div>
          <p className="eyebrow">Shared operating picture</p>
          <h2>Incidents</h2>
          <p>
            Establish response ownership, affected scope, and one attributable
            source of truth while service is at risk.
          </p>
        </div>
        {authorized && (
          <Button onClick={() => setCreating((value) => !value)}>
            <Plus size={14} />
            Declare incident
          </Button>
        )}
      </header>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      {creating && (
        <form className="panel incident-form" onSubmit={declare}>
          <label>
            Incident title
            <input name="title" required maxLength={200} />
          </label>
          <label>
            Severity
            <select name="severity" defaultValue="high">
              <option value="critical">Critical</option>
              <option value="high">High</option>
              <option value="medium">Medium</option>
              <option value="low">Low</option>
            </select>
          </label>
          <label>
            Affected environment
            <select name="environment" required>
              <option value="">Choose environment</option>
              {environments.map((item) => (
                <option value={item.id} key={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Deployment health signal (optional)
            <select name="deployment">
              <option value="">Manual declaration</option>
              {deployments
                .filter((item) =>
                  item.events.some(
                    (event) =>
                      event.signal &&
                      ["failed", "unhealthy"].includes(event.outcome ?? ""),
                  ),
                )
                .map((item) => (
                  <option value={item.id} key={item.id}>
                    {
                      environments.find(
                        (environment) => environment.id === item.environment_id,
                      )?.name
                    }{" "}
                    · {short(item.id)}
                  </option>
                ))}
            </select>
          </label>
          <label>
            Incident commander
            <select name="commander" defaultValue={actor}>
              {responders.map((id) => (
                <option value={id} key={id}>
                  {id === actor ? "Me" : short(id)}
                </option>
              ))}
            </select>
          </label>
          <label>
            Operations lead
            <select name="operations" defaultValue={actor}>
              <option value="">Unassigned</option>
              {responders.map((id) => (
                <option value={id} key={id}>
                  {id === actor ? "Me" : short(id)}
                </option>
              ))}
            </select>
          </label>
          <label>
            Communications lead
            <select name="communications">
              <option value="">Unassigned</option>
              {responders.map((id) => (
                <option value={id} key={id}>
                  {id === actor ? "Me" : short(id)}
                </option>
              ))}
            </select>
          </label>
          <label className="incident-summary">
            Additional affected scopes{" "}
            <small>
              One authorized repository ID and environment ID per line:
              repository:environment
            </small>
            <textarea name="affected" rows={2} />
          </label>
          <label className="incident-summary">
            Current impact and known facts
            <textarea name="summary" required rows={4} />
          </label>
          <div className="release-form-actions">
            <Button
              variant="secondary"
              type="button"
              onClick={() => setCreating(false)}
            >
              Cancel
            </Button>
            <Button disabled={busy === "create"} type="submit">
              Declare and notify responders
            </Button>
          </div>
        </form>
      )}
      <div className="incident-layout">
        <aside className="panel incident-list">
          {items.map((item) => (
            <Link
              className={current?.id === item.id ? "active" : ""}
              href={`/repositories/${repository.id}?view=incidents&incident=${item.id}`}
              key={item.id}
            >
              <span>
                <strong>{item.title}</strong>
                <small>{new Date(item.updated_at).toLocaleString()}</small>
              </span>
              <Badge tone={item.status === "resolved" ? "neutral" : "accent"}>
                {item.severity} · {item.status}
              </Badge>
            </Link>
          ))}
          {!items.length && <p>No incidents have been declared.</p>}
        </aside>
        {current && (
          <article className="panel incident-detail">
            <header>
              <div>
                <p className="eyebrow">{current.severity} severity</p>
                <h2>{current.title}</h2>
              </div>
              <Badge
                tone={current.status === "resolved" ? "neutral" : "accent"}
              >
                {current.status}
              </Badge>
            </header>
            <section className="incident-picture">
              <p>{current.summary}</p>
              <dl>
                <div>
                  <dt>Commander</dt>
                  <dd>
                    <Actor id={current.roles.commander} />
                  </dd>
                </div>
                {Object.entries(current.roles)
                  .filter(([role]) => role !== "commander")
                  .map(([role, id]) => (
                    <div key={role}>
                      <dt>{role}</dt>
                      <dd>
                        <Actor id={id} />
                      </dd>
                    </div>
                  ))}
                <div>
                  <dt>Affected</dt>
                  <dd>
                    {current.affected.map((scope) => (
                      <span
                        key={`${scope.repository_id}:${scope.environment_id}`}
                      >
                        {repository.name} ·{" "}
                        {environments.find(
                          (item) => item.id === scope.environment_id,
                        )?.name ?? "repository"}
                      </span>
                    ))}
                  </dd>
                </div>
                {current.source_signal && (
                  <div>
                    <dt>Declared from</dt>
                    <dd>
                      Health signal {current.source_signal.signal} ·{" "}
                      {current.source_signal.outcome} in{" "}
                      {current.source_signal.stage}
                    </dd>
                  </div>
                )}
              </dl>
            </section>
            {authorized && current.status !== "resolved" && (
              <section className="incident-controls">
                <label>
                  Current status
                  <select
                    value={current.status}
                    onChange={(event) =>
                      void mutate("", "PATCH", { status: event.target.value })
                    }
                  >
                    <option value="declared">Declared</option>
                    <option value="investigating">Investigating</option>
                    <option value="mitigating">Mitigating</option>
                    <option value="monitoring">Monitoring</option>
                  </select>
                </label>
                <Button
                  variant="secondary"
                  disabled={Boolean(busy)}
                  onClick={() =>
                    void mutate(
                      "/follow",
                      current.followers.includes(actor) ? "DELETE" : "PUT",
                    )
                  }
                >
                  {current.followers.includes(actor)
                    ? "Unfollow"
                    : "Follow updates"}
                </Button>
                {latestUpdate && !acknowledged && (
                  <Button
                    variant="secondary"
                    disabled={Boolean(busy)}
                    onClick={() =>
                      void mutate("/acknowledgements", "POST", {
                        update_sequence: latestUpdate.sequence,
                      })
                    }
                  >
                    Acknowledge latest
                  </Button>
                )}
              </section>
            )}
            <section className="incident-investigation">
              <header>
                <div>
                  <h3>Connected evidence</h3>
                  <p>
                    Durable source pointers and time windows let responders
                    inspect live state while retaining the exact historical
                    context behind a claim.
                  </p>
                </div>
                <Badge tone="neutral">
                  {current.evidence?.length ?? 0} sources
                </Badge>
              </header>
              {authorized && current.status !== "resolved" && (
                <form
                  className="incident-evidence-form"
                  onSubmit={(event) => void attachEvidence(event)}
                >
                  <label>
                    Source type
                    <select name="kind">
                      <option value="logs">Time-bounded deployment logs</option>
                      <option value="health_signal">Health signal</option>
                      <option value="deployment">Deployment</option>
                      <option value="release">Release</option>
                      <option value="commit">Commit</option>
                      <option value="pull_request">Pull request</option>
                      <option value="incident">Prior incident</option>
                    </select>
                  </label>
                  <label>
                    Source ID
                    <input
                      name="resource_id"
                      required
                      list="incident-sources"
                      placeholder="Exact resource ID"
                    />
                    <datalist id="incident-sources">
                      {deployments.map((item) => (
                        <option value={item.id} key={item.id} />
                      ))}
                    </datalist>
                  </label>
                  <label>
                    Event sequence <small>Required for a health signal</small>
                    <input name="event_sequence" type="number" min="1" />
                  </label>
                  <label>
                    Window start
                    <input name="start_at" type="datetime-local" />
                  </label>
                  <label>
                    Window end
                    <input name="end_at" type="datetime-local" />
                  </label>
                  <label className="incident-summary">
                    What this source shows
                    <input name="title" required maxLength={300} />
                  </label>
                  <label>
                    Access
                    <select name="audience">
                      <option value="participants">Participants</option>
                      <option value="public">Public</option>
                    </select>
                  </label>
                  <Button disabled={Boolean(busy)} type="submit">
                    Attach evidence
                  </Button>
                </form>
              )}
              <div className="incident-evidence-list">
                {(current.evidence ?? []).map((item) => {
                  const href =
                    item.kind === "deployment" ||
                    item.kind === "logs" ||
                    item.kind === "health_signal"
                      ? `/repositories/${item.repository_id}?view=releases`
                      : item.kind === "release"
                        ? `/repositories/${item.repository_id}?view=releases&release=${item.resource_id}`
                        : item.kind === "pull_request"
                          ? `/repositories/${item.repository_id}?view=pulls&pull=${item.resource_id}`
                          : item.kind === "incident"
                            ? `/repositories/${item.repository_id}?view=incidents&incident=${item.resource_id}`
                            : item.kind === "commit"
                              ? `/repositories/${item.repository_id}?view=commits&ref=${item.resource_id}`
                              : "#";
                  return (
                    <article key={item.id}>
                      <span>
                        <Badge
                          tone={
                            item.audience === "public" ? "accent" : "neutral"
                          }
                        >
                          {item.kind.replaceAll("_", " ")}
                        </Badge>
                        <strong>{item.title}</strong>
                      </span>
                      <p>
                        <Link href={href}>
                          Inspect source {short(item.resource_id)}
                        </Link>
                        {item.event_sequence
                          ? ` · event ${item.event_sequence}`
                          : ""}
                      </p>
                      {item.start_at && (
                        <small>
                          {new Date(item.start_at).toLocaleString()} →{" "}
                          {item.end_at &&
                            new Date(item.end_at).toLocaleString()}
                        </small>
                      )}
                      <small>
                        Attached by <Actor id={item.attached_by_id} /> ·{" "}
                        {new Date(item.captured_at).toLocaleString()}
                      </small>
                    </article>
                  );
                })}
                {!current.evidence?.length && (
                  <p>No evidence has been attached yet.</p>
                )}
              </div>
            </section>
            <section className="incident-investigation">
              <header>
                <div>
                  <h3>Investigation notebook</h3>
                  <p>
                    Separate observations, testable hypotheses, reproducible
                    queries, and conclusions—and cite the evidence each relies
                    on.
                  </p>
                </div>
              </header>
              {authorized && current.status !== "resolved" && (
                <form
                  className="incident-finding-form"
                  onSubmit={(event) => void addFinding(event)}
                >
                  <label>
                    Entry type
                    <select name="kind">
                      <option value="observation">Observation</option>
                      <option value="hypothesis">Hypothesis</option>
                      <option value="query">Query</option>
                      <option value="conclusion">Conclusion</option>
                    </select>
                  </label>
                  <label>
                    Access
                    <select name="audience">
                      <option value="participants">Participants</option>
                      <option value="public">Public</option>
                    </select>
                  </label>
                  <label className="incident-summary">
                    Finding
                    <textarea name="body" required rows={3} />
                  </label>
                  <label className="incident-summary">
                    Reproducible query or command
                    <textarea
                      name="query"
                      rows={2}
                      placeholder="Optional query, command, or procedure"
                    />
                  </label>
                  {current.evidence?.length > 0 && (
                    <fieldset>
                      <legend>Supporting evidence</legend>
                      {current.evidence.map((item) => (
                        <label key={item.id}>
                          <input
                            type="checkbox"
                            name="evidence"
                            value={item.id}
                          />
                          {item.title}
                        </label>
                      ))}
                    </fieldset>
                  )}
                  <Button disabled={Boolean(busy)} type="submit">
                    Record finding
                  </Button>
                </form>
              )}
              <ol className="incident-findings">
                {[...(current.findings ?? [])].reverse().map((item) => (
                  <li key={item.id}>
                    <span>
                      <Badge tone="accent">{item.kind}</Badge>
                      <Badge tone="neutral">{item.audience}</Badge>
                    </span>
                    <p>{item.body}</p>
                    {item.query && (
                      <pre>
                        <code>{item.query}</code>
                      </pre>
                    )}
                    {item.evidence_ids.length > 0 && (
                      <small>
                        Sources:{" "}
                        {item.evidence_ids.map((id) => (
                          <a href={`#evidence-${id}`} key={id}>
                            {current.evidence.find((e) => e.id === id)?.title ??
                              short(id)}
                          </a>
                        ))}
                      </small>
                    )}
                    <small>
                      <Actor id={item.author_id} /> ·{" "}
                      {new Date(item.created_at).toLocaleString()}
                    </small>
                  </li>
                ))}
              </ol>
            </section>
            <section className="incident-investigation">
              <header>
                <div>
                  <h3>Mitigation decisions</h3>
                  <p>
                    Propose an evidence-backed intervention, authorize it
                    independently, track governed execution, and verify the
                    declared recovery signal.
                  </p>
                </div>
                <Badge tone="neutral">
                  {current.mitigations?.length ?? 0} actions
                </Badge>
              </header>
              {authorized && current.status !== "resolved" && (
                <form
                  className="incident-finding-form"
                  onSubmit={(event) => void proposeMitigation(event)}
                >
                  <label>
                    Intervention
                    <select name="kind">
                      <option value="pause_rollout">Pause rollout</option>
                      <option value="restore_release">
                        Restore attested release
                      </option>
                      <option value="emergency_repair">Emergency repair</option>
                    </select>
                  </label>
                  <label>
                    Target deployment
                    <select name="deployment" required>
                      <option value="">Choose deployment</option>
                      {deployments.map((item) => (
                        <option value={item.id} key={item.id}>
                          {
                            environments.find(
                              (environment) =>
                                environment.id === item.environment_id,
                            )?.name
                          }{" "}
                          · {short(item.id)} · {item.state}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="incident-summary">
                    Decision title
                    <input name="title" required maxLength={300} />
                  </label>
                  <label className="incident-summary">
                    Expected change and risk
                    <textarea name="description" required rows={3} />
                  </label>
                  <label className="incident-summary">
                    Recovery criterion
                    <input
                      name="criterion"
                      required
                      placeholder="Availability probe is healthy"
                    />
                  </label>
                  <fieldset>
                    <legend>Exact supporting evidence</legend>
                    {current.evidence.map((item) => (
                      <label key={item.id}>
                        <input
                          type="checkbox"
                          name="evidence"
                          value={item.id}
                        />
                        {item.title}
                      </label>
                    ))}
                  </fieldset>
                  <Button disabled={Boolean(busy)} type="submit">
                    Propose mitigation
                  </Button>
                </form>
              )}
              <ol className="incident-findings">
                {[...(current.mitigations ?? [])].reverse().map((item) => (
                  <li key={item.id}>
                    <span>
                      <Badge
                        tone={item.state === "recovered" ? "accent" : "neutral"}
                      >
                        {item.state}
                      </Badge>
                      <Badge tone="neutral">
                        {item.kind.replaceAll("_", " ")}
                      </Badge>
                    </span>
                    <h4>{item.title}</h4>
                    <p>{item.description}</p>
                    <small>
                      Proposed by <Actor id={item.proposed_by_id} /> · sources{" "}
                      {item.evidence_ids.map(short).join(", ")}
                    </small>
                    {item.decisions.map((decision) => (
                      <p key={decision.created_at}>
                        <strong>
                          {decision.override ? "Override " : ""}
                          {decision.decision}
                        </strong>{" "}
                        by <Actor id={decision.actor_id} />: {decision.reason}
                      </p>
                    ))}
                    {item.attempts.map((attempt) => (
                      <p key={attempt.sequence}>
                        Attempt {attempt.sequence} · {attempt.outcome} by{" "}
                        <Actor id={attempt.actor_id} /> · {attempt.message}
                        {attempt.resource_id && (
                          <>
                            {" "}
                            · <code>{short(attempt.resource_id)}</code>
                          </>
                        )}
                      </p>
                    ))}
                    {item.recovery_criteria.map((criterion) => (
                      <p key={criterion.name}>
                        Recovery: {criterion.name}
                        {criterion.event_sequence
                          ? ` · event ${criterion.event_sequence}`
                          : ""}{" "}
                        · <strong>{criterion.outcome}</strong>
                        {criterion.verified_by_id && (
                          <>
                            {" "}
                            verified by <Actor id={criterion.verified_by_id} />
                          </>
                        )}
                      </p>
                    ))}
                    {authorized &&
                      item.state === "proposed" &&
                      item.proposed_by_id !== actor && (
                        <div className="release-actions">
                          <Button
                            onClick={() =>
                              void mutate(
                                `/mitigations/${item.id}/decision`,
                                "POST",
                                {
                                  decision: "approve",
                                  reason:
                                    "Reviewed evidence and authorized this exact intervention",
                                },
                              )
                            }
                          >
                            Approve
                          </Button>
                          <Button
                            variant="danger"
                            onClick={() =>
                              void mutate(
                                `/mitigations/${item.id}/decision`,
                                "POST",
                                {
                                  decision: "reject",
                                  reason:
                                    "Risk or evidence does not support execution",
                                },
                              )
                            }
                          >
                            Reject
                          </Button>
                        </div>
                      )}
                    {authorized && item.state === "approved" && (
                      <Button
                        disabled={busy === item.id}
                        onClick={() => void executeMitigation(item)}
                      >
                        Execute through governed workflow
                      </Button>
                    )}
                    {authorized &&
                      ["verifying", "failed"].includes(item.state) && (
                        <Button
                          variant="secondary"
                          disabled={Boolean(busy)}
                          onClick={() => void verifyMitigation(item)}
                        >
                          Verify latest healthy signal
                        </Button>
                      )}
                  </li>
                ))}
              </ol>
            </section>
            <section className="incident-investigation">
              <header>
                <div>
                  <h3>Review and corrective ownership</h3>
                  <p>
                    Close response with a durable account of impact and causes,
                    then carry prevention work into an assigned proposal plan.
                  </p>
                </div>
                {current.resolution && (
                  <Button
                    variant="secondary"
                    disabled={Boolean(busy)}
                    onClick={() => void mutate("/resolution/reconcile", "POST")}
                  >
                    Refresh delivery progress
                  </Button>
                )}
              </header>
              {authorized &&
                current.status !== "resolved" &&
                current.findings.some(
                  (finding) => finding.kind === "conclusion",
                ) &&
                baseRevision && (
                  <form
                    className="incident-finding-form"
                    onSubmit={(event) => void resolve(event)}
                  >
                    <label className="incident-summary">
                      Impact summary
                      <textarea name="impact" required rows={3} />
                    </label>
                    <label className="incident-summary">
                      Timeline summary
                      <textarea name="timeline" required rows={3} />
                    </label>
                    <label className="incident-summary">
                      Contributing factors <small>One factor per line</small>
                      <textarea name="factors" required rows={3} />
                    </label>
                    <fieldset>
                      <legend>Reviewable conclusions</legend>
                      {current.findings
                        .filter((finding) => finding.kind === "conclusion")
                        .map((finding) => (
                          <label key={finding.id}>
                            <input
                              type="checkbox"
                              name="conclusion"
                              value={finding.id}
                              required
                            />
                            {finding.body}
                          </label>
                        ))}
                    </fieldset>
                    <label>
                      Corrective proposal title
                      <input name="proposal_title" required maxLength={200} />
                    </label>
                    <label className="incident-summary">
                      Proposal context
                      <textarea name="proposal_body" rows={3} />
                    </label>
                    <label>
                      Owned task
                      <input name="task_title" required maxLength={200} />
                    </label>
                    <label className="incident-summary">
                      Observable outcome
                      <textarea name="outcome" required rows={2} />
                    </label>
                    <label>
                      Owner
                      <select name="owner" defaultValue={actor}>
                        {responders.map((id) => (
                          <option value={id} key={id}>
                            {id === actor ? "Me" : short(id)}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label className="incident-summary">
                      Mandate
                      <textarea name="mandate" required rows={2} />
                    </label>
                    <label>
                      Due date
                      <input name="due_at" type="datetime-local" required />
                    </label>
                    <Button disabled={Boolean(busy)} type="submit">
                      Resolve and assign follow-up
                    </Button>
                  </form>
                )}
              {current.resolution && (
                <article className="incident-picture">
                  <p>{current.resolution.impact_summary}</p>
                  <p>
                    <strong>Timeline:</strong>{" "}
                    {current.resolution.timeline_summary}
                  </p>
                  <ul>
                    {current.resolution.contributing_factors.map((factor) => (
                      <li key={factor}>{factor}</li>
                    ))}
                  </ul>
                  <small>
                    Resolved by <Actor id={current.resolution.resolved_by_id} />{" "}
                    ·{" "}
                    {new Date(current.resolution.resolved_at).toLocaleString()}
                  </small>
                  {current.resolution.corrective_work.map((work) => (
                    <div key={work.task_id}>
                      <span>
                        <Badge
                          tone={
                            work.overdue || work.invalidated
                              ? "neutral"
                              : "accent"
                          }
                        >
                          {work.invalidated
                            ? "invalidated"
                            : work.overdue
                              ? "overdue"
                              : work.state}
                        </Badge>{" "}
                        <Link
                          href={`/repositories/${repository.id}?view=proposals&proposal=${work.proposal_id}`}
                        >
                          Corrective task {short(work.task_id)}
                        </Link>
                      </span>
                      <small>
                        Owned by <Actor id={work.owner_id} />
                        {work.due_at && (
                          <> · due {new Date(work.due_at).toLocaleString()}</>
                        )}
                      </small>
                      {work.pull_request_ids?.map((id) => (
                        <Link
                          key={id}
                          href={`/repositories/${repository.id}?view=pulls&pull=${id}`}
                        >
                          Pull request {short(id)}
                        </Link>
                      ))}
                      {Boolean(work.release_ids?.length) && (
                        <small>
                          {work.release_ids?.length} release candidate(s) ·{" "}
                          {work.deployment_ids?.length ?? 0} deployment(s)
                        </small>
                      )}
                    </div>
                  ))}
                </article>
              )}
            </section>
            {authorized && current.status !== "resolved" && (
              <form
                className="incident-update"
                onSubmit={(event) => {
                  event.preventDefault();
                  const form = event.currentTarget;
                  const data = new FormData(form);
                  void mutate("/updates", "POST", {
                    audience: data.get("audience"),
                    message: data.get("message"),
                  }).then(() => form.reset());
                }}
              >
                <label>
                  Audience
                  <select name="audience">
                    <option value="participants">
                      Responders and participants
                    </option>
                    <option value="public">Audience-safe update</option>
                  </select>
                </label>
                <label>
                  Timeline update
                  <textarea
                    name="message"
                    required
                    rows={3}
                    placeholder="What changed, what responders know, and what happens next"
                  />
                </label>
                <Button disabled={Boolean(busy)} type="submit">
                  Publish update
                </Button>
              </form>
            )}
            <ol className="incident-timeline">
              {[...current.timeline].reverse().map((event) => (
                <li key={event.sequence}>
                  <span>
                    <strong>{event.type.replaceAll(".", " ")}</strong>
                    {event.audience && (
                      <Badge
                        tone={
                          event.audience === "public" ? "accent" : "neutral"
                        }
                      >
                        {event.audience}
                      </Badge>
                    )}
                  </span>
                  {event.actor_id && (
                    <small>
                      <Actor id={event.actor_id} /> ·{" "}
                      {new Date(event.created_at).toLocaleString()}
                    </small>
                  )}
                  {event.message && <p>{event.message}</p>}
                  {event.status && (
                    <p>
                      Status: <b>{event.status}</b>
                    </p>
                  )}
                  {event.type === "update" && (
                    <small>
                      {
                        current.acknowledgements.filter(
                          (item) => item.update_sequence === event.sequence,
                        ).length
                      }{" "}
                      acknowledged
                    </small>
                  )}
                </li>
              ))}
            </ol>
          </article>
        )}
      </div>
    </div>
  );
}

function IntegrationQueueWorkspace({
  repository,
  branches,
  initialBranch,
  actor,
  onBranch,
}: {
  repository: Repository;
  branches: BranchRecord[];
  initialBranch: string;
  actor: string;
  onBranch: (branch: string) => void;
}) {
  const [data, setData] = useState<IntegrationQueueEntries>();
  const [pulls, setPulls] = useState<PullRequest[]>([]);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    setError("");
    try {
      const [queue, requests] = await Promise.all([
        get<IntegrationQueueEntries>(
          `/repositories/${repository.id}/integration-queue/entries?branch=${encodeURIComponent(initialBranch)}`,
        ),
        get<PullRequestList>(
          `/repositories/${repository.id}/pull-requests?per_page=100`,
        ),
      ]);
      setData(queue);
      setPulls(requests.items);
    } catch {
      setError("The integration queue could not be loaded.");
    }
  }, [repository.id, initialBranch]);
  useEffect(() => {
    // Queue state follows the shareable target-branch URL.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);
  async function operate(
    entry: IntegrationQueueEntry,
    action: string,
    position?: number,
  ) {
    setBusy(entry.id + action);
    setError("");
    try {
      await send(
        `/repositories/${repository.id}/integration-queue/entries/${entry.id}`,
        "PATCH",
        { action, position },
      );
      await load();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "The queue action could not be completed.",
      );
    } finally {
      setBusy("");
    }
  }
  const active = data?.items.filter((entry) => !entry.completed_at) ?? [];
  const completed = data?.items.filter((entry) => entry.completed_at) ?? [];
  const title = (id: string) =>
    pulls.find((pull) => pull.id === id)?.title ?? `Pull request ${short(id)}`;
  return (
    <section className="queue-workspace">
      <header className="proposal-toolbar">
        <div>
          <p className="eyebrow">
            <Branch size={14} />
            Shared integration workflow
          </p>
          <h2>{initialBranch} queue</h2>
          <p>
            See why every change is waiting, the exact candidate evidence, and
            the next available intervention.
          </p>
        </div>
        <label className="queue-branch">
          Target branch
          <select
            value={initialBranch}
            onChange={(event) => onBranch(event.target.value)}
          >
            {branches.map((branch) => (
              <option key={branch.name}>{branch.name}</option>
            ))}
          </select>
        </label>
      </header>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <div className="queue-policy-banner panel">
        <span>
          <strong>
            {data?.policy.enabled
              ? "Queue protection active"
              : "Queue protection disabled"}
          </strong>
          <small>
            {data?.policy.concurrency ?? 1} concurrent candidate{" "}
            {(data?.policy.concurrency ?? 1) === 1 ? "slot" : "slots"} ·
            failures{" "}
            {data?.policy.failure_behavior === "remove"
              ? "leave the queue"
              : "pause for intervention"}
          </small>
        </span>
        <Badge tone={data?.policy.enabled ? "accent" : "neutral"}>
          {active.length} active
        </Badge>
      </div>
      <div className="queue-board">
        {active.map((entry, index) => (
          <article className={`queue-card panel ${entry.state}`} key={entry.id}>
            <div className="queue-order">
              <strong>#{entry.position}</strong>
              <span>{entry.state}</span>
            </div>
            <div className="queue-card-body">
              <header>
                <div>
                  <p className="eyebrow">Pull request</p>
                  <Link
                    href={`/repositories/${repository.id}?view=pulls&pull=${entry.pull_request_id}`}
                  >
                    {title(entry.pull_request_id)}
                  </Link>
                </div>
                <code>{short(entry.candidate_commit_id)}</code>
              </header>
              <p className="queue-next">
                <strong>Next:</strong> {entry.next_action}
              </p>
              {entry.blocker && (
                <p className="queue-blocker">
                  <Clock size={13} />
                  {entry.blocker.replaceAll("_", " ")}
                </p>
              )}
              <div className="queue-checks">
                {entry.checks.requirements.length ? (
                  entry.checks.requirements.map((check) => (
                    <span className={check.status} key={check.name}>
                      <Check size={11} />
                      {check.name} · {check.status}
                    </span>
                  ))
                ) : (
                  <span className="succeeded">
                    <Check size={11} />
                    No required checks
                  </span>
                )}
              </div>
              <details>
                <summary>
                  {entry.attempt_history.length} retained candidate{" "}
                  {entry.attempt_history.length === 1 ? "attempt" : "attempts"}
                </summary>
                {entry.attempt_history.map((attempt) => (
                  <div className="queue-attempt" key={attempt.generation}>
                    <span>
                      Generation {attempt.generation} ·{" "}
                      <code>{short(attempt.candidate_commit_id)}</code>
                    </span>
                    <small>
                      Base {short(attempt.target_commit_id)} ·{" "}
                      {attempt.checks.satisfied ? "passed" : "not passed"} ·{" "}
                      {new Date(attempt.created_at).toLocaleString()}
                    </small>
                  </div>
                ))}
              </details>
              {actor === repository.owner_id && (
                <div className="queue-controls">
                  {entry.state === "paused" ? (
                    <Button
                      size="sm"
                      variant="secondary"
                      disabled={Boolean(busy)}
                      onClick={() => void operate(entry, "resume")}
                    >
                      Resume
                    </Button>
                  ) : (
                    <Button
                      size="sm"
                      variant="secondary"
                      disabled={Boolean(busy)}
                      onClick={() => void operate(entry, "pause")}
                    >
                      Pause
                    </Button>
                  )}
                  {entry.state === "blocked" && (
                    <Button
                      size="sm"
                      variant="secondary"
                      disabled={Boolean(busy)}
                      onClick={() => void operate(entry, "retry")}
                    >
                      Retry checks
                    </Button>
                  )}
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={Boolean(busy) || index === 0}
                    onClick={() =>
                      void operate(entry, "reprioritize", entry.position - 1)
                    }
                  >
                    Move up
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={Boolean(busy) || index === active.length - 1}
                    onClick={() =>
                      void operate(entry, "reprioritize", entry.position + 1)
                    }
                  >
                    Move down
                  </Button>
                  <button
                    className="withdraw-review"
                    disabled={Boolean(busy)}
                    onClick={() => void operate(entry, "remove")}
                  >
                    Remove
                  </button>
                </div>
              )}
            </div>
          </article>
        ))}
        {!active.length && (
          <div className="proposal-empty panel">
            <Branch />
            <h3>No changes are waiting</h3>
            <p>
              Ready pull requests admitted to this branch will appear here in
              policy order.
            </p>
          </div>
        )}
      </div>
      {completed.length > 0 && (
        <section className="queue-history">
          <h3>Recent outcomes</h3>
          {completed.map((entry) => (
            <article className="panel" key={entry.id}>
              <span className={`pull-status ${entry.state}`}>
                {entry.state}
              </span>
              <div>
                <Link
                  href={`/repositories/${repository.id}?view=pulls&pull=${entry.pull_request_id}`}
                >
                  {title(entry.pull_request_id)}
                </Link>
                <small>
                  {entry.blocker
                    ? entry.blocker.replaceAll("_", " ")
                    : `Candidate ${short(entry.candidate_commit_id)}`}{" "}
                  · operated by{" "}
                  <Actor
                    id={entry.events.at(-1)?.actor_id || entry.enqueued_by_id}
                    compact
                  />
                </small>
              </div>
              <time>
                {new Date(
                  entry.completed_at ?? entry.created_at,
                ).toLocaleString()}
              </time>
            </article>
          ))}
        </section>
      )}
    </section>
  );
}

function ProposalWorkspace({
  repository,
  branches,
  owner,
  selected,
  initialState,
  initialQuery,
}: {
  repository: Repository;
  branches: BranchRecord[];
  owner: string;
  selected?: string;
  initialState?: string;
  initialQuery?: string;
}) {
  const router = useRouter();
  const state =
    initialState === "closed"
      ? "closed"
      : initialState === "all"
        ? "all"
        : "open";
  const [query, setQuery] = useState(initialQuery ?? "");
  const [items, setItems] = useState<Proposal[]>([]);
  const [proposal, setProposal] = useState<Proposal>();
  const [comments, setComments] = useState<ProposalComment[]>([]);
  const [plan, setPlan] = useState<ProposalPlan>();
  const [actor, setActor] = useState("");
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState(false);
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const session = fetch("/api/session")
        .then(async (response) =>
          response.ok
            ? ((await response.json()) as { user: { id: string } }).user.id
            : "",
        )
        .catch(() => "");
      if (selected) {
        const [item, conversation, proposalPlan, user] = await Promise.all([
          get<Proposal>(`/repositories/${repository.id}/proposals/${selected}`),
          get<CommentList>(
            `/repositories/${repository.id}/proposals/${selected}/comments?per_page=100`,
          ),
          get<ProposalPlan>(
            `/repositories/${repository.id}/proposals/${selected}/plan`,
          ),
          session,
        ]);
        setProposal(item);
        setComments(conversation.items);
        setPlan(proposalPlan);
        setActor(user);
      } else {
        const suffix = state === "all" ? "" : `&state=${state}`;
        const [list, user] = await Promise.all([
          get<ProposalList>(
            `/repositories/${repository.id}/proposals?per_page=100${suffix}`,
          ),
          session,
        ]);
        setItems(list.items);
        setProposal(undefined);
        setActor(user);
      }
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Proposals unavailable.",
      );
    } finally {
      setLoading(false);
    }
  }, [repository.id, selected, state]);
  useEffect(() => {
    // Proposal state follows the shareable state/query/detail URL.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);
  function go(next: { proposal?: string; state?: string; q?: string }) {
    const values = new URLSearchParams({ view: "proposals" });
    const nextState = next.state ?? state;
    if (nextState !== "open") values.set("state", nextState);
    const nextQuery = next.q ?? query;
    if (nextQuery) values.set("q", nextQuery);
    if (next.proposal) values.set("proposal", next.proposal);
    router.push(`/repositories/${repository.id}?${values}`);
  }
  const filtered = items.filter(
    (item) =>
      !query ||
      `${item.title} ${item.body}`.toLowerCase().includes(query.toLowerCase()),
  );
  if (loading)
    return <div className="repository-loading">Reading proposal context…</div>;
  if (error)
    return (
      <section className="proposal-notice panel">
        <Lightbulb />
        <h2>Proposals unavailable</h2>
        <p>{error}</p>
        <Button variant="secondary" onClick={() => void load()}>
          Try again
        </Button>
      </section>
    );
  if (proposal)
    return (
      <ProposalDetail
        item={proposal}
        comments={comments}
        plan={plan ?? { proposal_id: proposal.id, tasks: [], history: [] }}
        repository={repository.id}
        assignmentRepository={repository}
        branches={branches}
        actor={actor}
        canDiscuss={Boolean(actor)}
        canPlan={Boolean(actor)}
        canEdit={actor === proposal.author_id || actor === owner}
        onBack={() => go({ state })}
        onChanged={() => void load()}
        editing={editing}
        setEditing={setEditing}
      />
    );
  return (
    <section className="proposal-workspace">
      <header className="proposal-toolbar">
        <div>
          <p className="eyebrow">
            <Lightbulb size={14} />
            Shared context
          </p>
          <h2>Proposals</h2>
          <p>
            Search what collaborators have already explored before starting
            overlapping work.
          </p>
        </div>
        {actor && (
          <Button onClick={() => setCreating((value) => !value)}>
            <Plus size={14} />
            {creating ? "Cancel" : "New proposal"}
          </Button>
        )}
      </header>
      {creating && (
        <ProposalForm
          title="Start with the problem"
          submit="Create proposal"
          onCancel={() => setCreating(false)}
          onSubmit={async (title, body) => {
            const created = await send<Proposal>(
              `/repositories/${repository.id}/proposals`,
              "POST",
              { title, body },
            );
            go({ proposal: created.id });
          }}
        />
      )}
      <div className="proposal-filters panel">
        <label>
          <span className="sr-only">Search proposals</span>
          <input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            onBlur={() => go({ q: query })}
            placeholder="Search titles and descriptions…"
          />
        </label>
        <div role="group" aria-label="Proposal state">
          {(["open", "closed", "all"] as const).map((value) => (
            <button
              key={value}
              className={state === value ? "active" : ""}
              onClick={() => go({ state: value, q: query })}
            >
              {value[0].toUpperCase() + value.slice(1)}
            </button>
          ))}
        </div>
      </div>
      <div className="proposal-list panel">
        {filtered.length ? (
          filtered.map((item) => (
            <button key={item.id} onClick={() => go({ proposal: item.id })}>
              <span className={`proposal-state ${item.state}`}>
                <Lightbulb size={16} />
              </span>
              <span>
                <strong>{item.title}</strong>
                <p>{item.body || "No description was added."}</p>
                <small>
                  Opened {new Date(item.created_at).toLocaleDateString()} ·
                  updated {new Date(item.updated_at).toLocaleDateString()}
                </small>
              </span>
              <ChevronRight size={16} />
            </button>
          ))
        ) : (
          <div className="proposal-empty">
            <Lightbulb />
            <h3>{query ? "No matching context" : "No proposals here yet"}</h3>
            <p>
              {query
                ? "Try another phrase or state filter."
                : "Start a proposal so the next collaborator can discover the reasoning before duplicating work."}
            </p>
          </div>
        )}
      </div>
    </section>
  );
}

function ProposalForm({
  title,
  body = "",
  submit,
  onCancel,
  onSubmit,
}: {
  title: string;
  body?: string;
  submit: string;
  onCancel: () => void;
  onSubmit: (title: string, body: string) => Promise<void>;
}) {
  const [name, setName] = useState(title);
  const [description, setDescription] = useState(body);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  return (
    <form
      className="proposal-form panel"
      onSubmit={async (event) => {
        event.preventDefault();
        setBusy(true);
        setError("");
        try {
          await onSubmit(name, description);
        } catch (cause) {
          setError(
            cause instanceof Error ? cause.message : "Unable to save proposal.",
          );
          setBusy(false);
        }
      }}
    >
      <label>
        Title
        <input
          required
          maxLength={200}
          value={name}
          onChange={(event) => setName(event.target.value)}
          placeholder="What should collaborators consider?"
        />
      </label>
      <label>
        Description
        <textarea
          maxLength={65536}
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          placeholder="Describe the motivation, constraints, and desired outcome."
        />
      </label>
      {error && <p className="form-error">{error}</p>}
      <div className="form-actions">
        <Button type="button" variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="submit" disabled={busy}>
          {busy ? "Saving…" : submit}
        </Button>
      </div>
    </form>
  );
}

function ProposalDetail({
  item,
  comments,
  plan,
  repository,
  assignmentRepository,
  branches,
  actor,
  canDiscuss,
  canPlan,
  canEdit,
  onBack,
  onChanged,
  editing,
  setEditing,
}: {
  item: Proposal;
  comments: ProposalComment[];
  plan: ProposalPlan;
  repository: string;
  assignmentRepository: Repository;
  branches: BranchRecord[];
  actor: string;
  canDiscuss: boolean;
  canPlan: boolean;
  canEdit: boolean;
  onBack: () => void;
  onChanged: () => void;
  editing: boolean;
  setEditing: (value: boolean) => void;
}) {
  const [comment, setComment] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [pendingDecisions, setPendingDecisions] = useState<
    Array<{ id: string; title: string; scope: { owner_id: string } }>
  >([]);
  useEffect(() => {
    // Related decisions are advisory context and never gate proposal work.
    fetch(
      `/api/repositories/${repository}/decisions?context_kind=proposal&context_id=${item.id}`,
    )
      .then((response) => (response.ok ? response.json() : { items: [] }))
      .then(
        (body: {
          items: Array<{
            id: string;
            title: string;
            state: string;
            scope: { owner_id: string };
          }>;
        }) =>
          setPendingDecisions(
            body.items.filter((value) => value.state === "pending"),
          ),
      )
      .catch(() => undefined);
  }, [item.id, repository]);
  return (
    <section className="proposal-detail">
      <button className="proposal-back" onClick={onBack}>
        ← All proposals
      </button>
      <header className="proposal-detail-heading">
        <div>
          <span className={`proposal-status ${item.state}`}>
            <Lightbulb size={14} />
            {item.state}
          </span>
          <h2>{item.title}</h2>
          <p>
            Opened {new Date(item.created_at).toLocaleString()} by{" "}
            <Actor id={item.author_id} />
          </p>
        </div>
        {item.state === "open" && canEdit && (
          <div>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setEditing(!editing)}
            >
              <Edit size={13} />
              Edit
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={async () => {
                setError("");
                try {
                  await send(
                    `/repositories/${repository}/proposals/${item.id}`,
                    "PATCH",
                    { state: "closed" },
                  );
                  onChanged();
                } catch (cause) {
                  setError(
                    cause instanceof Error
                      ? cause.message
                      : "Unable to close proposal.",
                  );
                }
              }}
            >
              <Check size={13} />
              Close
            </Button>
          </div>
        )}
      </header>
      {pendingDecisions.length > 0 && (
        <aside className="panel">
          <Badge>{pendingDecisions.length} pending decision</Badge>
          <p>
            This proposal has a consequential choice under discussion. Work may
            continue where it does not depend on that choice.
          </p>
          {pendingDecisions.map((decision) => (
            <Link
              key={decision.id}
              href={`/repositories/${repository}?view=decisions&decision=${decision.id}`}
            >
              {decision.title} · owner {decision.scope.owner_id}
            </Link>
          ))}
        </aside>
      )}
      {error && <p className="form-error">{error}</p>}
      {editing ? (
        <ProposalForm
          title={item.title}
          body={item.body}
          submit="Save changes"
          onCancel={() => setEditing(false)}
          onSubmit={async (title, body) => {
            await send(
              `/repositories/${repository}/proposals/${item.id}`,
              "PATCH",
              { title, body },
            );
            setEditing(false);
            onChanged();
          }}
        />
      ) : (
        <article className="proposal-body panel">
          <div className="proposal-author">
            <span className="avatar sm">
              {item.author_id.slice(0, 2).toUpperCase()}
            </span>
            <span>
              <strong>Proposal author</strong>
              <small>{new Date(item.created_at).toLocaleString()}</small>
            </span>
          </div>
          <div>
            {item.body ? (
              <p>{item.body}</p>
            ) : (
              <p className="muted">No description was added.</p>
            )}
          </div>
        </article>
      )}
      <ProposalPlanView
        repository={repository}
        assignmentRepository={assignmentRepository}
        branches={branches}
        actor={actor}
        proposal={item.id}
        plan={plan}
        comments={comments}
        canEdit={canPlan}
        onChanged={onChanged}
      />
      <div className="conversation-heading">
        <MessageCircle size={16} />
        <h3>Conversation</h3>
        <span>{comments.length}</span>
      </div>
      <div className="proposal-comments">
        {comments.map((entry) => (
          <article className="panel" key={entry.id}>
            <div className="proposal-author">
              <span className="avatar sm">
                <User size={14} />
              </span>
              <span>
                <strong>
                  <Actor id={entry.author_id} />
                </strong>
                <small>{new Date(entry.created_at).toLocaleString()}</small>
              </span>
            </div>
            <p>{entry.body}</p>
          </article>
        ))}
        {!comments.length && (
          <p className="no-comments">
            No replies yet. Add a question or context that moves the idea
            forward.
          </p>
        )}
      </div>
      {item.state === "open" && canDiscuss ? (
        <form
          className="comment-form panel"
          onSubmit={async (event) => {
            event.preventDefault();
            setBusy(true);
            setError("");
            try {
              await send(
                `/repositories/${repository}/proposals/${item.id}/comments`,
                "POST",
                { body: comment },
              );
              setComment("");
              onChanged();
            } catch (cause) {
              setError(
                cause instanceof Error
                  ? cause.message
                  : "Unable to post comment.",
              );
            } finally {
              setBusy(false);
            }
          }}
        >
          <label>
            <strong>Join the conversation</strong>
            <textarea
              required
              value={comment}
              onChange={(event) => setComment(event.target.value)}
              placeholder="Ask a question, share a constraint, or suggest a next step."
            />
          </label>
          <div className="form-actions">
            <Button type="submit" disabled={busy}>
              {busy ? "Posting…" : "Comment"}
            </Button>
          </div>
        </form>
      ) : item.state === "closed" ? (
        <div className="closed-note panel">
          <Check size={16} />
          <span>
            <strong>This proposal is closed.</strong> Its context and
            conversation remain available for future work.
          </span>
        </div>
      ) : (
        <div className="closed-note panel">
          <MessageCircle size={16} />
          <span>
            <strong>Sign in to participate.</strong> Public context remains
            available to everyone.
          </span>
        </div>
      )}
    </section>
  );
}

function ProposalPlanView({
  repository,
  assignmentRepository,
  branches,
  actor,
  proposal,
  plan,
  comments,
  canEdit,
  onChanged,
}: {
  repository: string;
  assignmentRepository: Repository;
  branches: BranchRecord[];
  actor: string;
  proposal: string;
  plan: ProposalPlan;
  comments: ProposalComment[];
  canEdit: boolean;
  onChanged: () => void;
}) {
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<string>();
  const [historyOpen, setHistoryOpen] = useState(false);
  const [assigning, setAssigning] = useState<string>();
  const [publishing, setPublishing] = useState<string>();
  const [error, setError] = useState("");
  const completed = plan.tasks.filter(
    (task) => task.status === "completed",
  ).length;
  async function update(task: ProposalTask, changes: Partial<ProposalTask>) {
    setError("");
    try {
      await send(
        `/repositories/${repository}/proposals/${proposal}/plan/tasks/${task.id}`,
        "PATCH",
        {
          title: changes.title ?? task.title,
          outcome: changes.outcome ?? task.outcome,
          position: changes.position ?? task.position,
          status: changes.status ?? task.status,
          depends_on: changes.depends_on ?? task.depends_on,
          discussion_comment_ids:
            changes.discussion_comment_ids ?? task.discussion_comment_ids,
        },
      );
      setEditing(undefined);
      onChanged();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Unable to update task.",
      );
    }
  }
  return (
    <section className="proposal-plan">
      <header className="plan-heading">
        <div>
          <p className="eyebrow">
            <Check size={14} />
            Executable plan
          </p>
          <h3>Delivery tasks</h3>
          <p>
            {plan.tasks.length
              ? `${completed} of ${plan.tasks.length} outcomes completed`
              : "Turn the agreed idea into work collaborators can start."}
          </p>
        </div>
        <div>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => setHistoryOpen(!historyOpen)}
          >
            <Clock size={13} /> History ({plan.history.length})
          </Button>
          {canEdit && (
            <Button size="sm" onClick={() => setCreating(!creating)}>
              <Plus size={13} /> Add task
            </Button>
          )}
        </div>
      </header>
      {error && <p className="form-error">{error}</p>}
      {creating && (
        <TaskForm
          tasks={plan.tasks}
          comments={comments}
          submit="Add task"
          onCancel={() => setCreating(false)}
          onSubmit={async (input) => {
            await send(
              `/repositories/${repository}/proposals/${proposal}/plan/tasks`,
              "POST",
              input,
            );
            setCreating(false);
            onChanged();
          }}
        />
      )}
      <div className="plan-tasks">
        {plan.tasks.map((task) =>
          editing === task.id ? (
            <TaskForm
              key={task.id}
              task={task}
              tasks={plan.tasks}
              comments={comments}
              submit="Save task"
              onCancel={() => setEditing(undefined)}
              onSubmit={(input) => update(task, input as Partial<ProposalTask>)}
            />
          ) : (
            <article
              className={`plan-task panel ${task.ready ? "ready" : ""}`}
              key={task.id}
            >
              <span className="task-position">{task.position}</span>
              <div className="task-content">
                <div className="task-title">
                  <strong>{task.title}</strong>
                  <span className={`task-status ${task.status}`}>
                    {task.status.replace("_", " ")}
                  </span>
                  {task.ready && (
                    <span className="ready-label">Can start now</span>
                  )}
                </div>
                <p>
                  <b>Expected outcome</b>
                  {task.outcome}
                </p>
                {task.depends_on.length > 0 && (
                  <small>
                    After{" "}
                    {task.depends_on
                      .map(
                        (id) =>
                          plan.tasks.find((candidate) => candidate.id === id)
                            ?.title ?? id,
                      )
                      .join(", ")}
                  </small>
                )}
                {task.discussion_comment_ids.length > 0 && (
                  <small>
                    <MessageCircle size={11} /> Linked to{" "}
                    {task.discussion_comment_ids.length} discussion{" "}
                    {task.discussion_comment_ids.length === 1
                      ? "decision"
                      : "decisions"}
                  </small>
                )}
                <small>
                  Last changed by <Actor id={task.updated_by_id} /> ·{" "}
                  {new Date(task.updated_at).toLocaleString()}
                </small>
                {task.assignment && (
                  <div className="task-assignment">
                    <strong>
                      {task.assignment.kind === "agent" ? (
                        task.assignment.assignee_id
                      ) : (
                        <Actor id={task.assignment.assignee_id} />
                      )}{" "}
                      owns this task
                    </strong>
                    <span>{task.assignment.mandate}</span>
                    <small>
                      Base <code>{short(task.assignment.base_revision)}</code>{" "}
                      in this repository ·{" "}
                      {task.assignment.permissions.join(" + ")} · no credential
                      issued
                    </small>
                    <small>
                      Assigned by <Actor id={task.assignment.assigned_by_id} />{" "}
                      · {new Date(task.assignment.assigned_at).toLocaleString()}
                    </small>
                  </div>
                )}
                {task.contributions?.map((contribution) => (
                  <div
                    className="task-assignment"
                    key={contribution.pull_request_id}
                  >
                    <strong>
                      <Link
                        href={`/repositories/${repository}?view=pulls&pull=${contribution.pull_request_id}`}
                      >
                        Pull request {short(contribution.pull_request_id)}
                      </Link>{" "}
                      · {contribution.status}
                    </strong>
                    <span>
                      Exact candidate{" "}
                      <code>{short(contribution.source_commit_id)}</code>
                      {contribution.session_id
                        ? " with linked execution evidence"
                        : ""}
                    </span>
                    <small>
                      Published by <Actor id={contribution.published_by_id} /> ·{" "}
                      {new Date(contribution.published_at).toLocaleString()}
                    </small>
                  </div>
                ))}
                {assigning === task.id && (
                  <TaskAssignmentForm
                    task={task}
                    repository={assignmentRepository}
                    branches={branches}
                    actor={actor}
                    onCancel={() => setAssigning(undefined)}
                    onAssigned={() => {
                      setAssigning(undefined);
                      onChanged();
                    }}
                  />
                )}
                {publishing === task.id && (
                  <TaskContributionForm
                    task={task}
                    repository={repository}
                    branches={branches}
                    onCancel={() => setPublishing(undefined)}
                    onPublished={() => {
                      setPublishing(undefined);
                      onChanged();
                    }}
                  />
                )}
              </div>
              {canEdit && (
                <div className="task-actions">
                  {task.ready && (
                    <button
                      aria-label={`${task.assignment ? "Reassign" : "Assign"} ${task.title}`}
                      onClick={() =>
                        setAssigning(
                          assigning === task.id ? undefined : task.id,
                        )
                      }
                    >
                      <User size={13} />
                    </button>
                  )}
                  {task.assignment?.kind === "human" &&
                    task.assignment.assignee_id === actor && (
                      <button
                        aria-label={`Publish ${task.title} for review`}
                        onClick={() =>
                          setPublishing(
                            publishing === task.id ? undefined : task.id,
                          )
                        }
                      >
                        <GitPullRequest size={13} />
                      </button>
                    )}
                  {task.assignment && (
                    <button
                      aria-label={`Revoke assignment for ${task.title}`}
                      onClick={async () => {
                        try {
                          await send(
                            `/repositories/${repository}/proposals/${proposal}/plan/tasks/${task.id}/assignment?expected_assignment_id=${task.assignment?.id}`,
                            "DELETE",
                          );
                          onChanged();
                        } catch (cause) {
                          setError(
                            cause instanceof Error
                              ? cause.message
                              : "Unable to revoke assignment.",
                          );
                        }
                      }}
                    >
                      <Trash size={13} />
                    </button>
                  )}
                  {![
                    "draft",
                    "review",
                    "merged",
                    "closed",
                    "superseded",
                  ].includes(task.status) && (
                    <select
                      aria-label={`Status for ${task.title}`}
                      value={task.status}
                      onChange={(event) =>
                        void update(task, {
                          status: event.target.value as ProposalTask["status"],
                        })
                      }
                    >
                      <option value="planned">Planned</option>
                      <option value="in_progress">In progress</option>
                      <option value="completed">Completed</option>
                      <option value="canceled">Canceled</option>
                    </select>
                  )}
                  <button
                    aria-label={`Edit ${task.title}`}
                    onClick={() => setEditing(task.id)}
                  >
                    <Edit size={13} />
                  </button>
                  <button
                    aria-label={`Move ${task.title} up`}
                    disabled={task.position === 1}
                    onClick={() =>
                      void update(task, { position: task.position - 1 })
                    }
                  >
                    ↑
                  </button>
                  <button
                    aria-label={`Move ${task.title} down`}
                    disabled={task.position === plan.tasks.length}
                    onClick={() =>
                      void update(task, { position: task.position + 1 })
                    }
                  >
                    ↓
                  </button>
                </div>
              )}
            </article>
          ),
        )}
        {!plan.tasks.length && (
          <div className="plan-empty panel">
            <Check />
            <h4>No delivery tasks yet</h4>
            <p>
              Add outcomes in dependency order so the next collaborator knows
              where to begin.
            </p>
          </div>
        )}
      </div>
      {historyOpen && (
        <div className="plan-history panel">
          <h4>Plan history</h4>
          {[...plan.history].reverse().map((event) => (
            <div key={event.id}>
              <span className="avatar sm">
                <User size={12} />
              </span>
              <p>
                <strong>
                  <Actor id={event.actor_id} />
                </strong>{" "}
                {event.action.replace("task.", "").replaceAll("_", " ")}{" "}
                <b>{event.task.title}</b>
                <small>
                  {event.task.status.replace("_", " ")} ·{" "}
                  {new Date(event.created_at).toLocaleString()}
                </small>
              </p>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function TaskContributionForm({
  task,
  repository,
  branches,
  onCancel,
  onPublished,
}: {
  task: ProposalTask;
  repository: string;
  branches: BranchRecord[];
  onCancel: () => void;
  onPublished: () => void;
}) {
  const defaultBranch =
    branches.find((branch) => branch.is_default)?.name ?? "main";
  const candidates = branches.filter((branch) => branch.name !== defaultBranch);
  const [source, setSource] = useState(
    task.assignment?.working_branch ?? candidates[0]?.name ?? "",
  );
  const [title, setTitle] = useState(task.title);
  const [body, setBody] = useState(`Delivers proposal task: ${task.outcome}`);
  const [draft, setDraft] = useState(false);
  const stewarded = task.reasoning_context?.kind === "stewardship_opportunity";
  const [reasoning, setReasoning] = useState("");
  const [commands, setCommands] = useState("");
  const [risks, setRisks] = useState("");
  const [criteria, setCriteria] = useState<Record<string, { status: string; evidence: string }>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  return (
    <form
      className="assignment-form"
      onSubmit={async (event) => {
        event.preventDefault();
        setBusy(true);
        setError("");
        try {
          await send(
            `/repositories/${repository}/proposals/${task.proposal_id}/plan/tasks/${task.id}/contributions`,
            "POST",
            {
              expected_assignment_id: task.assignment?.id,
              source_branch: source,
              target_branch: defaultBranch,
              title,
              body,
              draft,
              session_id: task.assignment?.session_id,
              delivery_evidence: stewarded ? {
                reasoning,
                commands: commands.split("\n").map((value) => value.trim()).filter(Boolean),
                residual_risks: risks.split("\n").map((value) => value.trim()).filter(Boolean),
                completion_criteria: (task.completion_criteria ?? []).map((criterion) => ({ criterion, status: criteria[criterion]?.status ?? "", evidence: criteria[criterion]?.evidence ?? "" })),
              } : undefined,
            },
          );
          onPublished();
        } catch (cause) {
          setError(
            cause instanceof Error
              ? cause.message
              : "Unable to publish task work.",
          );
          setBusy(false);
        }
      }}
    >
      <div>
        <label>
          Candidate branch
          <select
            required
            value={source}
            onChange={(event) => setSource(event.target.value)}
          >
            {candidates.map((branch) => (
              <option key={branch.name} value={branch.name}>
                {branch.name} · {short(branch.commit_id)}
              </option>
            ))}
          </select>
        </label>
        <label>
          Pull request title
          <input
            required
            maxLength={200}
            value={title}
            onChange={(event) => setTitle(event.target.value)}
          />
        </label>
      </div>
      <label>
        Review context
        <textarea
          maxLength={65536}
          value={body}
          onChange={(event) => setBody(event.target.value)}
        />
      </label>
      {stewarded && <fieldset className="stack"><legend>Stewardship delivery evidence</legend><p>Retain the decisions a reviewer needs to verify this opportunity became trustworthy work.</p><label>Implementation reasoning<textarea required value={reasoning} onChange={(event) => setReasoning(event.target.value)} /></label><label>Commands run · one per line<textarea required value={commands} onChange={(event) => setCommands(event.target.value)} /></label><label>Residual risks · one per line<textarea value={risks} onChange={(event) => setRisks(event.target.value)} /></label>{(task.completion_criteria ?? []).map((criterion) => <div key={criterion} className="content-grid"><strong>{criterion}</strong><label>Status<select required value={criteria[criterion]?.status ?? ""} onChange={(event) => setCriteria((current) => ({ ...current, [criterion]: { status: event.target.value, evidence: current[criterion]?.evidence ?? "" } }))}><option value="">Choose…</option><option value="met">Met</option><option value="unmet">Unmet</option><option value="not_applicable">Not applicable</option></select></label><label>Evidence<textarea required value={criteria[criterion]?.evidence ?? ""} onChange={(event) => setCriteria((current) => ({ ...current, [criterion]: { status: current[criterion]?.status ?? "", evidence: event.target.value } }))} /></label></div>)}</fieldset>}
      <label>
        <input
          type="checkbox"
          checked={draft}
          onChange={(event) => setDraft(event.target.checked)}
        />{" "}
        Publish as draft
      </label>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <div className="form-actions">
        <Button type="button" variant="secondary" size="sm" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="submit" size="sm" disabled={busy || !source}>
          {busy ? "Publishing…" : draft ? "Publish draft" : "Request review"}
        </Button>
      </div>
    </form>
  );
}

function TaskForm({
  task,
  tasks,
  comments,
  submit,
  onCancel,
  onSubmit,
}: {
  task?: ProposalTask;
  tasks: ProposalTask[];
  comments: ProposalComment[];
  submit: string;
  onCancel: () => void;
  onSubmit: (input: Record<string, unknown>) => Promise<void>;
}) {
  const [title, setTitle] = useState(task?.title ?? "");
  const [outcome, setOutcome] = useState(task?.outcome ?? "");
  const [dependencies, setDependencies] = useState(task?.depends_on ?? []);
  const [links, setLinks] = useState(task?.discussion_comment_ids ?? []);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const choices = tasks.filter((candidate) => candidate.id !== task?.id);
  return (
    <form
      className="task-form panel"
      onSubmit={async (event) => {
        event.preventDefault();
        setBusy(true);
        setError("");
        try {
          await onSubmit({
            title,
            outcome,
            position: task?.position ?? tasks.length + 1,
            status: task?.status ?? "planned",
            depends_on: dependencies,
            discussion_comment_ids: links,
          });
        } catch (cause) {
          setError(
            cause instanceof Error ? cause.message : "Unable to save task.",
          );
          setBusy(false);
        }
      }}
    >
      <label>
        Task
        <input
          required
          maxLength={200}
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="What work should happen?"
        />
      </label>
      <label>
        Expected outcome
        <textarea
          required
          maxLength={4096}
          value={outcome}
          onChange={(event) => setOutcome(event.target.value)}
          placeholder="Describe the observable result, not just the activity."
        />
      </label>
      {choices.length > 0 && (
        <fieldset>
          <legend>Depends on</legend>
          {choices.map((candidate) => (
            <label key={candidate.id}>
              <input
                type="checkbox"
                checked={dependencies.includes(candidate.id)}
                onChange={() =>
                  setDependencies((current) =>
                    current.includes(candidate.id)
                      ? current.filter((id) => id !== candidate.id)
                      : [...current, candidate.id],
                  )
                }
              />
              {candidate.position}. {candidate.title}
            </label>
          ))}
        </fieldset>
      )}
      {comments.length > 0 && (
        <fieldset>
          <legend>Motivating discussion</legend>
          {comments.map((comment, index) => (
            <label key={comment.id}>
              <input
                type="checkbox"
                checked={links.includes(comment.id)}
                onChange={() =>
                  setLinks((current) =>
                    current.includes(comment.id)
                      ? current.filter((id) => id !== comment.id)
                      : [...current, comment.id],
                  )
                }
              />
              Reply {index + 1} by <Actor id={comment.author_id} />
            </label>
          ))}
        </fieldset>
      )}
      {error && <p className="form-error">{error}</p>}
      <div className="form-actions">
        <Button type="button" variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="submit" disabled={busy}>
          {busy ? "Saving…" : submit}
        </Button>
      </div>
    </form>
  );
}

function TaskAssignmentForm({
  task,
  repository,
  branches,
  actor,
  onCancel,
  onAssigned,
}: {
  task: ProposalTask;
  repository: Repository;
  branches: BranchRecord[];
  actor: string;
  onCancel: () => void;
  onAssigned: () => void;
}) {
  const [kind, setKind] = useState<"human" | "agent">(
    task.assignment?.kind ?? "human",
  );
  const members = Array.from(
    new Set(
      [
        repository.owner_id,
        ...(repository.collaborator_ids ?? []),
        actor,
      ].filter(Boolean),
    ),
  );
  const [assignee, setAssignee] = useState(
    task.assignment?.assignee_id ?? actor,
  );
  const [revision, setRevision] = useState(
    task.assignment?.base_revision ??
      branches.find((branch) => branch.is_default)?.commit_id ??
      branches[0]?.commit_id ??
      "",
  );
  const [mandate, setMandate] = useState(
    task.assignment?.mandate ?? task.outcome,
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  return (
    <form
      className="assignment-form"
      onSubmit={async (event) => {
        event.preventDefault();
        setBusy(true);
        setError("");
        try {
          await send(
            `/repositories/${repository.id}/proposals/${task.proposal_id}/plan/tasks/${task.id}/assignment`,
            "PUT",
            {
              kind,
              assignee_id: kind === "agent" ? "codex" : assignee,
              mandate,
              repository_id: repository.id,
              base_revision: revision,
              expected_assignment_id: task.assignment?.id ?? "",
            },
          );
          onAssigned();
        } catch (cause) {
          setError(
            cause instanceof Error ? cause.message : "Unable to assign task.",
          );
          setBusy(false);
        }
      }}
    >
      <div>
        <label>
          Owner type
          <select
            value={kind}
            onChange={(event) => {
              const next = event.target.value as "human" | "agent";
              setKind(next);
              setAssignee(next === "agent" ? "codex" : actor);
            }}
          >
            <option value="human">Human collaborator</option>
            <option value="agent">Available agent</option>
          </select>
        </label>
        <label>
          Accountable owner
          <select
            value={kind === "agent" ? "codex" : assignee}
            onChange={(event) => setAssignee(event.target.value)}
          >
            {kind === "agent" ? (
              <option value="codex">Codex</option>
            ) : (
              members.map((id) => (
                <option key={id} value={id}>
                  {id === actor ? "Claim for me" : id}
                </option>
              ))
            )}
          </select>
        </label>
        <label>
          Base revision
          <select
            required
            value={revision}
            onChange={(event) => setRevision(event.target.value)}
          >
            {branches.map((branch) => (
              <option key={branch.name} value={branch.commit_id}>
                {branch.name} · {short(branch.commit_id)}
              </option>
            ))}
          </select>
        </label>
      </div>
      <label>
        Mandate
        <textarea
          required
          maxLength={4096}
          value={mandate}
          onChange={(event) => setMandate(event.target.value)}
        />
      </label>
      <p>
        <strong>Authority preview:</strong> read repository contents and write
        only the future candidate branch created for this task. No credential is
        issued until work starts.
      </p>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <div className="form-actions">
        <Button type="button" variant="secondary" size="sm" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="submit" size="sm" disabled={busy || !revision}>
          {busy
            ? "Assigning…"
            : task.assignment
              ? "Reassign"
              : kind === "human" && assignee === actor
                ? "Claim task"
                : "Assign task"}
        </Button>
      </div>
    </form>
  );
}

function PullRequestWorkspace({
  repository,
  branches,
  selected,
  section,
}: {
  repository: string;
  branches: BranchRecord[];
  selected?: string;
  section?: string;
}) {
  const router = useRouter();
  const [items, setItems] = useState<PullRequest[]>([]);
  const [actor, setActor] = useState("");
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const session = fetch("/api/session")
        .then(async (response) =>
          response.ok
            ? ((await response.json()) as { user: { id: string } }).user.id
            : "",
        )
        .catch(() => "");
      const [list, user] = await Promise.all([
        get<PullRequestList>(
          `/repositories/${repository}/pull-requests?per_page=100`,
        ),
        session,
      ]);
      setItems(list.items);
      setActor(user);
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Pull requests unavailable.",
      );
    } finally {
      setLoading(false);
    }
  }, [repository]);
  useEffect(() => {
    // Pull request discovery follows the repository tab while detail context stays in the URL.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);
  function go(pull?: string, nextSection?: string) {
    const values = new URLSearchParams({ view: "pulls" });
    if (pull) values.set("pull", pull);
    if (pull && nextSection && nextSection !== "overview")
      values.set("section", nextSection);
    router.push(`/repositories/${repository}?${values}`);
  }
  if (loading)
    return <div className="repository-loading">Reading candidate changes…</div>;
  if (error)
    return (
      <section className="proposal-notice panel">
        <GitPullRequest />
        <h2>Pull requests unavailable</h2>
        <p>{error}</p>
        <Button variant="secondary" onClick={() => void load()}>
          Try again
        </Button>
      </section>
    );
  if (selected)
    return (
      <PullRequestDetail
        repository={repository}
        id={selected}
        branches={branches}
        actor={actor}
        section={section}
        onBack={() => go()}
        onSection={(value) => go(selected, value)}
      />
    );
  const open = items.filter((item) => item.status === "open");
  const merged = items.filter((item) => item.status === "merged");
  return (
    <section className="pull-workspace">
      <header className="proposal-toolbar">
        <div>
          <p className="eyebrow">
            <GitPullRequest size={14} />
            Reviewable work
          </p>
          <h2>Pull requests</h2>
          <p>
            Bring a published branch, its purpose, and the feedback it needs
            into one reviewable record.
          </p>
        </div>
        {actor && branches.length > 1 && (
          <Button onClick={() => setCreating((value) => !value)}>
            <Plus size={14} />
            {creating ? "Cancel" : "New pull request"}
          </Button>
        )}
      </header>
      {creating && (
        <PullRequestForm
          repository={repository}
          branches={branches}
          onCancel={() => setCreating(false)}
          onCreated={(item) => go(item.id)}
        />
      )}
      {!actor && (
        <div className="closed-note panel">
          <GitPullRequest size={16} />
          <span>
            <strong>Sign in to propose changes.</strong> Public pull requests
            and their discussion remain readable.
          </span>
        </div>
      )}
      {actor && branches.length < 2 && (
        <div className="closed-note panel">
          <Branch size={16} />
          <span>
            <strong>Publish a candidate branch first.</strong> A pull request
            compares two different branches that already exist on the remote.
          </span>
        </div>
      )}
      <div className="pull-list panel">
        {items.length ? (
          <>
            <PullRequestGroup
              title="Open for feedback"
              items={open}
              onSelect={(id) => go(id)}
            />
            <PullRequestGroup
              title="Merged work"
              items={merged}
              onSelect={(id) => go(id)}
            />
          </>
        ) : (
          <div className="proposal-empty">
            <GitPullRequest />
            <h3>No pull requests yet</h3>
            <p>
              Once a candidate branch is published, open a pull request to make
              its intent, changes, and questions reviewable.
            </p>
          </div>
        )}
      </div>
    </section>
  );
}

function PullRequestGroup({
  title,
  items,
  onSelect,
}: {
  title: string;
  items: PullRequest[];
  onSelect: (id: string) => void;
}) {
  if (!items.length) return null;
  return (
    <section>
      <h3>
        {title}
        <span>{items.length}</span>
      </h3>
      {items.map((item) => (
        <button key={item.id} onClick={() => onSelect(item.id)}>
          <span className={`pull-state ${item.status}`}>
            <GitPullRequest size={16} />
          </span>
          <span>
            <strong>{item.title}</strong>
            <p>{item.body || "No description was added."}</p>
            <small>
              {item.source_repository_id !== item.repository_id && (
                <>
                  fork <code>{short(item.source_repository_id)}</code> ·{" "}
                </>
              )}
              <code>{item.source_branch}</code> into{" "}
              <code>{item.target_branch}</code> · opened{" "}
              {new Date(item.created_at).toLocaleDateString()}
              {item.proposal_id ? " · linked proposal" : ""}
            </small>
          </span>
          <ChevronRight size={16} />
        </button>
      ))}
    </section>
  );
}

function PullRequestForm({
  repository,
  branches,
  onCancel,
  onCreated,
}: {
  repository: string;
  branches: BranchRecord[];
  onCancel: () => void;
  onCreated: (item: PullRequest) => void;
}) {
  const defaultBranch =
    branches.find((branch) => branch.is_default)?.name ??
    branches[0]?.name ??
    "";
  const candidate =
    branches.find((branch) => branch.name !== defaultBranch)?.name ?? "";
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [source, setSource] = useState(candidate);
  const [target, setTarget] = useState(defaultBranch);
  const [proposal, setProposal] = useState("");
  const [proposals, setProposals] = useState<Proposal[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  useEffect(() => {
    get<ProposalList>(
      `/repositories/${repository}/proposals?per_page=100&state=open`,
    )
      .then((result) => setProposals(result.items))
      .catch(() => setProposals([]));
  }, [repository]);
  return (
    <form
      className="pull-form panel"
      onSubmit={async (event) => {
        event.preventDefault();
        setBusy(true);
        setError("");
        try {
          onCreated(
            await send<PullRequest>(
              `/repositories/${repository}/pull-requests`,
              "POST",
              {
                title,
                body,
                source_branch: source,
                target_branch: target,
                proposal_id: proposal,
              },
            ),
          );
        } catch (cause) {
          setError(
            cause instanceof Error
              ? cause.message
              : "Unable to open pull request.",
          );
          setBusy(false);
        }
      }}
    >
      <div className="pull-form-intro">
        <GitPullRequest />
        <span>
          <strong>Open a candidate for review</strong>
          <small>
            The selected branch tips are captured exactly when you open the
            request.
          </small>
        </span>
      </div>
      <label>
        Title
        <input
          required
          maxLength={200}
          value={title}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="Summarize the change and its outcome"
        />
      </label>
      <label className="pull-body">
        Description
        <textarea
          maxLength={65536}
          value={body}
          onChange={(event) => setBody(event.target.value)}
          placeholder="Explain why this is needed, what changed, and where feedback would help."
        />
      </label>
      <div className="branch-compare">
        <label>
          Candidate branch
          <select
            value={source}
            onChange={(event) => setSource(event.target.value)}
          >
            {branches.map((branch) => (
              <option key={branch.name} value={branch.name}>
                {branch.name}
              </option>
            ))}
          </select>
        </label>
        <span>into</span>
        <label>
          Target branch
          <select
            value={target}
            onChange={(event) => setTarget(event.target.value)}
          >
            {branches.map((branch) => (
              <option key={branch.name} value={branch.name}>
                {branch.name}
              </option>
            ))}
          </select>
        </label>
      </div>
      <label>
        Related proposal <span className="optional">optional</span>
        <select
          value={proposal}
          onChange={(event) => setProposal(event.target.value)}
        >
          <option value="">No linked proposal</option>
          {proposals.map((item) => (
            <option key={item.id} value={item.id}>
              {item.title}
            </option>
          ))}
        </select>
      </label>
      {error && <p className="form-error">{error}</p>}
      <div className="form-actions">
        <Button type="button" variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="submit" disabled={busy || source === target}>
          {busy ? "Opening…" : "Open pull request"}
        </Button>
      </div>
    </form>
  );
}

function PullRequestDetail({
  repository,
  id,
  branches,
  actor,
  section,
  onBack,
  onSection,
}: {
  repository: string;
  id: string;
  branches: BranchRecord[];
  actor: string;
  section?: string;
  onBack: () => void;
  onSection: (section: string) => void;
}) {
  const active =
    section === "commits"
      ? "commits"
      : section === "files"
        ? "files"
        : section === "discussion"
          ? "discussion"
          : section === "sessions"
            ? "sessions"
            : section === "checks"
              ? "checks"
              : section === "previews"
                ? "previews"
              : section === "accessibility"
                ? "accessibility"
              : "overview";
  const [item, setItem] = useState<PullRequest>();
  const [commits, setCommits] = useState<PullRequestCommit[]>([]);
  const [files, setFiles] = useState<PullRequestFile[]>([]);
  const [comments, setComments] = useState<PullRequestComment[]>([]);
  const [reviews, setReviews] = useState<PullRequestReview[]>([]);
  const [federatedEvents, setFederatedEvents] = useState<FederatedPullEvent[]>([]);
  const [sessions, setSessions] = useState<ChangeSession[]>([]);
  const [checks, setChecks] = useState<CheckRun[]>([]);
  const [previews, setPreviews] = useState<Preview[]>([]);
  const [readiness, setReadiness] = useState<PullRequestReadiness>();
  const [queuePolicy, setQueuePolicy] = useState<IntegrationQueuePolicy>();
  const [queueEntries, setQueueEntries] = useState<IntegrationQueueEntry[]>([]);
  const [proposal, setProposal] = useState<Proposal>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const pull = await get<PullRequest>(
        `/repositories/${repository}/pull-requests/${id}`,
      );
      const [
        commitData,
        fileData,
        commentData,
        reviewData,
        federatedData,
        sessionData,
        checkData,
        previewData,
        readyData,
        linked,
        policy,
        queue,
      ] = await Promise.all([
        get<PullRequestCommitList>(
          `/repositories/${repository}/pull-requests/${id}/commits?per_page=100`,
        ),
        get<PullRequestFileList>(
          `/repositories/${repository}/pull-requests/${id}/files?per_page=100`,
        ),
        get<PullRequestCommentList>(
          `/repositories/${repository}/pull-requests/${id}/comments?per_page=100`,
        ),
        get<PullRequestReviewList>(
          `/repositories/${repository}/pull-requests/${id}/reviews?per_page=100`,
        ),
        get<FederatedPullEventList>(`/repositories/${repository}/pull-requests/${id}/federated-events`).catch(() => ({items:[],total_count:0,authoritative_local_revision:""})),
        get<ChangeSessionList>(
          `/repositories/${repository}/pull-requests/${id}/change-sessions?per_page=100`,
        ),
        get<CheckRunList>(
          `/repositories/${repository}/pull-requests/${id}/check-runs?per_page=100`,
        ),
        get<PreviewList>(
          `/repositories/${repository}/pull-requests/${id}/previews`,
        ),
        get<PullRequestReadiness>(
          `/repositories/${repository}/pull-requests/${id}/readiness`,
        ),
        pull.proposal_id
          ? get<Proposal>(
              `/repositories/${repository}/proposals/${pull.proposal_id}`,
            ).catch(() => undefined)
          : Promise.resolve(undefined),
        get<IntegrationQueuePolicy>(
          `/repositories/${repository}/integration-queue?branch=${encodeURIComponent(pull.target_branch)}`,
        ),
        get<IntegrationQueueEntries>(
          `/repositories/${repository}/integration-queue/entries?branch=${encodeURIComponent(pull.target_branch)}`,
        ),
      ]);
      setItem(pull);
      setCommits(commitData.items);
      setFiles(fileData.items);
      setComments(commentData.items);
      setReviews(reviewData.items);
      setFederatedEvents(federatedData.items);
      setSessions(sessionData.items);
      setChecks(checkData.items);
      setPreviews(previewData.items);
      setReadiness(readyData);
      setProposal(linked);
      setQueuePolicy(policy);
      setQueueEntries(queue.items);
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Pull request unavailable.",
      );
    } finally {
      setLoading(false);
    }
  }, [repository, id]);
  useEffect(() => {
    // Every pull request inspection section is addressable through the URL.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);
  if (loading)
    return <div className="repository-loading">Assembling the review…</div>;
  if (error || !item)
    return (
      <section className="proposal-notice panel">
        <GitPullRequest />
        <h2>Pull request unavailable</h2>
        <p>{error}</p>
        <Button variant="secondary" onClick={onBack}>
          All pull requests
        </Button>
      </section>
    );
  const source =
    item.source_repository_id === item.repository_id
      ? branches.find((branch) => branch.name === item.source_branch)
      : readiness?.source_branch.exists
        ? {
            name: item.source_branch,
            commit_id: readiness.source_branch.commit_id ?? "",
            is_default: false,
          }
        : undefined;
  const target = branches.find((branch) => branch.name === item.target_branch);
  const additions = files.reduce((total, file) => total + file.additions, 0);
  const deletions = files.reduce((total, file) => total + file.deletions, 0);
  return (
    <section className="pull-detail">
      <button className="proposal-back" onClick={onBack}>
        ← All pull requests
      </button>
      <header className="pull-detail-heading">
        <div>
          <span className={`pull-status ${item.status}`}>
            <GitPullRequest size={14} />
            {item.status}
          </span>
          <h2>{item.title}</h2>
          <p>
            {item.source_repository_id !== item.repository_id && (
              <>
                <Link href={`/repositories/${item.source_repository_id}`}>
                  Fork {short(item.source_repository_id)}
                </Link>
                {" · "}
              </>
            )}
            <code>{item.source_branch}</code> proposes {commits.length}{" "}
            {commits.length === 1 ? "commit" : "commits"} into{" "}
            <code>{item.target_branch}</code>
          </p>
        </div>
        <span className="pull-number">#{short(item.id)}</span>
      </header>
      <div className="pull-summary-grid">
        <article className="panel pull-purpose">
          <p className="eyebrow">Why this changed</p>
          <p>
            {item.body ||
              "The author did not add a description. Ask for the motivation and review focus in the discussion."}
          </p>
          <footer>
            Opened by <Actor id={item.author_id} /> on{" "}
            {new Date(item.created_at).toLocaleString()}
          </footer>
        </article>
        <BranchState item={item} source={source} target={target} />
        {proposal ? (
          <Link
            className="linked-proposal panel"
            href={`/repositories/${repository}?view=proposals&proposal=${proposal.id}`}
          >
            <span>
              <Lightbulb size={15} />
              Related proposal
            </span>
            <strong>{proposal.title}</strong>
            <p>{proposal.body || "No proposal description was added."}</p>
            <small>
              View shared context <ChevronRight size={12} />
            </small>
          </Link>
        ) : (
          <div className="linked-proposal panel unlinked">
            <span>
              <Lightbulb size={15} />
              Related proposal
            </span>
            <strong>No proposal linked</strong>
            <p>
              This change stands on its pull request description and discussion
              for context.
            </p>
          </div>
        )}
      </div>
      {readiness && (
        <ReviewWorkflow
          repository={repository}
          pull={item}
          actor={actor}
          reviews={reviews}
          readiness={readiness}
          queuePolicy={queuePolicy}
          queueEntries={queueEntries}
          onChanged={() => void load()}
        />
      )}
      <nav className="pull-sections" aria-label="Pull request">
        <button
          className={active === "accessibility" ? "active" : ""}
          onClick={() => onSection("accessibility")}
        >
          Accessibility
        </button>
        <button
          className={active === "previews" ? "active" : ""}
          onClick={() => onSection("previews")}
        >
          Previews <span>{previews.length}</span>
        </button>
        <button
          className={active === "overview" ? "active" : ""}
          onClick={() => onSection("overview")}
        >
          Overview
        </button>
        <button
          className={active === "checks" ? "active" : ""}
          onClick={() => onSection("checks")}
        >
          Checks <span>{checks.length}</span>
        </button>
        <button
          className={active === "commits" ? "active" : ""}
          onClick={() => onSection("commits")}
        >
          Commits <span>{commits.length}</span>
        </button>
        <button
          className={active === "files" ? "active" : ""}
          onClick={() => onSection("files")}
        >
          Files changed <span>{files.length}</span>
        </button>
        <button
          className={active === "discussion" ? "active" : ""}
          onClick={() => onSection("discussion")}
        >
          Discussion <span>{comments.length}</span>
        </button>
        <button
          className={active === "sessions" ? "active" : ""}
          onClick={() => onSection("sessions")}
        >
          Agent sessions <span>{sessions.length}</span>
        </button>
        <div>
          <strong className="additions">+{additions}</strong>
          <strong className="deletions">−{deletions}</strong>
        </div>
      </nav>
      {active === "overview" ? (
        <PullOverview
          pull={item}
          commits={commits}
          files={files}
          comments={comments}
          onSection={onSection}
        />
      ) : active === "commits" ? (
        <PullCommits commits={commits} />
      ) : active === "files" ? (
        <PullFiles files={files} />
      ) : active === "sessions" ? (
        <ChangeSessions
          repository={repository}
          pull={item}
          actor={actor}
          sessions={sessions}
          onChanged={() => void load()}
        />
      ) : active === "checks" ? (
        <PullChecks
          repository={repository}
          pull={item}
          actor={actor}
          runs={checks}
          readiness={readiness}
          onChanged={() => void load()}
          onRepair={() => {
            void load();
            onSection("sessions");
          }}
        />
      ) : active === "previews" ? (
        <PullPreviews
          repository={repository}
          pull={item}
          actor={actor}
          previews={previews}
          onChanged={() => void load()}
        />
      ) : active === "accessibility" ? (
		<><AccessibilityAssessments repository={repository} actor={actor} pull={item.id} revision={item.source_commit_id} /><PrivacyAssessments repository={repository} pull={item.id} revision={item.source_commit_id} targetRevision={item.target_commit_id} /></>
      ) : (
        <PullDiscussion
          repository={repository}
          pull={item}
          comments={comments}
          federatedEvents={federatedEvents}
          canDiscuss={Boolean(actor) && item.status === "open"}
          onChanged={() => void load()}
        />
      )}
    </section>
  );
}

function PullPreviews({ repository, pull, actor, previews, onChanged }: { repository: string; pull: PullRequest; actor: string; previews: Preview[]; onChanged: () => void }) {
  const [configuration, setConfiguration] = useState("{}");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const active = previews.some((preview) => preview.state === "setting_up");
  useEffect(() => {
    if (!active) return;
    const timer = window.setInterval(onChanged, 2000);
    return () => window.clearInterval(timer);
  }, [active, onChanged]);
  const launch = async () => {
    setBusy(true); setError("");
    try {
      const values = JSON.parse(configuration) as Record<string, string>;
      await send(`/repositories/${repository}/pull-requests/${pull.id}/previews`, "POST", { configuration: values });
      onChanged();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Preview could not be launched.");
    } finally { setBusy(false); }
  };
  const invite = async (preview: Preview, form: HTMLFormElement) => { const values=new FormData(form); setBusy(true);setError("");try{await send(`/repositories/${repository}/pull-requests/${pull.id}/previews/${preview.id}/invitations`,"POST",{user_id:values.get("user_id"),role:values.get("role"),source_kind:values.get("source_kind"),source_id:values.get("source_id"),expires_at:new Date(String(values.get("expires_at"))).toISOString()});form.reset();onChanged()}catch(cause){setError(cause instanceof Error?cause.message:"Invitation could not be created.")}finally{setBusy(false)}};
  const feedback = async (preview:Preview,form:HTMLFormElement) => { const values=new FormData(form);setBusy(true);setError("");try{const files=Array.from((form.elements.namedItem("evidence") as HTMLInputElement).files||[]);const kind=String(values.get("kind"));const evidence=await Promise.all(files.map(async file=>({kind,name:file.name,media_type:file.type||"application/octet-stream",content:await new Promise<string>((resolve,reject)=>{const reader=new FileReader();reader.onerror=()=>reject(reader.error);reader.onload=()=>resolve(String(reader.result).split(",")[1]||"");reader.readAsDataURL(file)})})));await send(`/repositories/${repository}/pull-requests/${pull.id}/previews/${preview.id}/findings`,"POST",{route:`${window.location.pathname}${window.location.search}`,title:values.get("title"),description:values.get("description"),reproduction_steps:String(values.get("steps")).split("\n").map(x=>x.trim()).filter(Boolean),evidence});form.reset();onChanged()}catch(cause){setError(cause instanceof Error?cause.message:"Feedback could not be submitted.")}finally{setBusy(false)}};
  const comment = async(preview:Preview,finding:PreviewFinding,form:HTMLFormElement)=>{const values=new FormData(form);await send(`/repositories/${repository}/pull-requests/${pull.id}/previews/${preview.id}/findings/${finding.id}/comments`,"POST",{body:values.get("body")});form.reset();onChanged()};
  const updateFinding=async(preview:Preview,finding:PreviewFinding,values:Record<string,unknown>)=>{await send(`/repositories/${repository}/pull-requests/${pull.id}/previews/${preview.id}/findings/${finding.id}`,"PATCH",values);onChanged()};
  const createWork=async(preview:Preview,finding:PreviewFinding,form:HTMLFormElement)=>{const values=new FormData(form);setBusy(true);setError("");try{await send(`/repositories/${repository}/pull-requests/${pull.id}/previews/${preview.id}/findings/${finding.id}/work`,"POST",{kind:values.get("kind"),proposal_id:values.get("proposal_id"),title:values.get("title"),owner_kind:values.get("owner_kind"),owner_id:values.get("owner_id"),acceptance_criteria:String(values.get("criteria")).split("\n").map(x=>x.trim()).filter(Boolean),evidence_ids:finding.evidence.map(x=>x.id)});onChanged()}catch(cause){setError(cause instanceof Error?cause.message:"Repair work could not be created.")}finally{setBusy(false)}};
  const publishRepair=async(preview:Preview,finding:PreviewFinding,form:HTMLFormElement)=>{const values=new FormData(form);const lines=(name:string)=>String(values.get(name)).split("\n").map(x=>x.trim()).filter(Boolean);setBusy(true);setError("");try{await send(`/repositories/${repository}/pull-requests/${pull.id}/previews/${preview.id}/findings/${finding.id}/repairs`,"POST",{revision:pull.source_commit_id,commit_ids:lines("commits"),commands:lines("commands"),checks:lines("checks"),author_ids:lines("authors"),change_session_id:finding.work?.change_session_id,workspace_id:finding.work?.workspace_id,configuration:JSON.parse(String(values.get("configuration")||"{}"))});onChanged()}catch(cause){setError(cause instanceof Error?cause.message:"Repair could not be published.")}finally{setBusy(false)}};
  return <section className="checks-workspace">
    <aside className="check-attempts panel">
      <header><div><h3>Preview attempts</h3><p>Immutable environments for exact review revisions.</p></div><Badge>{previews.length}</Badge></header>
      {previews.length ? previews.map((preview) => <article key={preview.id} className="card">
        <p><Badge>{preview.stale ? "stale" : preview.state}</Badge> <code>{short(preview.revision)}</code></p>
        <p>Created by <Actor id={preview.creator_id} /> · {new Date(preview.created_at).toLocaleString()}</p>
        {preview.url && preview.state === "ready" && <p><a href={preview.url} target="_blank" rel="noreferrer">Open exact preview ↗</a></p>}
        {preview.failure && <p className="form-error">{preview.failure}</p>}
        <p><strong>Audience boundary:</strong> {preview.definition.audience.identity} identity · {preview.definition.audience.data} data · {preview.definition.audience.network} network · actions {preview.definition.audience.actions.join(", ") || "none"}. Invitations grant preview access only—never code, workspace, deployment, or production authority.</p>
        {preview.invitations?.map((invitation)=><p key={invitation.id}><Badge>{invitation.revoked_at?"revoked":invitation.role}</Badge> <Actor id={invitation.user_id}/> via {invitation.source_kind}{invitation.source_id?` ${invitation.source_id}`:""} · expires {new Date(invitation.expires_at).toLocaleString()} {!invitation.revoked_at&&<Button size="sm" variant="secondary" onClick={async()=>{await send(`/repositories/${repository}/pull-requests/${pull.id}/previews/${preview.id}/invitations/${invitation.id}`,"DELETE");onChanged()}}>Revoke</Button>}</p>)}
        {actor&&<details><summary>Invite an affected person</summary><form className="form-stack" onSubmit={(event)=>{event.preventDefault();void invite(preview,event.currentTarget)}}><label>User ID<input name="user_id" required/></label><label>Role<select name="role"><option value="view">View</option><option value="test">Test</option><option value="feedback">Feedback</option></select></label><label>Audience source<select name="source_kind"><option value="user">Named user</option><option value="issue">Issue participant</option><option value="decision">Decision participant</option><option value="proposal">Proposal participant</option></select></label><label>Source ID (for participant invitations)<input name="source_id"/></label><label>Expires at<input name="expires_at" type="datetime-local" required/></label><Button disabled={busy} type="submit">Invite to this preview</Button></form></details>}
        <details><summary>Report what you observed</summary><form className="form-stack" onSubmit={(event)=>{event.preventDefault();void feedback(preview,event.currentTarget)}}><p>The current route and exact <code>{short(preview.revision)}</code> revision are captured automatically.</p><label>Finding title<input name="title" required maxLength={300}/></label><label>What happened<textarea name="description" required maxLength={10000}/></label><label>Reproduction steps<textarea name="steps" required placeholder="One step per line"/></label><label>Evidence type<select name="kind"><option value="screenshot">Screenshot</option><option value="recording">Recording</option><option value="console">Console output</option><option value="trace">Trace</option><option value="annotation">Annotation</option></select></label><label>Evidence files<input name="evidence" type="file" multiple accept="image/png,image/jpeg,image/webp,video/webm,video/mp4,text/plain,application/json"/><small>Up to 10 files, 1 MiB each and 5 MiB total. Credential-like text fields are redacted.</small></label><Button disabled={busy} type="submit">Attach feedback to candidate</Button></form></details>
        {(preview.findings||[]).map(finding=><section className="card" key={finding.id}><p><Badge>{finding.status}</Badge> <Badge>{finding.classification}</Badge> <code>{short(finding.revision)}</code></p><h4>{finding.title}</h4><p>{finding.description}</p><p><strong>Route:</strong> <code>{finding.route}</code></p><ol>{finding.reproduction_steps.map((step,index)=><li key={index}>{step}</li>)}</ol>{finding.duplicate_of&&<p>Duplicate of <code>{short(finding.duplicate_of)}</code></p>}{finding.evidence.map(item=><p key={item.id}><a href={`/api/repositories/${repository}/pull-requests/${pull.id}/previews/${preview.id}/findings/${finding.id}/evidence/${item.id}`} target="_blank" rel="noreferrer">{item.kind}: {item.name}</a> · {item.size} bytes · exact-preview audience {item.redacted&&<Badge>redacted</Badge>}</p>)}{finding.comments.map(item=><blockquote key={item.id}>{item.body}<footer><Actor id={item.author_id}/></footer></blockquote>)}<form className="form-stack" onSubmit={event=>{event.preventDefault();void comment(preview,finding,event.currentTarget)}}><label>Discuss this finding<textarea name="body" required/></label><Button size="sm" type="submit">Comment</Button></form>{actor&&<div className="form-actions"><select aria-label="Classification" value={finding.classification} onChange={event=>void updateFinding(preview,finding,{classification:event.target.value})}><option value="unclassified" disabled>Unclassified</option><option value="bug">Bug</option><option value="usability">Usability</option><option value="accessibility">Accessibility</option><option value="content">Content</option><option value="performance">Performance</option><option value="question">Question</option></select><Button size="sm" variant="secondary" onClick={()=>void updateFinding(preview,finding,{status:finding.status==="resolved"?"open":"resolved"})}>{finding.status==="resolved"?"Reopen":"Resolve"}</Button><select aria-label="Link duplicate" defaultValue="" onChange={event=>event.target.value&&void updateFinding(preview,finding,{duplicate_of:event.target.value})}><option value="">Link duplicate…</option>{(preview.findings||[]).filter(x=>x.id!==finding.id).map(x=><option value={x.id} key={x.id}>{x.title}</option>)}</select></div>}{actor&&!finding.work&&<details><summary>Turn into accountable repair work</summary><form className="form-stack" onSubmit={event=>{event.preventDefault();void createWork(preview,finding,event.currentTarget)}}><label>Work surface<select name="kind"><option value="task">Assigned task</option><option value="change_session">Task and change session</option><option value="workspace">Task and shared workspace</option></select></label><label>Proposal ID<input name="proposal_id" defaultValue={pull.proposal_id} required/></label><label>Task title<input name="title" defaultValue={`Repair ${finding.title}`} required/></label><label>Owner type<select name="owner_kind"><option value="human">Human</option><option value="agent">Agent</option></select></label><label>Owner ID<input name="owner_id" required/></label><label>Acceptance criteria<textarea name="criteria" defaultValue={finding.reproduction_steps.map(step=>`Does not reproduce: ${step}`).join("\n")} required/></label><p>All listed evidence metadata stays linked; bodies remain behind exact-preview access.</p><Button disabled={busy} type="submit">Create bounded repair</Button></form></details>}{finding.work&&<section><p><Badge>repair assigned</Badge> {finding.work.owner_kind} <Actor id={finding.work.owner_id}/> · task <code>{short(finding.work.task_id)}</code>{finding.work.change_session_id&&<> · session <code>{short(finding.work.change_session_id)}</code></>}{finding.work.workspace_id&&<> · workspace <code>{short(finding.work.workspace_id)}</code></>}</p><ul>{finding.work.acceptance_criteria.map(item=><li key={item}>{item}</li>)}</ul>{actor&&pull.status==="open"&&<details><summary>Record published fix and launch verification preview</summary><form className="form-stack" onSubmit={event=>{event.preventDefault();void publishRepair(preview,finding,event.currentTarget)}}><label>Repair commits, one per line<textarea name="commits" defaultValue={pull.source_commit_id} required/></label><label>Authors, one ID per line<textarea name="authors" defaultValue={actor} required/></label><label>Commands run<textarea name="commands"/></label><label>Checks passed<textarea name="checks"/></label><label>Preview configuration JSON<textarea name="configuration" defaultValue="{}" required/></label><Button disabled={busy} type="submit">Publish provenance and preview</Button></form></details>}{finding.repairs?.map(repair=><p key={repair.preview_id}><Badge>verified next</Badge> <code>{short(repair.revision)}</code> → preview <code>{short(repair.preview_id)}</code> · {repair.commit_ids.length} commit(s), {repair.checks?.length??0} check(s), {repair.author_ids.length} author(s)</p>)}</section>}</section>)}
        <small>Definition {short(preview.build_attestation.definition_digest)} · expires {new Date(preview.expires_at).toLocaleString()}</small>
        {preview.events.some((event) => event.type === "log") && <pre className="check-log">{preview.events.filter((event) => event.type === "log").map((event) => `[${event.stream}] ${event.message}`).join("\n")}</pre>}
      </article>) : <p>No candidate preview has been launched.</p>}
    </aside>
    <div className="panel form-stack">
      <p className="eyebrow">Revision-exact experience</p><h3>Launch this candidate</h3>
      <p>The repository-defined <code>.komodo/previews.json</code> build runs with bounded CPU, memory, disk, and lifetime. Updating the source branch leaves this attempt intact and marks it stale.</p>
      <label>Scoped configuration (JSON object)<textarea value={configuration} onChange={(event) => setConfiguration(event.target.value)} spellCheck={false} /><small>Only names declared by this revision are accepted. Values are attested with a digest.</small></label>
      {error && <p className="form-error">{error}</p>}
      <Button disabled={!actor || pull.status !== "open" || busy} onClick={() => void launch()}>{busy ? "Launching…" : "Launch preview"}</Button>
    </div>
  </section>;
}

function PullChecks({
  repository,
  pull,
  actor,
  runs,
  readiness,
  onChanged,
  onRepair,
}: {
  repository: string;
  pull: PullRequest;
  actor: string;
  runs: CheckRun[];
  readiness?: PullRequestReadiness;
  onChanged: () => void;
  onRepair: () => void;
}) {
  const [selected, setSelected] = useState(runs[0]?.id ?? "");
  const [detail, setDetail] = useState<CheckRun | undefined>(runs[0]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [savingPolicy, setSavingPolicy] = useState(false);
  const [required, setRequired] = useState(
    readiness?.checks.requirements.map((requirement) => requirement.name) ?? [],
  );
  const current = runs.find((run) => run.id === selected) ?? runs[0];

  useEffect(() => {
    if (!current) return;
    let active = true;
    const refresh = async () => {
      try {
        const value = await get<CheckRun>(
          `/repositories/${repository}/pull-requests/${pull.id}/check-runs/${current.id}`,
        );
        if (active) setDetail(value);
      } catch {
        // Keep the last durable snapshot visible during transient reconnects.
      }
    };
    void refresh();
    const timer =
      current.state === "queued" || current.state === "running"
        ? window.setInterval(refresh, 2000)
        : undefined;
    return () => {
      active = false;
      if (timer) window.clearInterval(timer);
    };
  }, [current, repository, pull.id]);

  const control = async (action: "cancel" | "rerun") => {
    if (!current) return;
    setBusy(true);
    setError("");
    try {
      const next = await send<CheckRun>(
        `/repositories/${repository}/pull-requests/${pull.id}/check-runs/${current.id}/${action}`,
        "POST",
      );
      setSelected(next.id);
      setDetail(next);
      onChanged();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Check control failed.",
      );
    } finally {
      setBusy(false);
    }
  };

  const startRepair = async () => {
    if (!current) return;
    setBusy(true);
    setError("");
    try {
      await send<ChangeSession>(
        `/repositories/${repository}/pull-requests/${pull.id}/check-runs/${current.id}/change-session`,
        "POST",
        {},
      );
      onRepair();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Repair session could not be started.",
      );
    } finally {
      setBusy(false);
    }
  };

  const availableNames = Array.from(
    new Set(runs.map((run) => run.definition.name)),
  ).sort();
  const savePolicy = async () => {
    setSavingPolicy(true);
    setError("");
    try {
      await send(`/repositories/${repository}/required-checks`, "PUT", {
        branch: pull.target_branch,
        checks: required,
      });
      onChanged();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Required checks could not be saved.",
      );
    } finally {
      setSavingPolicy(false);
    }
  };

  const policy = readiness?.can_merge ? (
    <section className="check-policy panel">
      <div>
        <p className="eyebrow">Target branch policy</p>
        <h3>
          Required checks for <code>{pull.target_branch}</code>
        </h3>
        <p>
          Only successful attempts for revision{" "}
          <code>{short(pull.source_commit_id)}</code> satisfy this policy.
        </p>
      </div>
      <div className="check-policy-options">
        {availableNames.length ? (
          availableNames.map((name) => (
            <label key={name}>
              <input
                type="checkbox"
                checked={required.includes(name)}
                onChange={(event) =>
                  setRequired((current) =>
                    event.target.checked
                      ? [...current, name]
                      : current.filter((item) => item !== name),
                  )
                }
              />
              <span>{name}</span>
            </label>
          ))
        ) : (
          <p>No declared check names are available on this pull request.</p>
        )}
      </div>
      <Button
        variant="secondary"
        size="sm"
        disabled={savingPolicy}
        onClick={() => void savePolicy()}
      >
        {savingPolicy ? "Saving…" : "Save requirements"}
      </Button>
    </section>
  ) : null;

  if (!runs.length)
    return (
      <>
        {policy}
        <section className="checks-empty panel">
          <Check size={20} />
          <h3>No verification configured</h3>
          <p>
            This revision does not declare checks in{" "}
            <code>.komodo/checks.json</code>.
          </p>
        </section>
      </>
    );

  const shown = detail?.id === current?.id ? detail : current;
  const logs = shown?.events.filter((event) => event.type === "log") ?? [];
  const artifacts =
    shown?.events.flatMap((event) =>
      event.artifact ? [event.artifact] : [],
    ) ?? [];
  const active = shown?.state === "queued" || shown?.state === "running";
  return (
    <>
      {policy}
      <section className="checks-workspace">
        <aside className="check-attempts panel" aria-label="Check attempts">
          <header>
            <div>
              <h3>Verification attempts</h3>
              <p>Live and historical runs for every revision.</p>
            </div>
            <Badge>{runs.length}</Badge>
          </header>
          {runs.map((run) => (
            <button
              key={run.id}
              className={run.id === shown?.id ? "selected" : ""}
              onClick={() => {
                setSelected(run.id);
                setDetail(run);
              }}
            >
              <span className={`check-state ${run.state}`}>
                {run.state === "succeeded" ? (
                  <Check size={14} />
                ) : (
                  <Clock size={14} />
                )}
              </span>
              <span>
                <strong>{run.definition.name}</strong>
                <small>
                  {short(run.commit_id)} ·{" "}
                  {new Date(run.created_at).toLocaleString()}
                </small>
              </span>
              <Badge tone={run.state === "succeeded" ? "accent" : "neutral"}>
                {run.state}
              </Badge>
            </button>
          ))}
        </aside>
        {shown && (
          <article className="check-detail panel">
            <header>
              <div>
                <p className="eyebrow">
                  {shown.retry_of_id ? "Rerun attempt" : "Automatic attempt"}
                </p>
                <h3>{shown.definition.name}</h3>
                <code>{shown.definition.command}</code>
              </div>
              <span className={`check-status ${shown.state}`}>
                {shown.state}
              </span>
            </header>
            <dl className="check-meta">
              <div>
                <dt>Revision</dt>
                <dd>
                  <code>{short(shown.commit_id)}</code>
                </dd>
              </div>
              <div>
                <dt>Started</dt>
                <dd>
                  {shown.started_at
                    ? new Date(shown.started_at).toLocaleString()
                    : "Waiting"}
                </dd>
              </div>
              <div>
                <dt>Requested by</dt>
                <dd>
                  {shown.triggered_by_id ? (
                    <Actor id={shown.triggered_by_id} compact />
                  ) : (
                    "Automatic trigger"
                  )}
                </dd>
              </div>
              {shown.canceled_by_id && (
                <div>
                  <dt>Canceled by</dt>
                  <dd>
                    <Actor id={shown.canceled_by_id} compact />
                  </dd>
                </div>
              )}
            </dl>
            <div className="check-controls">
              {actor && active && (
                <Button
                  variant="secondary"
                  disabled={busy}
                  onClick={() => void control("cancel")}
                >
                  Cancel check
                </Button>
              )}
              {actor && !active && (
                <Button
                  variant="secondary"
                  disabled={busy}
                  onClick={() => void control("rerun")}
                >
                  Rerun check
                </Button>
              )}
              {actor &&
                shown.state === "failed" &&
                shown.commit_id === pull.source_commit_id &&
                pull.status === "open" && (
                  <Button disabled={busy} onClick={() => void startRepair()}>
                    <Sparkles size={14} />
                    Start agent repair
                  </Button>
                )}
              {active && (
                <span className="live-indicator">Live · refreshing output</span>
              )}
            </div>
            {error && (
              <p className="form-error" role="alert">
                {error}
              </p>
            )}
            <div className="check-log-heading">
              <h4>Logs</h4>
              <span>{logs.length} chunks</span>
            </div>
            <pre className="check-log" aria-live="polite">
              {logs.length ? (
                logs.map((event) => (
                  <span className={event.stream} key={event.sequence}>
                    {event.message}
                  </span>
                ))
              ) : (
                <span className="quiet">No output captured.</span>
              )}
            </pre>
            <div className="check-artifacts">
              <h4>Artifacts</h4>
              {artifacts.length ? (
                artifacts.map((artifact) => (
                  <a
                    className="artifact-row"
                    key={artifact.id}
                    href={`/api/repositories/${repository}/pull-requests/${pull.id}/check-runs/${shown.id}/artifacts/${artifact.id}`}
                  >
                    <File size={15} />
                    <span>
                      <strong>{artifact.path}</strong>
                      <small>
                        {formatSize(artifact.size)} · SHA-256{" "}
                        {short(artifact.sha256)}
                      </small>
                    </span>
                    <span>Download</span>
                  </a>
                ))
              ) : (
                <p>No artifacts retained for this attempt.</p>
              )}
            </div>
          </article>
        )}
      </section>
    </>
  );
}

function ChangeSessions({
  repository,
  pull,
  actor,
  sessions,
  onChanged,
}: {
  repository: string;
  pull: PullRequest;
  actor: string;
  sessions: ChangeSession[];
  onChanged: () => void;
}) {
  const [selected, setSelected] = useState(sessions[0]?.id ?? "");
  const [events, setEvents] = useState<ChangeSessionEvent[]>(
    sessions[0]?.events ?? [],
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [instructions, setInstructions] = useState("");
  const [paths, setPaths] = useState("");
  const branch = pull.source_branch;
  const [credential, setCredential] = useState("");
  const [followUp, setFollowUp] = useState("");
  const [followUpType, setFollowUpType] = useState<"guidance" | "answer">(
    "guidance",
  );
  const current = sessions.find((item) => item.id === selected) ?? sessions[0];
  useEffect(() => {
    if (!current) return;
    let active = true;
    const loadEvents = () =>
      get<{ items: ChangeSessionEvent[] }>(
        `/repositories/${repository}/pull-requests/${pull.id}/change-sessions/${current.id}/events?per_page=100`,
      )
        .then((data) => {
          if (active) setEvents(data.items);
        })
        .catch(() => {
          if (active) setEvents(current.events ?? []);
        });
    void loadEvents();
    const live = current.runs?.some(
      (run) =>
        run.state === "queued" ||
        run.state === "running" ||
        run.state === "paused",
    );
    const timer = live
      ? window.setInterval(() => void loadEvents(), 3000)
      : undefined;
    return () => {
      active = false;
      if (timer) window.clearInterval(timer);
    };
  }, [repository, pull.id, current]);
  async function start() {
    setBusy(true);
    setError("");
    try {
      const item = await send<ChangeSession>(
        `/repositories/${repository}/pull-requests/${pull.id}/change-sessions`,
        "POST",
        {},
      );
      setSelected(item.id);
      onChanged();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Unable to start a session.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function delegate(event: React.FormEvent) {
    event.preventDefault();
    if (!current) return;
    setBusy(true);
    setError("");
    try {
      const result = await send<{
        run: AgentRun;
        credential: { token: string };
      }>(
        `/repositories/${repository}/pull-requests/${pull.id}/change-sessions/${current.id}/runs`,
        "POST",
        {
          instructions,
          revision_id: current.source_commit_id,
          context_paths: paths
            .split(",")
            .map((value) => value.trim())
            .filter(Boolean),
          working_branch: branch,
        },
      );
      setCredential(result.credential.token);
      onChanged();
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Unable to launch the run.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function intervene(
    run: AgentRun,
    type: "guidance" | "answer" | "pause" | "resume" | "cancel",
    message = "",
  ) {
    if (!current) return;
    setBusy(true);
    setError("");
    try {
      await send(
        `/repositories/${repository}/pull-requests/${pull.id}/change-sessions/${current.id}/runs/${run.id}/interventions`,
        "POST",
        { type, message },
      );
      setFollowUp("");
      onChanged();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "The run changed before this intervention was applied.",
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="change-sessions">
      <header className="agent-session-intro panel">
        <span className="agent-mark">
          <Sparkles />
        </span>
        <div>
          <p className="eyebrow">Shared agent workspace</p>
          <h3>Change sessions</h3>
          <p>
            Give an agent an exact revision, relevant context, mandate, and one
            pull request branch.
          </p>
        </div>
        {actor && pull.status === "open" && (
          <Button disabled={busy} onClick={() => void start()}>
            <Sparkles size={14} />
            {busy ? "Starting…" : "Start change session"}
          </Button>
        )}
      </header>
      {error && <p className="form-error">{error}</p>}
      {sessions.length ? (
        <div className="session-layout">
          <nav className="session-list panel" aria-label="Change sessions">
            {sessions.map((item) => (
              <button
                className={current?.id === item.id ? "active" : ""}
                key={item.id}
                onClick={() => {
                  setSelected(item.id);
                  setCredential("");
                }}
              >
                <span>
                  <strong>Session {short(item.id)}</strong>
                  <small>
                    Started by <Actor id={item.initiator_id} compact />
                  </small>
                </span>
                <span>{item.state.replaceAll("_", " ")}</span>
                <time>{new Date(item.created_at).toLocaleString()}</time>
              </button>
            ))}
          </nav>
          {current && (
            <article className="session-detail panel">
              <header>
                <div>
                  <span className="session-state">
                    <Clock size={13} />
                    {current.state.replaceAll("_", " ")}
                  </span>
                  <h3>Session {short(current.id)}</h3>
                </div>
                <code title={current.source_commit_id}>
                  {short(current.source_commit_id)}
                </code>
              </header>
              {current.check_failure && (
                <section className="check-failure-context">
                  <div>
                    <p className="eyebrow">Repair context</p>
                    <h4>Failed check · {current.check_failure.name}</h4>
                    <p>
                      Evidence captured from revision{" "}
                      <code>{short(current.check_failure.commit_id)}</code>,
                      exit {current.check_failure.exit_code}
                      {current.check_failure.timed_out ? " (timed out)" : ""}.
                    </p>
                    <code>
                      {current.check_failure.working_directory
                        ? `${current.check_failure.working_directory} · `
                        : ""}
                      {current.check_failure.command}
                    </code>
                  </div>
                  <pre className="check-log">
                    {current.check_failure.logs.length ? (
                      current.check_failure.logs.map((log) => (
                        <span className={log.stream} key={log.sequence}>
                          {log.message}
                        </span>
                      ))
                    ) : (
                      <span className="quiet">No output captured.</span>
                    )}
                  </pre>
                  {current.check_failure.artifacts.length > 0 && (
                    <div className="check-artifacts">
                      <h4>Captured artifacts</h4>
                      {current.check_failure.artifacts.map((artifact) => (
                        <a
                          className="artifact-row"
                          key={artifact.id}
                          href={`/api/repositories/${repository}/pull-requests/${pull.id}/check-runs/${current.check_failure!.run_id}/artifacts/${artifact.id}`}
                        >
                          <File size={15} />
                          <span>
                            <strong>{artifact.path}</strong>
                            <small>
                              {formatSize(artifact.size)} · SHA-256{" "}
                              {short(artifact.sha256)}
                            </small>
                          </span>
                          <span>Download</span>
                        </a>
                      ))}
                    </div>
                  )}
                </section>
              )}
              {current.state === "awaiting_instructions" &&
              actor &&
              pull.status === "open" ? (
                <form
                  className="delegate-form"
                  onSubmit={(event) => void delegate(event)}
                >
                  <label>
                    <strong>Mandate</strong>
                    <textarea
                      required
                      maxLength={10000}
                      value={instructions}
                      onChange={(event) => setInstructions(event.target.value)}
                      placeholder="Describe the outcome, constraints, and acceptance criteria."
                    />
                  </label>
                  <label>
                    <strong>Pull request revision</strong>
                    <select disabled>
                      <option>
                        {short(current.source_commit_id)} · captured source
                      </option>
                    </select>
                  </label>
                  <label>
                    <strong>Relevant paths</strong>
                    <input
                      value={paths}
                      onChange={(event) => setPaths(event.target.value)}
                      placeholder="apps/api, docs/README.md (optional)"
                    />
                  </label>
                  <label>
                    <strong>Pull request branch</strong>
                    <input readOnly value={branch} />
                    <small>
                      The worker can push only to this review branch.
                    </small>
                  </label>
                  <Button type="submit" disabled={busy}>
                    {busy ? "Launching…" : "Launch agent run"}
                  </Button>
                </form>
              ) : (
                <p>
                  This session retains its exact mandate, revision, repository
                  context, and bounded working branch.
                </p>
              )}
              {credential && (
                <div className="credential-reveal">
                  <strong>Copy the worker credential now</strong>
                  <code>{credential}</code>
                  <small>
                    Shown once; expires in 24 hours and can be revoked.
                  </small>
                </div>
              )}
              {current.runs?.map((run) => (
                <section className={`run-card ${run.state}`} key={run.id}>
                  <header>
                    <strong>
                      {run.agent} · {run.state}
                    </strong>
                    <code>{run.working_branch}</code>
                  </header>
                  <p>{run.instructions}</p>
                  <small>
                    Authorized by <Actor id={run.initiator_id} compact /> at
                    revision {short(run.revision_id)} ·{" "}
                    {run.context_paths.length
                      ? run.context_paths.join(", ")
                      : "whole repository context"}
                  </small>
                  {run.publication && (
                    <div className="run-publication">
                      <strong>Published for review</strong>
                      <p>{run.publication.summary}</p>
                      <p>
                        <b>Commits:</b>{" "}
                        {run.publication.commit_ids.map((commit, index) => (
                          <Link
                            key={commit}
                            href={`/repositories/${repository}?view=pulls&pull=${pull.id}&section=commits`}
                          >
                            {index > 0 ? ", " : ""}
                            <code title={commit}>{short(commit)}</code>
                          </Link>
                        ))}
                      </p>
                      {run.publication.changed_files.length > 0 && (
                        <p>
                          <b>Changed files:</b>{" "}
                          <Link
                            href={`/repositories/${repository}?view=pulls&pull=${pull.id}&section=files`}
                          >
                            {run.publication.changed_files.join(", ")}
                          </Link>
                        </p>
                      )}
                      {run.publication.checks.length > 0 && (
                        <p>
                          <b>Checks:</b> {run.publication.checks.join(" · ")}
                        </p>
                      )}
                      <p>
                        <b>Unresolved concerns:</b>{" "}
                        {run.publication.concerns.length
                          ? run.publication.concerns.join(" · ")
                          : "None reported"}
                      </p>
                    </div>
                  )}
                  {actor &&
                    (["queued", "running", "paused"] as string[]).includes(
                      run.state,
                    ) && (
                      <div className="run-controls">
                        <form
                          onSubmit={(event) => {
                            event.preventDefault();
                            void intervene(run, followUpType, followUp);
                          }}
                        >
                          <select
                            aria-label="Follow-up type"
                            value={followUpType}
                            onChange={(event) =>
                              setFollowUpType(
                                event.target.value as "guidance" | "answer",
                              )
                            }
                          >
                            <option value="guidance">Guidance</option>
                            <option value="answer">Answer</option>
                          </select>
                          <input
                            required
                            maxLength={10000}
                            value={followUp}
                            onChange={(event) =>
                              setFollowUp(event.target.value)
                            }
                            placeholder="Redirect the work or answer the agent"
                          />
                          <Button type="submit" disabled={busy}>
                            Send
                          </Button>
                        </form>
                        <div>
                          {run.state === "paused" ? (
                            <Button
                              variant="secondary"
                              disabled={busy}
                              onClick={() => void intervene(run, "resume")}
                            >
                              Resume
                            </Button>
                          ) : (
                            <Button
                              variant="secondary"
                              disabled={busy}
                              onClick={() => void intervene(run, "pause")}
                            >
                              Pause
                            </Button>
                          )}
                          <Button
                            variant="secondary"
                            disabled={busy}
                            onClick={() =>
                              void intervene(
                                run,
                                "cancel",
                                "Canceled from the shared session.",
                              )
                            }
                          >
                            Cancel run
                          </Button>
                        </div>
                      </div>
                    )}
                  {actor === run.initiator_id &&
                    !run.credential_revoked_at &&
                    run.state !== "canceled" && (
                      <button
                        onClick={async () => {
                          await send(
                            `/repositories/${repository}/pull-requests/${pull.id}/change-sessions/${current.id}/runs/${run.id}/credential`,
                            "DELETE",
                          );
                          onChanged();
                        }}
                      >
                        Revoke worker credential
                      </button>
                    )}
                </section>
              ))}
              <ol className="session-timeline">
                {events.map((event) => (
                  <SessionEvent
                    event={event}
                    session={current}
                    key={event.id}
                  />
                ))}
              </ol>
            </article>
          )}
        </div>
      ) : (
        <div className="proposal-empty panel">
          <Sparkles />
          <h3>No agent sessions yet</h3>
          <p>
            {actor
              ? "Start a session to make this pull request a durable shared workspace for delegated change."
              : "Sign in as a repository collaborator to start one. Public sessions remain readable here."}
          </p>
        </div>
      )}
    </section>
  );
}

function SessionEvent({
  event,
  session,
}: {
  event: ChangeSessionEvent;
  session: ChangeSession;
}) {
  const metadata = event.metadata ?? {};
  const intervention = metadata.action
    ? `${metadata.action[0].toUpperCase()}${metadata.action.slice(1)} by collaborator`
    : "";
  const labels: Record<string, string> = {
    "session.started": "Change session started",
    "run.delegated": "Agent run delegated",
    "run.started": "Agent started work",
    "agent.message": "Agent message",
    "tool.started": "Tool action started",
    "tool.completed": "Tool action completed",
    "artifact.produced": "Artifact produced",
    "branch.updated": "Working branch updated",
    "run.failed": "Agent run failed",
    "run.completed": "Agent run completed",
    "run.published": "Agent work published for review",
    "run.intervention": intervention,
  };
  const detail =
    metadata.message ??
    metadata.summary ??
    metadata.status ??
    metadata.error ??
    metadata.path ??
    metadata.tool ??
    metadata.working_branch ??
    metadata.source_commit_id ??
    "Control took effect.";
  return (
    <li
      className={
        event.type === "run.failed" || metadata.action === "cancel"
          ? "failed"
          : ""
      }
    >
      <span>
        <Sparkles size={13} />
      </span>
      <div>
        <strong>{labels[event.type] ?? event.type.replaceAll(".", " ")}</strong>
        <p>
          {event.type === "run.intervention" ? (
            <>
              <Actor id={event.actor_id} /> · {detail}
            </>
          ) : event.agent ? (
            <>
              <b>{event.agent}</b> · {detail}
            </>
          ) : (
            <>
              <Actor id={event.actor_id} /> · {detail}
            </>
          )}{" "}
          {event.revision_id && (
            <code title={event.revision_id}>@ {short(event.revision_id)}</code>
          )}
        </p>
        {event.type === "branch.updated" && metadata.commit_id && (
          <small>
            <Branch size={11} />
            {metadata.branch ?? "Working branch"} →{" "}
            <code>{short(metadata.commit_id)}</code>
          </small>
        )}
        {event.run_id && (
          <small>
            Authorized by{" "}
            <Actor id={event.initiator_id ?? event.actor_id} compact /> · run{" "}
            {short(event.run_id)} · revision{" "}
            {short(event.revision_id ?? session.source_commit_id)}
          </small>
        )}
        <time>{new Date(event.created_at).toLocaleString()}</time>
      </div>
    </li>
  );
}

function BranchState({
  item,
  source,
  target,
}: {
  item: PullRequest;
  source?: BranchRecord;
  target?: BranchRecord;
}) {
  const sourceCurrent = source?.commit_id === item.source_commit_id;
  return (
    <article className="panel branch-state">
      <p className="eyebrow">
        <Branch size={13} />
        Branch state
      </p>
      <div>
        <span className={sourceCurrent ? "current" : "changed"}>
          {sourceCurrent ? <Check size={13} /> : <Clock size={13} />}
        </span>
        <span>
          <strong>{item.source_branch}</strong>
          <small>
            {source
              ? sourceCurrent
                ? "Matches captured revision"
                : "Has moved since this opened"
              : "Branch is no longer available"}
          </small>
        </span>
        <code>{short(item.source_commit_id)}</code>
      </div>
      <div>
        <span className={target ? "current" : "changed"}>
          {target ? <Check size={13} /> : <Clock size={13} />}
        </span>
        <span>
          <strong>{item.target_branch}</strong>
          <small>
            {target
              ? target.commit_id === item.target_commit_id
                ? "Matches captured base"
                : "Base has advanced since this opened"
              : "Branch is no longer available"}
          </small>
        </span>
        <code>{short(item.target_commit_id)}</code>
      </div>
    </article>
  );
}

function ReviewWorkflow({
  repository,
  pull,
  actor,
  reviews,
  readiness,
  queuePolicy,
  queueEntries,
  onChanged,
}: {
  repository: string;
  pull: PullRequest;
  actor: string;
  reviews: PullRequestReview[];
  readiness: PullRequestReadiness;
  queuePolicy?: IntegrationQueuePolicy;
  queueEntries: IntegrationQueueEntry[];
  onChanged: () => void;
}) {
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const mine = reviews.find((review) => review.reviewer_id === actor);
  const sourceMoved =
    readiness.source_branch.exists &&
    !readiness.source_branch.matches_pull_request;
  const blockers = readiness.blockers;
  const queued = queueEntries.find(
    (entry) => entry.pull_request_id === pull.id && !entry.completed_at,
  );
  const [concurrency, setConcurrency] = useState(queuePolicy?.concurrency ?? 1);
  const [failureBehavior, setFailureBehavior] = useState<"pause" | "remove">(
    queuePolicy?.failure_behavior ?? "pause",
  );
  async function act(
    name: string,
    path: string,
    method: string,
    body?: unknown,
  ) {
    setBusy(name);
    setError("");
    try {
      await send(path, method, body);
      onChanged();
    } catch (cause) {
      setError(
        cause instanceof Error
          ? cause.message
          : "Unable to update this review.",
      );
    } finally {
      setBusy("");
    }
  }
  return (
    <section
      className={`review-workflow panel ${readiness.ready ? "ready" : "blocked"}`}
    >
      <div className="review-readiness">
        <span className="readiness-icon">
          {readiness.ready ? <Check size={18} /> : <Clock size={18} />}
        </span>
        <div>
          <p className="eyebrow">Merge readiness</p>
          <h3>
            {pull.status === "merged"
              ? "Merged"
              : readiness.ready
                ? "Ready to merge"
                : `${blockers.length} ${blockers.length === 1 ? "blocker" : "blockers"} before merge`}
          </h3>
          <p>
            {pull.status === "merged"
              ? `Merged as ${short(pull.merge_commit_id ?? "")}.`
              : readiness.has_conflicts === false
                ? "Branches merge cleanly. Review decisions and live branch state determine the remaining path."
                : "The checks below update against the live branch tips."}
          </p>
        </div>
      </div>
      <div className="readiness-checks" aria-label="Merge checks">
        <span
          className={
            readiness.source_branch.matches_pull_request ? "passed" : "failed"
          }
        >
          <Check size={13} />
          Candidate snapshot
        </span>
        <span
          className={readiness.has_conflicts === false ? "passed" : "failed"}
        >
          <Check size={13} />
          No conflicts
        </span>
        <span
          className={
            readiness.reviews.current_owner_approvals > 0 ? "passed" : "failed"
          }
        >
          <Check size={13} />
          Maintainer approval
        </span>
        <span
          className={
            readiness.reviews.current_change_requests === 0
              ? "passed"
              : "failed"
          }
        >
          <Check size={13} />
          No change requests
        </span>
        <span className={readiness.checks.satisfied ? "passed" : "failed"}>
          <Check size={13} />
          {readiness.checks.requirements.length
            ? `${readiness.checks.requirements.length} required ${readiness.checks.requirements.length === 1 ? "check" : "checks"}`
            : "No required checks"}
        </span>
      </div>
      {readiness.checks.requirements.length > 0 && (
        <div className="required-check-summary">
          <strong>
            Quality policy for <code>{readiness.checks.target_branch}</code>
          </strong>
          <small>
            Evaluated revision <code>{short(readiness.checks.commit_id)}</code>
          </small>
          {readiness.checks.requirements.map((requirement) => (
            <span className={requirement.status} key={requirement.name}>
              <b>{requirement.name}</b> {requirement.status}
              {requirement.commit_id &&
              requirement.commit_id !== readiness.checks.commit_id ? (
                <>
                  {" "}
                  · result from <code>{short(requirement.commit_id)}</code>
                </>
              ) : null}
            </span>
          ))}
        </div>
      )}
      {blockers.length > 0 && pull.status === "open" && (
        <ul className="readiness-blockers">
          {blockers.map((blocker) => (
            <li key={blocker.code}>{blocker.message}</li>
          ))}
        </ul>
      )}
      <div className="review-decisions">
        <div>
          <strong>Review decisions</strong>
          <small>
            {reviews.length
              ? `${reviews.length} current ${reviews.length === 1 ? "decision" : "decisions"}${readiness.reviews.stale_reviews ? `, ${readiness.reviews.stale_reviews} stale` : ""}`
              : "No decisions submitted yet"}
          </small>
        </div>
        {reviews.map((review) => (
          <span
            className={`review-chip ${review.decision} ${review.stale ? "stale" : ""}`}
            key={review.reviewer_id}
          >
            {review.decision === "approve" ? (
              <Check size={12} />
            ) : (
              <Clock size={12} />
            )}
            <Actor id={review.reviewer_id} compact />{" "}
            {review.decision === "approve" ? "approved" : "requested changes"}
            {review.stale ? " · stale" : ""}
          </span>
        ))}
      </div>
      {error && <p className="form-error">{error}</p>}
      <div className="review-actions">
        {pull.status === "open" && pull.draft && actor === pull.author_id && (
          <Button
            size="sm"
            disabled={Boolean(busy)}
            onClick={() =>
              void act(
                "request-review",
                `/repositories/${repository}/pull-requests/${pull.id}/request-review`,
                "POST",
                {},
              )
            }
          >
            {busy === "request-review" ? "Requesting…" : "Request review"}
          </Button>
        )}
        {pull.status === "open" && actor && (
          <>
            <Button
              variant="secondary"
              size="sm"
              disabled={Boolean(busy)}
              onClick={() =>
                void act(
                  "approve",
                  `/repositories/${repository}/pull-requests/${pull.id}/reviews/me`,
                  "PUT",
                  { decision: "approve" },
                )
              }
            >
              <Check size={13} />
              {busy === "approve"
                ? "Submitting…"
                : mine?.decision === "approve" && !mine.stale
                  ? "Approved"
                  : "Approve"}
            </Button>
            <Button
              variant="secondary"
              size="sm"
              disabled={Boolean(busy)}
              onClick={() =>
                void act(
                  "changes",
                  `/repositories/${repository}/pull-requests/${pull.id}/reviews/me`,
                  "PUT",
                  { decision: "request_changes" },
                )
              }
            >
              <Clock size={13} />
              {busy === "changes" ? "Submitting…" : "Request changes"}
            </Button>
            {mine && (
              <button
                className="withdraw-review"
                disabled={Boolean(busy)}
                onClick={() =>
                  void act(
                    "withdraw",
                    `/repositories/${repository}/pull-requests/${pull.id}/reviews/me`,
                    "DELETE",
                  )
                }
              >
                Withdraw my decision
              </button>
            )}
          </>
        )}
        {pull.status === "open" && actor === pull.author_id && sourceMoved && (
          <Button
            variant="secondary"
            size="sm"
            disabled={Boolean(busy)}
            onClick={() =>
              void act(
                "sync",
                `/repositories/${repository}/pull-requests/${pull.id}/synchronize`,
                "POST",
                {},
              )
            }
          >
            {busy === "sync" ? "Synchronizing…" : "Synchronize updated work"}
          </Button>
        )}
        {pull.status === "open" &&
          readiness.can_merge &&
          queuePolicy?.enabled && (
            <Button
              size="sm"
              disabled={!readiness.ready || Boolean(busy) || Boolean(queued)}
              onClick={() =>
                void act(
                  "queue",
                  `/repositories/${repository}/pull-requests/${pull.id}/queue`,
                  "POST",
                  {},
                )
              }
            >
              {busy === "queue"
                ? "Adding…"
                : queued
                  ? `Queued #${queued.position}`
                  : "Add to integration queue"}
            </Button>
          )}
        {pull.status === "open" &&
          readiness.can_merge &&
          !queuePolicy?.enabled && (
            <Button
              size="sm"
              disabled={!readiness.ready || Boolean(busy)}
              onClick={() => {
                if (
                  window.confirm(
                    `Merge ${pull.source_branch} into ${pull.target_branch}?`,
                  )
                )
                  void act(
                    "merge",
                    `/repositories/${repository}/pull-requests/${pull.id}/merge`,
                    "POST",
                    {},
                  );
              }}
            >
              {busy === "merge" ? "Merging…" : "Merge pull request"}
            </Button>
          )}
      </div>
      {queued && (
        <div className="required-check-summary">
          <strong>
            Queue candidate #{queued.position} · {queued.state}
          </strong>
          <small>
            Prospective merge <code>{short(queued.candidate_commit_id)}</code>{" "}
            combines base <code>{short(queued.target_commit_id)}</code> with
            reviewed source <code>{short(queued.source_commit_id)}</code>.
          </small>
          {queued.checks.requirements.length ? (
            queued.checks.requirements.map((requirement) => (
              <span className={requirement.status} key={requirement.name}>
                <b>{requirement.name}</b> {requirement.status}
                {requirement.run_id ? (
                  <>
                    {" "}
                    ·{" "}
                    <a href={`?view=pulls&pull=${pull.id}&section=checks`}>
                      logs and artifacts
                    </a>
                  </>
                ) : null}
              </span>
            ))
          ) : (
            <span className="succeeded">No required candidate checks</span>
          )}
          {queued.blocker && (
            <span className="failed">
              <b>Blocker</b> {queued.blocker.replaceAll("_", " ")}
            </span>
          )}
          <small>
            <b>Next:</b> {queued.next_action}{" "}
            <Link
              href={`/repositories/${repository}?view=queue&ref=${encodeURIComponent(pull.target_branch)}`}
            >
              Open the branch queue and retained attempts →
            </Link>
          </small>
        </div>
      )}
      {readiness.can_merge && pull.status === "open" && (
        <section className="queue-policy">
          <div>
            <strong>
              Integration policy for <code>{pull.target_branch}</code>
            </strong>
            <small>
              Admission still requires the current maintainer approval, no
              change requests or conflicts, and{" "}
              {queuePolicy?.required_checks?.length ?? 0} required checks.
            </small>
          </div>
          <label>
            <input
              type="checkbox"
              checked={queuePolicy?.enabled ?? false}
              onChange={(event) =>
                void act(
                  "policy",
                  `/repositories/${repository}/integration-queue`,
                  "PUT",
                  {
                    branch: pull.target_branch,
                    enabled: event.target.checked,
                    concurrency,
                    failure_behavior: failureBehavior,
                  },
                )
              }
            />{" "}
            Require queue
          </label>
          <label>
            Concurrency{" "}
            <select
              value={concurrency}
              onChange={(event) => setConcurrency(Number(event.target.value))}
            >
              {Array.from({ length: 10 }, (_, index) => index + 1).map(
                (value) => (
                  <option key={value}>{value}</option>
                ),
              )}
            </select>
          </label>
          <label>
            On failure{" "}
            <select
              value={failureBehavior}
              onChange={(event) =>
                setFailureBehavior(event.target.value as "pause" | "remove")
              }
            >
              <option value="pause">Pause queue</option>
              <option value="remove">Remove failed item</option>
            </select>
          </label>
          {queuePolicy?.enabled && (
            <Button
              variant="secondary"
              size="sm"
              disabled={Boolean(busy)}
              onClick={() =>
                void act(
                  "policy",
                  `/repositories/${repository}/integration-queue`,
                  "PUT",
                  {
                    branch: pull.target_branch,
                    enabled: true,
                    concurrency,
                    failure_behavior: failureBehavior,
                  },
                )
              }
            >
              {busy === "policy" ? "Saving…" : "Save queue policy"}
            </Button>
          )}
        </section>
      )}
      {!actor && pull.status === "open" && (
        <p className="review-note">
          Sign in as a repository participant to leave a review decision.
        </p>
      )}
      {actor === pull.author_id && sourceMoved && (
        <p className="review-note">
          Your branch has new work. Synchronize it to make the updated commit
          the review target; existing decisions will remain visible as stale.
        </p>
      )}
    </section>
  );
}

function PullOverview({
  pull,
  commits,
  files,
  comments,
  onSection,
}: {
  pull: PullRequest;
  commits: PullRequestCommit[];
  files: PullRequestFile[];
  comments: PullRequestComment[];
  onSection: (section: string) => void;
}) {
  return (
    <div className="pull-overview">
      {pull.delivery_evidence && <article className="panel"><h3>Stewardship delivery evidence</h3><p>{pull.delivery_evidence.reasoning}</p><dl><div><dt>Recorded by</dt><dd>{pull.delivery_evidence.recorded_by_id} · {new Date(pull.delivery_evidence.recorded_at).toLocaleString()}</dd></div><div><dt>Opportunity</dt><dd>{pull.reasoning_context?.opportunity_id}</dd></div></dl><h4>Commands</h4><ul>{pull.delivery_evidence.commands.map((command) => <li key={command}><code>{command}</code></li>)}</ul><h4>Completion criteria</h4><ul>{pull.delivery_evidence.completion_criteria.map((criterion) => <li key={criterion.criterion}><strong>{criterion.status}</strong> · {criterion.criterion}{criterion.evidence ? ` — ${criterion.evidence}` : ""}</li>)}</ul><h4>Residual risks</h4>{pull.delivery_evidence.residual_risks.length ? <ul>{pull.delivery_evidence.residual_risks.map((risk) => <li key={risk}>{risk}</li>)}</ul> : <p>None reported.</p>}</article>}
      <article className="panel overview-card">
        <header>
          <span>
            <Clock size={15} />
            Commits
          </span>
          <button onClick={() => onSection("commits")}>Inspect all</button>
        </header>
        {commits
          .slice(-3)
          .reverse()
          .map((commit) => (
            <div key={commit.id}>
              <span className="commit-node">
                <Clock size={13} />
              </span>
              <span>
                <strong>{summary(commit.message)}</strong>
                <small>{commit.author || "Unknown author"}</small>
              </span>
              <code>{short(commit.id)}</code>
            </div>
          ))}
      </article>
      <article className="panel overview-card">
        <header>
          <span>
            <File size={15} />
            Changed files
          </span>
          <button onClick={() => onSection("files")}>Inspect diff</button>
        </header>
        {files.slice(0, 3).map((file) => (
          <div key={file.path}>
            <span className={`file-status ${file.status}`}>
              {file.status[0].toUpperCase()}
            </span>
            <span>
              <strong>{file.path}</strong>
              <small>
                {file.binary
                  ? "Binary change"
                  : `${file.additions} additions, ${file.deletions} deletions`}
              </small>
            </span>
            <code className="diff-count">
              <i>+{file.additions}</i> <b>−{file.deletions}</b>
            </code>
          </div>
        ))}
      </article>
      <article className="panel overview-card">
        <header>
          <span>
            <MessageCircle size={15} />
            Feedback needed
          </span>
          <button onClick={() => onSection("discussion")}>
            Join discussion
          </button>
        </header>
        <div className="feedback-copy">
          <p>
            {comments.length
              ? `${comments.length} discussion ${comments.length === 1 ? "reply is" : "replies are"} attached to this candidate.`
              : "No questions or feedback have been added yet."}
          </p>
          <span>
            Use the discussion to clarify intent, flag edge cases, and record
            follow-up work.
          </span>
        </div>
      </article>
    </div>
  );
}

function PullCommits({ commits }: { commits: PullRequestCommit[] }) {
  return (
    <section className="pull-commits panel">
      <div className="browser-header">
        <strong>
          {commits.length} source-only{" "}
          {commits.length === 1 ? "commit" : "commits"}
        </strong>
        <span>Oldest to newest</span>
      </div>
      {commits.map((commit, index) => (
        <article key={commit.id}>
          <span className="commit-order">{index + 1}</span>
          <div>
            <h3>{summary(commit.message)}</h3>
            <p>{commit.author || "Unknown author"}</p>
            {commit.message.includes("\n") && (
              <pre>{commit.message.split("\n").slice(1).join("\n").trim()}</pre>
            )}
          </div>
          <code>{short(commit.id)}</code>
        </article>
      ))}
    </section>
  );
}

function PullFiles({ files }: { files: PullRequestFile[] }) {
  return (
    <section className="pull-files">
      {files.map((file) => (
        <article className="panel" key={file.path}>
          <header>
            <span className={`file-status ${file.status}`}>
              {file.status[0].toUpperCase()}
            </span>
            <strong>{file.path}</strong>
            <span>
              {file.status} · <i>+{file.additions}</i> <b>−{file.deletions}</b>
            </span>
          </header>
          {file.binary ? (
            <div className="binary-diff">
              <File />
              <p>
                Binary file changed. Object and mode metadata remain available
                for review.
              </p>
              <code>
                {short(file.old_object_id ?? "new")} →{" "}
                {short(file.new_object_id ?? "deleted")}
              </code>
            </div>
          ) : file.patch ? (
            <Diff patch={file.patch} />
          ) : (
            <p className="empty-diff">
              No text patch is available for this change.
            </p>
          )}
        </article>
      ))}
    </section>
  );
}
function Diff({ patch }: { patch: string }) {
  return (
    <pre className="diff-view">
      {patch.split("\n").map((line, index) => (
        <span
          className={
            line.startsWith("+") && !line.startsWith("+++")
              ? "added"
              : line.startsWith("-") && !line.startsWith("---")
                ? "removed"
                : line.startsWith("@@")
                  ? "hunk"
                  : ""
          }
          key={index}
        >
          {line || " "}
        </span>
      ))}
    </pre>
  );
}

function PullDiscussion({
  repository,
  pull,
  comments,
  federatedEvents,
  canDiscuss,
  onChanged,
}: {
  repository: string;
  pull: PullRequest;
  comments: PullRequestComment[];
  federatedEvents: FederatedPullEvent[];
  canDiscuss: boolean;
  onChanged: () => void;
}) {
  const [comment, setComment] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  return (
    <section className="pull-discussion">
      <div className="proposal-comments">
        {federatedEvents.map((entry) => <article className="panel" key={entry.id}><div className="proposal-author"><span className="avatar sm"><User size={14}/></span><span><strong>{entry.actor_subject}</strong><small>{new Date(entry.occurred_at).toLocaleString()} · {entry.source_instance}</small></span></div><p><Badge tone={entry.current?"accent":undefined}>{entry.kind}{entry.state?` · ${entry.state}`:""}</Badge> <Badge>{entry.verification}</Badge> <Badge>{entry.audience}</Badge>{!entry.current&&<Badge>stale revision</Badge>}</p>{entry.body&&<p>{entry.body}</p>}{entry.evidence&&Object.entries(entry.evidence).map(([name,value])=><p key={name}><strong>{name}:</strong> <code>{value}</code></p>)}</article>)}
        {comments.map((entry) => (
          <article className="panel" key={entry.id}>
            <div className="proposal-author">
              <span className="avatar sm">
                <User size={14} />
              </span>
              <span>
                <strong>
                  <Actor id={entry.author_id} />
                </strong>
                <small>{new Date(entry.created_at).toLocaleString()}</small>
              </span>
            </div>
            <p>{entry.body}</p>
          </article>
        ))}
        {!comments.length && !federatedEvents.length && (
          <p className="no-comments">
            No discussion yet. Ask about intent, implementation tradeoffs, or a
            specific changed file.
          </p>
        )}
      </div>
      {canDiscuss ? (
        <form
          className="comment-form panel"
          onSubmit={async (event) => {
            event.preventDefault();
            setBusy(true);
            setError("");
            try {
              await send(
                `/repositories/${repository}/pull-requests/${pull.id}/comments`,
                "POST",
                { body: comment },
              );
              setComment("");
              onChanged();
            } catch (cause) {
              setError(
                cause instanceof Error
                  ? cause.message
                  : "Unable to post comment.",
              );
            } finally {
              setBusy(false);
            }
          }}
        >
          <label>
            <strong>Discuss this change</strong>
            <textarea
              required
              value={comment}
              onChange={(event) => setComment(event.target.value)}
              placeholder="Ask a question, identify an edge case, or explain what needs another look."
            />
          </label>
          {error && <p className="form-error">{error}</p>}
          <div className="form-actions">
            <Button type="submit" disabled={busy}>
              {busy ? "Posting…" : "Comment"}
            </Button>
          </div>
        </form>
      ) : (
        <div className="closed-note panel">
          <MessageCircle size={16} />
          <span>
            <strong>
              {pull.status === "merged"
                ? "This pull request is merged."
                : "Sign in to participate."}
            </strong>{" "}
            The complete conversation remains available as review context.
          </span>
        </div>
      )}
    </section>
  );
}
