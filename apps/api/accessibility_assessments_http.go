package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilityassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/accessibilitybarriers"
	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	"github.com/greptile-projects/vivarium-komodo/apps/api/previews"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type accessibilityAssessmentSources struct {
	pulls interface {
		Get(string, string) (pullrequests.PullRequest, error)
	}
	runs interface {
		Get(string, string, string) (checkruns.Run, error)
	}
	previews interface {
		GetByID(string) (previews.Preview, error)
	}
	barriers interface {
		Get(string, string) (accessibilitybarriers.Barrier, error)
	}
	repositories interface {
		Open(storage.ID) (*storage.Repository, error)
	}
}

func registerAccessibilityAssessmentsHTTP(mux *http.ServeMux, s *accessibilityassessments.Store, repos proposalRepositoryStore, c authStore, src accessibilityAssessmentSources) {
	base := "/repositories/{repository}/pull-requests/{pull}/accessibility-assessments"
	read := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	project := func(repo, pull string, a *accessibilityassessments.Assessment) {
		p, e := src.pulls.Get(repo, pull)
		if e != nil {
			return
		}
		blobs := assessmentBlobs(src.repositories, repo, p.SourceCommitID, a)
		accessibilityassessments.Derive(a, p.SourceCommitID, blobs)
	}
	mux.HandleFunc("GET /repositories/{repository}/accessibility-assessments", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := read(w, r)
		if !ok {
			return
		}
		items, e := s.ListRepository(repo)
		if assessmentError(w, e) {
			return
		}
		for i := range items {
			project(repo, items[i].PullRequestID, &items[i])
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := read(w, r)
		if !ok {
			return
		}
		items, e := s.List(repo, r.PathValue("pull"))
		if assessmentError(w, e) {
			return
		}
		for i := range items {
			project(repo, r.PathValue("pull"), &items[i])
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := read(w, r)
		if !ok {
			return
		}
		p, e := src.pulls.Get(repo, r.PathValue("pull"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "pull_request_not_found"})
			return
		}
		var in accessibilityassessments.Input
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		if in.Revision != p.SourceCommitID {
			writeJSON(w, 409, map[string]string{"error": "exact_pull_request_revision_required"})
			return
		}
		if !captureAssessmentLocations(src.repositories, repo, in.Revision, &in) {
			writeJSON(w, 422, map[string]string{"error": "invalid_accessibility_source_location"})
			return
		}
		a, e := s.Create(repo, p.ID, actor, in)
		if assessmentError(w, e) {
			return
		}
		project(repo, p.ID, &a)
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("GET "+base+"/{assessment}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := read(w, r)
		if !ok {
			return
		}
		a, e := s.Get(repo, r.PathValue("pull"), r.PathValue("assessment"))
		if assessmentError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &a)
		writeJSON(w, 200, a)
	})
	mux.HandleFunc("POST "+base+"/{assessment}/automation", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := read(w, r)
		if !ok {
			return
		}
		var in struct {
			RunID string `json:"run_id"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		run, e := src.runs.Get(repo, r.PathValue("pull"), in.RunID)
		if e != nil || run.Definition.Accessibility == nil {
			writeJSON(w, 422, map[string]string{"error": "accessibility_check_run_required"})
			return
		}
		a, e := s.Get(repo, r.PathValue("pull"), r.PathValue("assessment"))
		if assessmentError(w, e) {
			return
		}
		if run.CommitID != a.Revision {
			writeJSON(w, 409, map[string]string{"error": "check_revision_mismatch"})
			return
		}
		spec := run.Definition.Accessibility
		inputs := make([]accessibilityassessments.Location, 0, len(spec.Inputs))
		rr, openErr := src.repositories.Open(storage.ID(repo))
		if openErr != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		for _, path := range spec.Inputs {
			oid, exists := assessmentBlob(rr, storage.ObjectID(a.Revision), path)
			if !exists {
				writeJSON(w, 422, map[string]string{"error": "accessibility_check_input_missing"})
				return
			}
			inputs = append(inputs, accessibilityassessments.Location{Path: path, BlobID: string(oid)})
		}
		a, e = s.AddAutomation(repo, r.PathValue("pull"), a.ID, actor, accessibilityassessments.Automation{RunID: run.ID, Name: run.Definition.Name, ScenarioIDs: spec.ScenarioIDs, Evaluations: spec.Evaluations, Status: string(run.State), RequiresHumanEvaluation: spec.RequiresHumanEvaluation, Inputs: inputs})
		if assessmentError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &a)
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("POST "+base+"/{assessment}/findings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := read(w, r)
		if !ok {
			return
		}
		a, e := s.Get(repo, r.PathValue("pull"), r.PathValue("assessment"))
		if assessmentError(w, e) {
			return
		}
		var in accessibilityassessments.FindingInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		if !captureFindingLocations(src.repositories, repo, a.Revision, &in) || !validAssessmentCitation(repo, a.Revision, in.Citation, src) {
			writeJSON(w, 422, map[string]string{"error": "permitted_revision_exact_evidence_required"})
			return
		}
		a, e = s.AddFinding(repo, r.PathValue("pull"), a.ID, actor, in)
		if assessmentError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &a)
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("POST "+base+"/{assessment}/findings/{finding}/decisions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := read(w, r)
		if !ok {
			return
		}
		if _, _, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryWrite, true); !ok {
			return
		}
		var in accessibilityassessments.DecisionInput
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		a, e := s.Decide(repo, r.PathValue("pull"), r.PathValue("assessment"), r.PathValue("finding"), actor, in)
		if assessmentError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &a)
		writeJSON(w, 201, a)
	})
}

func captureAssessmentLocations(opener interface {
	Open(storage.ID) (*storage.Repository, error)
}, repo, revision string, in *accessibilityassessments.Input) bool {
	r, e := opener.Open(storage.ID(repo))
	if e != nil {
		return false
	}
	for i := range in.Scenarios {
		h := sha256.New()
		for j := range in.Scenarios[i].Locations {
			oid, ok := assessmentBlob(r, storage.ObjectID(revision), in.Scenarios[i].Locations[j].Path)
			if !ok {
				return false
			}
			in.Scenarios[i].Locations[j].BlobID = string(oid)
			fmt.Fprintf(h, "%s\x00%s\x00", in.Scenarios[i].Locations[j].Path, oid)
		}
		in.Scenarios[i].Digest = fmt.Sprintf("%x", h.Sum(nil))
	}
	return true
}
func captureFindingLocations(opener interface {
	Open(storage.ID) (*storage.Repository, error)
}, repo, revision string, in *accessibilityassessments.FindingInput) bool {
	r, e := opener.Open(storage.ID(repo))
	if e != nil {
		return false
	}
	for i := range in.Locations {
		oid, ok := assessmentBlob(r, storage.ObjectID(revision), in.Locations[i].Path)
		if !ok {
			return false
		}
		in.Locations[i].BlobID = string(oid)
	}
	return true
}
func assessmentBlob(r *storage.Repository, revision storage.ObjectID, path string) (storage.ObjectID, bool) {
	commit, e := r.ReadCommit(revision)
	if e != nil {
		return "", false
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var walk func(storage.ObjectID, int) (storage.ObjectID, bool)
	walk = func(tree storage.ObjectID, i int) (storage.ObjectID, bool) {
		t, e := r.ReadTree(tree)
		if e != nil {
			return "", false
		}
		for _, x := range t.Entries {
			if x.Name == parts[i] {
				if i == len(parts)-1 {
					return x.ObjectID, x.Type == storage.BlobObject
				}
				if x.Type != storage.TreeObject {
					return "", false
				}
				return walk(x.ObjectID, i+1)
			}
		}
		return "", false
	}
	if len(parts) == 0 || parts[0] == "" {
		return "", false
	}
	return walk(commit.Tree, 0)
}
func assessmentBlobs(opener interface {
	Open(storage.ID) (*storage.Repository, error)
}, repo, revision string, a *accessibilityassessments.Assessment) map[string]string {
	out := map[string]string{}
	r, e := opener.Open(storage.ID(repo))
	if e != nil {
		return out
	}
	for _, sc := range a.Scenarios {
		for _, x := range sc.Locations {
			if oid, ok := assessmentBlob(r, storage.ObjectID(revision), x.Path); ok {
				out[x.Path] = string(oid)
			}
		}
	}
	for _, f := range a.Findings {
		for _, x := range f.Locations {
			if _, exists := out[x.Path]; !exists {
				if oid, ok := assessmentBlob(r, storage.ObjectID(revision), x.Path); ok {
					out[x.Path] = string(oid)
				}
			}
		}
	}
	for _, automation := range a.Automation {
		for _, x := range automation.Inputs {
			if _, exists := out[x.Path]; !exists {
				if oid, ok := assessmentBlob(r, storage.ObjectID(revision), x.Path); ok {
					out[x.Path] = string(oid)
				}
			}
		}
	}
	return out
}
func validAssessmentCitation(repo, revision string, c accessibilityassessments.Citation, s accessibilityAssessmentSources) bool {
	if c.Kind == "preview" {
		p, e := s.previews.GetByID(c.ResourceID)
		return e == nil && p.RepositoryID == repo && p.Revision == revision
	}
	if c.Kind == "reproduction" {
		b, e := s.barriers.Get(repo, c.ResourceID)
		if e != nil {
			return false
		}
		for _, a := range b.Attempts {
			if a.Revision == revision {
				return true
			}
		}
	}
	return false
}
func assessmentError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, accessibilityassessments.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "accessibility_assessment_not_found"})
	} else if errors.Is(e, accessibilityassessments.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_accessibility_assessment"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
