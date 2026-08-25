package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/stackedchanges"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func registerStackedChangesHTTP(mux *http.ServeMux, s *stackedchanges.Store, repos codeIntelligenceStore, credentials authStore) {
	base := "/repositories/{repository}/change-stacks"
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in stackedchanges.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		opened, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		members, blockers := analyzeChangeStack(opened, in, true, false)
		x, e := s.Create(string(repo.ID), actor.UserID, in, members, blockers)
		if stackError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := s.List(string(repo.ID))
		if stackError(w, e) {
			return
		}
		for i := range items {
			items[i] = projectStackPermissions(items[i], actor)
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	})
	mux.HandleFunc("GET "+base+"/{stack}", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("stack"))
		if stackError(w, e) {
			return
		}
		writeJSON(w, 200, projectStackPermissions(x, actor))
	})
	mux.HandleFunc("GET /repositories/{repository}/pull-requests/{pull}/stack-context", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, m, e := s.FindByPull(string(repo.ID), r.PathValue("pull"))
		if stackError(w, e) {
			return
		}
		x = projectStackPermissions(x, actor)
		for _, candidate := range x.Members {
			if candidate.ID == m.ID {
				m = candidate
				break
			}
		}
		writeJSON(w, 200, map[string]any{"stack_id": x.ID, "title": x.Title, "outcome": x.Outcome, "target_branch": x.TargetBranch, "target_revision": x.TargetRevision, "member": m, "members": x.Members, "authority_granted": []string{}})
	})
	mux.HandleFunc("POST "+base+"/{stack}/members/{member}/publications", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Revision string `json:"revision"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.Publish(string(repo.ID), r.PathValue("stack"), r.PathValue("member"), in.Revision, actor.UserID)
		if stackError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/{stack}/members/{member}/evidence", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Revision  string `json:"revision"`
			Kind      string `json:"kind"`
			Reference string `json:"reference"`
			Scope     string `json:"scope"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.BindEvidence(string(repo.ID), r.PathValue("stack"), r.PathValue("member"), in.Revision, actor.UserID, in.Kind, in.Reference, in.Scope)
		if stackError(w, e) {
			return
		}
		writeJSON(w, 201, stackedchanges.Project(x))
	})
	mux.HandleFunc("POST "+base+"/{stack}/members/{member}/assignments", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ParticipantID      string   `json:"participant_id"`
			ParticipantKind    string   `json:"participant_kind"`
			AgentApprovalID    string   `json:"agent_approval_id"`
			AuthorizedBranches []string `json:"authorized_branches"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.Assign(string(repo.ID), r.PathValue("stack"), r.PathValue("member"), actor.UserID, in.ParticipantID, in.ParticipantKind, in.AgentApprovalID, in.AuthorizedBranches)
		if stackError(w, e) {
			return
		}
		writeJSON(w, 201, projectStackPermissions(x, actor))
	})
	mux.HandleFunc("POST "+base+"/{stack}/members/{member}/workspaces", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			AssignmentID string `json:"assignment_id"`
			Kind         string `json:"kind"`
			Audience     string `json:"audience"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.OpenWorkspace(string(repo.ID), r.PathValue("stack"), r.PathValue("member"), actor.UserID, in.AssignmentID, in.Kind, in.Audience)
		if stackError(w, e) {
			return
		}
		writeJSON(w, 201, projectStackPermissions(x, actor))
	})
	mux.HandleFunc("POST "+base+"/{stack}/timeline", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			MemberID        string                       `json:"member_id"`
			WorkspaceID     string                       `json:"workspace_id"`
			Kind            string                       `json:"kind"`
			Summary         string                       `json:"summary"`
			Audience        string                       `json:"audience"`
			ProposedMembers []stackedchanges.MemberInput `json:"proposed_members"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.AppendTimeline(string(repo.ID), r.PathValue("stack"), in.MemberID, in.WorkspaceID, actor.UserID, in.Kind, in.Summary, in.Audience, in.ProposedMembers)
		if stackError(w, e) {
			return
		}
		writeJSON(w, 201, projectStackPermissions(x, actor))
	})
	mux.HandleFunc("POST "+base+"/{stack}/revisions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedRevision int                          `json:"expected_revision"`
			Reason           string                       `json:"reason"`
			Members          []stackedchanges.MemberInput `json:"members"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		current, e := s.Get(string(repo.ID), r.PathValue("stack"))
		if stackError(w, e) {
			return
		}
		candidate := current.Input
		candidate.Members = in.Members
		opened, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		members, blockers := analyzeChangeStack(opened, candidate, true, true)
		for i := range members {
			for _, prior := range current.Members {
				if prior.ID == members[i].ID {
					members[i].Assignments = prior.Assignments
					members[i].Workspaces = prior.Workspaces
				}
			}
		}
		otherStacks, _ := s.List(string(repo.ID))
		for _, m := range members {
			for _, other := range otherStacks {
				if other.ID != current.ID {
					for _, used := range other.Members {
						if used.Branch == m.Branch {
							blockers = append(blockers, stackedchanges.Blocker{Kind: "shared_branch", MemberID: m.ID, Detail: "branch is also retained by stack " + other.ID})
						}
					}
				}
			}
			if m.RepositoryID != "" && m.RepositoryID != string(repo.ID) {
				blockers = append(blockers, stackedchanges.Blocker{Kind: "other_repository_owned", MemberID: m.ID, Detail: "member belongs to an independently governed repository"})
			}
			if m.BranchAccess == "revoked" {
				blockers = append(blockers, stackedchanges.Blocker{Kind: "revoked_access", MemberID: m.ID, Detail: "declared branch access was revoked before publication"})
			}
			owners := m.BranchOwnerIDs
			if len(owners) == 0 {
				owners = m.Authors
			}
			if !stackContains(owners, actor.UserID) {
				blockers = append(blockers, stackedchanges.Blocker{Kind: "other_contributor_owned", MemberID: m.ID, Detail: "caller does not own this branch"})
			}
		}
		for _, b := range blockers {
			if b.Kind == "unrelated_history" {
				blockers = append(blockers, stackedchanges.Blocker{Kind: "rewrite_conflict", MemberID: b.MemberID, Detail: "proposed commit does not apply on its new declared base"})
			}
		}
		for i := range members {
			members[i].Blockers = append(members[i].Blockers, memberBlockers(blockers, members[i].ID)...)
			members[i].Reviewable = len(members[i].Blockers) == 0
		}
		x, e := s.PreviewRevision(string(repo.ID), current.ID, actor.UserID, in.Reason, in.ExpectedRevision, candidate, members, dedupeBlockers(blockers))
		if stackError(w, e) {
			return
		}
		writeJSON(w, 201, projectStackPermissions(x, actor))
	})
	mux.HandleFunc("POST "+base+"/{stack}/revisions/{revision}/apply", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		number, e := strconv.Atoi(r.PathValue("revision"))
		if e != nil || number < 1 {
			writeJSON(w, 400, map[string]string{"error": "invalid_revision"})
			return
		}
		x, revision, e := s.RevisionForApply(string(repo.ID), r.PathValue("stack"), number)
		if stackError(w, e) {
			return
		}
		for _, m := range revision.Members {
			owners := m.BranchOwnerIDs
			if len(owners) == 0 {
				owners = m.Authors
			}
			if !stackContains(owners, actor.UserID) {
				writeJSON(w, 403, map[string]string{"error": "branch_owner_required"})
				return
			}
		}
		opened, e := repos.Open(repo.ID)
		if e == nil {
			e = atomicStackBranches(opened, revision)
		}
		x, e = s.FinishApply(string(repo.ID), x.ID, number, actor.UserID, e)
		if stackError(w, e) {
			return
		}
		status := 200
		for _, applied := range x.Revisions {
			if applied.Number == number && applied.Status == "failed" {
				status = 409
			}
		}
		writeJSON(w, status, projectStackPermissions(x, actor))
	})
	mux.HandleFunc("POST "+base+"/{stack}/landings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedStackRevision  int      `json:"expected_stack_revision"`
			ExpectedTargetRevision string   `json:"expected_target_revision"`
			Mode                   string   `json:"mode"`
			AtomicPermitted        bool     `json:"atomic_permitted"`
			RequiredEvidence       []string `json:"required_evidence"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("stack"))
		if stackError(w, e) {
			return
		}
		opened, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		liveTarget, e := opened.ReadReference(storage.ReferenceName("refs/heads/" + strings.TrimPrefix(x.TargetBranch, "refs/heads/")))
		if in.ExpectedStackRevision != x.CurrentRevision || e != nil || in.ExpectedTargetRevision != string(liveTarget.ObjectID) {
			writeJSON(w, 409, map[string]string{"error": "stale_stack_or_target"})
			return
		}
		if len(in.RequiredEvidence) == 0 {
			in.RequiredEvidence = []string{"required_check", "reproduction", "contract", "preview", "policy", "approval"}
		}
		candidates := assembleLandingCandidates(r.Context(), opened, x, in.ExpectedTargetRevision, in.RequiredEvidence, 1, 0)
		x, e = s.CreateLanding(string(repo.ID), x.ID, actor.UserID, in.Mode, in.AtomicPermitted, in.RequiredEvidence, in.ExpectedTargetRevision, candidates)
		if stackError(w, e) {
			return
		}
		writeJSON(w, 201, projectStackPermissions(x, actor))
	})
	mux.HandleFunc("POST "+base+"/{stack}/landings/{landing}/candidates/{candidate}/evidence", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Kind      string `json:"kind"`
			Reference string `json:"reference"`
			Status    string `json:"status"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.AddLandingEvidence(string(repo.ID), r.PathValue("stack"), r.PathValue("landing"), r.PathValue("candidate"), actor.UserID, in.Kind, in.Reference, in.Status)
		if stackError(w, e) {
			return
		}
		writeJSON(w, 201, projectStackPermissions(x, actor))
	})
	mux.HandleFunc("POST "+base+"/{stack}/landings/{landing}/merge", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			MemberID string `json:"member_id"`
			Atomic   bool   `json:"atomic"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, l, c, e := s.LandingForMerge(string(repo.ID), r.PathValue("stack"), r.PathValue("landing"), in.MemberID, in.Atomic)
		if stackError(w, e) {
			return
		}
		opened, e := repos.Open(repo.ID)
		if e == nil {
			expected := c.BaseRevision
			if in.Atomic {
				expected = l.CurrentTargetRevision
			}
			e = opened.CompareAndSwapReference(storage.ReferenceName("refs/heads/"+strings.TrimPrefix(l.TargetBranch, "refs/heads/")), storage.ObjectID(expected), storage.ObjectID(c.CandidateRevision))
		}
		mergeErr := e
		x, e = s.FinishLandingMerge(string(repo.ID), x.ID, l.ID, c.MemberID, actor.UserID, in.Atomic, mergeErr)
		if stackError(w, e) {
			return
		}
		status := 200
		if mergeErr != nil {
			status = 409
		}
		writeJSON(w, status, projectStackPermissions(x, actor))
	})
	mux.HandleFunc("POST "+base+"/{stack}/landings/{landing}/rebuild", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedTargetRevision string `json:"expected_target_revision"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.Get(string(repo.ID), r.PathValue("stack"))
		if stackError(w, e) {
			return
		}
		opened, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		ref, e := opened.ReadReference(storage.ReferenceName("refs/heads/" + strings.TrimPrefix(x.TargetBranch, "refs/heads/")))
		if e != nil || string(ref.ObjectID) != in.ExpectedTargetRevision {
			writeJSON(w, 409, map[string]string{"error": "target_moved"})
			return
		}
		var landing stackedchanges.Landing
		found := false
		for _, l := range x.Landings {
			if l.ID == r.PathValue("landing") {
				landing = l
				found = true
			}
		}
		if !found {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		start := len(landing.MergedMembers)
		generation := 1
		for _, c := range landing.Candidates {
			if c.Generation >= generation {
				generation = c.Generation + 1
			}
		}
		candidates := assembleLandingCandidates(r.Context(), opened, x, in.ExpectedTargetRevision, landing.Candidates[0].RequiredEvidence, generation, start)
		x, e = s.RebuildLanding(string(repo.ID), x.ID, landing.ID, actor.UserID, in.ExpectedTargetRevision, candidates)
		if stackError(w, e) {
			return
		}
		writeJSON(w, 201, projectStackPermissions(x, actor))
	})
}

func assembleLandingCandidates(ctx context.Context, repo *storage.Repository, stack stackedchanges.Stack, target string, required []string, generation, start int) []stackedchanges.LandingCandidate {
	now := time.Now().UTC()
	base := storage.ObjectID(target)
	out := []stackedchanges.LandingCandidate{}
	for i := start; i < len(stack.Members); i++ {
		m := stack.Members[i]
		c := stackedchanges.LandingCandidate{ID: fmt.Sprintf("g%d-%s", generation, m.ID), Generation: generation, Position: i + 1, MemberID: m.ID, BaseRevision: string(base), SourceRevision: m.Revision, Status: "verifying", RequiredEvidence: append([]string{}, required...), Evidence: []stackedchanges.LandingEvidence{}, Blockers: []stackedchanges.Blocker{}, CreatedAt: now}
		source := storage.ObjectID(m.Revision)
		if ancestor(repo, base, source) {
			commit, e := repo.ReadCommit(source)
			if e == nil {
				c.CandidateRevision, c.CandidateTree, base = m.Revision, string(commit.Tree), source
			} else {
				c.Blockers = append(c.Blockers, stackedchanges.Blocker{Kind: "missing_source", MemberID: m.ID, Detail: "source revision is unavailable"})
			}
		} else if conflicts, e := mergeHasConflicts(ctx, repo, base, source); e != nil || conflicts {
			c.Blockers = append(c.Blockers, stackedchanges.Blocker{Kind: "merge_conflict", MemberID: m.ID, Detail: "change cannot be applied to the current ready prefix"})
		} else if tree, e := materializeMergeTree(ctx, repo, base, source); e != nil {
			c.Blockers = append(c.Blockers, stackedchanges.Blocker{Kind: "candidate_failed", MemberID: m.ID, Detail: "candidate tree could not be assembled"})
		} else {
			stamp := fmt.Sprintf("%d +0000", now.Unix())
			body := fmt.Sprintf("tree %s\nparent %s\nparent %s\nauthor %s <%s@users.local> %s\ncommitter stack-landing <stack-landing@users.local> %s\n\nStack landing candidate for %s\n", tree, base, source, stack.CreatedBy, stack.CreatedBy, stamp, stamp, m.ID)
			candidate, writeErr := repo.WriteObject(storage.CommitObject, []byte(body))
			if writeErr != nil {
				c.Blockers = append(c.Blockers, stackedchanges.Blocker{Kind: "candidate_failed", MemberID: m.ID, Detail: "candidate commit could not be retained"})
			} else {
				c.CandidateRevision, c.CandidateTree, base = string(candidate), string(tree), candidate
			}
		}
		out = append(out, c)
		if len(c.Blockers) > 0 {
			for j := i + 1; j < len(stack.Members); j++ {
				n := stack.Members[j]
				out = append(out, stackedchanges.LandingCandidate{ID: fmt.Sprintf("g%d-%s", generation, n.ID), Generation: generation, Position: j + 1, MemberID: n.ID, BaseRevision: string(base), SourceRevision: n.Revision, Status: "paused_suffix", RequiredEvidence: append([]string{}, required...), Evidence: []stackedchanges.LandingEvidence{}, Blockers: []stackedchanges.Blocker{{Kind: "unsafe_prefix", MemberID: n.ID, Detail: "an earlier member must recover first"}}, CreatedAt: now})
			}
			break
		}
	}
	return out
}

func stackContains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
func atomicStackBranches(repo *storage.Repository, revision stackedchanges.Revision) error {
	if len(revision.Blockers) > 0 {
		return stackedchanges.ErrInvalid
	}
	var b bytes.Buffer
	b.WriteString("start\n")
	zero := "0000000000000000000000000000000000000000"
	for _, u := range revision.BranchUpdates {
		old := u.ExpectedRevision
		if old == "" {
			old = zero
		}
		b.WriteString("update refs/heads/" + strings.TrimPrefix(u.Branch, "refs/heads/") + " " + u.PublishedRevision + " " + old + "\n")
	}
	b.WriteString("prepare\ncommit\n")
	cmd := exec.Command("git", "--git-dir="+repo.GitDir(), "update-ref", "--stdin")
	cmd.Stdin = &b
	if output, e := cmd.CombinedOutput(); e != nil {
		return errors.New(strings.TrimSpace(string(output)))
	}
	return nil
}

func projectStackPermissions(x stackedchanges.Stack, actor auth.Grant) stackedchanges.Stack {
	x = stackedchanges.Project(x)
	canPublish := false
	for _, scope := range actor.Scopes {
		if scope == auth.RepositoryWrite {
			canPublish = true
			break
		}
	}
	for i := range x.Members {
		owners := x.Members[i].BranchOwnerIDs
		if len(owners) == 0 {
			owners = x.Members[i].Authors
		}
		canUpdate := canPublish && stackContains(owners, actor.UserID)
		reason := "branch is owned by another contributor or repository"
		if canUpdate {
			reason = "caller may request an optimistic atomic update of this owned branch"
		}
		x.Members[i].EffectivePermissions = stackedchanges.Permission{Read: true, Publish: canPublish, UpdateBranch: canUpdate, Reason: reason}
		for j := range x.Members[i].Workspaces {
			workspace := &x.Members[i].Workspaces[j]
			if workspace.Audience == "repository" || workspace.ParticipantID == actor.UserID || stackContains(owners, actor.UserID) {
				continue
			}
			workspace.Outcome = "restricted collaboration workspace"
			workspace.AcceptanceCriteria = nil
			workspace.Evidence = nil
			workspace.UpstreamRevisions = nil
			workspace.EditableBranches = nil
		}
	}
	// Restricted collaboration metadata remains visible, but its body and proposed
	// restack are disclosed only to the originating participant or branch owner.
	for i := range x.Timeline {
		if x.Timeline[i].Audience == "repository" {
			continue
		}
		visible := x.Timeline[i].ActorID == actor.UserID
		for _, m := range x.Members {
			if m.ID == x.Timeline[i].MemberID {
				owners := m.BranchOwnerIDs
				if len(owners) == 0 {
					owners = m.Authors
				}
				visible = visible || stackContains(owners, actor.UserID)
			}
		}
		if !visible {
			x.Timeline[i].Summary = "embargoed collaboration event"
			x.Timeline[i].ProposedMembers = nil
		}
	}
	return x
}

func stackError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, stackedchanges.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(e, stackedchanges.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_change_stack"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}

func analyzeChangeStack(repo *storage.Repository, in stackedchanges.Input, canWrite, rewritePreview bool) ([]stackedchanges.Member, []stackedchanges.Blocker) {
	all := []stackedchanges.Blocker{}
	ids := map[string]stackedchanges.MemberInput{}
	for _, m := range in.Members {
		ids[m.ID] = m
	}
	// Parent links are retained even when malformed so reviewers see the graph problem.
	for _, m := range in.Members {
		seen := map[string]bool{}
		at := m.ID
		for at != "" {
			if seen[at] {
				all = append(all, stackedchanges.Blocker{Kind: "dependency_cycle", MemberID: m.ID, Detail: "parent links form a cycle"})
				break
			}
			seen[at] = true
			n, ok := ids[at]
			if !ok {
				all = append(all, stackedchanges.Blocker{Kind: "missing_dependency", MemberID: m.ID, Detail: "declared parent is not in this stack"})
				break
			}
			at = n.ParentID
		}
	}
	targetOK := commitExists(repo, in.TargetRevision)
	if !targetOK {
		all = append(all, stackedchanges.Blocker{Kind: "missing_target_commit", Detail: "target revision is unavailable"})
	}
	seenRev := map[string]string{}
	seenTree := map[storage.ObjectID]string{}
	out := make([]stackedchanges.Member, 0, len(in.Members))
	for i, m := range in.Members {
		base := in.TargetRevision
		if p, ok := ids[m.ParentID]; ok {
			base = p.Revision
		}
		x := stackedchanges.Member{MemberInput: m, Position: i + 1, BaseRevision: base, EffectivePermissions: stackedchanges.Permission{Read: true, Publish: canWrite, UpdateBranch: false, Reason: "repository API publication is available; branch updates require separate Git authority"}}
		if prior := seenRev[m.Revision]; prior != "" {
			x.Blockers = append(x.Blockers, stackedchanges.Blocker{Kind: "duplicate_change", MemberID: m.ID, Detail: "same exact revision as " + prior})
		}
		seenRev[m.Revision] = m.ID
		commit, e := repo.ReadCommit(storage.ObjectID(m.Revision))
		if e != nil {
			x.Blockers = append(x.Blockers, stackedchanges.Blocker{Kind: "missing_commit", MemberID: m.ID, Detail: "exact revision is unavailable"})
		} else {
			if prior := seenTree[commit.Tree]; prior != "" {
				x.Blockers = append(x.Blockers, stackedchanges.Blocker{Kind: "duplicate_change", MemberID: m.ID, Detail: "same resulting tree as " + prior})
			}
			seenTree[commit.Tree] = m.ID
			if commitExists(repo, base) {
				if !ancestor(repo, storage.ObjectID(base), storage.ObjectID(m.Revision)) {
					x.Blockers = append(x.Blockers, stackedchanges.Blocker{Kind: "unrelated_history", MemberID: m.ID, Detail: "revision does not descend from its declared base"})
				}
				x.IndividualScope = scope(repo, base, m.Revision)
				x.CumulativeScope = scope(repo, in.TargetRevision, m.Revision)
			}
		}
		if m.BranchState == "existing" {
			if ref, e := repo.ReadReference(storage.ReferenceName("refs/heads/" + strings.TrimPrefix(m.Branch, "refs/heads/"))); e != nil {
				x.Blockers = append(x.Blockers, stackedchanges.Blocker{Kind: "inaccessible_branch", MemberID: m.ID, Detail: "existing branch is missing or inaccessible"})
			} else if string(ref.ObjectID) != m.Revision && !rewritePreview {
				x.Blockers = append(x.Blockers, stackedchanges.Blocker{Kind: "branch_moved", MemberID: m.ID, Detail: "branch does not point to the declared exact revision"})
			}
		}
		x.Blockers = append(x.Blockers, memberBlockers(all, m.ID)...)
		x.Reviewable = len(x.Blockers) == 0
		x.Publications = []stackedchanges.Publication{}
		out = append(out, x)
		all = append(all, x.Blockers...)
	}
	return out, dedupeBlockers(all)
}
func memberBlockers(xs []stackedchanges.Blocker, id string) []stackedchanges.Blocker {
	out := []stackedchanges.Blocker{}
	for _, x := range xs {
		if x.MemberID == id {
			out = append(out, x)
		}
	}
	return out
}
func dedupeBlockers(xs []stackedchanges.Blocker) []stackedchanges.Blocker {
	seen := map[string]bool{}
	out := []stackedchanges.Blocker{}
	for _, x := range xs {
		k := x.Kind + "|" + x.MemberID + "|" + x.Detail
		if !seen[k] {
			seen[k] = true
			out = append(out, x)
		}
	}
	return out
}
func commitExists(r *storage.Repository, id string) bool {
	_, e := r.ReadCommit(storage.ObjectID(id))
	return e == nil
}
func ancestor(r *storage.Repository, base, tip storage.ObjectID) bool {
	seen := map[storage.ObjectID]bool{}
	q := []storage.ObjectID{tip}
	for len(q) > 0 {
		x := q[0]
		q = q[1:]
		if x == base {
			return true
		}
		if seen[x] {
			continue
		}
		seen[x] = true
		c, e := r.ReadCommit(x)
		if e == nil {
			q = append(q, c.Parents...)
		}
	}
	return false
}
func scope(r *storage.Repository, from, to string) stackedchanges.Scope {
	s := stackedchanges.Scope{FromRevision: from, ToRevision: to, ChangedPaths: []string{}, CommitIDs: []string{}, Changes: []stackedchanges.Change{}}
	if !ancestor(r, storage.ObjectID(from), storage.ObjectID(to)) {
		return s
	}
	base, _, _, _ := collectSources(r, storage.ObjectID(from))
	tip, _, _, _ := collectSources(r, storage.ObjectID(to))
	bm := map[string]storage.ObjectID{}
	for _, f := range base {
		bm[f.path] = f.objectID
	}
	tm := map[string]storage.ObjectID{}
	for _, f := range tip {
		tm[f.path] = f.objectID
	}
	paths := map[string]bool{}
	for p, id := range bm {
		if tm[p] != id {
			paths[p] = true
		}
	}
	for p, id := range tm {
		if bm[p] != id {
			paths[p] = true
		}
	}
	for p := range paths {
		s.ChangedPaths = append(s.ChangedPaths, p)
	}
	sort.Strings(s.ChangedPaths)
	if changes, e := filesBetween(r, storage.ObjectID(to), storage.ObjectID(from)); e == nil {
		for _, c := range changes {
			s.Changes = append(s.Changes, stackedchanges.Change{Path: c.Path, Status: c.Status, Additions: c.Additions, Deletions: c.Deletions, Binary: c.Binary, Patch: c.Patch})
		}
	}
	if commits, e := commitsBetween(r, storage.ObjectID(to), storage.ObjectID(from)); e == nil {
		for _, c := range commits {
			s.CommitIDs = append(s.CommitIDs, c.ID)
		}
	}
	seen := map[storage.ObjectID]bool{}
	q := []storage.ObjectID{storage.ObjectID(to)}
	for len(q) > 0 {
		x := q[0]
		q = q[1:]
		if x == storage.ObjectID(from) || seen[x] {
			continue
		}
		seen[x] = true
		s.CommitCount++
		c, e := r.ReadCommit(x)
		if e == nil {
			q = append(q, c.Parents...)
		}
	}
	return s
}
