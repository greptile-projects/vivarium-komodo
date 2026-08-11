package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/contributorpathways"
	"github.com/greptile-projects/vivarium-komodo/apps/api/issues"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type contributorPathwayStore interface {
	Publish(string, string, int64, contributorpathways.VersionInput) (contributorpathways.Pathway, error)
	Get(string) (contributorpathways.Pathway, error)
	Acknowledge(string, string, int64, string) (contributorpathways.Pathway, error)
}
type contributorPathwayRepositories interface {
	proposalRepositoryStore
	Open(storage.ID) (*storage.Repository, error)
}
type contributorPathwayResources struct {
	releases interface {
		Get(string, string) (releases.Release, error)
	}
	issues interface {
		Get(string, string) (issues.Issue, error)
	}
	proposals interface {
		Get(string, string) (proposals.Proposal, error)
	}
}

func registerContributorPathwaysHTTP(mux *http.ServeMux, store contributorPathwayStore, repos contributorPathwayRepositories, credentials authStore, releaseStore interface {
	Get(string, string) (releases.Release, error)
}, issueStore interface {
	Get(string, string) (issues.Issue, error)
}, proposalStore interface {
	Get(string, string) (proposals.Proposal, error)
}) {
	resources := contributorPathwayResources{releaseStore, issueStore, proposalStore}
	mux.HandleFunc("GET /repositories/{repository}/contributor-pathway", getContributorPathway(store, repos, credentials, resources))
	mux.HandleFunc("POST /repositories/{repository}/contributor-pathway/versions", publishContributorPathway(store, repos, credentials, resources))
	mux.HandleFunc("POST /repositories/{repository}/contributor-pathway/acknowledgements", acknowledgeContributorPathway(store, repos, credentials, resources))
}

func publishContributorPathway(store contributorPathwayStore, repos contributorPathwayRepositories, credentials authStore, resources contributorPathwayResources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			contributorpathways.VersionInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		p, err := store.Publish(string(repo.ID), actor.UserID, in.ExpectedVersion, in.VersionInput)
		if pathwayError(w, err) {
			return
		}
		p = resolveContributorPathway(p, repo, repos, resources)
		writeJSON(w, http.StatusCreated, p)
	}
}
func getContributorPathway(store contributorPathwayStore, repos contributorPathwayRepositories, credentials authStore, resources contributorPathwayResources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		p, err := store.Get(string(repo.ID))
		if errors.Is(err, contributorpathways.ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "contributor_pathway_not_found"})
			return
		}
		if pathwayError(w, err) {
			return
		}
		writeJSON(w, 200, resolveContributorPathway(p, repo, repos, resources))
	}
}
func acknowledgeContributorPathway(store contributorPathwayStore, repos contributorPathwayRepositories, credentials authStore, resources contributorPathwayResources) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Version int64  `json:"version"`
			Note    string `json:"note"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		p, err := store.Acknowledge(string(repo.ID), actor.UserID, in.Version, in.Note)
		if pathwayError(w, err) {
			return
		}
		writeJSON(w, http.StatusCreated, resolveContributorPathway(p, repo, repos, resources))
	}
}
func pathwayError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, contributorpathways.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_contributor_pathway"})
	case errors.Is(err, contributorpathways.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "contributor_pathway_changed"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}

func resolveContributorPathway(p contributorpathways.Pathway, repo repositories.Repository, repos contributorPathwayRepositories, resources contributorPathwayResources) contributorpathways.Pathway {
	opened, _ := repos.Open(repo.ID)
	var head string
	if opened != nil {
		if branch, e := opened.DefaultBranch(); e == nil {
			if ref, e := opened.ReadReference(branch); e == nil {
				head = string(ref.ObjectID)
			}
		}
	}
	for vi := range p.Versions {
		for ri := range p.Versions[vi].References {
			ref := &p.Versions[vi].References[ri]
			ref.Status = "current"
			ref.Detail = "Reference is available."
			switch ref.Kind {
			case "documentation", "workspace_definition":
				if opened == nil || !repositoryPathExists(opened, ref.Revision, ref.Path) {
					ref.Status = "inaccessible"
					ref.Detail = "The referenced file or revision is unavailable."
				} else if head != "" && ref.Revision != head {
					ref.Status = "stale"
					ref.Detail = "The repository default branch has moved beyond this referenced revision."
				}
			case "ownership":
				participant := ref.ResourceID == repo.OwnerID
				if !participant {
					participant, _ = repos.IsCollaborator(repo.ID, ref.ResourceID)
				}
				if !participant {
					ref.Status = "inaccessible"
					ref.Detail = "The named owner is no longer a repository participant."
				}
			case "release":
				if _, e := resources.releases.Get(string(repo.ID), ref.ResourceID); e != nil {
					ref.Status = "inaccessible"
					ref.Detail = "The linked release is unavailable."
				}
			case "issue":
				if _, e := resources.issues.Get(string(repo.ID), ref.ResourceID); e != nil {
					ref.Status = "inaccessible"
					ref.Detail = "The linked issue is unavailable."
				}
			case "proposal":
				if _, e := resources.proposals.Get(string(repo.ID), ref.ResourceID); e != nil {
					ref.Status = "inaccessible"
					ref.Detail = "The linked proposal is unavailable."
				}
			}
		}
	}
	return p
}
func repositoryPathExists(repo *storage.Repository, revision, path string) bool {
	id := storage.ObjectID(revision)
	commit, err := repo.ReadCommit(id)
	if err != nil {
		return false
	}
	tree, err := repo.ReadTree(commit.Tree)
	if err != nil {
		return false
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
		found := false
		for _, entry := range tree.Entries {
			if entry.Name != part {
				continue
			}
			found = true
			if i == len(parts)-1 {
				return true
			}
			if entry.Type != storage.TreeObject {
				return false
			}
			tree, err = repo.ReadTree(entry.ObjectID)
			if err != nil {
				return false
			}
			break
		}
		if !found {
			return false
		}
	}
	return false
}
