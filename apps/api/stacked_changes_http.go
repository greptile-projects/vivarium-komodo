package main

import (
	"errors"
	"net/http"
	"sort"
	"strings"

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
		members, blockers := analyzeChangeStack(opened, in, true)
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
}

func projectStackPermissions(x stackedchanges.Stack, actor auth.Grant) stackedchanges.Stack {
	canPublish := false
	for _, scope := range actor.Scopes {
		if scope == auth.RepositoryWrite {
			canPublish = true
			break
		}
	}
	for i := range x.Members {
		x.Members[i].EffectivePermissions = stackedchanges.Permission{Read: true, Publish: canPublish, UpdateBranch: false, Reason: "effective caller access; branch updates require separate Git authority"}
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

func analyzeChangeStack(repo *storage.Repository, in stackedchanges.Input, canWrite bool) ([]stackedchanges.Member, []stackedchanges.Blocker) {
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
			} else if string(ref.ObjectID) != m.Revision {
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
	s := stackedchanges.Scope{FromRevision: from, ToRevision: to, ChangedPaths: []string{}}
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
