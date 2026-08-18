package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designgovernance"
	"github.com/greptile-projects/vivarium-komodo/apps/api/interfacechecks"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type designGovernanceRepositories interface {
	proposalRepositoryStore
	Open(storage.ID) (*storage.Repository, error)
}

func registerDesignGovernanceHTTP(mux *http.ServeMux, s *designgovernance.Store, evidence *interfacechecks.Store, repos designGovernanceRepositories, orgs *organizations.Store, credentials authStore, pulls pullRequestStore) {
	base := "/repositories/{repository}/design-governance"
	mux.HandleFunc("POST "+base+"/policies", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in designgovernance.PolicyInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		p, e := s.CreatePolicy("repository", string(repo.ID), "", actor.UserID, in)
		if designGovernanceError(w, e) {
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("GET "+base+"/policies", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := s.ListPolicies("repository", string(repo.ID))
		if designGovernanceError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": v, "total_count": len(v)})
	})
	mux.HandleFunc("POST "+base+"/pull-requests/{pull}/acceptances", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			PolicyID  string `json:"policy_id"`
			Revision  string `json:"revision"`
			PreviewID string `json:"preview_id"`
			Role      string `json:"role"`
			Decision  string `json:"decision"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		p, e := pulls.Get(string(repo.ID), r.PathValue("pull"))
		if e != nil || p.SourceCommitID != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "exact_pull_revision_required"})
			return
		}
		a, e := s.Accept(string(repo.ID), p.ID, in.PolicyID, in.Revision, in.PreviewID, in.Role, in.Decision, in.Rationale, actor.UserID)
		if designGovernanceError(w, e) {
			return
		}
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("POST "+base+"/pull-requests/{pull}/exceptions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			PolicyID     string    `json:"policy_id"`
			Revision     string    `json:"revision"`
			Reason       string    `json:"reason"`
			OwnerID      string    `json:"owner_id"`
			FollowUpKind string    `json:"follow_up_kind"`
			FollowUpID   string    `json:"follow_up_id"`
			ExpiresAt    time.Time `json:"expires_at"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		p, e := pulls.Get(string(repo.ID), r.PathValue("pull"))
		if e != nil || p.SourceCommitID != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "exact_pull_revision_required"})
			return
		}
		x, e := s.Except(string(repo.ID), p.ID, in.PolicyID, in.Revision, in.Reason, in.OwnerID, in.FollowUpKind, in.FollowUpID, actor.UserID, in.ExpiresAt)
		if designGovernanceError(w, e) {
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/work", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in designgovernance.WorkInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		if in.AffectedRepository != string(repo.ID) {
			writeJSON(w, 422, map[string]string{"error": "affected_repository_mismatch"})
			return
		}
		v, e := s.CreateWork(actor.UserID, in)
		if designGovernanceError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/release-readiness", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		var in struct {
			PullRequestID string   `json:"pull_request_id"`
			Revision      string   `json:"revision"`
			TargetBranch  string   `json:"target_branch"`
			Components    []string `json:"components"`
			Journeys      []string `json:"journeys"`
			Paths         []string `json:"paths"`
			RiskClasses   []string `json:"risk_classes"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		p, e := pulls.Get(string(repo.ID), in.PullRequestID)
		if e != nil || p.SourceCommitID != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "exact_release_candidate_required"})
			return
		}
		policies, e := s.ListPolicies("repository", string(repo.ID))
		if designGovernanceError(w, e) {
			return
		}
		if repo.OrganizationID != "" {
			v, er := s.ListPolicies("organization", repo.OrganizationID)
			if designGovernanceError(w, er) {
				return
			}
			policies = append(policies, v...)
		}
		opened, e := repos.Open(storage.ID(p.SourceRepositoryID))
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "source_repository_unavailable"})
			return
		}
		runs, e := evidence.List(string(repo.ID), p.ID)
		if designGovernanceError(w, e) {
			return
		}
		for i := range runs {
			cfg, _, _ := interfaceCheckBlob(opened, in.Revision, runs[i].ConfigPath)
			blobs := map[string]string{}
			for _, cs := range runs[i].Cases {
				for _, input := range cs.Inputs {
					oid, _, found := interfaceCheckBlob(opened, in.Revision, input.Path)
					if found {
						blobs[input.Path] = oid
					}
				}
			}
			interfacechecks.DeriveCurrent(&runs[i], in.Revision, cfg, blobs)
		}
		a, e := s.Assess(string(repo.ID), p.ID, in.Revision, in.TargetBranch, designgovernance.Selector{Components: in.Components, Journeys: in.Journeys, Paths: in.Paths, RiskClasses: in.RiskClasses}, policies, runs, designUsages(opened, in.Revision))
		if designGovernanceError(w, e) {
			return
		}
		writeJSON(w, 200, a)
	})
	mux.HandleFunc("POST /organizations/{organization}/design-governance/policies", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		org, e := orgs.Get(r.PathValue("organization"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		owner := false
		for _, m := range org.Members {
			owner = owner || (m.UserID == actor.UserID && m.Role == "owner")
		}
		if !owner {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in designgovernance.PolicyInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		p, e := s.CreatePolicy("organization", org.ID, org.ID, actor.UserID, in)
		if designGovernanceError(w, e) {
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("GET /organizations/{organization}/design-governance/policies", func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		id := r.PathValue("organization")
		if !orgs.IsMember(id, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		v, e := s.ListPolicies("organization", id)
		if designGovernanceError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": v, "total_count": len(v)})
	})
}

func designUsages(repository *storage.Repository, revision string) []designgovernance.Usage {
	_, body, ok := interfaceCheckBlob(repository, revision, ".komodo/design-usage.json")
	if !ok {
		return nil
	}
	var v struct {
		SchemaVersion int                      `json:"schema_version"`
		Usages        []designgovernance.Usage `json:"usages"`
	}
	if json.Unmarshal(body, &v) != nil || v.SchemaVersion != 1 {
		return nil
	}
	return v.Usages
}

func designSelector(paths []string, runs []interfacechecks.Run, uses []designgovernance.Usage) designgovernance.Selector {
	v := designgovernance.Selector{Paths: paths}
	seenComponent := map[string]bool{}
	seenJourney := map[string]bool{}
	for _, p := range paths {
		if strings.Contains(p, "security") || strings.HasPrefix(p, "migrations/") {
			v.RiskClasses = []string{"high"}
			break
		}
	}
	for _, u := range uses {
		if !seenComponent[u.ComponentID] {
			v.Components = append(v.Components, u.ComponentID)
			seenComponent[u.ComponentID] = true
		}
	}
	for _, run := range runs {
		for _, c := range run.Cases {
			if !seenComponent[c.Surface] {
				v.Components = append(v.Components, c.Surface)
				seenComponent[c.Surface] = true
			}
			if !seenJourney[c.Journey] {
				v.Journeys = append(v.Journeys, c.Journey)
				seenJourney[c.Journey] = true
			}
		}
	}
	return v
}
func designGovernanceError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, designgovernance.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_design_governance_record"})
	} else if errors.Is(e, designgovernance.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
