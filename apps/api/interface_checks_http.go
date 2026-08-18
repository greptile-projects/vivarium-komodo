package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designproposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/interfacechecks"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type interfaceCheckSources struct {
	pulls interface {
		Get(string, string) (pullrequests.PullRequest, error)
	}
	repositories interface {
		Open(storage.ID) (*storage.Repository, error)
	}
	designs interface {
		Get(string, string) (designproposals.Proposal, error)
	}
}
type interfaceCheckConfig struct {
	SchemaVersion int `json:"schema_version"`
	Specification struct {
		Kind    string `json:"kind"`
		ID      string `json:"id"`
		Version int64  `json:"version"`
	} `json:"specification"`
	Cases []struct {
		Name           string                  `json:"name"`
		Journey        string                  `json:"journey"`
		Surface        string                  `json:"surface"`
		Context        interfacechecks.Context `json:"context"`
		RequirementIDs []string                `json:"requirement_ids"`
		Inputs         []string                `json:"inputs"`
	} `json:"cases"`
}
type interfaceCaseResult struct {
	Status      string                       `json:"status"`
	Summary     string                       `json:"summary"`
	Coverage    []string                     `json:"coverage"`
	DurationMS  int64                        `json:"duration_ms"`
	Performance map[string]float64           `json:"performance"`
	Artifacts   []interfacechecks.Artifact   `json:"artifacts"`
	Differences []interfacechecks.Difference `json:"differences"`
}

func registerInterfaceChecksHTTP(mux *http.ServeMux, s *interfacechecks.Store, repos proposalRepositoryStore, c authStore, src interfaceCheckSources) {
	base := "/repositories/{repository}/pull-requests/{pull}/interface-checks"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		scope := auth.RepositoryRead
		if write {
			scope = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, scope, write)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	project := func(repo, pull string, v *interfacechecks.Run) {
		p, e := src.pulls.Get(repo, pull)
		if e != nil {
			return
		}
		opened, e := src.repositories.Open(storage.ID(p.SourceRepositoryID))
		if e != nil {
			return
		}
		cfgBlob, _, _ := interfaceCheckBlob(opened, p.SourceCommitID, v.ConfigPath)
		blobs := map[string]string{}
		for _, cs := range v.Cases {
			for _, in := range cs.Inputs {
				oid, _, ok := interfaceCheckBlob(opened, p.SourceCommitID, in.Path)
				if ok {
					blobs[in.Path] = oid
				}
			}
		}
		interfacechecks.DeriveCurrent(v, p.SourceCommitID, cfgBlob, blobs)
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		items, e := s.List(repo, r.PathValue("pull"))
		if interfaceCheckError(w, e) {
			return
		}
		for i := range items {
			project(repo, r.PathValue("pull"), &items[i])
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET "+base+"/{run}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := s.Get(repo, r.PathValue("pull"), r.PathValue("run"))
		if interfaceCheckError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &v)
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST "+base+"/runs", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Revision   string                         `json:"revision"`
			ConfigPath string                         `json:"config_path"`
			Results    map[string]interfaceCaseResult `json:"results"`
		}
		if !readJSON(w, r, &in, 2<<20) {
			return
		}
		if in.ConfigPath == "" {
			in.ConfigPath = ".komodo/interface-checks.json"
		}
		p, e := src.pulls.Get(repo, r.PathValue("pull"))
		if e != nil || p.SourceCommitID != in.Revision {
			writeJSON(w, 409, map[string]string{"error": "exact_pull_request_revision_required"})
			return
		}
		opened, e := src.repositories.Open(storage.ID(p.SourceRepositoryID))
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "interface_source_unavailable"})
			return
		}
		configBlob, body, ok := interfaceCheckBlob(opened, in.Revision, in.ConfigPath)
		var cfg interfaceCheckConfig
		if !ok || json.Unmarshal(body, &cfg) != nil || cfg.SchemaVersion != 1 || len(cfg.Cases) == 0 {
			writeJSON(w, 422, map[string]string{"error": "interface_checks_invalid"})
			return
		}
		if !acceptedInterfaceSpecification(src.designs, repo, cfg.Specification.Kind, cfg.Specification.ID, cfg.Specification.Version) {
			writeJSON(w, 422, map[string]string{"error": "accepted_interface_specification_required"})
			return
		}
		run := interfacechecks.Run{RepositoryID: repo, PullRequestID: p.ID, Revision: in.Revision, SpecificationKind: cfg.Specification.Kind, SpecificationID: cfg.Specification.ID, SpecificationVersion: cfg.Specification.Version, ConfigPath: in.ConfigPath, ConfigBlobID: configBlob, CreatedByID: actor}
		names := map[string]bool{}
		for _, d := range cfg.Cases {
			if names[d.Name] || len(d.Inputs) == 0 {
				writeJSON(w, 422, map[string]string{"error": "interface_checks_invalid"})
				return
			}
			names[d.Name] = true
			result, exists := in.Results[d.Name]
			if !exists || strings.TrimSpace(result.Summary) == "" {
				writeJSON(w, 422, map[string]string{"error": "interface_check_result_missing"})
				return
			}
			cs := interfacechecks.Case{Name: d.Name, Journey: d.Journey, Surface: d.Surface, Context: d.Context, RequirementIDs: d.RequirementIDs, Status: result.Status, Summary: result.Summary, Coverage: result.Coverage, DurationMS: result.DurationMS, Performance: result.Performance, Artifacts: result.Artifacts, Differences: result.Differences}
			for _, path := range d.Inputs {
				oid, _, exists := interfaceCheckBlob(opened, in.Revision, path)
				if !exists {
					writeJSON(w, 422, map[string]string{"error": "interface_check_input_missing"})
					return
				}
				cs.Inputs = append(cs.Inputs, interfacechecks.Input{Path: path, BlobID: oid})
			}
			run.Cases = append(run.Cases, cs)
		}
		v, e := s.Create(run)
		if interfaceCheckError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{run}/cases/{case}/differences/{difference}/classification", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Classification string `json:"classification"`
			Rationale      string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.Classify(repo, r.PathValue("pull"), r.PathValue("run"), r.PathValue("case"), r.PathValue("difference"), actor, in.Classification, in.Rationale)
		if interfaceCheckError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &v)
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/{run}/cases/{case}/approvals", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Decision      string   `json:"decision"`
			Note          string   `json:"note"`
			DifferenceIDs []string `json:"difference_ids"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := s.Approve(repo, r.PathValue("pull"), r.PathValue("run"), r.PathValue("case"), actor, in.Decision, in.Note, in.DifferenceIDs)
		if interfaceCheckError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &v)
		writeJSON(w, 201, v)
	})
}

func acceptedInterfaceSpecification(designs interface {
	Get(string, string) (designproposals.Proposal, error)
}, repo, kind, id string, version int64) bool {
	if designs == nil || version < 1 {
		return false
	}
	if kind == "design_proposal" {
		p, e := designs.Get(repo, id)
		if e != nil || p.CurrentRevision != version {
			return false
		}
		accepted := false
		for _, a := range p.Acknowledgements {
			if a.ProposalRevision == version && a.Current {
				accepted = true
				if a.Status != "acknowledged" {
					return false
				}
			}
		}
		return accepted
	}
	if kind == "implementation_contract" {
		// Implementation IDs are repository-unique within their proposal store;
		// scan the bounded repository collection and require its frozen proposal
		// revision to match the manifest version.
		items, e := designs.(interface {
			List(string) ([]designproposals.Proposal, error)
		}).List(repo)
		if e != nil {
			return false
		}
		for _, p := range items {
			for _, implementation := range p.Implementations {
				if implementation.ID == id && implementation.ProposalRevision == version {
					return true
				}
			}
		}
	}
	return false
}
func interfaceCheckBlob(r *storage.Repository, revision, path string) (string, []byte, bool) {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" || strings.Contains(path, "..") {
		return "", nil, false
	}
	commit, e := r.ReadCommit(storage.ObjectID(revision))
	if e != nil {
		return "", nil, false
	}
	tree, e := r.ReadTree(commit.Tree)
	if e != nil {
		return "", nil, false
	}
	parts := strings.Split(path, "/")
	for i, p := range parts {
		var entry *storage.TreeEntry
		for j := range tree.Entries {
			if tree.Entries[j].Name == p {
				entry = &tree.Entries[j]
				break
			}
		}
		if entry == nil {
			return "", nil, false
		}
		if i == len(parts)-1 {
			if entry.Type != storage.BlobObject {
				return "", nil, false
			}
			obj, e := r.ReadObject(entry.ObjectID)
			return string(entry.ObjectID), obj.Content, e == nil
		}
		if entry.Type != storage.TreeObject {
			return "", nil, false
		}
		tree, e = r.ReadTree(entry.ObjectID)
		if e != nil {
			return "", nil, false
		}
	}
	return "", nil, false
}
func interfaceCheckError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	status := 500
	code := "internal_error"
	if errors.Is(e, interfacechecks.ErrNotFound) {
		status = 404
		code = "interface_check_not_found"
	} else if errors.Is(e, interfacechecks.ErrInvalid) {
		status = 422
		code = "invalid_interface_check"
	}
	writeJSON(w, status, map[string]string{"error": code})
	return true
}
