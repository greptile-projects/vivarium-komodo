package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/apicontracts"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributorpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/learningpathways"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
)

type learningPathwaySources struct {
	decisions interface {
		Get(string, string) (decisions.Decision, error)
	}
	issues interface {
		Get(string, string) (issues.Issue, error)
	}
	apis interface {
		Get(string, string) (apicontracts.Contract, error)
	}
	packages interface {
		Get(string, string) (packagecatalog.Version, error)
	}
	contributors interface {
		Get(string) (contributorpathways.Pathway, error)
	}
}

func registerLearningPathwaysHTTP(mux *http.ServeMux, store *learningpathways.Store, repos contributorPathwayRepositories, credentials authStore, sources learningPathwaySources) {
	mux.HandleFunc("GET /repositories/{repository}/learning-pathways", listLearningPathways(store, repos, credentials, sources))
	mux.HandleFunc("GET /repositories/{repository}/learning-pathways/{pathway}", getLearningPathway(store, repos, credentials, sources))
	mux.HandleFunc("POST /repositories/{repository}/learning-pathways/{pathway}/versions", publishLearningPathway(store, repos, credentials, sources))
}

func publishLearningPathway(store *learningpathways.Store, repos contributorPathwayRepositories, credentials authStore, sources learningPathwaySources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			learningpathways.VersionInput
		}
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		p, e := store.Publish(string(repo.ID), r.PathValue("pathway"), actor.UserID, in.ExpectedVersion, in.VersionInput)
		if learningPathwayError(w, e) {
			return
		}
		writeJSON(w, http.StatusCreated, resolveLearningPathway(p, repo, repos, sources))
	}
}
func getLearningPathway(store *learningpathways.Store, repos contributorPathwayRepositories, credentials authStore, sources learningPathwaySources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		p, e := store.Get(string(repo.ID), r.PathValue("pathway"))
		if errors.Is(e, learningpathways.ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "learning_pathway_not_found"})
			return
		}
		if learningPathwayError(w, e) {
			return
		}
		writeJSON(w, 200, resolveLearningPathway(p, repo, repos, sources))
	}
}
func listLearningPathways(store *learningpathways.Store, repos contributorPathwayRepositories, credentials authStore, sources learningPathwaySources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := store.List(string(repo.ID))
		if learningPathwayError(w, e) {
			return
		}
		for i := range items {
			items[i] = resolveLearningPathway(items[i], repo, repos, sources)
		}
		writeJSON(w, 200, map[string]any{"items": items})
	}
}
func learningPathwayError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, learningpathways.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_learning_pathway"})
	case errors.Is(e, learningpathways.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "learning_pathway_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}

func resolveLearningPathway(p learningpathways.Pathway, repo repositories.Repository, repos contributorPathwayRepositories, s learningPathwaySources) learningpathways.Pathway {
	opened, _ := repos.Open(repo.ID)
	head := ""
	if opened != nil {
		if branch, e := opened.DefaultBranch(); e == nil {
			if ref, e := opened.ReadReference(branch); e == nil {
				head = string(ref.ObjectID)
			}
		}
	}
	for vi := range p.Versions {
		v := &p.Versions[vi]
		v.Findings = []learningpathways.Finding{}
		if len(v.MentorIDs) == 0 {
			v.Findings = append(v.Findings, learningpathways.Finding{Kind: "missing_owner", Detail: "No mentor owns learner support for this version."})
		}
		for _, id := range v.MentorIDs {
			participant := id == repo.OwnerID
			if !participant {
				participant, _ = repos.IsCollaborator(repo.ID, id)
			}
			if !participant {
				v.Findings = append(v.Findings, learningpathways.Finding{Kind: "missing_owner", OwnerID: id, Detail: "A named mentor is no longer a repository participant."})
			}
		}
		for _, env := range v.LearnerEnvironments {
			if !env.Supported {
				v.Findings = append(v.Findings, learningpathways.Finding{Kind: "unsupported_environment", Detail: env.Name + ": " + env.Requirement})
			}
		}
		for mi := range v.Modules {
			m := &v.Modules[mi]
			for ri := range m.Resources {
				ref := &m.Resources[ri]
				ref.Status = "current"
				ref.Detail = "Exact learning material is available."
				unavailable := false
				switch ref.Kind {
				case "documentation", "symbol", "contributor_guidance":
					unavailable = opened == nil || !repositoryPathExists(opened, ref.Revision, ref.Path)
				case "decision":
					_, e := s.decisions.Get(string(repo.ID), ref.ResourceID)
					unavailable = e != nil
				case "issue":
					_, e := s.issues.Get(string(repo.ID), ref.ResourceID)
					unavailable = e != nil
				case "api":
					_, e := s.apis.Get(string(repo.ID), ref.ResourceID)
					unavailable = e != nil
				case "package":
					_, e := s.packages.Get(string(repo.ID), ref.ResourceID)
					unavailable = e != nil
				}
				if unavailable {
					ref.Status = "inaccessible"
					ref.Detail = "The exact referenced resource is unavailable."
					v.Findings = append(v.Findings, learningpathways.Finding{Kind: "inaccessible_resource", ModuleID: m.ID, ResourceLabel: ref.Label, Detail: ref.Detail})
				} else if head != "" && ref.Revision != head {
					ref.Status = "stale"
					ref.Detail = "The project has moved beyond this material's supported revision."
					v.Findings = append(v.Findings, learningpathways.Finding{Kind: "stale_material", ModuleID: m.ID, ResourceLabel: ref.Label, Detail: ref.Detail})
				}
				if ref.Kind == "symbol" && strings.TrimSpace(ref.Symbol) == "" {
					ref.Status = "inaccessible"
					ref.Detail = "The symbol locator is missing."
				}
			}
		}
	}
	return p
}
