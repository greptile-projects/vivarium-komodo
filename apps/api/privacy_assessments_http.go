package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/datacommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dataflows"
	"github.com/greptile-projects/vivarium-komodo/apps/api/privacyassessments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type privacyAssessmentSources struct {
	pulls interface {
		Get(string, string) (pullrequests.PullRequest, error)
	}
	flows interface {
		Get(string, string) (dataflows.Flow, error)
	}
	commitments interface {
		Get(string, string) (datacommitments.Commitment, error)
	}
	repositories interface {
		Open(storage.ID) (*storage.Repository, error)
	}
}

func registerPrivacyAssessmentsHTTP(mux *http.ServeMux, s *privacyassessments.Store, repos proposalRepositoryStore, c authStore, src privacyAssessmentSources) {
	base := "/repositories/{repository}/pull-requests/{pull}/privacy-assessments"
	access := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, c, auth.RepositoryRead, true)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	project := func(repo, pull string, a *privacyassessments.Assessment) {
		p, e := src.pulls.Get(repo, pull)
		if e != nil {
			return
		}
		blobs := map[string]string{}
		opened, e := src.repositories.Open(storage.ID(repo))
		if e == nil {
			for _, x := range a.Comparisons {
				for _, l := range x.Evidence {
					if oid, ok := assessmentBlob(opened, storage.ObjectID(p.SourceCommitID), l.Path); ok {
						blobs[l.Path] = string(oid)
					}
				}
			}
		}
		privacyassessments.Derive(a, p.SourceCommitID, blobs)
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r)
		if !ok {
			return
		}
		items, e := s.List(repo, r.PathValue("pull"))
		if privacyAssessmentError(w, e) {
			return
		}
		for i := range items {
			project(repo, r.PathValue("pull"), &items[i])
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		p, e := src.pulls.Get(repo, r.PathValue("pull"))
		if e != nil {
			writeJSON(w, 404, map[string]string{"error": "pull_request_not_found"})
			return
		}
		var in privacyassessments.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if in.Revision != p.SourceCommitID || in.TargetRevision != p.TargetCommitID {
			writeJSON(w, 409, map[string]string{"error": "exact_pull_request_revisions_required"})
			return
		}
		opened, e := src.repositories.Open(storage.ID(repo))
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_privacy_evidence"})
			return
		}
		for i := range in.Comparisons {
			for j := range in.Comparisons[i].Evidence {
				l := &in.Comparisons[i].Evidence[j]
				oid, ok := assessmentBlob(opened, storage.ObjectID(in.Revision), l.Path)
				if !ok {
					writeJSON(w, 422, map[string]string{"error": "invalid_privacy_evidence"})
					return
				}
				l.BlobID = string(oid)
			}
			x := in.Comparisons[i]
			for _, fid := range []string{x.BaselineFlowID, x.CandidateFlowID} {
				if fid != "" {
					f, e := src.flows.Get(repo, fid)
					if e != nil || (fid == x.BaselineFlowID && f.Revision != in.TargetRevision) || (fid == x.CandidateFlowID && f.Revision != in.Revision) {
						writeJSON(w, 422, map[string]string{"error": "invalid_privacy_flow_reference"})
						return
					}
				}
			}
		}
		for _, ref := range in.Commitments {
			d, e := src.commitments.Get(repo, ref.ID)
			if e != nil || ref.BaselineVersion > int64(len(d.Versions)) || ref.CandidateVersion > int64(len(d.Versions)) {
				writeJSON(w, 422, map[string]string{"error": "invalid_privacy_commitment_reference"})
				return
			}
		}
		a, e := s.Create(repo, p.ID, actor, in)
		if privacyAssessmentError(w, e) {
			return
		}
		project(repo, p.ID, &a)
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("GET "+base+"/{assessment}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r)
		if !ok {
			return
		}
		a, e := s.Get(repo, r.PathValue("pull"), r.PathValue("assessment"))
		if privacyAssessmentError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &a)
		writeJSON(w, 200, a)
	})
	mux.HandleFunc("POST "+base+"/{assessment}/entries", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		a, e := s.Get(repo, r.PathValue("pull"), r.PathValue("assessment"))
		if privacyAssessmentError(w, e) {
			return
		}
		var in privacyassessments.EntryInput
		if !readJSON(w, r, &in, 512<<10) {
			return
		}
		opened, e := src.repositories.Open(storage.ID(repo))
		if e != nil {
			return
		}
		for i := range in.Evidence {
			oid, ok := assessmentBlob(opened, storage.ObjectID(a.Revision), in.Evidence[i].Path)
			if !ok {
				writeJSON(w, 422, map[string]string{"error": "invalid_privacy_evidence"})
				return
			}
			in.Evidence[i].BlobID = string(oid)
		}
		a, e = s.AddEntry(repo, r.PathValue("pull"), a.ID, actor, in)
		if privacyAssessmentError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &a)
		writeJSON(w, 201, a)
	})
	mux.HandleFunc("POST "+base+"/{assessment}/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := access(w, r)
		if !ok {
			return
		}
		var in struct {
			RequirementID string `json:"requirement_id"`
			Decision      string `json:"decision"`
			Rationale     string `json:"rationale"`
			Revision      string `json:"revision"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		a, e := s.Acknowledge(repo, r.PathValue("pull"), r.PathValue("assessment"), actor, in.RequirementID, in.Decision, in.Rationale, in.Revision)
		if privacyAssessmentError(w, e) {
			return
		}
		project(repo, r.PathValue("pull"), &a)
		writeJSON(w, 201, a)
	})
}
func privacyAssessmentError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	if errors.Is(e, privacyassessments.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "privacy_assessment_not_found"})
	} else if errors.Is(e, privacyassessments.ErrInvalid) {
		writeJSON(w, 422, map[string]string{"error": "invalid_privacy_assessment"})
	} else {
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
