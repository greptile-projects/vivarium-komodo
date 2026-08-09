package main

import (
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/relationships"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type relationshipStore interface {
	Publish(relationships.Interface) (relationships.Interface, error)
	Declare(relationships.Dependency) (relationships.Dependency, error)
	Interfaces() ([]relationships.Interface, error)
	Dependencies() ([]relationships.Dependency, error)
}
type relationshipReleaseStore interface {
	Get(string, string) (releases.Release, error)
	List(string) ([]releases.Release, error)
}
type relationshipDeploymentStore interface {
	ListEnvironments(string) ([]deployments.Environment, error)
	ListDeployments(string) ([]deployments.Deployment, error)
}
type relationshipRepositoryStore interface {
	proposalRepositoryStore
	Open(storage.ID) (*storage.Repository, error)
}

func registerRelationshipsHTTP(mux *http.ServeMux, store relationshipStore, releaseStore relationshipReleaseStore, deploymentStore relationshipDeploymentStore, repositoryStore relationshipRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories/{repository}/interfaces", publishInterface(store, releaseStore, repositoryStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/dependencies", declareDependency(store, releaseStore, repositoryStore, credentials))
	mux.HandleFunc("GET /repositories/{repository}/relationships", getRelationshipGraph(store, releaseStore, deploymentStore, repositoryStore, credentials))
}

func publishInterface(store relationshipStore, releaseStore relationshipReleaseStore, repositoryStore proposalRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositoryStore, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			Name       string `json:"name"`
			Version    string `json:"version"`
			ReleaseID  string `json:"release_id"`
			SchemaPath string `json:"schema_path"`
		}
		if !readJSON(w, r, &input, 4096) {
			return
		}
		release, err := releaseStore.Get(string(repository.ID), input.ReleaseID)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "release_not_found"})
			return
		}
		item, err := store.Publish(relationships.Interface{RepositoryID: string(repository.ID), Name: input.Name, Version: input.Version, CommitID: release.CommitID, ReleaseID: release.ID, SchemaPath: input.SchemaPath, PublishedByID: actor.UserID})
		if errors.Is(err, relationships.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_interface"})
			return
		}
		if errors.Is(err, relationships.ErrConflict) {
			writeJSON(w, 409, map[string]string{"error": "interface_version_exists"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 201, item)
	}
}

func declareDependency(store relationshipStore, releaseStore relationshipReleaseStore, repositoryStore relationshipRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositoryStore, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var input struct {
			ProviderRepositoryID string `json:"provider_repository_id"`
			InterfaceName        string `json:"interface_name"`
			Constraint           string `json:"constraint"`
			ReleaseID            string `json:"release_id"`
			CommitID             string `json:"commit_id"`
		}
		if !readJSON(w, r, &input, 4096) {
			return
		}
		provider, err := repositoryStore.Inspect(storage.ID(input.ProviderRepositoryID))
		if err != nil || !relationshipCanRead(provider, actor.UserID, repositoryStore) {
			writeJSON(w, 404, map[string]string{"error": "provider_not_found"})
			return
		}
		commit := strings.TrimSpace(input.CommitID)
		if input.ReleaseID != "" {
			release, err := releaseStore.Get(string(repository.ID), input.ReleaseID)
			if err != nil {
				writeJSON(w, 422, map[string]string{"error": "release_not_found"})
				return
			}
			if commit != "" && commit != release.CommitID {
				writeJSON(w, 422, map[string]string{"error": "release_commit_mismatch"})
				return
			}
			commit = release.CommitID
		} else {
			opened, openErr := repositoryStore.Open(repository.ID)
			if openErr != nil {
				writeJSON(w, 422, map[string]string{"error": "commit_not_found"})
				return
			}
			object, objectErr := opened.ReadObject(storage.ObjectID(commit))
			if objectErr != nil || object.Type != storage.CommitObject {
				writeJSON(w, 422, map[string]string{"error": "commit_not_found"})
				return
			}
		}
		item, err := store.Declare(relationships.Dependency{RepositoryID: string(repository.ID), CommitID: commit, ReleaseID: input.ReleaseID, ProviderRepositoryID: string(provider.ID), InterfaceName: input.InterfaceName, Constraint: input.Constraint, DeclaredByID: actor.UserID})
		if errors.Is(err, relationships.ErrInvalid) {
			writeJSON(w, 422, map[string]string{"error": "invalid_dependency"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 201, item)
	}
}

type relationshipNode struct {
	RepositoryID string                  `json:"repository_id"`
	Name         string                  `json:"name"`
	OwnerID      string                  `json:"owner_id"`
	Visibility   repositories.Visibility `json:"visibility"`
}
type relationshipEnvironment struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DeploymentID string `json:"deployment_id"`
	State        string `json:"state"`
	ReleaseID    string `json:"release_id"`
	CommitID     string `json:"commit_id"`
}
type relationshipEdge struct {
	Dependency           relationships.Dependency  `json:"dependency"`
	Provider             *relationships.Interface  `json:"provider,omitempty"`
	Status               string                    `json:"status"`
	Reasons              []string                  `json:"reasons"`
	ConsumerRelease      *releases.Release         `json:"consumer_release,omitempty"`
	ProviderRelease      *releases.Release         `json:"provider_release,omitempty"`
	ConsumerEnvironments []relationshipEnvironment `json:"consumer_environments"`
	ProviderEnvironments []relationshipEnvironment `json:"provider_environments"`
}

func getRelationshipGraph(store relationshipStore, releaseStore relationshipReleaseStore, deploymentStore relationshipDeploymentStore, repositoryStore relationshipRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		anchor, actor, ok := proposalRepositoryAccess(w, r, repositoryStore, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		interfaces, err := store.Interfaces()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		dependencies, err := store.Dependencies()
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		readable := map[string]repositories.Repository{}
		load := func(id string) (repositories.Repository, bool) {
			if v, ok := readable[id]; ok {
				return v, true
			}
			v, e := repositoryStore.Inspect(storage.ID(id))
			if e != nil || !relationshipCanRead(v, actor.UserID, repositoryStore) {
				return v, false
			}
			readable[id] = v
			return v, true
		}
		readable[string(anchor.ID)] = anchor
		nodes := map[string]relationshipNode{}
		edges := []relationshipEdge{}
		addNode := func(repo repositories.Repository) {
			nodes[string(repo.ID)] = relationshipNode{RepositoryID: string(repo.ID), Name: repo.Name, OwnerID: repo.OwnerID, Visibility: repo.Visibility}
		}
		addNode(anchor)
		for _, dep := range dependencies {
			if dep.RepositoryID != string(anchor.ID) && dep.ProviderRepositoryID != string(anchor.ID) {
				continue
			}
			consumer, cok := load(dep.RepositoryID)
			provider, pok := load(dep.ProviderRepositoryID)
			if !cok || !pok {
				continue
			}
			addNode(consumer)
			addNode(provider)
			edge := relationshipEdge{Dependency: dep, Status: "unresolved", Reasons: []string{}, ConsumerEnvironments: []relationshipEnvironment{}, ProviderEnvironments: []relationshipEnvironment{}}
			consumerReleases, _ := releaseStore.List(dep.RepositoryID)
			if dep.ReleaseID != "" {
				if v, e := releaseStore.Get(dep.RepositoryID, dep.ReleaseID); e == nil {
					edge.ConsumerRelease = &v
				} else {
					edge.Reasons = append(edge.Reasons, "consumer_release_missing")
				}
			}
			if len(consumerReleases) > 0 && consumerReleases[0].CommitID != dep.CommitID {
				edge.Status = "stale"
				edge.Reasons = append(edge.Reasons, "newer_consumer_release")
			}
			var candidates []relationships.Interface
			for _, pub := range interfaces {
				if pub.RepositoryID == dep.ProviderRepositoryID && strings.EqualFold(pub.Name, dep.InterfaceName) && relationships.Satisfies(pub.Version, dep.Constraint) {
					candidates = append(candidates, pub)
				}
			}
			sort.Slice(candidates, func(i, j int) bool { return candidates[i].PublishedAt.After(candidates[j].PublishedAt) })
			if len(candidates) == 0 {
				edge.Reasons = append(edge.Reasons, "no_compatible_publication")
			} else {
				pub := candidates[0]
				edge.Provider = &pub
				release, e := releaseStore.Get(pub.RepositoryID, pub.ReleaseID)
				if e != nil || release.CommitID != pub.CommitID {
					edge.Status = "stale"
					edge.Reasons = append(edge.Reasons, "provider_release_missing_or_changed")
				} else {
					edge.ProviderRelease = &release
					if edge.Status != "stale" {
						edge.Status = "resolved"
					}
				}
			}
			if edge.Status == "unresolved" && len(edge.Reasons) == 0 {
				edge.Reasons = append(edge.Reasons, "unresolved")
			}
			edge.ConsumerEnvironments = relationshipEnvironments(dep.RepositoryID, dep.ReleaseID, deploymentStore)
			if edge.Provider != nil {
				edge.ProviderEnvironments = relationshipEnvironments(edge.Provider.RepositoryID, edge.Provider.ReleaseID, deploymentStore)
			}
			edges = append(edges, edge)
		}
		nodeList := make([]relationshipNode, 0, len(nodes))
		for _, n := range nodes {
			nodeList = append(nodeList, n)
		}
		sort.Slice(nodeList, func(i, j int) bool { return nodeList[i].Name < nodeList[j].Name })
		writeJSON(w, 200, map[string]any{"repository_id": anchor.ID, "can_write": actor.UserID != "" && relationshipCanWrite(anchor, actor.UserID, repositoryStore), "nodes": nodeList, "interfaces": filterRelationshipInterfaces(interfaces, readable), "edges": edges, "summary": map[string]int{"repositories": len(nodeList), "relationships": len(edges), "resolved": countRelationship(edges, "resolved"), "stale": countRelationship(edges, "stale"), "unresolved": countRelationship(edges, "unresolved")}})
	}
}

func relationshipCanRead(repo repositories.Repository, user string, store proposalRepositoryStore) bool {
	if repo.Visibility == repositories.Public || repo.OwnerID == user {
		return true
	}
	ok, _ := store.IsCollaborator(repo.ID, user)
	return ok
}
func relationshipCanWrite(repo repositories.Repository, user string, store proposalRepositoryStore) bool {
	if repo.OwnerID == user {
		return true
	}
	ok, _ := store.IsCollaborator(repo.ID, user)
	return ok
}
func relationshipEnvironments(repositoryID, releaseID string, store relationshipDeploymentStore) []relationshipEnvironment {
	if releaseID == "" {
		return []relationshipEnvironment{}
	}
	envs, _ := store.ListEnvironments(repositoryID)
	byID := map[string]string{}
	for _, e := range envs {
		byID[e.ID] = e.Name
	}
	items, _ := store.ListDeployments(repositoryID)
	out := []relationshipEnvironment{}
	seen := map[string]bool{}
	for _, d := range items {
		if d.ReleaseID == releaseID && !seen[d.EnvironmentID] {
			out = append(out, relationshipEnvironment{ID: d.EnvironmentID, Name: byID[d.EnvironmentID], DeploymentID: d.ID, State: d.State, ReleaseID: d.ReleaseID, CommitID: d.SourceCommitID})
			seen[d.EnvironmentID] = true
		}
	}
	return out
}
func filterRelationshipInterfaces(items []relationships.Interface, repos map[string]repositories.Repository) []relationships.Interface {
	out := []relationships.Interface{}
	for _, v := range items {
		if _, ok := repos[v.RepositoryID]; ok {
			out = append(out, v)
		}
	}
	return out
}
func countRelationship(items []relationshipEdge, status string) int {
	n := 0
	for _, v := range items {
		if v.Status == status {
			n++
		}
	}
	return n
}
