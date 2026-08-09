package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
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
	CreateEvolution(relationships.EvolutionPlan) (relationships.EvolutionPlan, error)
	Evolution(string) (relationships.EvolutionPlan, error)
	Evolutions(string) ([]relationships.EvolutionPlan, error)
	UpdateEvolution(string, string, relationships.EvolutionUpdate) (relationships.EvolutionPlan, error)
	AcknowledgeEvolution(string, string, string, string, []string) (relationships.EvolutionPlan, error)
	StartEvolutionAnalysis(string, string, string, string, []string) (relationships.EvolutionPlan, string, error)
	EvolutionAnalysisContext(string) (relationships.EvolutionPlan, relationships.EvolutionAnalysis, error)
	AddEvolutionFinding(string, string, string, string, []string) (relationships.EvolutionPlan, error)
	CreateMigrationTask(string, string, relationships.MigrationTaskInput) (relationships.EvolutionPlan, error)
	AssignMigrationTask(string, string, string, string, string, string, string, string) (relationships.EvolutionPlan, error)
	StartMigrationTask(string, string, string, string, string, string, string, string) (relationships.EvolutionPlan, error)
	SynchronizeMigrationTask(string, string, string, string, string, bool) (relationships.EvolutionPlan, error)
	CommentMigrationTask(string, string, string, string) (relationships.EvolutionPlan, error)
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
type relationshipSourceStore interface {
	Get(string, string) (proposals.Proposal, error)
}
type relationshipPullStore interface {
	Get(string, string) (pullrequests.PullRequest, error)
	Create(pullrequests.CreateParams) (pullrequests.PullRequest, error)
}
type relationshipSessionStore interface {
	CreateForTask(string, string, string, string, changesessions.TaskContext) (changesessions.Session, error)
}

func registerRelationshipsHTTP(mux *http.ServeMux, store relationshipStore, releaseStore relationshipReleaseStore, deploymentStore relationshipDeploymentStore, repositoryStore relationshipRepositoryStore, proposalStore relationshipSourceStore, pullStore relationshipPullStore, credentials authStore, sessionStores ...relationshipSessionStore) {
	var sessions relationshipSessionStore
	if len(sessionStores) > 0 {
		sessions = sessionStores[0]
	}
	mux.HandleFunc("POST /repositories/{repository}/interfaces", publishInterface(store, releaseStore, repositoryStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/dependencies", declareDependency(store, releaseStore, repositoryStore, credentials))
	mux.HandleFunc("GET /repositories/{repository}/relationships", getRelationshipGraph(store, releaseStore, deploymentStore, repositoryStore, credentials))
	mux.HandleFunc("GET /repositories/{repository}/evolution-plans", listEvolutionPlans(store, repositoryStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/evolution-plans", createEvolutionPlan(store, repositoryStore, proposalStore, pullStore, credentials))
	mux.HandleFunc("GET /repositories/{repository}/evolution-plans/{plan}", getEvolutionPlan(store, repositoryStore, credentials))
	mux.HandleFunc("PUT /repositories/{repository}/evolution-plans/{plan}/contract", updateEvolutionPlan(store, repositoryStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/evolution-plans/{plan}/acknowledgements", acknowledgeEvolutionPlan(store, repositoryStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/evolution-plans/{plan}/analyses", startEvolutionAnalysis(store, repositoryStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/evolution-plans/{plan}/tasks", createEvolutionTask(store, repositoryStore, credentials))
	mux.HandleFunc("PUT /repositories/{repository}/evolution-plans/{plan}/tasks/{task}/assignment", assignEvolutionTask(store, repositoryStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/evolution-plans/{plan}/tasks/{task}/comments", commentEvolutionTask(store, repositoryStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/evolution-plans/{plan}/tasks/{task}/start", startEvolutionTask(store, repositoryStore, credentials, sessions))
	mux.HandleFunc("POST /repositories/{repository}/evolution-plans/{plan}/tasks/{task}/pull-request", publishEvolutionTask(store, repositoryStore, pullStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/evolution-plans/{plan}/tasks/{task}/complete", completeEvolutionTask(store, repositoryStore, pullStore, credentials))
	mux.HandleFunc("GET /relationship-evolution-analysis/context", evolutionAnalysisContext(store))
	mux.HandleFunc("GET /relationship-evolution-analysis/repositories/{repository}/blobs", evolutionAnalysisBlob(store, repositoryStore))
	mux.HandleFunc("POST /relationship-evolution-analysis/findings", addEvolutionAnalysisFinding(store))
}

func evolutionPlanForRepo(w http.ResponseWriter, store relationshipStore, repo repositories.Repository, id string) (relationships.EvolutionPlan, bool) {
	v, err := store.Evolution(id)
	if err != nil || v.RepositoryID != string(repo.ID) {
		writeJSON(w, 404, map[string]string{"error": "evolution_plan_not_found"})
		return v, false
	}
	return v, true
}
func evolutionTask(v relationships.EvolutionPlan, id string) (relationships.MigrationTask, bool) {
	for _, t := range v.Tasks {
		if t.ID == id {
			return t, true
		}
	}
	return relationships.MigrationTask{}, false
}
func createEvolutionTask(store relationshipStore, repositories relationshipRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if _, ok = evolutionPlanForRepo(w, store, repo, r.PathValue("plan")); !ok {
			return
		}
		var in relationships.MigrationTaskInput
		if !readJSON(w, r, &in, 30<<10) {
			return
		}
		target, e := repositories.Inspect(storage.ID(in.TargetRepositoryID))
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_target_repository"})
			return
		}
		work, e := repositories.Inspect(storage.ID(in.RepositoryID))
		if e != nil || (work.ID != target.ID && work.UpstreamID != target.ID) {
			writeJSON(w, 422, map[string]string{"error": "invalid_work_repository"})
			return
		}
		v, e := store.CreateMigrationTask(r.PathValue("plan"), actor.UserID, in)
		writeEvolution(w, v, e, 201)
	}
}
func assignEvolutionTask(store relationshipStore, repositories relationshipRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		anchor, err := repositories.Inspect(storage.ID(r.PathValue("repository")))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "evolution_plan_not_found"})
			return
		}
		v, ok := evolutionPlanForRepo(w, store, anchor, r.PathValue("plan"))
		if !ok {
			return
		}
		task, ok := evolutionTask(v, r.PathValue("task"))
		if !ok {
			writeJSON(w, 404, map[string]string{"error": "migration_task_not_found"})
			return
		}
		work, actor, ok := proposalRepositoryAccessPath(w, r, repositories, credentials, task.RepositoryID, auth.RepositoryWrite)
		if !ok {
			return
		}
		var in struct {
			ExpectedAssignmentID string `json:"expected_assignment_id"`
			Kind                 string `json:"kind"`
			AssigneeID           string `json:"assignee_id"`
			Mandate              string `json:"mandate"`
			BaseRevision         string `json:"base_revision"`
		}
		if !readJSON(w, r, &in, 10<<10) {
			return
		}
		if in.Kind == "human" && !relationshipParticipant(work, in.AssigneeID, repositories) {
			writeJSON(w, 422, map[string]string{"error": "assignee_not_repository_participant"})
			return
		}
		if in.Kind == "agent" && in.AssigneeID != "codex" {
			writeJSON(w, 422, map[string]string{"error": "agent_unavailable"})
			return
		}
		if !relationshipCommit(repositories, work.ID, in.BaseRevision) {
			writeJSON(w, 422, map[string]string{"error": "base_revision_not_found"})
			return
		}
		v, e := store.AssignMigrationTask(v.ID, task.ID, actor.UserID, in.ExpectedAssignmentID, in.Kind, in.AssigneeID, in.Mandate, in.BaseRevision)
		writeEvolution(w, v, e, 200)
	}
}
func commentEvolutionTask(store relationshipStore, repositories relationshipRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		anchor, err := repositories.Inspect(storage.ID(r.PathValue("repository")))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "evolution_plan_not_found"})
			return
		}
		v, ok := evolutionPlanForRepo(w, store, anchor, r.PathValue("plan"))
		if !ok {
			return
		}
		task, ok := evolutionTask(v, r.PathValue("task"))
		if !ok {
			return
		}
		_, actor, ok := proposalRepositoryAccessPath(w, r, repositories, credentials, task.RepositoryID, auth.RepositoryRead)
		if !ok {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 12<<10) {
			return
		}
		v, err = store.CommentMigrationTask(v.ID, task.ID, actor.UserID, in.Body)
		writeEvolution(w, v, err, 201)
	}
}
func startEvolutionTask(store relationshipStore, repositories relationshipRepositoryStore, credentials authStore, sessions relationshipSessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		anchor, err := repositories.Inspect(storage.ID(r.PathValue("repository")))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "evolution_plan_not_found"})
			return
		}
		v, ok := evolutionPlanForRepo(w, store, anchor, r.PathValue("plan"))
		if !ok {
			return
		}
		task, ok := evolutionTask(v, r.PathValue("task"))
		if !ok || task.Assignment == nil {
			writeJSON(w, 409, map[string]string{"error": "task_not_assigned"})
			return
		}
		work, actor, ok := proposalRepositoryAccessPath(w, r, repositories, credentials, task.RepositoryID, auth.RepositoryWrite)
		if !ok {
			return
		}
		if task.Assignment.Kind == "human" && actor.UserID != task.Assignment.AssigneeID {
			writeJSON(w, 403, map[string]string{"error": "assignee_required"})
			return
		}
		var in struct {
			ExpectedAssignmentID string `json:"expected_assignment_id"`
			Branch               string `json:"branch"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		if in.ExpectedAssignmentID != task.Assignment.ID {
			writeJSON(w, 409, map[string]string{"error": "evolution_plan_conflict"})
			return
		}
		branch := strings.TrimSpace(in.Branch)
		if branch == "" {
			branch = "migration/" + task.ID
		}
		opened, e := repositories.Open(work.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		ref := storage.ReferenceName("refs/heads/" + branch)
		if _, e = opened.ReadReference(ref); e == nil {
			writeJSON(w, 409, map[string]string{"error": "branch_exists"})
			return
		}
		if e = opened.CreateReference(storage.Reference{Name: ref, ObjectID: storage.ObjectID(task.Assignment.BaseRevision)}); e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_branch"})
			return
		}
		sessionID := ""
		if sessions != nil {
			ctx := changesessions.TaskContext{TaskID: task.ID, TaskTitle: task.Title, TaskOutcome: task.Outcome, Mandate: task.Assignment.Mandate, Repository: changesessions.RepositoryContext{ID: string(work.ID), Name: work.Name, Description: work.Description, BaseRevision: task.Assignment.BaseRevision, WorkingBranch: branch}}
			s, se := sessions.CreateForTask(string(work.ID), "evolution-"+task.ID, actor.UserID, task.Assignment.BaseRevision, ctx)
			if se == nil {
				sessionID = s.ID
			}
		}
		v, e = store.StartMigrationTask(v.ID, task.ID, actor.UserID, in.ExpectedAssignmentID, string(work.ID), branch, task.Assignment.BaseRevision, sessionID)
		writeEvolution(w, v, e, 201)
	}
}
func publishEvolutionTask(store relationshipStore, repositories relationshipRepositoryStore, pulls relationshipPullStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		anchor, err := repositories.Inspect(storage.ID(r.PathValue("repository")))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "evolution_plan_not_found"})
			return
		}
		v, ok := evolutionPlanForRepo(w, store, anchor, r.PathValue("plan"))
		if !ok {
			return
		}
		task, ok := evolutionTask(v, r.PathValue("task"))
		if !ok || task.Work == nil {
			writeJSON(w, 409, map[string]string{"error": "task_not_started"})
			return
		}
		work, actor, ok := proposalRepositoryAccessPath(w, r, repositories, credentials, task.RepositoryID, auth.RepositoryWrite)
		if !ok {
			return
		}
		if task.Assignment.Kind == "human" && actor.UserID != task.Assignment.AssigneeID {
			writeJSON(w, 403, map[string]string{"error": "assignee_required"})
			return
		}
		var in struct {
			Title        string `json:"title"`
			Body         string `json:"body"`
			TargetBranch string `json:"target_branch"`
			Draft        bool   `json:"draft"`
		}
		if !readJSON(w, r, &in, 70<<10) {
			return
		}
		sourceOpened, _ := repositories.Open(work.ID)
		source, sourceName, found := branchTip(sourceOpened, task.Work.Branch)
		targetRepo, e := repositories.Inspect(storage.ID(task.TargetRepositoryID))
		if e != nil || (!found) {
			writeJSON(w, 422, map[string]string{"error": "branch_unavailable"})
			return
		}
		targetOpened, _ := repositories.Open(targetRepo.ID)
		target, targetName, found := branchTip(targetOpened, in.TargetBranch)
		if !found {
			writeJSON(w, 422, map[string]string{"error": "target_branch_unavailable"})
			return
		}
		pr, e := pulls.Create(pullrequests.CreateParams{RepositoryID: string(targetRepo.ID), SourceRepositoryID: string(work.ID), TaskID: task.ID, ChangeSessionID: task.Work.SessionID, AuthorID: actor.UserID, Title: in.Title, Body: in.Body, SourceBranch: sourceName, TargetBranch: targetName, SourceCommitID: string(source), TargetCommitID: string(target), Draft: in.Draft})
		if e != nil {
			writePullRequestError(w, e)
			return
		}
		v, e = store.SynchronizeMigrationTask(v.ID, task.ID, actor.UserID, string(source), pr.ID, false)
		if e != nil {
			writeEvolution(w, v, e, 201)
			return
		}
		writeJSON(w, 201, map[string]any{"plan": v, "pull_request": pr})
	}
}
func completeEvolutionTask(store relationshipStore, repositories relationshipRepositoryStore, pulls relationshipPullStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		anchor, err := repositories.Inspect(storage.ID(r.PathValue("repository")))
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "evolution_plan_not_found"})
			return
		}
		v, ok := evolutionPlanForRepo(w, store, anchor, r.PathValue("plan"))
		if !ok {
			return
		}
		task, ok := evolutionTask(v, r.PathValue("task"))
		if !ok || task.Work == nil || task.Work.PullRequestID == "" {
			writeJSON(w, 409, map[string]string{"error": "pull_request_required"})
			return
		}
		_, actor, ok := proposalRepositoryAccessPath(w, r, repositories, credentials, task.RepositoryID, auth.RepositoryWrite)
		if !ok {
			return
		}
		pr, err := pulls.Get(task.TargetRepositoryID, task.Work.PullRequestID)
		if err != nil || pr.Status != pullrequests.Merged {
			writeJSON(w, 409, map[string]string{"error": "pull_request_not_merged"})
			return
		}
		v, err = store.SynchronizeMigrationTask(v.ID, task.ID, actor.UserID, pr.SourceCommitID, pr.ID, true)
		writeEvolution(w, v, err, 200)
	}
}
func proposalRepositoryAccessPath(w http.ResponseWriter, r *http.Request, repositories relationshipRepositoryStore, credentials authStore, id string, scope auth.Scope) (repositories.Repository, auth.Grant, bool) {
	copy := r.Clone(r.Context())
	copy.SetPathValue("repository", id)
	return proposalRepositoryAccess(w, copy, repositories, credentials, scope, true)
}
func relationshipParticipant(repo repositories.Repository, user string, store relationshipRepositoryStore) bool {
	if repo.OwnerID == user {
		return true
	}
	ok, _ := store.IsCollaborator(repo.ID, user)
	return ok
}
func relationshipCommit(store relationshipRepositoryStore, id storage.ID, commit string) bool {
	opened, e := store.Open(id)
	if e != nil {
		return false
	}
	o, e := opened.ReadObject(storage.ObjectID(commit))
	return e == nil && o.Type == storage.CommitObject
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

func createEvolutionPlan(store relationshipStore, repositories relationshipRepositoryStore, proposals relationshipSourceStore, pulls relationshipPullStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			InterfaceName       string `json:"interface_name"`
			SourceKind          string `json:"source_kind"`
			SourceID            string `json:"source_id"`
			CandidateCommitID   string `json:"candidate_commit_id"`
			CandidateSchemaPath string `json:"candidate_schema_path"`
		}
		if !readJSON(w, r, &in, 20<<10) {
			return
		}
		candidateRepositoryID := string(repo.ID)
		switch strings.ToLower(strings.TrimSpace(in.SourceKind)) {
		case "proposal":
			if _, err := proposals.Get(string(repo.ID), in.SourceID); err != nil {
				writeJSON(w, 422, map[string]string{"error": "proposal_not_found"})
				return
			}
		case "pull_request":
			pr, err := pulls.Get(string(repo.ID), in.SourceID)
			if err != nil {
				writeJSON(w, 422, map[string]string{"error": "pull_request_not_found"})
				return
			}
			in.CandidateCommitID = pr.SourceCommitID
			candidateRepositoryID = pr.SourceRepositoryID
		default:
			writeJSON(w, 422, map[string]string{"error": "invalid_source"})
			return
		}
		interfaces, _ := store.Interfaces()
		var predecessor relationships.Interface
		for _, v := range interfaces {
			if v.RepositoryID == string(repo.ID) && strings.EqualFold(v.Name, in.InterfaceName) && (predecessor.ID == "" || v.PublishedAt.After(predecessor.PublishedAt)) {
				predecessor = v
			}
		}
		if predecessor.ID == "" || predecessor.SchemaPath == "" {
			writeJSON(w, 422, map[string]string{"error": "released_predecessor_schema_not_found"})
			return
		}
		priorBody, err := relationshipBlob(repositories, predecessor.RepositoryID, predecessor.CommitID, predecessor.SchemaPath)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "predecessor_schema_not_found"})
			return
		}
		candidateBody, err := relationshipBlob(repositories, candidateRepositoryID, in.CandidateCommitID, in.CandidateSchemaPath)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "candidate_schema_not_found"})
			return
		}
		dependencies, _ := store.Dependencies()
		affected := []relationships.AffectedConsumer{}
		for _, d := range dependencies {
			if d.ProviderRepositoryID != string(repo.ID) || !strings.EqualFold(d.InterfaceName, in.InterfaceName) {
				continue
			}
			consumer, e := repositories.Inspect(storage.ID(d.RepositoryID))
			if e != nil || !relationshipCanRead(consumer, actor.UserID, repositories) {
				continue
			}
			affected = append(affected, relationships.AffectedConsumer{DependencyID: d.ID, RepositoryID: d.RepositoryID, OwnerID: consumer.OwnerID, CommitID: d.CommitID, Constraint: d.Constraint})
		}
		priorHash, candidateHash := sha256.Sum256(priorBody), sha256.Sum256(candidateBody)
		plan, err := store.CreateEvolution(relationships.EvolutionPlan{RepositoryID: string(repo.ID), InterfaceName: in.InterfaceName, SourceKind: in.SourceKind, SourceID: in.SourceID, CandidateCommitID: in.CandidateCommitID, CandidateSchemaPath: in.CandidateSchemaPath, CandidateSchemaSHA256: hex.EncodeToString(candidateHash[:]), Predecessor: predecessor, PredecessorSchemaSHA256: hex.EncodeToString(priorHash[:]), AffectedConsumers: affected, CreatedByID: actor.UserID})
		writeEvolution(w, plan, err, 201)
	}
}
func listEvolutionPlans(store relationshipStore, repositories relationshipRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.Evolutions(string(repo.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, map[string]any{"items": items, "total_count": len(items)})
	}
}
func getEvolutionPlan(store relationshipStore, repositories relationshipRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, err := store.Evolution(r.PathValue("plan"))
		if err != nil || v.RepositoryID != string(repo.ID) {
			writeJSON(w, 404, map[string]string{"error": "evolution_plan_not_found"})
			return
		}
		scrubEvolutionHTTP(&v)
		writeJSON(w, 200, v)
	}
}
func updateEvolutionPlan(store relationshipStore, repositories relationshipRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		v, err := store.Evolution(r.PathValue("plan"))
		if err != nil || v.RepositoryID != string(repo.ID) {
			writeJSON(w, 404, map[string]string{"error": "evolution_plan_not_found"})
			return
		}
		var in relationships.EvolutionUpdate
		if !readJSON(w, r, &in, 100<<10) {
			return
		}
		v, err = store.UpdateEvolution(v.ID, actor.UserID, in)
		writeEvolution(w, v, err, 200)
	}
}
func acknowledgeEvolutionPlan(store relationshipStore, repositories relationshipRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		v, err := store.Evolution(r.PathValue("plan"))
		if err != nil || v.RepositoryID != string(repo.ID) {
			writeJSON(w, 404, map[string]string{"error": "evolution_plan_not_found"})
			return
		}
		owned := []string{}
		if repo.OwnerID == actor.UserID {
			owned = append(owned, string(repo.ID))
		}
		for _, c := range v.AffectedConsumers {
			cr, e := repositories.Inspect(storage.ID(c.RepositoryID))
			if e == nil && cr.OwnerID == actor.UserID {
				owned = append(owned, c.RepositoryID)
			}
		}
		if len(owned) == 0 {
			writeJSON(w, 403, map[string]string{"error": "owner_acknowledgement_required"})
			return
		}
		var in struct {
			Decision string `json:"decision"`
			Note     string `json:"note"`
		}
		if !readJSON(w, r, &in, 10<<10) {
			return
		}
		v, err = store.AcknowledgeEvolution(v.ID, actor.UserID, in.Decision, in.Note, owned)
		writeEvolution(w, v, err, 201)
	}
}
func startEvolutionAnalysis(store relationshipStore, repositories relationshipRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		v, err := store.Evolution(r.PathValue("plan"))
		if err != nil || v.RepositoryID != string(repo.ID) {
			writeJSON(w, 404, map[string]string{"error": "evolution_plan_not_found"})
			return
		}
		var in struct {
			Agent         string   `json:"agent"`
			Mandate       string   `json:"mandate"`
			RepositoryIDs []string `json:"repository_ids"`
		}
		if !readJSON(w, r, &in, 20<<10) {
			return
		}
		for _, id := range in.RepositoryIDs {
			selected, e := repositories.Inspect(storage.ID(id))
			if e != nil || !relationshipCanRead(selected, actor.UserID, repositories) {
				writeJSON(w, 404, map[string]string{"error": "repository_not_found"})
				return
			}
		}
		v, token, err := store.StartEvolutionAnalysis(v.ID, actor.UserID, in.Agent, in.Mandate, in.RepositoryIDs)
		if err != nil {
			writeEvolution(w, v, err, 201)
			return
		}
		scrubEvolutionHTTP(&v)
		writeJSON(w, 201, map[string]any{"plan": v, "worker_credential": token, "credential_notice": "shown once; selected repository reads and attributable findings only"})
	}
}
func evolutionAnalysisContext(store relationshipStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, a, err := store.EvolutionAnalysisContext(investigationBearer(r))
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "invalid_worker_credential"})
			return
		}
		writeJSON(w, 200, map[string]any{"plan": v, "analysis": a})
	}
}
func evolutionAnalysisBlob(store relationshipStore, repositories relationshipRepositoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, a, err := store.EvolutionAnalysisContext(investigationBearer(r))
		if err != nil {
			writeJSON(w, 401, map[string]string{"error": "invalid_worker_credential"})
			return
		}
		allowed := false
		for _, id := range a.RepositoryIDs {
			if id == r.PathValue("repository") {
				allowed = true
			}
		}
		if !allowed {
			writeJSON(w, 404, map[string]string{"error": "repository_not_selected"})
			return
		}
		commit, path := r.URL.Query().Get("commit"), r.URL.Query().Get("path")
		body, err := relationshipBlob(repositories, r.PathValue("repository"), commit, path)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "blob_not_found"})
			return
		}
		sum := sha256.Sum256(body)
		writeJSON(w, 200, map[string]any{"repository_id": r.PathValue("repository"), "commit_id": commit, "path": path, "content": string(body), "sha256": hex.EncodeToString(sum[:])})
	}
}
func addEvolutionAnalysisFinding(store relationshipStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Kind          string   `json:"kind"`
			Body          string   `json:"body"`
			Uncertainty   string   `json:"uncertainty"`
			RepositoryIDs []string `json:"repository_ids"`
		}
		if !readJSON(w, r, &in, 30<<10) {
			return
		}
		v, err := store.AddEvolutionFinding(investigationBearer(r), in.Kind, in.Body, in.Uncertainty, in.RepositoryIDs)
		writeEvolution(w, v, err, 201)
	}
}
func relationshipBlob(repositories relationshipRepositoryStore, repositoryID, commitID, path string) ([]byte, error) {
	opened, err := repositories.Open(storage.ID(repositoryID))
	if err != nil {
		return nil, err
	}
	commit, err := opened.ReadCommit(storage.ObjectID(commitID))
	if err != nil || !visibleCommit(opened, commit.ID) {
		return nil, err
	}
	parts := strings.Split(strings.Trim(strings.TrimSpace(path), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		return nil, errors.New("path")
	}
	tree := commit.Tree
	for i, part := range parts {
		entries, er := opened.ReadTree(tree)
		if er != nil {
			return nil, er
		}
		found := false
		for _, entry := range entries.Entries {
			if entry.Name != part {
				continue
			}
			found = true
			if i == len(parts)-1 {
				if entry.Type != storage.BlobObject {
					return nil, errors.New("not blob")
				}
				object, er := opened.ReadObject(entry.ObjectID)
				if er != nil {
					return nil, er
				}
				if len(object.Content) > 1<<20 {
					return nil, errors.New("blob too large")
				}
				return object.Content, nil
			}
			if entry.Type != storage.TreeObject {
				return nil, errors.New("not tree")
			}
			tree = entry.ObjectID
			break
		}
		if !found {
			return nil, errors.New("not found")
		}
	}
	return nil, errors.New("not found")
}
func writeEvolution(w http.ResponseWriter, v relationships.EvolutionPlan, err error, status int) {
	scrubEvolutionHTTP(&v)
	switch {
	case errors.Is(err, relationships.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "evolution_plan_not_found"})
	case errors.Is(err, relationships.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "evolution_plan_conflict"})
	case errors.Is(err, relationships.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_evolution_plan"})
	case err != nil:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	default:
		writeJSON(w, status, v)
	}
}
func scrubEvolutionHTTP(v *relationships.EvolutionPlan) {
	for i := range v.Analyses {
		v.Analyses[i].CredentialDigest = ""
	}
}
