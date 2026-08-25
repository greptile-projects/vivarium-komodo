package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/provenancegraphs"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func registerProvenanceGraphsHTTP(mux *http.ServeMux, s *provenancegraphs.Store, repos pullRequestRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories/{repository}/provenance-graphs", createProvenanceGraph(s, repos, credentials))
	mux.HandleFunc("GET /repositories/{repository}/provenance-graphs", listProvenanceGraphs(s, repos, credentials))
}
func createProvenanceGraph(s *provenancegraphs.Store, repos pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			Revision        string                  `json:"revision"`
			DeclarationPath string                  `json:"declaration_path"`
			Nodes           []provenancegraphs.Node `json:"nodes"`
			Edges           []provenancegraphs.Edge `json:"edges"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		opened, e := repos.Open(repo.ID)
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		commit, e := opened.ReadCommit(storage.ObjectID(in.Revision))
		if e != nil || !visibleCommit(opened, commit.ID) {
			writeJSON(w, 422, map[string]string{"error": "revision_not_visible"})
			return
		}
		if in.DeclarationPath == "" {
			in.DeclarationPath = ".komodo/provenance.json"
		}
		decl, e := relationshipBlob(repos, string(repo.ID), in.Revision, in.DeclarationPath)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "provenance_declaration_missing"})
			return
		}
		var declared struct {
			Nodes []provenancegraphs.Node `json:"nodes"`
			Edges []provenancegraphs.Edge `json:"edges"`
		}
		if json.Unmarshal(decl, &declared) != nil || len(declared.Nodes) == 0 {
			writeJSON(w, 422, map[string]string{"error": "invalid_provenance_declaration"})
			return
		}
		if len(in.Nodes) == 0 {
			in.Nodes = declared.Nodes
		}
		if len(in.Edges) == 0 {
			in.Edges = declared.Edges
		}
		gaps := validateProvenance(opened, commit.ID, in.Nodes, in.Edges)
		sum := sha256.Sum256(decl)
		v, e := s.Create(provenancegraphs.Graph{RepositoryID: string(repo.ID), Revision: in.Revision, DeclarationPath: in.DeclarationPath, DeclarationSHA256: hex.EncodeToString(sum[:]), Nodes: in.Nodes, Edges: in.Edges, Gaps: gaps, CreatedByID: actor.UserID})
		if errors.Is(e, provenancegraphs.ErrConflict) {
			writeJSON(w, 409, map[string]string{"error": "analysis_exists"})
		} else if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_provenance_graph"})
		} else {
			writeJSON(w, 201, projectGraph(v, true, false))
		}
	}
}
func validateProvenance(repo *storage.Repository, revision storage.ObjectID, nodes []provenancegraphs.Node, edges []provenancegraphs.Edge) []provenancegraphs.Gap {
	g := []provenancegraphs.Gap{}
	ids := map[string]bool{}
	claims := map[string]string{}
	claimNodes := map[string]string{}
	for i := range nodes {
		n := &nodes[i]
		n.ID = strings.TrimSpace(n.ID)
		n.Kind = strings.TrimSpace(n.Kind)
		n.Label = strings.TrimSpace(n.Label)
		if n.Audience == "" {
			n.Audience = "repository"
		}
		n.Obligations = provenancegraphs.Clean(n.Obligations)
		n.Claims = provenancegraphs.Clean(n.Claims)
		if n.ID == "" || n.Kind == "" || n.Label == "" || ids[n.ID] || !map[string]bool{"public": true, "repository": true, "restricted": true}[n.Audience] || n.Confidence < 0 || n.Confidence > 1 {
			g = append(g, provenancegraphs.Gap{Kind: "invalid_claim", Subject: n.ID, Detail: "node identity, kind, audience, or confidence is invalid"})
			continue
		}
		ids[n.ID] = true
		if len(n.Citations) == 0 {
			g = append(g, provenancegraphs.Gap{Kind: "missing_origin", Subject: n.ID, Detail: "no exact citation supports this claim"})
		}
		for _, c := range n.Citations {
			if c.Path != "" {
				body, e := blobAt(repo, revision, c.Path)
				sum := sha256.Sum256(body)
				if e != nil || hex.EncodeToString(sum[:]) != c.BlobSHA256 {
					g = append(g, provenancegraphs.Gap{Kind: "stale_citation", Subject: n.ID, Detail: "cited blob is absent or digest does not match the exact revision"})
				}
			}
		}
		for _, c := range n.Claims {
			parts := strings.SplitN(c, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key, value := strings.ToLower(strings.TrimSpace(parts[0])), strings.ToLower(strings.TrimSpace(parts[1]))
			if prior, ok := claims[key]; ok && prior != value {
				g = append(g, provenancegraphs.Gap{Kind: "contradictory_claim", Subject: n.ID, Detail: "claim conflicts with " + claimNodes[key] + " on " + key})
			}
			claims[key], claimNodes[key] = value, n.ID
		}
	}
	for _, e := range edges {
		if !ids[e.From] || !ids[e.To] || e.Kind == "" {
			g = append(g, provenancegraphs.Gap{Kind: "broken_lineage", Subject: e.From + "->" + e.To, Detail: "edge endpoint or transformation kind is missing"})
		}
	}
	return g
}
func blobAt(repo *storage.Repository, revision storage.ObjectID, path string) ([]byte, error) {
	c, e := repo.ReadCommit(revision)
	if e != nil {
		return nil, e
	}
	tree := c.Tree
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, p := range parts {
		t, e := repo.ReadTree(tree)
		if e != nil {
			return nil, e
		}
		found := false
		for _, x := range t.Entries {
			if x.Name != p {
				continue
			}
			found = true
			if i == len(parts)-1 {
				if x.Type != storage.BlobObject {
					return nil, errors.New("not blob")
				}
				o, e := repo.ReadObject(x.ObjectID)
				if e != nil {
					return nil, e
				}
				return o.Content, nil
			}
			if x.Type != storage.TreeObject {
				return nil, errors.New("not tree")
			}
			tree = x.ObjectID
		}
		if !found {
			return nil, errors.New("missing")
		}
	}
	return nil, errors.New("missing")
}
func listProvenanceGraphs(s *provenancegraphs.Store, repos pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, e := s.List(string(repo.ID))
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		opened, _ := repos.Open(repo.ID)
		out := make([]provenancegraphs.Graph, 0, len(items))
		for _, v := range items {
			current := false
			if c, e := opened.ReadCommit(storage.ObjectID(v.Revision)); e == nil {
				current = visibleCommit(opened, c.ID)
			}
			out = append(out, projectGraph(v, actor.UserID != "", !current))
		}
		writeJSON(w, 200, map[string]any{"items": out, "total_count": len(out)})
	}
}
func projectGraph(v provenancegraphs.Graph, reader bool, rewritten bool) provenancegraphs.Graph {
	if rewritten {
		v.Status = "stale"
		v.Gaps = append(v.Gaps, provenancegraphs.Gap{Kind: "rewritten_history", Subject: v.Revision, Detail: "the analyzed revision is no longer reachable from a visible reference"})
	}
	if reader {
		return v
	}
	for i := range v.Nodes {
		if v.Nodes[i].Audience != "public" {
			v.Nodes[i] = provenancegraphs.Node{ID: v.Nodes[i].ID, Kind: "restricted", Label: "inaccessible provenance node", Audience: v.Nodes[i].Audience, Obligations: []string{}, Citations: []provenancegraphs.Citation{}, Claims: []string{}}
		}
	}
	return v
}
