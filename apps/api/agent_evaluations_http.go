package main

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/agentevaluations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprofiles"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
)

func registerAgentEvaluationsHTTP(m *http.ServeMux, s *agentevaluations.Store, profiles *agentprofiles.Store, repos dataFlowRepositories, orgs *organizations.Store, credentials authStore) {
	base := "/repositories/{repository}/agent-evaluations"
	m.HandleFunc("GET "+base+"/suites", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		xs, e := s.ListSuites(string(repo.ID))
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, map[string]any{"items": xs})
		}
	})
	m.HandleFunc("POST "+base+"/suites", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentevaluations.SuiteInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(string(repo.ID), a.UserID, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/suites/{suite}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		x, e := s.GetSuite(string(repo.ID), r.PathValue("suite"), false)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	m.HandleFunc("POST "+base+"/suites/{suite}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentevaluations.SuiteInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(string(repo.ID), r.PathValue("suite"), a.UserID, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/trials", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		xs, e := s.ListTrials(string(repo.ID))
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, map[string]any{"items": xs})
		}
	})
	m.HandleFunc("POST "+base+"/trials", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentevaluations.TrialInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		profile, e := profiles.Get(in.ProfileID)
		if e != nil || in.ProfileVersion > profile.CurrentVersion {
			agentEvaluationError(w, agentevaluations.ErrInvalid)
			return
		}
		x, e := s.Start(string(repo.ID), a.UserID, in)
		if !agentEvaluationError(w, e) {
			w.Header().Set("Location", "/repositories/"+string(repo.ID)+"/agent-evaluations/trials/"+x.ID)
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/trials/{trial}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		x, e := s.GetTrial(string(repo.ID), r.PathValue("trial"))
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	m.HandleFunc("POST "+base+"/trials/{trial}/result", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentevaluations.ResultInput
		if !readJSON(w, r, &in, 4<<20) {
			return
		}
		x, e := s.Complete(string(repo.ID), r.PathValue("trial"), in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	m.HandleFunc("POST "+base+"/trials/{trial}/decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in agentevaluations.DecisionInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Decide(string(repo.ID), r.PathValue("trial"), a.UserID, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	registerAgentOnboardingHTTP(m, base+"/onboardings", "repository", s, profiles, func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, map[bool]auth.Scope{true: auth.RepositoryWrite, false: auth.RepositoryRead}[write], write)
		if !ok {
			return "", "", false
		}
		if write && string(repo.OwnerID) != actor.UserID {
			writeJSON(w, 403, map[string]string{"error": "repository_owner_required"})
			return "", "", false
		}
		return string(repo.ID), actor.UserID, true
	})
	registerAgentOnboardingHTTP(m, "/organizations/{organization}/agent-onboardings", "organization", s, profiles, func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		actor, ok := authenticateRequest(w, r, credentials, map[bool]auth.Scope{true: auth.RepositoryWrite, false: auth.RepositoryRead}[write])
		if !ok {
			return "", "", false
		}
		id := r.PathValue("organization")
		if (write && !orgs.IsOwner(id, actor.UserID)) || (!write && !orgs.IsMember(id, actor.UserID)) {
			writeJSON(w, 403, map[string]string{"error": "organization_owner_required"})
			return "", "", false
		}
		return id, actor.UserID, true
	})
}

type onboardingAccess func(http.ResponseWriter, *http.Request, bool) (string, string, bool)

func registerAgentOnboardingHTTP(m *http.ServeMux, base, kind string, s *agentevaluations.Store, profiles *agentprofiles.Store, access onboardingAccess) {
	m.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, _, ok := access(w, r, false)
		if !ok {
			return
		}
		x, e := s.ListOnboardings(kind, scope)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, map[string]any{"items": x})
		}
	})
	m.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in agentevaluations.OnboardingInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.CreateOnboarding(kind, scope, actor, in)
		if !agentEvaluationError(w, e) {
			w.Header().Set("Location", base+"/"+x.ID)
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/{onboarding}", func(w http.ResponseWriter, r *http.Request) {
		scope, _, ok := access(w, r, false)
		if !ok {
			return
		}
		x, e := s.GetOnboarding(kind, scope, r.PathValue("onboarding"))
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	m.HandleFunc("POST "+base+"/{onboarding}/versions", func(w http.ResponseWriter, r *http.Request) {
		scope, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in agentevaluations.OnboardingInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.ReviseOnboarding(kind, scope, r.PathValue("onboarding"), actor, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{onboarding}/decisions", func(w http.ResponseWriter, r *http.Request) {
		scope, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Decision string `json:"decision"`
			Note     string `json:"note"`
			Version  int64  `json:"version"`
		}
		if !readJSON(w, r, &in, 8192) {
			return
		}
		x, e := s.DecideOnboarding(kind, scope, r.PathValue("onboarding"), actor, in.Decision, in.Note, in.Version)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{onboarding}/operator-agreement", func(w http.ResponseWriter, r *http.Request) {
		scope, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Terms   string `json:"terms"`
			Version int64  `json:"version"`
		}
		if !readJSON(w, r, &in, 8192) {
			return
		}
		x, e := s.AgreeOnboarding(kind, scope, r.PathValue("onboarding"), actor, in.Terms, in.Version)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{onboarding}/activation", func(w http.ResponseWriter, r *http.Request) {
		scope, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Version int64 `json:"version"`
		}
		if !readJSON(w, r, &in, 1024) {
			return
		}
		x, e := s.ActivateOnboarding(kind, scope, r.PathValue("onboarding"), actor, in.Version)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{onboarding}/revocation", func(w http.ResponseWriter, r *http.Request) {
		scope, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &in, 8192) {
			return
		}
		x, e := s.RevokeOnboarding(kind, scope, r.PathValue("onboarding"), actor, in.Reason)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/{onboarding}/profile-comparison", func(w http.ResponseWriter, r *http.Request) {
		scope, _, ok := access(w, r, false)
		if !ok {
			return
		}
		x, e := s.GetOnboarding(kind, scope, r.PathValue("onboarding"))
		if agentEvaluationError(w, e) {
			return
		}
		p, e := profiles.Get(x.Versions[len(x.Versions)-1].ProfileID)
		if e != nil {
			agentEvaluationError(w, agentevaluations.ErrInvalid)
			return
		}
		to := p.CurrentVersion
		if q := r.URL.Query().Get("to_version"); q != "" {
			if _, e := fmt.Sscan(q, &to); e != nil {
				agentEvaluationError(w, agentevaluations.ErrInvalid)
				return
			}
		}
		c, e := agentprofiles.CompareVersions(p, x.Trust.ConsentProfileVersion, to)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_profile_comparison"})
			return
		}
		writeJSON(w, 200, c)
	})
	m.HandleFunc("PUT "+base+"/{onboarding}/trust-policy", func(w http.ResponseWriter, r *http.Request) {
		scope, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in agentevaluations.ReevaluationPolicy
		if !readJSON(w, r, &in, 8192) {
			return
		}
		x, e := s.SetTrustPolicy(kind, scope, r.PathValue("onboarding"), actor, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	m.HandleFunc("POST "+base+"/{onboarding}/outcomes", func(w http.ResponseWriter, r *http.Request) {
		scope, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in agentevaluations.OutcomeInput
		if !readJSON(w, r, &in, 32768) {
			return
		}
		x, e := s.RecordOutcome(kind, scope, r.PathValue("onboarding"), actor, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{onboarding}/reevaluations", func(w http.ResponseWriter, r *http.Request) {
		scope, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			TrialID        string `json:"trial_id"`
			ProfileVersion int64  `json:"profile_version"`
			Result         string `json:"result"`
			Rationale      string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 8192) {
			return
		}
		x, e := s.RecordReevaluation(kind, scope, r.PathValue("onboarding"), actor, in.TrialID, in.Result, in.Rationale, in.ProfileVersion)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{onboarding}/authority-controls", func(w http.ResponseWriter, r *http.Request) {
		scope, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Action          string   `json:"action"`
			Resources       []string `json:"resources"`
			Actions         []string `json:"actions"`
			Reason          string   `json:"reason"`
			ExpectedVersion int64    `json:"expected_version"`
		}
		if !readJSON(w, r, &in, 16384) {
			return
		}
		x, e := s.ControlAuthority(kind, scope, r.PathValue("onboarding"), actor, in.Action, in.Reason, in.Resources, in.Actions, in.ExpectedVersion)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{onboarding}/handoffs", func(w http.ResponseWriter, r *http.Request) {
		scope, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in agentevaluations.HandoffInput
		if !readJSON(w, r, &in, 32768) {
			return
		}
		x, e := s.CreateHandoff(kind, scope, r.PathValue("onboarding"), actor, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{onboarding}/handoffs/{handoff}/acceptance", func(w http.ResponseWriter, r *http.Request) {
		scope, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Verification    string `json:"verification"`
			ExpectedVersion int64  `json:"expected_version"`
		}
		if !readJSON(w, r, &in, 8192) {
			return
		}
		x, e := s.AcceptHandoff(kind, scope, r.PathValue("onboarding"), r.PathValue("handoff"), actor, in.Verification, in.ExpectedVersion)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
}
func agentEvaluationError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, agentevaluations.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "agent_evaluation_not_found"})
	case errors.Is(e, agentevaluations.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "stale_agent_evaluation_version"})
	case errors.Is(e, agentevaluations.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_agent_evaluation"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
