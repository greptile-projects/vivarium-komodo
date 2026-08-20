package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securitydelivery"
	"github.com/greptile-projects/vivarium-komodo/apps/api/securityscenarios"
	"github.com/greptile-projects/vivarium-komodo/apps/api/threatmodels"
)

type securityDeliveryInput struct {
	SubjectKind    string   `json:"subject_kind"`
	SubjectID      string   `json:"subject_id"`
	Revision       string   `json:"revision"`
	Branch         string   `json:"branch"`
	Components     []string `json:"components"`
	Assets         []string `json:"assets"`
	RiskClasses    []string `json:"risk_classes"`
	ThreatModelIDs []string `json:"threat_model_ids"`
	ScenarioIDs    []string `json:"scenario_ids"`
}

type securityDeliverySources struct {
	store     *securitydelivery.Store
	models    *threatmodels.Store
	scenarios *securityscenarios.Store
}

func (x securityDeliverySources) assess(repo, organization, kind, subject, revision, branch string, components, assets, risks []string) (securitydelivery.Assessment, error) {
	scopes := []string{"repository:" + repo}
	if organization != "" {
		scopes = append(scopes, "organization:"+organization)
	}
	modelIDs, scenarioIDs := []string{}, []string{}
	for _, scope := range scopes {
		policies, err := x.store.ListPolicies(scope)
		if err != nil {
			return securitydelivery.Assessment{}, err
		}
		for _, p := range policies {
			for _, id := range p.RequiredThreatModels {
				if !securityContains(modelIDs, id) {
					modelIDs = append(modelIDs, id)
				}
			}
			for _, id := range p.RequiredScenarios {
				if !securityContains(scenarioIDs, id) {
					scenarioIDs = append(scenarioIDs, id)
				}
			}
		}
	}
	ev, err := securityEvidence(repo, securityDeliveryInput{Revision: revision, ThreatModelIDs: modelIDs, ScenarioIDs: scenarioIDs}, x.models, x.scenarios)
	if err != nil {
		return securitydelivery.Assessment{}, err
	}
	return x.store.Assess(scopes, kind, subject, revision, branch, components, assets, risks, ev)
}

func securityContains(xs []string, value string) bool {
	for _, x := range xs {
		if x == value {
			return true
		}
	}
	return false
}

func securityEvidence(repo string, in securityDeliveryInput, models *threatmodels.Store, scenarios *securityscenarios.Store) ([]securitydelivery.Evidence, error) {
	out := []securitydelivery.Evidence{}
	byModel := map[string]int{}
	for _, id := range in.ThreatModelIDs {
		m, err := models.Get(repo, id)
		if err != nil {
			return nil, err
		}
		e := securitydelivery.Evidence{ThreatModelID: m.ID, ThreatModelRevision: m.Origin.Revision, Current: !m.Stale, ResidualRisk: m.ResidualRisk, InputKeys: []string{}}
		for _, b := range m.Inputs {
			e.InputKeys = append(e.InputKeys, b.Kind+":"+b.Reference)
			if b.Kind == "code" && b.Revision != in.Revision {
				e.Current = false
			}
		}
		for _, f := range m.Findings {
			if f.Classification != nil && f.Classification.Kind == "confirmed" && (f.Delivery == nil || f.Delivery.VerifiedAt == nil) {
				e.UnresolvedFindingIDs = append(e.UnresolvedFindingIDs, f.ID)
			}
		}
		byModel[m.ID] = len(out)
		out = append(out, e)
	}
	for _, id := range in.ScenarioIDs {
		s, err := scenarios.Get(repo, id)
		if err != nil {
			return nil, err
		}
		v := s.Versions[len(s.Versions)-1]
		e := securitydelivery.Evidence{ThreatModelID: v.ThreatModelID, ThreatModelRevision: v.ThreatModelRevision, ScenarioID: s.ID, ScenarioVersion: s.CurrentVersion, Current: s.Approved, InputKeys: []string{"scenario:" + s.ID, "threat_model:" + v.ThreatModelID}}
		for i := range s.Attempts {
			a := s.Attempts[i]
			if a.ScenarioVersion == s.CurrentVersion && a.Revision == in.Revision {
				e.AttemptID = a.ID
				e.AttemptRevision = a.Revision
				e.AttemptStatus = a.Status
				if len(a.Coverage.ContainmentIDs) > 0 {
					e.Coverage = append(e.Coverage, "containment")
				}
				if len(a.Coverage.DetectionIDs) > 0 {
					e.Coverage = append(e.Coverage, "detection")
				}
				if len(a.Coverage.RecoveryIDs) > 0 {
					e.Coverage = append(e.Coverage, "recovery")
				}
			}
		}
		if i, ok := byModel[v.ThreatModelID]; ok {
			e.ResidualRisk = out[i].ResidualRisk
			e.UnresolvedFindingIDs = out[i].UnresolvedFindingIDs
		}
		out = append(out, e)
	}
	return out, nil
}

func registerSecurityDeliveryHTTP(mux *http.ServeMux, s *securitydelivery.Store, models *threatmodels.Store, scenarios *securityscenarios.Store, repos proposalRepositoryStore, orgs *organizations.Store, credentials authStore) {
	mux.HandleFunc("POST /repositories/{repository}/security-delivery-policies", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if a.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in securitydelivery.PolicyInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		in.ScopeKind = "repository"
		in.ScopeID = string(repo.ID)
		p, e := s.CreatePolicy("repository:"+string(repo.ID), a.UserID, in)
		if errors.Is(e, securitydelivery.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_security_delivery_policy"})
			return
		}
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("GET /repositories/{repository}/security-delivery-policies", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.ListPolicies("repository:" + string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": x, "total_count": len(x)})
	})
	mux.HandleFunc("POST /organizations/{organization}/security-delivery-policies", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		oid := r.PathValue("organization")
		o, e := orgs.Get(oid)
		owner := false
		for _, member := range o.Members {
			owner = owner || member.UserID == a.UserID && member.Role == "owner"
		}
		if e != nil || !owner {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in securitydelivery.PolicyInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		in.ScopeKind = "organization"
		in.ScopeID = oid
		p, e := s.CreatePolicy("organization:"+oid, a.UserID, in)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_security_delivery_policy"})
			return
		}
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("GET /organizations/{organization}/security-delivery-policies", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		oid := r.PathValue("organization")
		if !orgs.IsMember(oid, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		x, e := s.ListPolicies("organization:" + oid)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": x, "total_count": len(x)})
	})
	mux.HandleFunc("POST /organizations/{organization}/security-delivery/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.RepositoryRead)
		if !ok {
			return
		}
		oid := r.PathValue("organization")
		if !orgs.IsMember(oid, a.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			PolicyID    string `json:"policy_id"`
			SubjectKind string `json:"subject_kind"`
			SubjectID   string `json:"subject_id"`
			Revision    string `json:"revision"`
			Decision    string `json:"decision"`
			Rationale   string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.Acknowledge("organization:"+oid, in.PolicyID, in.SubjectKind, in.SubjectID, in.Revision, a.UserID, in.Decision, in.Rationale)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "control_owner_acknowledgement_required"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST /organizations/{organization}/security-delivery/exceptions", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return
		}
		oid := r.PathValue("organization")
		o, e := orgs.Get(oid)
		owner := false
		for _, m := range o.Members {
			owner = owner || m.UserID == a.UserID && m.Role == "owner"
		}
		if e != nil || !owner {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			PolicyID         string    `json:"policy_id"`
			SubjectKind      string    `json:"subject_kind"`
			SubjectID        string    `json:"subject_id"`
			Revision         string    `json:"revision"`
			Reason           string    `json:"reason"`
			RequirementKinds []string  `json:"requirement_kinds"`
			ExpiresAt        time.Time `json:"expires_at"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.Except("organization:"+oid, in.PolicyID, in.SubjectKind, in.SubjectID, in.Revision, a.UserID, in.Reason, in.RequirementKinds, in.ExpiresAt)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_security_delivery_exception"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST /repositories/{repository}/security-delivery/assessments", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		var in securityDeliveryInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		ev, e := securityEvidence(string(repo.ID), in, models, scenarios)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "exact_security_evidence_required"})
			return
		}
		scopes := []string{"repository:" + string(repo.ID)}
		if repo.OrganizationID != "" {
			scopes = append(scopes, "organization:"+repo.OrganizationID)
		}
		a, e := s.Assess(scopes, in.SubjectKind, in.SubjectID, in.Revision, in.Branch, in.Components, in.Assets, in.RiskClasses, ev)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, a)
	})
	mux.HandleFunc("POST /repositories/{repository}/security-delivery/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var in struct {
			PolicyID    string `json:"policy_id"`
			SubjectKind string `json:"subject_kind"`
			SubjectID   string `json:"subject_id"`
			Revision    string `json:"revision"`
			Decision    string `json:"decision"`
			Rationale   string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.Acknowledge("repository:"+string(repo.ID), in.PolicyID, in.SubjectKind, in.SubjectID, in.Revision, a.UserID, in.Decision, in.Rationale)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "control_owner_acknowledgement_required"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST /repositories/{repository}/security-delivery/exceptions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if a.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			PolicyID         string    `json:"policy_id"`
			SubjectKind      string    `json:"subject_kind"`
			SubjectID        string    `json:"subject_id"`
			Revision         string    `json:"revision"`
			Reason           string    `json:"reason"`
			RequirementKinds []string  `json:"requirement_kinds"`
			ExpiresAt        time.Time `json:"expires_at"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.Except("repository:"+string(repo.ID), in.PolicyID, in.SubjectKind, in.SubjectID, in.Revision, a.UserID, in.Reason, in.RequirementKinds, in.ExpiresAt)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_security_delivery_exception"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST /repositories/{repository}/security-signals", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			securitydelivery.SignalInput
			Sanitized bool `json:"sanitized"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		x, e := s.RecordSignal("repository:"+string(repo.ID), a.UserID, in.SignalInput, in.Sanitized)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "sanitized_revision_exact_signal_required"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST /repositories/{repository}/security-signals/{signal}/responses", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if a.UserID != repo.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var in struct {
			Kind       string `json:"kind"`
			ResourceID string `json:"resource_id"`
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		x, e := s.OpenResponse("repository:"+string(repo.ID), r.PathValue("signal"), a.UserID, in.Kind, in.ResourceID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "violated_signal_and_private_response_required"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("GET /repositories/{repository}/security-signals", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.ListSignals("repository:" + string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": x, "total_count": len(x)})
	})
}
