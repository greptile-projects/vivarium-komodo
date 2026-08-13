package main

import (
	"errors"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productexperiments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"net/http"
)

type productExperimentReleases interface {
	Get(string, string) (releases.Release, error)
}
type productExperimentDeployments interface {
	GetEnvironment(string, string) (deployments.Environment, error)
	GetDeployment(string, string) (deployments.Deployment, error)
}

func registerProductExperimentsHTTP(mux *http.ServeMux, s *productexperiments.Store, repos proposalRepositoryStore, c authStore, pulls previewPullStore, releaseStore productExperimentReleases, deploymentStore productExperimentDeployments) {
	base := "/repositories/{repository}/product-experiments"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		perm := auth.RepositoryRead
		if write {
			perm = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, perm, write)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		items, e := s.List(repo)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in productexperiments.PlanInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Create(repo, a, in)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("GET "+base+"/{experiment}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("experiment"))
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			productexperiments.PlanInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		v, e := s.Revise(repo, r.PathValue("experiment"), a, in.ExpectedVersion, in.PlanInput)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/comments", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Body string `json:"body"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.Comment(repo, r.PathValue("experiment"), a, in.Body)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/approvals", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Decision string `json:"decision"`
			Note     string `json:"note"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.Approve(repo, r.PathValue("experiment"), a, in.Decision, in.Note)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/assumption-changes", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Assumption string `json:"assumption"`
			Detail     string `json:"detail"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.ChangeAssumption(repo, r.PathValue("experiment"), a, in.Assumption, in.Detail)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/work-items", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in productexperiments.WorkItem
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.AddWorkItem(repo, r.PathValue("experiment"), a, in)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/implementations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in productexperiments.ImplementationInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		pull, e := pulls.Get(repo, in.PullRequestID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_product_experiment_pull_request"})
			return
		}
		v, e := s.AddImplementation(repo, r.PathValue("experiment"), a, pull.SourceCommitID, in)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/audience-policies", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		inspected, e := repos.Inspect(storage.ID(r.PathValue("repository")))
		if e != nil || inspected.OwnerID != a {
			writeJSON(w, 403, map[string]string{"error": "repository_owner_required"})
			return
		}
		var in productexperiments.AudiencePolicyInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		rel, e := releaseStore.Get(repo, in.ReleaseID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_product_experiment_release"})
			return
		}
		v, e := s.PutAudiencePolicy(repo, r.PathValue("experiment"), a, rel.ID, rel.CommitID, in)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/audience-policies/approval", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Decision string `json:"decision"`
			Note     string `json:"note"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.ApproveAudiencePolicy(repo, r.PathValue("experiment"), a, in.Decision, in.Note)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/audience-policies/assignments", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		inspected, e := repos.Inspect(storage.ID(r.PathValue("repository")))
		if e != nil || inspected.OwnerID != a {
			writeJSON(w, 403, map[string]string{"error": "repository_owner_required"})
			return
		}
		var in productexperiments.AssignmentInput
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.Assign(repo, r.PathValue("experiment"), a, in)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/runs", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			EnvironmentID string                        `json:"environment_id"`
			DeploymentID  string                        `json:"deployment_id"`
			Stages        []productexperiments.RunStage `json:"stages"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		env, e := deploymentStore.GetEnvironment(repo, in.EnvironmentID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_product_experiment_environment"})
			return
		}
		deployment, e := deploymentStore.GetDeployment(repo, in.DeploymentID)
		if e != nil || deployment.EnvironmentID != env.ID {
			writeJSON(w, 422, map[string]string{"error": "invalid_product_experiment_deployment"})
			return
		}
		x, e := s.Get(repo, r.PathValue("experiment"))
		if e != nil {
			experimentError(w, e)
			return
		}
		if len(x.AudiencePolicies) == 0 {
			writeJSON(w, 409, map[string]string{"error": "experiment_audience_policy_required"})
			return
		}
		policy := x.AudiencePolicies[len(x.AudiencePolicies)-1]
		if deployment.ReleaseID != policy.ReleaseID || deployment.SourceCommitID != policy.ReleaseCommitID || deployment.State != "succeeded" {
			writeJSON(w, 409, map[string]string{"error": "experiment_release_not_deployed"})
			return
		}
		v, e := s.Launch(repo, x.ID, a, env.ID, deployment.ID, in.Stages)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/runs/{run}/stages/advance", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.Advance(repo, r.PathValue("experiment"), r.PathValue("run"), a, in.Reason)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/runs/{run}/observations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in productexperiments.ObservationInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		v, e := s.Observe(repo, r.PathValue("experiment"), r.PathValue("run"), a, in)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{experiment}/runs/{run}/controls", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Action string `json:"action"`
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.Control(repo, r.PathValue("experiment"), r.PathValue("run"), a, in.Action, in.Reason)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	sig := base + "/signals"
	mux.HandleFunc("GET "+sig, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Signals(repo)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": v})
	})
	mux.HandleFunc("POST "+sig, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in productexperiments.SignalVersion
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.CreateSignal(repo, a, in)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+sig+"/{signal}/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			productexperiments.SignalVersion
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.ReviseSignal(repo, r.PathValue("signal"), a, in.ExpectedVersion, in.SignalVersion)
		if experimentError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
}
func experimentError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	code, key := 500, "internal_error"
	if errors.Is(e, productexperiments.ErrInvalid) {
		code, key = 422, "invalid_product_experiment"
	}
	if errors.Is(e, productexperiments.ErrNotFound) {
		code, key = 404, "product_experiment_not_found"
	}
	if errors.Is(e, productexperiments.ErrConflict) {
		code, key = 409, "product_experiment_version_conflict"
	}
	writeJSON(w, code, map[string]string{"error": key})
	return true
}
