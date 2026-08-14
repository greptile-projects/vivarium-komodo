// Package dataflows retains revision-exact declarations and bounded observations of governed data movement.
package dataflows

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("data flow not found")
var ErrInvalid = errors.New("invalid data flow")

type Location struct {
	Path      string `json:"path"`
	BlobID    string `json:"blob_id"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
}
type CommitmentRef struct {
	ID         string   `json:"id"`
	Version    int64    `json:"version"`
	DataUseIDs []string `json:"data_use_ids"`
}
type Node struct {
	ID                    string    `json:"id"`
	Kind                  string    `json:"kind"`
	Name                  string    `json:"name"`
	ResourceID            string    `json:"resource_id,omitempty"`
	Location              *Location `json:"location,omitempty"`
	EvidenceAccessible    bool      `json:"evidence_accessible"`
	RestrictedEvidenceRef string    `json:"restricted_evidence_ref,omitempty"`
}
type Edge struct {
	ID         string   `json:"id"`
	From       string   `json:"from"`
	To         string   `json:"to"`
	Action     string   `json:"action"`
	Categories []string `json:"categories"`
	Purpose    string   `json:"purpose"`
}
type DeclarationInput struct {
	Revision    string          `json:"revision"`
	Title       string          `json:"title"`
	Manifest    Location        `json:"manifest"`
	Commitments []CommitmentRef `json:"commitments"`
	Nodes       []Node          `json:"nodes"`
	Edges       []Edge          `json:"edges"`
}
type Citation struct {
	Kind                  string    `json:"kind"`
	ResourceID            string    `json:"resource_id,omitempty"`
	Location              *Location `json:"location,omitempty"`
	EvidenceAccessible    bool      `json:"evidence_accessible"`
	RestrictedEvidenceRef string    `json:"restricted_evidence_ref,omitempty"`
}
type FindingInput struct {
	Summary       string     `json:"summary"`
	Uncertainty   string     `json:"uncertainty,omitempty"`
	ObservedEdges []Edge     `json:"observed_edges"`
	Citations     []Citation `json:"citations"`
}
type Finding struct {
	ID string `json:"id"`
	FindingInput
	ActorID      string    `json:"actor_id"`
	CreatedAt    time.Time `json:"created_at"`
	Stale        bool      `json:"stale"`
	StaleReasons []string  `json:"stale_reasons"`
}
type Blocker struct {
	Kind      string `json:"kind"`
	NodeID    string `json:"node_id,omitempty"`
	EdgeID    string `json:"edge_id,omitempty"`
	FindingID string `json:"finding_id,omitempty"`
	Detail    string `json:"detail"`
}
type Flow struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	DeclarationInput
	CreatedByID string    `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
	Findings    []Finding `json:"findings"`
	Stale       bool      `json:"stale"`
	Blockers    []Blocker `json:"blockers"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func id() string                { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func text(s string, n int) bool { return strings.TrimSpace(s) != "" && len(s) <= n }
func list(xs []string) bool {
	if len(xs) == 0 || len(xs) > 100 {
		return false
	}
	seen := map[string]bool{}
	for _, x := range xs {
		if !text(x, 500) || seen[x] {
			return false
		}
		seen[x] = true
	}
	return true
}
func location(x Location) bool {
	return text(x.Path, 500) && text(x.BlobID, 100) && x.StartLine >= 0 && x.EndLine >= x.StartLine
}

var nodeKinds = map[string]bool{"interaction": true, "interface": true, "package": true, "store": true, "extension": true, "release": true, "environment": true, "audience": true, "external_recipient": true}
var actions = map[string]bool{"enters": true, "moves": true, "persists": true, "leaves": true}

func validEdge(x Edge, nodes map[string]bool) bool {
	return text(x.ID, 100) && nodes[x.From] && nodes[x.To] && actions[x.Action] && list(x.Categories) && text(x.Purpose, 2000)
}
func validDeclaration(in DeclarationInput) bool {
	if !text(in.Revision, 100) || !text(in.Title, 500) || !location(in.Manifest) || len(in.Commitments) == 0 || len(in.Commitments) > 100 || len(in.Nodes) < 2 || len(in.Nodes) > 500 || len(in.Edges) == 0 || len(in.Edges) > 1000 {
		return false
	}
	for _, c := range in.Commitments {
		if !text(c.ID, 100) || c.Version < 1 || !list(c.DataUseIDs) {
			return false
		}
	}
	nodes := map[string]bool{}
	for _, n := range in.Nodes {
		if !text(n.ID, 100) || nodes[n.ID] || !nodeKinds[n.Kind] || !text(n.Name, 500) || (n.Location != nil && !location(*n.Location)) || (n.EvidenceAccessible && n.RestrictedEvidenceRef != "") || (!n.EvidenceAccessible && (!text(n.RestrictedEvidenceRef, 500) || n.Location != nil)) {
			return false
		}
		nodes[n.ID] = true
	}
	edges := map[string]bool{}
	for _, e := range in.Edges {
		if edges[e.ID] || !validEdge(e, nodes) {
			return false
		}
		edges[e.ID] = true
	}
	return true
}
func validFinding(f Flow, in FindingInput) bool {
	if !text(in.Summary, 65536) || len(in.Uncertainty) > 65536 || len(in.ObservedEdges) > 1000 || len(in.Citations) == 0 || len(in.Citations) > 100 {
		return false
	}
	nodes := map[string]bool{}
	for _, n := range f.Nodes {
		nodes[n.ID] = true
	}
	for _, e := range in.ObservedEdges {
		if !validEdge(e, nodes) {
			return false
		}
	}
	for _, c := range in.Citations {
		if !map[string]bool{"code": true, "interface": true, "runtime": true, "dependency": true}[c.Kind] || (c.Location == nil && c.ResourceID == "") || (c.Location != nil && !location(*c.Location)) || (c.EvidenceAccessible && c.RestrictedEvidenceRef != "") || (!c.EvidenceAccessible && (!text(c.RestrictedEvidenceRef, 500) || c.Location != nil)) {
			return false
		}
	}
	return true
}
func (s *Store) path(repo, fid string) string { return filepath.Join(s.root, repo, fid+".json") }
func (s *Store) write(f Flow) error {
	if e := os.MkdirAll(filepath.Dir(s.path(f.RepositoryID, f.ID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(f, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(f.RepositoryID, f.ID), b, 0640)
	}
	return e
}
func (s *Store) read(repo, fid string) (Flow, error) {
	var f Flow
	b, e := os.ReadFile(s.path(repo, fid))
	if errors.Is(e, fs.ErrNotExist) {
		return f, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &f)
	}
	if e != nil || f.ID != fid || f.RepositoryID != repo {
		return Flow{}, ErrNotFound
	}
	return f, nil
}
func (s *Store) Create(repo, actor string, in DeclarationInput) (Flow, error) {
	if repo == "" || actor == "" || !validDeclaration(in) {
		return Flow{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f := Flow{ID: id(), RepositoryID: repo, DeclarationInput: in, CreatedByID: actor, CreatedAt: s.now().UTC(), Findings: []Finding{}}
	return f, s.write(f)
}
func (s *Store) AddFinding(repo, fid, actor string, in FindingInput) (Flow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, e := s.read(repo, fid)
	if e != nil {
		return f, e
	}
	if actor == "" || !validFinding(f, in) {
		return f, ErrInvalid
	}
	f.Findings = append(f.Findings, Finding{ID: id(), FindingInput: in, ActorID: actor, CreatedAt: s.now().UTC()})
	e = s.write(f)
	projected := []Flow{f}
	derive(projected)
	return projected[0], e
}
func (s *Store) List(repo string) ([]Flow, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Flow{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Flow{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		f, e := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, f)
	}
	derive(out)
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Get(repo, fid string) (Flow, error) {
	all, e := s.List(repo)
	if e != nil {
		return Flow{}, e
	}
	for _, f := range all {
		if f.ID == fid {
			return f, nil
		}
	}
	return Flow{}, ErrNotFound
}
func derive(all []Flow) {
	latest := map[string]time.Time{}
	for _, f := range all {
		if f.CreatedAt.After(latest[f.Title]) {
			latest[f.Title] = f.CreatedAt
		}
	}
	for i := range all {
		f := &all[i]
		f.Blockers = nil
		f.Stale = f.CreatedAt.Before(latest[f.Title])
		if f.Stale {
			f.Blockers = append(f.Blockers, Blocker{Kind: "stale_analysis", Detail: "a newer revision-exact declaration exists for this flow"})
		}
		for _, n := range f.Nodes {
			if !n.EvidenceAccessible {
				f.Blockers = append(f.Blockers, Blocker{Kind: "inaccessible_dependency", NodeID: n.ID, Detail: "dependency evidence is not accessible; only its bounded reference is retained"})
			}
		}
		declared := map[string]Edge{}
		for _, e := range f.Edges {
			declared[e.From+"\x00"+e.To+"\x00"+e.Action] = e
		}
		observed := map[string]bool{}
		for j := range f.Findings {
			finding := &f.Findings[j]
			finding.Stale = f.Stale
			finding.StaleReasons = nil
			if finding.Stale {
				finding.StaleReasons = append(finding.StaleReasons, "newer declaration revision")
			}
			for _, e := range finding.ObservedEdges {
				k := e.From + "\x00" + e.To + "\x00" + e.Action
				observed[k] = true
				if d, ok := declared[k]; !ok {
					f.Blockers = append(f.Blockers, Blocker{Kind: "undeclared_flow", EdgeID: e.ID, FindingID: finding.ID, Detail: "observed movement is absent from the declaration"})
				} else if strings.Join(d.Categories, "\x00") != strings.Join(e.Categories, "\x00") || d.Purpose != e.Purpose {
					f.Blockers = append(f.Blockers, Blocker{Kind: "declared_observed_difference", EdgeID: e.ID, FindingID: finding.ID, Detail: "observed categories or purpose differ from the declaration"})
				}
			}
			for _, c := range finding.Citations {
				if !c.EvidenceAccessible {
					f.Blockers = append(f.Blockers, Blocker{Kind: "inaccessible_dependency", FindingID: finding.ID, Detail: "finding cites inaccessible evidence by bounded reference only"})
				}
			}
		}
		if len(f.Findings) > 0 {
			for k, e := range declared {
				if !observed[k] {
					f.Blockers = append(f.Blockers, Blocker{Kind: "declared_observed_difference", EdgeID: e.ID, Detail: "declared movement was not observed by current analysis"})
				}
			}
		}
	}
}
