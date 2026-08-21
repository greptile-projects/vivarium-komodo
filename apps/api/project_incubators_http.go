package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/agentevaluations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/governance"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productfeedback"
	"github.com/greptile-projects/vivarium-komodo/apps/api/projectincubators"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/supportquestions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

type incubatorSources struct {
	feedback interface {
		Get(string, string) (productfeedback.Feedback, error)
	}
	support interface {
		Get(string, string) (supportquestions.Question, error)
	}
	governance interface {
		GetProposal(string, string, string) (governance.GovernedProposal, error)
	}
	repos interface {
		Inspect(storage.ID) (repositories.Repository, error)
		IsCollaborator(storage.ID, string) (bool, error)
	}
	agents interface {
		GetOnboarding(string, string, string) (agentevaluations.Onboarding, error)
	}
	users interface {
		Get(users.ID) (users.User, error)
	}
}

func registerProjectIncubatorsHTTP(m *http.ServeMux, s *projectincubators.Store, c authStore, src incubatorSources) {
	authn := func(w http.ResponseWriter, r *http.Request, write bool) (auth.Grant, bool) {
		scope := auth.RepositoryRead
		if write {
			scope = auth.RepositoryWrite
		}
		return authenticateRequest(w, r, c, scope)
	}
	visibleRepo := func(id, actor string) bool {
		v, e := src.repos.Inspect(storage.ID(id))
		if e != nil {
			return false
		}
		if v.Visibility == repositories.Public || v.OwnerID == actor {
			return true
		}
		ok, _ := src.repos.IsCollaborator(v.ID, actor)
		return ok
	}
	resolve := func(x projectincubators.Source, actor string) projectincubators.Source {
		x.Status = "accessible"
		ok := true
		switch x.Kind {
		case "idea":
		case "feedback":
			if !visibleRepo(x.RepositoryID, actor) {
				ok = false
			} else {
				_, e := src.feedback.Get(x.RepositoryID, x.ResourceID)
				ok = e == nil
			}
		case "support_gap":
			if !visibleRepo(x.RepositoryID, actor) {
				ok = false
			} else {
				_, e := src.support.Get(x.RepositoryID, x.ResourceID)
				ok = e == nil
			}
		case "governed_proposal":
			scopeKind, scopeID := "repository", x.RepositoryID
			if x.OrganizationID != "" {
				scopeKind, scopeID = "organization", x.OrganizationID
			}
			if scopeKind == "repository" && !visibleRepo(scopeID, actor) {
				ok = false
			} else {
				_, e := src.governance.GetProposal(scopeKind, scopeID, x.ResourceID)
				ok = e == nil
			}
		default:
			ok = false
		}
		if !ok {
			x.Status = "inaccessible"
			x.Detail = "The cited source does not exist or is not readable by the creator; no source content was copied."
		}
		return x
	}
	m.HandleFunc("GET /project-incubators", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, false)
		if !ok {
			return
		}
		items, e := s.List(a.UserID)
		if !incubatorError(w, e) {
			writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
		}
	})
	m.HandleFunc("POST /project-incubators", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, true)
		if !ok {
			return
		}
		var in struct {
			projectincubators.Input
			Source projectincubators.Source `json:"source"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.Create(a.UserID, in.Input, resolve(in.Source, a.UserID))
		if !incubatorError(w, e) {
			w.Header().Set("Location", "/project-incubators/"+v.ID)
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("GET /project-incubators/{incubator}", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(r.PathValue("incubator"), a.UserID)
		if !incubatorError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	m.HandleFunc("POST /project-incubators/{incubator}/participants", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, true)
		if !ok {
			return
		}
		var p projectincubators.Participant
		if !readJSON(w, r, &p, 32768) {
			return
		}
		if p.Kind == "human" {
			if _, e := src.users.Get(users.ID(p.UserID)); e != nil {
				writeJSON(w, 422, map[string]string{"error": "invalid_participant"})
				return
			}
		} else if p.Kind == "agent" {
			o, e := src.agents.GetOnboarding(p.OnboardingScopeKind, p.OnboardingScopeID, p.OnboardingID)
			if e != nil || o.State != "active" || o.Identity != p.AgentIdentity {
				writeJSON(w, 422, map[string]string{"error": "agent_not_approved"})
				return
			}
		}
		v, e := s.Invite(r.PathValue("incubator"), a.UserID, p)
		if !incubatorError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-incubators/{incubator}/participants/{participant}/consent", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Decision string `json:"decision"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		v, e := s.Consent(r.PathValue("incubator"), r.PathValue("participant"), a.UserID, in.Decision)
		if !incubatorError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	m.HandleFunc("POST /project-incubators/{incubator}/comments", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		v, e := s.Comment(r.PathValue("incubator"), a.UserID, in.Body)
		if !incubatorError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-incubators/{incubator}/evidence", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, true)
		if !ok {
			return
		}
		var in projectincubators.Evidence
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		v, e := s.AddEvidence(r.PathValue("incubator"), a.UserID, in)
		if !incubatorError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-incubators/{incubator}/assumptions", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Statement string `json:"statement"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		v, e := s.AddAssumption(r.PathValue("incubator"), a.UserID, in.Statement)
		if !incubatorError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-incubators/{incubator}/assumptions/{assumption}/status", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Status string `json:"status"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		v, e := s.ResolveAssumption(r.PathValue("incubator"), r.PathValue("assumption"), a.UserID, in.Status)
		if !incubatorError(w, e) {
			writeJSON(w, 200, v)
		}
	})
	m.HandleFunc("POST /project-incubators/{incubator}/scope-changes", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Rationale string `json:"rationale"`
			projectincubators.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.ChangeScope(r.PathValue("incubator"), a.UserID, in.Rationale, in.Input)
		if !incubatorError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-incubators/{incubator}/alternatives", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, true)
		if !ok {
			return
		}
		var in projectincubators.Alternative
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.AddAlternative(r.PathValue("incubator"), a.UserID, in)
		if !incubatorError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-incubators/{incubator}/findings", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, true)
		if !ok {
			return
		}
		var in projectincubators.Finding
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.AddFinding(r.PathValue("incubator"), a.UserID, in)
		if !incubatorError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-incubators/{incubator}/experiments", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, true)
		if !ok {
			return
		}
		var in projectincubators.Experiment
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.AddExperiment(r.PathValue("incubator"), a.UserID, in)
		if !incubatorError(w, e) {
			writeJSON(w, 201, v)
		}
	})
	m.HandleFunc("POST /project-incubators/{incubator}/experiments/{experiment}/attempts", func(w http.ResponseWriter, r *http.Request) {
		a, ok := authn(w, r, true)
		if !ok {
			return
		}
		var in projectincubators.ExperimentAttempt
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		v, e := s.AddAttempt(r.PathValue("incubator"), r.PathValue("experiment"), a.UserID, in)
		if !incubatorError(w, e) {
			writeJSON(w, 201, v)
		}
	})
}
func incubatorError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, projectincubators.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "project_incubator_not_found"})
	case errors.Is(e, projectincubators.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_project_incubator"})
	case errors.Is(e, projectincubators.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "incubator_participant_required"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
