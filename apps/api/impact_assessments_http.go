package main

import (
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deployments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/impactassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/investigations"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type impactStore interface {
	Create(impactassessments.Assessment) (impactassessments.Assessment, error)
	Get(string, string) (impactassessments.Assessment, error)
	List(string) ([]impactassessments.Assessment, error)
	Update(string, string, string, string, string, string) (impactassessments.Assessment, error)
	AddImpact(string, string, string, impactassessments.Impact) (impactassessments.Assessment, error)
	Request(string, string, string, string, string) (impactassessments.Assessment, error)
	Decide(string, string, string, string, string, string) (impactassessments.Assessment, error)
	StartAgent(string, string, string) (impactassessments.Assessment, string, error)
	AgentContext(string) (impactassessments.Assessment, error)
	AddFinding(string, string, string, []impactassessments.Evidence) (impactassessments.Assessment, error)
}
type impactInvestigationStore interface {
	Get(string, string) (investigations.Investigation, error)
}
type impactReleaseStore interface {
	List(string) ([]releases.Release, error)
}
type impactDeploymentStore interface {
	ListEnvironments(string) ([]deployments.Environment, error)
	ListDeployments(string) ([]deployments.Deployment, error)
}
type impactPackageStore interface {
	List(string) ([]packagecatalog.Version, error)
}

func registerImpactAssessmentsHTTP(mux *http.ServeMux, store impactStore, repositories codeIntelligenceStore, credentials authStore, relations codeRelationshipStore, investigationStore impactInvestigationStore, releaseStore impactReleaseStore, deploymentStore impactDeploymentStore, packages impactPackageStore) {
	base := "/repositories/{repository}/impact-assessments"
	mux.HandleFunc("POST "+base, createImpactAssessment(store, repositories, credentials, relations, investigationStore, releaseStore, deploymentStore, packages))
	mux.HandleFunc("GET "+base, listImpactAssessments(store, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{assessment}", getImpactAssessment(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{assessment}/impacts", addImpact(store, repositories, credentials))
	mux.HandleFunc("PATCH "+base+"/{assessment}/impacts/{impact}", updateImpact(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{assessment}/impacts/{impact}/acknowledgements", requestImpactAck(store, repositories, credentials))
	mux.HandleFunc("PATCH "+base+"/{assessment}/impacts/{impact}/acknowledgements", decideImpactAck(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{assessment}/agent-runs", startImpactAgent(store, repositories, credentials))
	mux.HandleFunc("GET /impact-assessment-agent/context", impactAgentContext(store))
	mux.HandleFunc("POST /impact-assessment-agent/findings", impactAgentFinding(store))
}

func createImpactAssessment(store impactStore, repositories codeIntelligenceStore, credentials authStore, relations codeRelationshipStore, canvases impactInvestigationStore, releaseStore impactReleaseStore, deploymentStore impactDeploymentStore, packages impactPackageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Title, Revision string
			Sources         []impactassessments.Source
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		opened, e := repositories.Open(repo.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		commit, revision, e := resolveRevision(opened, in.Revision)
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "revision_not_found"})
			return
		}
		impacts := []impactassessments.Impact{}
		unknown := []string{}
		paths := map[string]bool{}
		symbols := []string{}
		for _, src := range in.Sources {
			switch src.Kind {
			case "code":
				if !validInvestigationPath(src.Path) {
					writeJSON(w, 422, map[string]string{"error": "invalid_source"})
					return
				}
				if _, e = blobAtPath(opened, commit, src.Path); e != nil {
					writeJSON(w, 422, map[string]string{"error": "invalid_source"})
					return
				}
				paths[src.Path] = true
			case "symbol":
				if strings.TrimSpace(src.Symbol) == "" {
					writeJSON(w, 422, map[string]string{"error": "invalid_source"})
					return
				}
				symbols = append(symbols, src.Symbol)
			case "investigation_conclusion":
				canvas, x := canvases.Get(string(repo.ID), src.InvestigationID)
				if x != nil || canvas.CommitID != string(commit) || !investigationParticipant(canvas, actor.UserID) || !impactConclusion(canvas, src.ConclusionID) {
					writeJSON(w, 422, map[string]string{"error": "invalid_conclusion"})
					return
				}
			case "diff":
				for _, line := range strings.Split(src.Diff, "\n") {
					if strings.HasPrefix(line, "+++ b/") {
						changed := strings.TrimPrefix(line, "+++ b/")
						if !validInvestigationPath(changed) {
							writeJSON(w, 422, map[string]string{"error": "invalid_source"})
							return
						}
						paths[changed] = true
					}
				}
			default:
				writeJSON(w, 422, map[string]string{"error": "invalid_source"})
				return
			}
		}
		sources, skipped, limited, e := collectSources(opened, commit)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		for _, name := range symbols {
			found := buildSymbols(opened, commit, sources, "", name)
			if len(found) == 0 {
				unknown = append(unknown, "Symbol "+name+" was not found at the assessed revision.")
				continue
			}
			s := found[0]
			paths[s.Definition.Path] = true
			ev := func(loc sourceLocation, kind string) impactassessments.Evidence {
				return impactassessments.Evidence{RepositoryID: string(repo.ID), CommitID: string(commit), Kind: kind, Path: loc.Path, Line: loc.Line, Label: name}
			}
			for _, loc := range s.References {
				impacts = append(impacts, impactassessments.Impact{Category: "reference", Summary: name + " is referenced from " + loc.Path, RepositoryID: string(repo.ID), Evidence: []impactassessments.Evidence{ev(loc, "source")}})
			}
			for _, loc := range s.Tests {
				impacts = append(impacts, impactassessments.Impact{Category: "test", Summary: "Verify " + name + " through " + loc.Path, RepositoryID: string(repo.ID), Verification: []string{loc.Path}, Evidence: []impactassessments.Evidence{ev(loc, "test")}})
			}
			if s.Owner != nil {
				impacts = append(impacts, impactassessments.Impact{Category: "owner", Summary: "Last-touch owner for " + name + " is " + s.Owner.Author, RepositoryID: string(repo.ID), Evidence: []impactassessments.Evidence{ev(s.Definition, "history")}})
			}
		}
		for p := range paths {
			impacts = append(impacts, impactassessments.Impact{Category: "reference", Summary: "Selected change scope includes " + p, RepositoryID: string(repo.ID), Evidence: []impactassessments.Evidence{{RepositoryID: string(repo.ID), CommitID: string(commit), Kind: "source", Path: p, Label: "selected scope"}}})
		}
		interfaces, _ := relations.Interfaces()
		dependencies, _ := relations.Dependencies()
		for _, it := range interfaces {
			if it.RepositoryID == string(repo.ID) && it.CommitID == string(commit) {
				impacts = append(impacts, impactassessments.Impact{Category: "interface", Summary: "Published interface " + it.Name + " " + it.Version + " may change", RepositoryID: string(repo.ID), OwnerIDs: []string{repo.OwnerID}, Evidence: []impactassessments.Evidence{{RepositoryID: string(repo.ID), CommitID: it.CommitID, Kind: "interface", ResourceID: it.ID, Path: it.SchemaPath, Label: it.Name}}})
				for _, d := range dependencies {
					if d.ProviderRepositoryID == string(repo.ID) && strings.EqualFold(d.InterfaceName, it.Name) {
						consumer, x := repositories.Inspect(storageID(d.RepositoryID))
						if x == nil && relationshipRepositoryReadable(consumer, actor.UserID) {
							impacts = append(impacts, impactassessments.Impact{Category: "consumer", Summary: consumer.Name + " consumes " + it.Name + " " + d.Constraint, RepositoryID: d.RepositoryID, OwnerIDs: []string{consumer.OwnerID}, Evidence: []impactassessments.Evidence{{RepositoryID: d.RepositoryID, CommitID: d.CommitID, Kind: "dependency", ResourceID: d.ID, Label: d.Constraint}}})
						}
					}
				}
			}
		}
		rels, _ := releaseStore.List(string(repo.ID))
		for _, x := range rels {
			if x.CommitID == string(commit) {
				impacts = append(impacts, impactassessments.Impact{Category: "release", Summary: "Release " + x.Version + " contains this revision", RepositoryID: string(repo.ID), OwnerIDs: []string{repo.OwnerID}, Evidence: []impactassessments.Evidence{{RepositoryID: string(repo.ID), CommitID: x.CommitID, Kind: "release", ResourceID: x.ID, Label: x.Version}}})
			}
		}
		versions, _ := packages.List(string(repo.ID))
		for _, version := range versions {
			if version.SourceCommitID == string(commit) {
				impacts = append(impacts, impactassessments.Impact{Category: "package", Summary: "Package " + version.Identity + " " + version.Version + " publishes this revision", RepositoryID: string(repo.ID), OwnerIDs: []string{repo.OwnerID}, Evidence: []impactassessments.Evidence{{RepositoryID: string(repo.ID), CommitID: string(commit), Kind: "package", ResourceID: version.ID, Label: version.Identity + "@" + version.Version}}})
			}
		}
		envs, _ := deploymentStore.ListEnvironments(string(repo.ID))
		deps, _ := deploymentStore.ListDeployments(string(repo.ID))
		for _, d := range deps {
			if d.SourceCommitID != string(commit) {
				continue
			}
			for _, env := range envs {
				if env.ID == d.EnvironmentID {
					impacts = append(impacts, impactassessments.Impact{Category: "environment", Summary: env.Name + " has deployment evidence for this revision", RepositoryID: string(repo.ID), OwnerIDs: []string{repo.OwnerID}, Evidence: []impactassessments.Evidence{{RepositoryID: string(repo.ID), CommitID: string(commit), Kind: "deployment", ResourceID: d.ID, Label: env.Name}}})
				}
			}
		}
		reasons := []string{}
		status := "complete"
		if skipped > 0 {
			reasons = append(reasons, "unsupported_files_skipped")
		}
		if limited {
			reasons = append(reasons, "analysis_limit_reached")
		}
		if len(reasons) > 0 {
			status = "incomplete"
		}
		v, e := store.Create(impactassessments.Assessment{RepositoryID: string(repo.ID), Title: in.Title, Revision: revision, CommitID: string(commit), CreatorID: actor.UserID, Sources: in.Sources, Impacts: impacts, Unknowns: unknown, AnalysisStatus: status, AnalysisReasons: reasons})
		writeImpact(w, v, e, 201)
	}
}

// storageID keeps the storage identity conversion local without leaking filesystem concerns.
func storageID(v string) storage.ID { return storage.ID(v) }
func impactConclusion(v investigations.Investigation, id string) bool {
	for _, e := range v.Entries {
		if e.ID == id && e.Type == "conclusion" && !e.Stale {
			return true
		}
	}
	return false
}
func listImpactAssessments(s impactStore, repos codeIntelligenceStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := s.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		out := items[:0]
		for _, v := range items {
			if impactParticipant(v, actor.UserID) {
				out = append(out, v)
			}
		}
		writeJSON(w, 200, map[string]any{"items": out, "total_count": len(out)})
	}
}
func getImpactAssessment(s impactStore, repos codeIntelligenceStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, c, auth.RepositoryRead)
		if !ok {
			return
		}
		v, e := s.Get(r.PathValue("repository"), r.PathValue("assessment"))
		if e != nil || !impactParticipant(v, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, v)
	}
}
func addImpact(s impactStore, repos codeIntelligenceStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in impactassessments.Impact
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		in.ID = ""
		in.CreatedByID = ""
		v, e := s.AddImpact(string(repo.ID), r.PathValue("assessment"), a.UserID, in)
		writeImpact(w, v, e, 201)
	}
}
func updateImpact(s impactStore, repos codeIntelligenceStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct{ State, Rationale string }
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.Update(string(repo.ID), r.PathValue("assessment"), a.UserID, r.PathValue("impact"), in.State, in.Rationale)
		writeImpact(w, v, e, 200)
	}
}
func requestImpactAck(s impactStore, repos codeIntelligenceStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			OwnerID string `json:"owner_id"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		v, e := s.Request(string(repo.ID), r.PathValue("assessment"), a.UserID, r.PathValue("impact"), in.OwnerID)
		writeImpact(w, v, e, 201)
	}
}
func decideImpactAck(s impactStore, repos codeIntelligenceStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := authenticateRequest(w, r, c, auth.RepositoryRead)
		if !ok {
			return
		}
		var in struct{ State, Note string }
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		v, e := s.Decide(r.PathValue("repository"), r.PathValue("assessment"), a.UserID, r.PathValue("impact"), in.State, in.Note)
		writeImpact(w, v, e, 200)
	}
}
func startImpactAgent(s impactStore, repos codeIntelligenceStore, c authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		v, t, e := s.StartAgent(string(repo.ID), r.PathValue("assessment"), a.UserID)
		if e != nil {
			writeImpact(w, v, e, 201)
			return
		}
		writeJSON(w, 201, map[string]any{"assessment": v, "worker_credential": t, "credential_notice": "shown once; read-only assessment context and finding publication only; no Git or repository-write authority"})
	}
}
func impactAgentContext(s impactStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, e := s.AgentContext(bearer(r))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, v)
	}
}
func impactAgentFinding(s impactStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Body, Uncertainty string
			Evidence          []impactassessments.Evidence
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.AddFinding(bearer(r), in.Body, in.Uncertainty, in.Evidence)
		writeImpact(w, v, e, 201)
	}
}
func bearer(r *http.Request) string {
	return strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
}
func impactParticipant(v impactassessments.Assessment, a string) bool {
	for _, x := range v.Participants {
		if x == a {
			return true
		}
	}
	return false
}
func writeImpact(w http.ResponseWriter, v impactassessments.Assessment, e error, status int) {
	if e == nil {
		writeJSON(w, status, v)
		return
	}
	if errors.Is(e, impactassessments.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if errors.Is(e, impactassessments.ErrConflict) {
		writeJSON(w, 409, map[string]string{"error": "conflict"})
		return
	}
	writeJSON(w, 500, map[string]string{"error": "internal_error"})
}
