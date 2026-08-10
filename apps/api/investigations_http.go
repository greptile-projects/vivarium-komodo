package main

import (
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/investigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

type investigationStore interface {
	Create(string, string, string, string, string, string) (investigations.Investigation, error)
	Get(string, string) (investigations.Investigation, error)
	List(string) ([]investigations.Investigation, error)
	Invite(string, string, string, string) (investigations.Investigation, error)
	Add(string, string, string, investigations.Entry) (investigations.Investigation, error)
	Rerun(string, string, string, string, string, string) (investigations.Investigation, error)
}
type investigationWorkspaceStore interface {
	Get(string, string) (workspaces.Workspace, error)
}

func registerInvestigationsHTTP(mux *http.ServeMux, store investigationStore, repositories codeIntelligenceStore, credentials authStore, workspaceStore investigationWorkspaceStore) {
	base := "/repositories/{repository}/investigations"
	mux.HandleFunc("POST "+base, createInvestigation(store, repositories, credentials))
	mux.HandleFunc("GET "+base, listInvestigations(store, repositories, credentials))
	mux.HandleFunc("GET "+base+"/{investigation}", getInvestigation(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{investigation}/participants", inviteInvestigation(store, repositories, credentials))
	mux.HandleFunc("POST "+base+"/{investigation}/entries", addInvestigationEntry(store, repositories, credentials, workspaceStore))
	mux.HandleFunc("POST "+base+"/{investigation}/runs", rerunInvestigation(store, repositories, credentials))
}

func createInvestigation(store investigationStore, repositories codeIntelligenceStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct{ Title, Question, Revision string }
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		in.Title = strings.TrimSpace(in.Title)
		in.Question = strings.TrimSpace(in.Question)
		if in.Title == "" || len(in.Title) > 200 || in.Question == "" || len(in.Question) > 4000 {
			writeJSON(w, 422, map[string]string{"error": "invalid_investigation"})
			return
		}
		repo, err := repositories.Open(item.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		commit, revision, err := resolveRevision(repo, in.Revision)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "revision_not_found"})
			return
		}
		v, err := store.Create(string(item.ID), in.Title, in.Question, revision, string(commit), actor.UserID)
		writeInvestigation(w, v, err, 201)
	}
}
func listInvestigations(store investigationStore, repositories codeIntelligenceStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.List(string(item.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		visible := items[:0]
		for _, v := range items {
			if investigationParticipant(v, actor.UserID) {
				visible = append(visible, v)
			}
		}
		writeJSON(w, 200, map[string]any{"items": visible, "total_count": len(visible)})
	}
}
func getInvestigation(store investigationStore, repositories codeIntelligenceStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, err := store.Get(string(item.ID), r.PathValue("investigation"))
		if err != nil || !investigationParticipant(v, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, v)
	}
}

func inviteInvestigation(store investigationStore, repositories codeIntelligenceStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			UserID string `json:"user_id"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		in.UserID = strings.TrimSpace(in.UserID)
		allowed := in.UserID == item.OwnerID
		for _, id := range item.CollaboratorIDs {
			allowed = allowed || id == in.UserID
		}
		if !allowed {
			writeJSON(w, 422, map[string]string{"error": "repository_participant_required"})
			return
		}
		v, err := store.Invite(string(item.ID), r.PathValue("investigation"), actor.UserID, in.UserID)
		writeInvestigation(w, v, err, 200)
	}
}

func addInvestigationEntry(store investigationStore, repositories codeIntelligenceStore, credentials authStore, workspaceStore investigationWorkspaceStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in investigations.Entry
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, err := store.Get(string(item.ID), r.PathValue("investigation"))
		if err != nil || !investigationParticipant(v, actor.UserID) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if len(in.Citations) > 20 {
			writeJSON(w, 422, map[string]string{"error": "invalid_citations"})
			return
		}
		repo, err := repositories.Open(item.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		for i := range in.Citations {
			c := &in.Citations[i]
			if c.RepositoryID == "" {
				c.RepositoryID = string(item.ID)
			}
			if c.RepositoryID != string(item.ID) {
				writeJSON(w, 422, map[string]string{"error": "cross_repository_citation_not_permitted"})
				return
			}
			if c.Kind == "workspace_observation" {
				ws, e := workspaceStore.Get(string(item.ID), c.WorkspaceID)
				if e != nil || ws.Revision != v.CommitID || !workspaceEvent(ws, c.WorkspaceSequence) {
					writeJSON(w, 422, map[string]string{"error": "invalid_workspace_observation"})
					return
				}
				c.CommitID = ws.Revision
				continue
			}
			if c.Path != "" {
				if c.CommitID == "" {
					c.CommitID = v.CommitID
				}
				if c.CommitID != v.CommitID || !validInvestigationPath(c.Path) {
					writeJSON(w, 422, map[string]string{"error": "invalid_code_reference"})
					return
				}
				blob, e := blobAtPath(repo, storage.ObjectID(c.CommitID), c.Path)
				if e != nil || c.ObjectID != "" && c.ObjectID != string(blob) {
					writeJSON(w, 422, map[string]string{"error": "invalid_code_reference"})
					return
				}
				c.ObjectID = string(blob)
			} else if c.CommitID != "" && c.CommitID != v.CommitID {
				writeJSON(w, 422, map[string]string{"error": "invalid_revision"})
				return
			}
		}
		if in.Type == "runtime_observation" && (len(in.Citations) != 1 || in.Citations[0].Kind != "workspace_observation") {
			writeJSON(w, 422, map[string]string{"error": "workspace_observation_required"})
			return
		}
		in.ActorID = ""
		in.CommitID = ""
		in.Stale = false
		// Agent identity is not accepted from this human-session endpoint. Imported
		// findings remain attributed to the participant who put them on the canvas.
		in.Agent = ""
		v, err = store.Add(string(item.ID), v.ID, actor.UserID, in)
		writeInvestigation(w, v, err, 201)
	}
}

func rerunInvestigation(store investigationStore, repositories codeIntelligenceStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct{ Revision, Reason string }
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		repo, err := repositories.Open(item.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		commit, revision, err := resolveRevision(repo, in.Revision)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "revision_not_found"})
			return
		}
		v, err := store.Rerun(string(item.ID), r.PathValue("investigation"), actor.UserID, revision, string(commit), in.Reason)
		writeInvestigation(w, v, err, 201)
	}
}

func investigationParticipant(v investigations.Investigation, actor string) bool {
	for _, id := range v.Participants {
		if id == actor {
			return true
		}
	}
	return false
}
func workspaceEvent(v workspaces.Workspace, sequence int64) bool {
	if sequence < 1 {
		return false
	}
	for _, e := range v.Activity {
		if e.Sequence == sequence {
			return true
		}
	}
	for _, e := range v.Events {
		if e.Sequence == sequence {
			return true
		}
	}
	return false
}
func validInvestigationPath(v string) bool {
	return v != "" && len(v) <= 1000 && path.Clean(v) == v && !strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "../") && !strings.ContainsRune(v, 0)
}
func blobAtPath(repo *storage.Repository, commitID storage.ObjectID, p string) (storage.ObjectID, error) {
	commit, err := repo.ReadCommit(commitID)
	if err != nil {
		return "", err
	}
	tree := commit.Tree
	parts := strings.Split(p, "/")
	for i, name := range parts {
		entries, err := repo.ReadTree(tree)
		if err != nil {
			return "", err
		}
		found := false
		for _, entry := range entries.Entries {
			if entry.Name != name {
				continue
			}
			found = true
			if i == len(parts)-1 && entry.Type == storage.BlobObject {
				return entry.ObjectID, nil
			}
			if entry.Type != storage.TreeObject {
				return "", errors.New("not a tree")
			}
			tree = entry.ObjectID
			break
		}
		if !found {
			return "", errors.New("not found")
		}
	}
	return "", errors.New("not a blob")
}
func writeInvestigation(w http.ResponseWriter, v investigations.Investigation, err error, status int) {
	if err == nil {
		writeJSON(w, status, v)
		return
	}
	if errors.Is(err, investigations.ErrNotFound) {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return
	}
	if errors.Is(err, investigations.ErrConflict) {
		writeJSON(w, 409, map[string]string{"error": "conflict"})
		return
	}
	writeJSON(w, 500, map[string]string{"error": "internal_error"})
}
