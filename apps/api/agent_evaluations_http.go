package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/greptile-projects/vivarium-komodo/apps/api/agentevaluations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprofiles"
	"github.com/greptile-projects/vivarium-komodo/apps/api/agentprojects"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type agentEvaluationSources struct {
	projects *agentprojects.Store
	pulls    interface {
		Get(string, string) (pullrequests.PullRequest, error)
	}
}

func registerAgentEvaluationsHTTP(m *http.ServeMux, s *agentevaluations.Store, profiles *agentprofiles.Store, repos dataFlowRepositories, orgs *organizations.Store, credentials authStore, optional ...agentEvaluationSources) {
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
	repositoryAccess := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, map[bool]auth.Scope{true: auth.RepositoryWrite, false: auth.RepositoryRead}[write], write)
		if !ok {
			return "", "", false
		}
		if write && string(repo.OwnerID) != actor.UserID {
			writeJSON(w, 403, map[string]string{"error": "repository_owner_required"})
			return "", "", false
		}
		return string(repo.ID), actor.UserID, true
	}
	operatorAccess := func(w http.ResponseWriter, r *http.Request, kind string) (string, string, bool) {
		a, ok := authenticateRequest(w, r, credentials, auth.RepositoryWrite)
		if !ok {
			return "", "", false
		}
		if kind == "repository" {
			return r.PathValue("repository"), a.UserID, true
		}
		return r.PathValue("organization"), a.UserID, true
	}
	registerAgentOnboardingHTTP(m, base+"/onboardings", "repository", s, profiles, repositoryAccess, operatorAccess)
	registerAgentPilotHTTP(m, base+"/pilots", s, repos, credentials, repositoryAccess, optional)
	organizationAccess := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
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
	}
	registerAgentOnboardingHTTP(m, "/organizations/{organization}/agent-onboardings", "organization", s, profiles, organizationAccess, operatorAccess)
	if len(optional) > 0 && optional[0].projects != nil && optional[0].pulls != nil {
		registerAgentCandidateHTTP(m, s, repos, credentials, optional[0])
	}
}

func registerAgentPilotHTTP(m *http.ServeMux, base string, s *agentevaluations.Store, repos dataFlowRepositories, credentials authStore, ownerAccess onboardingAccess, optional []agentEvaluationSources) {
	reader := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	reconcile := func(repo string, x agentevaluations.Pilot) agentevaluations.Pilot {
		if len(optional) > 0 && optional[0].pulls != nil {
			if p, e := optional[0].pulls.Get(repo, x.PullRequestID); e == nil && p.SourceCommitID != x.CandidateRevision {
				x, _ = s.ReconcilePilotCandidate(repo, x.ID, "changed")
			}
		}
		return x
	}
	syncPilot := func(repo, pilot string) {
		if x, e := s.GetPilot(repo, pilot); e == nil {
			reconcile(repo, x)
		}
	}
	m.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := reader(w, r)
		if !ok {
			return
		}
		xs, e := s.ListPilots(repo)
		if !agentEvaluationError(w, e) {
			for i := range xs {
				xs[i] = reconcile(repo, xs[i])
			}
			writeJSON(w, 200, map[string]any{"items": xs})
		}
	})
	m.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := ownerAccess(w, r, true)
		if !ok {
			return
		}
		var in agentevaluations.PilotInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		for _, selected := range in.Repositories {
			target, e := repos.Inspect(storage.ID(selected))
			if e != nil || string(target.OwnerID) != actor {
				writeJSON(w, 403, map[string]string{"error": "selected_repository_owner_required"})
				return
			}
		}
		x, e := s.CreatePilot(repo, actor, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/{pilot}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := reader(w, r)
		if !ok {
			return
		}
		x, e := s.GetPilot(repo, r.PathValue("pilot"))
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, reconcile(repo, x))
		}
	})
	m.HandleFunc("POST "+base+"/{pilot}/consent", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := reader(w, r)
		if !ok {
			return
		}
		var in struct {
			State  string `json:"state"`
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &in, 8192) {
			return
		}
		x, e := s.SetPilotConsent(repo, r.PathValue("pilot"), actor, in.State, in.Reason)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{pilot}/sessions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := reader(w, r)
		if !ok {
			return
		}
		var in agentevaluations.PilotSessionInput
		if !readJSON(w, r, &in, 8192) {
			return
		}
		syncPilot(repo, r.PathValue("pilot"))
		x, e := s.StartPilotSession(repo, r.PathValue("pilot"), actor, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{pilot}/sessions/{session}/events", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := reader(w, r)
		if !ok {
			return
		}
		var in agentevaluations.PilotEventInput
		if !readJSON(w, r, &in, 32768) {
			return
		}
		syncPilot(repo, r.PathValue("pilot"))
		x, e := s.RecordPilotEvent(repo, r.PathValue("pilot"), r.PathValue("session"), actor, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{pilot}/feedback", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := reader(w, r)
		if !ok {
			return
		}
		var in agentevaluations.PilotFeedbackInput
		if !readJSON(w, r, &in, 32768) {
			return
		}
		x, e := s.RecordPilotFeedback(repo, r.PathValue("pilot"), actor, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("POST "+base+"/{pilot}/controls", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := ownerAccess(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Action string `json:"action"`
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &in, 8192) {
			return
		}
		x, e := s.ControlPilot(repo, r.PathValue("pilot"), actor, in.Action, in.Reason)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
}

func contractVersion(p agentprojects.Project, n int64) (agentprojects.Version, bool) {
	for _, v := range p.Versions {
		if v.Number == n {
			return v, true
		}
	}
	return agentprojects.Version{}, false
}
func revisionValue(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func assembledInputs(v agentprojects.Version, selections []agentevaluations.SuiteSelection, s *agentevaluations.Store, repo string) ([]agentevaluations.BoundInput, error) {
	out := []agentevaluations.BoundInput{{Key: "contract", Revision: strconv.FormatInt(v.Number, 10)}}
	for _, x := range v.Prompts {
		out = append(out, agentevaluations.BoundInput{Key: "prompt:" + x.ID, Revision: x.Revision})
	}
	for _, x := range v.Instructions {
		out = append(out, agentevaluations.BoundInput{Key: "instruction:" + x.ID, Revision: x.Revision})
	}
	for _, x := range v.Tools {
		out = append(out, agentevaluations.BoundInput{Key: "tool:" + x.Name, Revision: x.Revision})
	}
	for _, x := range v.Models {
		out = append(out, agentevaluations.BoundInput{Key: "model:" + x.Provider + "/" + x.Name, Revision: x.Revision})
	}
	for _, x := range v.KnowledgeSources {
		out = append(out, agentevaluations.BoundInput{Key: "knowledge:" + x.Reference, Revision: x.Revision})
	}
	for _, sel := range selections {
		suite, e := s.GetSuite(repo, sel.SuiteID, true)
		if e != nil {
			return nil, e
		}
		var sv *agentevaluations.SuiteVersion
		for i := range suite.Versions {
			if suite.Versions[i].Number == sel.SuiteVersion {
				sv = &suite.Versions[i]
			}
		}
		if sv == nil {
			return nil, agentevaluations.ErrInvalid
		}
		wanted := map[string]bool{}
		for _, id := range sel.ScenarioIDs {
			wanted[id] = true
		}
		for _, q := range sv.Scenarios {
			if wanted[q.ID] {
				out = append(out, agentevaluations.BoundInput{Key: "scenario:" + sel.SuiteID + ":" + q.ID, Revision: q.RepositoryRevision})
				for _, j := range q.Checks {
					out = append(out, agentevaluations.BoundInput{Key: "judge:" + sel.SuiteID + ":" + q.ID + ":" + j.ID, Revision: revisionValue(j)})
				}
				delete(wanted, q.ID)
			}
		}
		if len(wanted) > 0 {
			return nil, agentevaluations.ErrInvalid
		}
	}
	return out, nil
}
func registerAgentCandidateHTTP(m *http.ServeMux, s *agentevaluations.Store, repos dataFlowRepositories, credentials authStore, src agentEvaluationSources) {
	base := "/repositories/{repository}/pull-requests/{pull}/agent-candidates"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, pullrequests.PullRequest, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, map[bool]auth.Scope{true: auth.RepositoryWrite, false: auth.RepositoryRead}[write], write)
		if !ok {
			return "", "", pullrequests.PullRequest{}, false
		}
		p, e := src.pulls.Get(string(repo.ID), r.PathValue("pull"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "pull_request_not_found"})
			return "", "", p, false
		}
		return string(repo.ID), a.UserID, p, true
	}
	m.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, _, ok := access(w, r, false)
		if !ok {
			return
		}
		x, e := s.ListCandidates(repo, r.PathValue("pull"))
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, map[string]any{"items": x})
		}
	})
	m.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, p, ok := access(w, r, true)
		if !ok {
			return
		}
		var in agentevaluations.CandidateInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		in.PullRequestID = p.ID
		if in.Revision != p.SourceCommitID {
			writeJSON(w, 422, map[string]string{"error": "exact_pull_revision_required"})
			return
		}
		project, e := src.projects.Get(repo, in.AgentProjectID)
		if e != nil {
			agentEvaluationError(w, agentevaluations.ErrInvalid)
			return
		}
		v, ok := contractVersion(project, in.AgentProjectVersion)
		if !ok || v.RepositoryRevision != in.Revision {
			writeJSON(w, 422, map[string]string{"error": "exact_behavior_contract_required"})
			return
		}
		in.Inputs, e = assembledInputs(v, in.Suites, s, repo)
		if e != nil {
			agentEvaluationError(w, e)
			return
		}
		x, e := s.CreateCandidate(repo, actor, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/{candidate}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, _, ok := access(w, r, false)
		if !ok {
			return
		}
		x, e := s.GetCandidate(repo, r.PathValue("candidate"))
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	m.HandleFunc("POST "+base+"/{candidate}/attempts", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, p, ok := access(w, r, true)
		if !ok {
			return
		}
		x, e := s.GetCandidate(repo, r.PathValue("candidate"))
		if e != nil {
			agentEvaluationError(w, e)
			return
		}
		if x.PullRequestID != p.ID || x.Revision != p.SourceCommitID {
			writeJSON(w, 422, map[string]string{"error": "candidate_revision_stale"})
			return
		}
		var in agentevaluations.CandidateAttemptInput
		if !readJSON(w, r, &in, 4<<20) {
			return
		}
		x, e = s.RecordCandidateAttempt(repo, x.ID, actor, in)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	m.HandleFunc("GET "+base+"/{candidate}/comparison", func(w http.ResponseWriter, r *http.Request) {
		repo, _, p, ok := access(w, r, false)
		if !ok {
			return
		}
		candidate, e := s.GetCandidate(repo, r.PathValue("candidate"))
		if e != nil {
			agentEvaluationError(w, e)
			return
		}
		if candidate.Revision != p.SourceCommitID {
			writeJSON(w, 409, map[string]string{"error": "candidate_revision_stale"})
			return
		}
		x, e := s.CompareCandidates(repo, r.URL.Query().Get("baseline"), candidate.ID)
		if !agentEvaluationError(w, e) {
			writeJSON(w, 200, x)
		}
	})
}

type onboardingAccess func(http.ResponseWriter, *http.Request, bool) (string, string, bool)

func registerAgentOnboardingHTTP(m *http.ServeMux, base, kind string, s *agentevaluations.Store, profiles *agentprofiles.Store, access onboardingAccess, operatorAccess func(http.ResponseWriter, *http.Request, string) (string, string, bool)) {
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
		scope, actor, ok := operatorAccess(w, r, kind)
		if !ok {
			return
		}
		onboarding, e := s.GetOnboarding(kind, scope, r.PathValue("onboarding"))
		if e != nil {
			agentEvaluationError(w, e)
			return
		}
		profile, e := profiles.Get(onboarding.Versions[len(onboarding.Versions)-1].ProfileID)
		if e != nil || profile.OperatorID != actor {
			writeJSON(w, 403, map[string]string{"error": "agent_operator_required"})
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
