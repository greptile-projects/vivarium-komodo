package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/docscollections"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productexperiments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productfeedback"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type feedbackSources struct {
	releases interface {
		Get(string, string) (releases.Release, error)
	}
	docs interface {
		Get(string, string) (docscollections.Collection, error)
	}
	previews interface {
		GetByID(string) (previews.Preview, error)
	}
	issues interface {
		Get(string, string) (issues.Issue, error)
	}
	experiments interface {
		Get(string, string) (productexperiments.Experiment, error)
	}
	organizations interface{ IsMember(string, string) bool }
}

func registerProductFeedbackHTTP(mux *http.ServeMux, s *productfeedback.Store, repos proposalRepositoryStore, c authStore, src feedbackSources) {
	base := "/repositories/{repository}/product-feedback"
	access := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	visible := func(v productfeedback.Feedback, actor string) (productfeedback.Feedback, bool) {
		maintainer := actor == v.ReporterID
		repo, _ := repos.Inspect(storage.ID(v.RepositoryID))
		maintainer = maintainer || actor == repo.OwnerID
		for _, id := range repo.CollaboratorIDs {
			maintainer = maintainer || id == actor
		}
		if v.Audience == "organization" && !src.organizations.IsMember(v.OrganizationID, actor) {
			return v, false
		}
		reporter := v.ReporterID
		if v.IdentityVisibility == "maintainers" && !maintainer {
			v.ReporterID = ""
		}
		if !maintainer {
			v.ContactValue = ""
		}
		for i := range v.Evidence {
			if v.Evidence[i].Visibility == "maintainers" && !maintainer {
				v.Evidence[i].Content = ""
			}
		}
		for i := range v.History {
			if v.IdentityVisibility == "maintainers" && !maintainer && v.History[i].ActorID == reporter {
				v.History[i].ActorID = ""
			}
		}
		for i := range v.Discussion {
			if v.IdentityVisibility == "maintainers" && !maintainer && v.Discussion[i].AuthorID == reporter {
				v.Discussion[i].AuthorID = ""
			}
		}
		for i := range v.Links {
			if v.IdentityVisibility == "maintainers" && !maintainer && v.Links[i].AddedByID == reporter {
				v.Links[i].AddedByID = ""
			}
		}
		return v, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r)
		if !ok {
			return
		}
		items, e := s.List(repo)
		if feedbackError(w, e) {
			return
		}
		out := []productfeedback.Feedback{}
		for _, v := range items {
			if p, yes := visible(v, a); yes {
				out = append(out, p)
			}
		}
		writeJSON(w, 200, map[string]any{"items": out, "total_count": len(out)})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r)
		if !ok {
			return
		}
		var in productfeedback.Input
		if !readJSON(w, r, &in, 6<<20) {
			return
		}
		if in.Audience == "organization" && !src.organizations.IsMember(in.OrganizationID, a) {
			writeJSON(w, 422, map[string]string{"error": "invalid_feedback_organization"})
			return
		}
		if !validFeedbackContext(repo, in.Context, src) {
			writeJSON(w, 422, map[string]string{"error": "invalid_feedback_context"})
			return
		}
		v, e := s.Create(repo, a, in)
		if feedbackError(w, e) {
			return
		}
		p, _ := visible(v, a)
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("GET "+base+"/{feedback}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("feedback"))
		if feedbackError(w, e) {
			return
		}
		p, yes := visible(v, a)
		if !yes {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, p)
	})
	mux.HandleFunc("POST "+base+"/{feedback}/comments", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("feedback"))
		if e == nil {
			_, eok := visible(v, a)
			if !eok {
				writeJSON(w, 404, map[string]string{"error": "not_found"})
				return
			}
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		v, e = s.Comment(repo, r.PathValue("feedback"), a, in.Body)
		if feedbackError(w, e) {
			return
		}
		p, _ := visible(v, a)
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("POST "+base+"/{feedback}/links", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			Kind       string `json:"kind"`
			ResourceID string `json:"resource_id"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		valid := false
		if in.Kind == "issue" {
			_, e := src.issues.Get(repo, in.ResourceID)
			valid = e == nil
		}
		if in.Kind == "experiment" {
			_, e := src.experiments.Get(repo, in.ResourceID)
			valid = e == nil
		}
		if !valid {
			writeJSON(w, 422, map[string]string{"error": "invalid_feedback_link"})
			return
		}
		v, e := s.Link(repo, r.PathValue("feedback"), a, in.Kind, in.ResourceID)
		if feedbackError(w, e) {
			return
		}
		p, _ := visible(v, a)
		writeJSON(w, 201, p)
	})
	mux.HandleFunc("POST "+base+"/{feedback}/consent-withdrawal", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("feedback"))
		if feedbackError(w, e) {
			return
		}
		if v.ReporterID != a {
			writeJSON(w, 403, map[string]string{"error": "feedback_reporter_required"})
			return
		}
		v, e = s.Withdraw(repo, v.ID, a)
		if feedbackError(w, e) {
			return
		}
		p, _ := visible(v, a)
		writeJSON(w, 200, p)
	})
}
func validFeedbackContext(repo string, c productfeedback.Context, s feedbackSources) bool {
	switch c.Kind {
	case "project":
		return c.ResourceID == "" || c.ResourceID == repo
	case "release":
		v, e := s.releases.Get(repo, c.ResourceID)
		return e == nil && v.RepositoryID == repo
	case "journey":
		_, e := s.docs.Get(repo, c.ResourceID)
		return e == nil
	case "preview":
		v, e := s.previews.GetByID(c.ResourceID)
		return e == nil && v.RepositoryID == repo
	}
	return false
}
func feedbackError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, productfeedback.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "not_found"})
	case errors.Is(e, productfeedback.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_product_feedback"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
