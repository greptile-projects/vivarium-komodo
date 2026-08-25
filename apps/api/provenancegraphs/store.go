// Package provenancegraphs persists immutable, revision-exact software origin graphs.
package provenancegraphs

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

var ErrInvalid = errors.New("invalid provenance graph")
var ErrConflict = errors.New("provenance graph already exists")

type Citation struct {
	Path       string `json:"path,omitempty"`
	BlobSHA256 string `json:"blob_sha256,omitempty"`
	Reference  string `json:"reference,omitempty"`
	Revision   string `json:"revision,omitempty"`
}
type Node struct {
	ID             string     `json:"id"`
	Kind           string     `json:"kind"`
	Label          string     `json:"label"`
	Audience       string     `json:"audience"`
	License        string     `json:"license,omitempty"`
	Obligations    []string   `json:"obligations"`
	Transformation string     `json:"transformation,omitempty"`
	Confidence     float64    `json:"confidence"`
	Citations      []Citation `json:"citations"`
	Claims         []string   `json:"claims"`
}
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}
type Gap struct {
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Detail  string `json:"detail"`
}
type Graph struct {
	ID                string    `json:"id"`
	RepositoryID      string    `json:"repository_id"`
	Revision          string    `json:"revision"`
	DeclarationPath   string    `json:"declaration_path"`
	DeclarationSHA256 string    `json:"declaration_sha256"`
	Nodes             []Node    `json:"nodes"`
	Edges             []Edge    `json:"edges"`
	Gaps              []Gap     `json:"gaps"`
	Status            string    `json:"status"`
	CreatedByID       string    `json:"created_by_id"`
	CreatedAt         time.Time `json:"created_at"`
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
	if e != nil {
		return nil, e
	}
	return &Store{root: a, now: time.Now}, nil
}
func (s *Store) Create(v Graph) (Graph, error) {
	if v.RepositoryID == "" || v.Revision == "" || v.DeclarationSHA256 == "" || v.CreatedByID == "" || len(v.Nodes) == 0 {
		return Graph{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.list()
	if e != nil {
		return Graph{}, e
	}
	for _, x := range all {
		if x.RepositoryID == v.RepositoryID && x.Revision == v.Revision && x.DeclarationSHA256 == v.DeclarationSHA256 {
			return Graph{}, ErrConflict
		}
	}
	b := make([]byte, 16)
	if _, e = rand.Read(b); e != nil {
		return Graph{}, e
	}
	v.ID = hex.EncodeToString(b)
	v.CreatedAt = s.now().UTC()
	v.Status = "complete"
	if len(v.Gaps) > 0 {
		v.Status = "incomplete"
	}
	d, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return Graph{}, e
	}
	e = os.WriteFile(filepath.Join(s.root, v.ID+".json"), append(d, '\n'), 0600)
	return v, e
}
func (s *Store) List(repositoryID string) ([]Graph, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.list()
	o := []Graph{}
	for _, v := range a {
		if v.RepositoryID == repositoryID {
			o = append(o, v)
		}
	}
	return o, e
}
func (s *Store) list() ([]Graph, error) {
	es, e := os.ReadDir(s.root)
	if errors.Is(e, fs.ErrNotExist) {
		return []Graph{}, nil
	}
	if e != nil {
		return nil, e
	}
	o := []Graph{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		d, er := os.ReadFile(filepath.Join(s.root, x.Name()))
		var v Graph
		if er != nil || json.Unmarshal(d, &v) != nil {
			return nil, ErrInvalid
		}
		o = append(o, v)
	}
	sort.Slice(o, func(i, j int) bool { return o[i].CreatedAt.After(o[j].CreatedAt) })
	return o, nil
}
func Clean(v []string) []string {
	o := []string{}
	seen := map[string]bool{}
	for _, x := range v {
		x = strings.TrimSpace(x)
		if x != "" && !seen[x] {
			seen[x] = true
			o = append(o, x)
		}
	}
	return o
}
