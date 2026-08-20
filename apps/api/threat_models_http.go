package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reasoning"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityscenarios"
	"github.com/greptile-projects/vivarium-komodo/apps/api/threatmodels"
)

type threatModelSources struct {
	pulls interface {
		Get(string, string) (pullrequests.PullRequest, error)
	}
	plans     proposalStore
	scenarios *securityscenarios.Store
}

func registerThreatModelsHTTP(mux *http.ServeMux, s *threatmodels.Store, repos proposalRepositoryStore, c authStore, src threatModelSources) {
	base := "/repositories/{repository}/threat-models"
	access := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	project := func(repo, actor string, m *threatmodels.Model) {
		current := map[string]string{}
		if m.Origin.Kind == "pull_request" {
			if p, e := src.pulls.Get(repo, m.Origin.Reference); e == nil {
				current["origin:"+m.Origin.Reference] = p.SourceCommitID
				for _, x := range m.Inputs {
					if x.Kind == "code" {
						current[x.Kind+":"+x.Reference] = p.SourceCommitID
					}
				}
			}
		}
		threatmodels.Derive(m, current)
		isOwner := false
		for _, owner := range m.OwnerIDs {
			isOwner = isOwner || owner == actor
		}
		if !isOwner {
			visible := make([]threatmodels.Finding, 0, len(m.Findings))
			for _, f := range m.Findings {
				if f.Classification == nil || (f.Classification.Audience != "owners" && f.Classification.Audience != "embargoed") {
					visible = append(visible, f)
				}
			}
			m.Findings = visible
		}
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		items, e := s.List(repo)
		if threatModelError(w, e) {
			return
		}
		for i := range items {
			project(repo, actor, &items[i])
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in threatmodels.Input
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		if in.Origin.Kind == "pull_request" {
			p, e := src.pulls.Get(repo, in.Origin.Reference)
			if e != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_threat_model_origin"})
				return
			}
			if p.SourceCommitID != in.Origin.Revision {
				writeJSON(w, 409, map[string]string{"error": "exact_origin_revision_required"})
				return
			}
		}
		m, e := s.Create(repo, actor, in)
		if threatModelError(w, e) {
			return
		}
		project(repo, actor, &m)
		writeJSON(w, 201, m)
	})
	mux.HandleFunc("GET "+base+"/{model}", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		m, e := s.Get(repo, r.PathValue("model"))
		if threatModelError(w, e) {
			return
		}
		project(repo, actor, &m)
		writeJSON(w, 200, m)
	})
	mux.HandleFunc("POST "+base+"/{model}/findings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in threatmodels.FindingInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		m, e := s.AddFinding(repo, r.PathValue("model"), actor, in)
		if threatModelError(w, e) {
			return
		}
		project(repo, actor, &m)
		writeJSON(w, 201, m)
	})
	mux.HandleFunc("POST "+base+"/{model}/findings/{finding}/classification", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			Kind      string `json:"kind"`
			Audience  string `json:"audience"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		m, e := s.ClassifyFinding(repo, r.PathValue("model"), r.PathValue("finding"), actor, in.Kind, in.Audience, in.Rationale)
		if !threatModelError(w, e) {
			project(repo, actor, &m)
			writeJSON(w, 201, m)
		}
	})
	mux.HandleFunc("POST "+base+"/{model}/findings/{finding}/delivery", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		if src.plans == nil {
			writeJSON(w, 500, map[string]string{"error": "delivery_unavailable"})
			return
		}
		var in struct {
			OwnerKind                   string   `json:"owner_kind"`
			OwnerID                     string   `json:"owner_id"`
			CandidateRevision           string   `json:"candidate_revision"`
			AbusePathIDs                []string `json:"abuse_path_ids"`
			PermittedEvidenceReferences []string `json:"permitted_evidence_references"`
			AcceptanceCriteria          []string `json:"acceptance_criteria"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		m, e := s.Get(repo, r.PathValue("model"))
		if threatModelError(w, e) {
			return
		}
		var f *threatmodels.Finding
		for i := range m.Findings {
			if m.Findings[i].ID == r.PathValue("finding") {
				f = &m.Findings[i]
			}
		}
		if f == nil {
			writeJSON(w, 404, map[string]string{"error": "threat_finding_not_found"})
			return
		}
		isOwner := false
		for _, owner := range m.OwnerIDs {
			isOwner = isOwner || owner == actor
		}
		allowedPaths, allowedEvidence := map[string]bool{}, map[string]bool{}
		for _, path := range f.AbusePathIDs {
			allowedPaths[path] = true
		}
		if f.Classification != nil {
			for _, citation := range f.Citations {
				if citation.Visibility == "public" || (citation.Visibility == "repository" && f.Classification.Audience != "public") {
					allowedEvidence[citation.Reference] = true
				}
			}
		}
		valid := isOwner && f.Classification != nil && f.Classification.Kind == "confirmed" && f.Delivery == nil && f.Resolution == nil && in.CandidateRevision == m.Origin.Revision && len(in.AbusePathIDs) > 0 && len(in.PermittedEvidenceReferences) > 0 && len(in.AcceptanceCriteria) > 0 && (in.OwnerKind == "human" || in.OwnerKind == "agent") && in.OwnerID != ""
		for _, path := range in.AbusePathIDs {
			valid = valid && allowedPaths[path]
		}
		for _, evidence := range in.PermittedEvidenceReferences {
			valid = valid && allowedEvidence[evidence]
		}
		if !valid {
			writeJSON(w, 422, map[string]string{"error": "invalid_finding_delivery"})
			return
		}
		p, e := src.plans.Create(repo, actor, "Repair security finding", "Governed repair of threat-model finding "+f.ID+" at "+in.CandidateRevision+".")
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_finding_delivery"})
			return
		}
		ctx := &reasoning.Context{Kind: "threat_finding_repair", RepositoryID: repo, CommitID: in.CandidateRevision, Claim: f.Body, State: "confirmed", Rationale: strings.Join(in.PermittedEvidenceReferences, "; "), Verification: in.AcceptanceCriteria}
		t, e := src.plans.CreateTask(repo, p.ID, actor, proposals.TaskInput{Title: "Contain security finding", Outcome: "Contain abuse path and preserve regression protection", OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, CompletionCriteria: in.AcceptanceCriteria, VerificationPlan: append(append([]string{}, in.AcceptanceCriteria...), "demonstrate the abuse against the exact base and containment on the repair", "publish a reviewed security scenario when its audience is safe"), BaseRevision: in.CandidateRevision, ReasoningContext: ctx})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_finding_delivery"})
			return
		}
		kind := proposals.HumanAssignee
		if in.OwnerKind == "agent" {
			kind = proposals.AgentAssignee
		}
		t, e = src.plans.AssignTask(repo, p.ID, t.ID, actor, "", proposals.AssignmentInput{Kind: kind, AssigneeID: in.OwnerID, Mandate: "Repair only the classified threat using the permitted evidence and ordinary review.", RepositoryID: repo, BaseRevision: in.CandidateRevision})
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_finding_delivery"})
			return
		}
		m, e = s.LinkDelivery(repo, m.ID, f.ID, actor, threatmodels.DeliveryInput{ProposalID: p.ID, TaskID: t.ID, ResourceKind: "task", ResourceID: t.ID, OwnerKind: in.OwnerKind, OwnerID: in.OwnerID, CandidateRevision: in.CandidateRevision, AbusePathIDs: in.AbusePathIDs, PermittedCitationReferences: in.PermittedEvidenceReferences, AcceptanceCriteria: in.AcceptanceCriteria})
		if !threatModelError(w, e) {
			writeJSON(w, 201, map[string]any{"model": m, "proposal": p, "task": t, "authority": map[string]bool{"repository_write": false, "secret": false, "environment": false, "review": false, "merge": false}})
		}
	})
	mux.HandleFunc("POST "+base+"/{model}/findings/{finding}/verification", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			PullRequestID          string   `json:"pull_request_id"`
			DesignChangeReferences []string `json:"design_change_references"`
			CommitIDs              []string `json:"commit_ids"`
			ReviewID               string   `json:"review_id"`
			ScenarioID             string   `json:"security_scenario_id"`
			BaseAttemptID          string   `json:"base_attempt_id"`
			RepairAttemptID        string   `json:"repair_attempt_id"`
			MitigationCoverage     []string `json:"mitigation_coverage"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		m, e := s.Get(repo, r.PathValue("model"))
		if threatModelError(w, e) {
			return
		}
		p, e := src.pulls.Get(repo, in.PullRequestID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "exact_repair_pull_required"})
			return
		}
		var f *threatmodels.Finding
		for i := range m.Findings {
			if m.Findings[i].ID == r.PathValue("finding") {
				f = &m.Findings[i]
			}
		}
		if f == nil || f.Delivery == nil || p.SourceCommitID == f.Delivery.CandidateRevision {
			writeJSON(w, 422, map[string]string{"error": "distinct_repair_revision_required"})
			return
		}
		if src.scenarios == nil {
			writeJSON(w, 500, map[string]string{"error": "verification_unavailable"})
			return
		}
		sc, e := src.scenarios.Get(repo, in.ScenarioID)
		if e != nil || !sc.Approved || len(sc.Versions) == 0 || sc.Versions[len(sc.Versions)-1].ThreatModelID != m.ID {
			writeJSON(w, 422, map[string]string{"error": "security_scenario_required"})
			return
		}
		baseOK, repairOK := false, false
		for _, a := range sc.Attempts {
			baseOK = baseOK || (a.ID == in.BaseAttemptID && a.Revision == f.Delivery.CandidateRevision && a.Status == "failed")
			repairOK = repairOK || (a.ID == in.RepairAttemptID && a.Revision == p.SourceCommitID && a.Status == "passed")
		}
		if !baseOK || !repairOK {
			writeJSON(w, 422, map[string]string{"error": "base_failure_and_repair_containment_required"})
			return
		}
		m, e = s.VerifyDelivery(repo, m.ID, f.ID, actor, threatmodels.VerificationInput{PullRequestID: in.PullRequestID, DesignChangeReferences: in.DesignChangeReferences, CommitIDs: in.CommitIDs, ReviewID: in.ReviewID, ScenarioID: in.ScenarioID, BaseAttemptID: in.BaseAttemptID, RepairAttemptID: in.RepairAttemptID, MitigationCoverage: in.MitigationCoverage})
		if !threatModelError(w, e) {
			writeJSON(w, 201, m)
		}
	})
	mux.HandleFunc("POST "+base+"/{model}/findings/{finding}/resolution", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in threatmodels.ResolutionInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		m, e := s.ResolveFinding(repo, r.PathValue("model"), r.PathValue("finding"), actor, in)
		if !threatModelError(w, e) {
			writeJSON(w, 201, m)
		}
	})
	mux.HandleFunc("POST "+base+"/{model}/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			Decision       string `json:"decision"`
			Rationale      string `json:"rationale"`
			OriginRevision string `json:"origin_revision"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		m, e := s.Acknowledge(repo, r.PathValue("model"), actor, in.Decision, in.Rationale, in.OriginRevision)
		if threatModelError(w, e) {
			return
		}
		project(repo, actor, &m)
		writeJSON(w, 201, m)
	})
}
func threatModelError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, threatmodels.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "threat_model_not_found"})
	} else if errors.Is(e, threatmodels.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_threat_model"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
