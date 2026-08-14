package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/datacommitments"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dataflows"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type dataFlowRepositories interface {
	proposalRepositoryStore
	Open(storage.ID) (*storage.Repository, error)
}

func registerDataFlowsHTTP(mux *http.ServeMux, s *dataflows.Store, commitments dataCommitmentStore, repos dataFlowRepositories, credentials authStore) {
	base := "/repositories/{repository}/data-flows"
	read := func(w http.ResponseWriter, r *http.Request) (string, string, bool) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := read(w, r)
		if !ok {
			return
		}
		items, e := s.List(repo)
		if dataFlowError(w, e) {
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in dataflows.DeclarationInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if !captureDataFlowDeclaration(repos, commitments, string(repo.ID), &in) {
			writeJSON(w, 422, map[string]string{"error": "invalid_revision_exact_data_flow"})
			return
		}
		f, e := s.Create(string(repo.ID), a.UserID, in)
		if dataFlowError(w, e) {
			return
		}
		writeJSON(w, 201, f)
	})
	mux.HandleFunc("GET "+base+"/{flow}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := read(w, r)
		if !ok {
			return
		}
		f, e := s.Get(repo, r.PathValue("flow"))
		if dataFlowError(w, e) {
			return
		}
		writeJSON(w, 200, f)
	})
	mux.HandleFunc("POST "+base+"/{flow}/findings", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := read(w, r)
		if !ok {
			return
		}
		f, e := s.Get(repo, r.PathValue("flow"))
		if dataFlowError(w, e) {
			return
		}
		var in dataflows.FindingInput
		if !readJSON(w, r, &in, 1<<20) {
			return
		}
		if !captureDataFlowCitations(repos, repo, f.Revision, &in) {
			writeJSON(w, 422, map[string]string{"error": "invalid_data_flow_citation"})
			return
		}
		f, e = s.AddFinding(repo, f.ID, actor, in)
		if dataFlowError(w, e) {
			return
		}
		writeJSON(w, 201, f)
	})
}

func captureDataFlowDeclaration(repos interface {
	Open(storage.ID) (*storage.Repository, error)
}, commitments dataCommitmentStore, repo string, in *dataflows.DeclarationInput) bool {
	r, e := repos.Open(storage.ID(repo))
	if e != nil {
		return false
	}
	if _, e = r.ReadCommit(storage.ObjectID(in.Revision)); e != nil {
		return false
	}
	if oid, ok := assessmentBlob(r, storage.ObjectID(in.Revision), in.Manifest.Path); !ok {
		return false
	} else {
		in.Manifest.BlobID = string(oid)
	}
	for i := range in.Nodes {
		if in.Nodes[i].Location == nil {
			continue
		}
		oid, ok := assessmentBlob(r, storage.ObjectID(in.Revision), in.Nodes[i].Location.Path)
		if !ok {
			return false
		}
		in.Nodes[i].Location.BlobID = string(oid)
	}
	for _, ref := range in.Commitments {
		c, e := commitments.Get(repo, ref.ID)
		if e != nil || ref.Version < 1 || ref.Version > int64(len(c.Versions)) {
			return false
		}
		uses := map[string]bool{}
		for _, u := range c.Versions[ref.Version-1].DataUses {
			uses[u.ID] = true
		}
		for _, id := range ref.DataUseIDs {
			if !uses[id] {
				return false
			}
		}
	}
	return true
}
func captureDataFlowCitations(repos interface {
	Open(storage.ID) (*storage.Repository, error)
}, repo, revision string, in *dataflows.FindingInput) bool {
	r, e := repos.Open(storage.ID(repo))
	if e != nil {
		return false
	}
	for i := range in.Citations {
		c := &in.Citations[i]
		if c.Location == nil {
			continue
		}
		oid, ok := assessmentBlob(r, storage.ObjectID(revision), c.Location.Path)
		if !ok {
			return false
		}
		c.Location.BlobID = string(oid)
	}
	return true
}
func dataFlowError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, dataflows.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "data_flow_not_found"})
	case errors.Is(e, dataflows.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_data_flow"})
	case errors.Is(e, datacommitments.ErrNotFound):
		writeJSON(w, 422, map[string]string{"error": "data_commitment_not_found"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
