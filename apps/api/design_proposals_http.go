package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/designproposals"
)

func registerDesignProposalsHTTP(mux *http.ServeMux, s *designproposals.Store, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/design-proposals"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		scope := auth.RepositoryRead
		if write {
			scope = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, scope, write)
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
		x, e := s.List(repo)
		if !designProposalError(w, e) {
			writeJSON(w, 200, map[string]any{"items": x})
		}
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in designproposals.Input
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Create(repo, a, in)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("GET "+base+"/{proposal}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := access(w, r, false)
		if !ok {
			return
		}
		x, e := s.Get(repo, r.PathValue("proposal"))
		if !designProposalError(w, e) {
			writeJSON(w, 200, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/revisions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			designproposals.Input
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Revise(repo, r.PathValue("proposal"), a, in.ExpectedVersion, in.Input)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/participants", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion     int64    `json:"expected_version"`
			SubjectID           string   `json:"subject_id"`
			Kind                string   `json:"kind"`
			Role                string   `json:"role"`
			GroundedEvidenceIDs []string `json:"grounded_evidence_ids"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Invite(repo, r.PathValue("proposal"), a, in.SubjectID, in.Kind, in.Role, in.GroundedEvidenceIDs, in.ExpectedVersion)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/artifacts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in designproposals.ArtifactInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.AddArtifact(repo, r.PathValue("proposal"), a, in)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/artifacts/{artifact}/revisions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			designproposals.ArtifactInput
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.ReviseArtifact(repo, r.PathValue("proposal"), r.PathValue("artifact"), a, in.ExpectedVersion, in.ArtifactInput)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/comments", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			SubjectKind     string   `json:"subject_kind"`
			SubjectID       string   `json:"subject_id"`
			SubjectRevision int64    `json:"subject_revision"`
			Body            string   `json:"body"`
			Stance          string   `json:"stance"`
			EvidenceIDs     []string `json:"evidence_ids"`
			Uncertainty     string   `json:"uncertainty"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Comment(repo, r.PathValue("proposal"), a, in.SubjectKind, in.SubjectID, in.Body, in.Stance, in.Uncertainty, in.SubjectRevision, in.EvidenceIDs)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			OwnerID         string `json:"owner_id"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.RequestAcknowledgement(repo, r.PathValue("proposal"), a, in.OwnerID, in.ExpectedVersion)
		if !designProposalError(w, e) {
			writeJSON(w, 201, x)
		}
	})
	mux.HandleFunc("POST "+base+"/{proposal}/acknowledgements/{ack}/response", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		var in struct {
			Status    string `json:"status"`
			Rationale string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		x, e := s.Respond(repo, r.PathValue("proposal"), r.PathValue("ack"), a, in.Status, in.Rationale)
		if !designProposalError(w, e) {
			writeJSON(w, 200, x)
		}
	})
}

func designProposalError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, designproposals.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "design_proposal_not_found"})
	case errors.Is(e, designproposals.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "design_proposal_changed"})
	case errors.Is(e, designproposals.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_design_proposal"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
