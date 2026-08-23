// Package workflowcomponents owns attested reusable workflow component publications
// and the local, revision-exact installations that consume them.
package workflowcomponents

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("workflow component not found")
var ErrInvalid = errors.New("invalid workflow component")
var ErrConflict = errors.New("workflow component changed")

type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}
type TestEvidence struct {
	Name        string `json:"name"`
	Revision    string `json:"revision"`
	Status      string `json:"status"`
	Attestation string `json:"attestation"`
}
type Compatibility struct {
	Engine         string   `json:"engine"`
	MinimumVersion string   `json:"minimum_version"`
	Breaking       bool     `json:"breaking"`
	Replaces       []string `json:"replaces,omitempty"`
}
type DataUse struct {
	Classes          []string `json:"classes"`
	Purposes         []string `json:"purposes"`
	Retention        string   `json:"retention"`
	ExternalTransfer bool     `json:"external_transfer"`
}
type Support struct {
	Policy  string     `json:"policy"`
	Contact string     `json:"contact"`
	Until   *time.Time `json:"until,omitempty"`
}
type PublishInput struct {
	Name                     string         `json:"name"`
	Version                  string         `json:"version"`
	Summary                  string         `json:"summary"`
	PackageVersionID         string         `json:"package_version_id"`
	SourceRepositoryID       string         `json:"source_repository_id"`
	SourceRevision           string         `json:"source_revision"`
	SourcePath               string         `json:"source_path"`
	ArtifactDigest           string         `json:"artifact_digest"`
	Attestation              string         `json:"attestation"`
	Inputs                   []Field        `json:"inputs"`
	Outputs                  []Field        `json:"outputs"`
	RequestedCapabilities    []string       `json:"requested_capabilities"`
	Compatibility            Compatibility  `json:"compatibility"`
	DataUse                  DataUse        `json:"data_use"`
	Tests                    []TestEvidence `json:"tests"`
	Support                  Support        `json:"support"`
	PublisherSubject         string         `json:"publisher_subject"`
	PublisherInstance        string         `json:"publisher_instance,omitempty"`
	FederationDocumentDigest string         `json:"federation_document_digest,omitempty"`
	Visibility               string         `json:"visibility"`
}
type Component struct {
	ID string `json:"id"`
	PublishInput
	PublishedBy string    `json:"published_by"`
	PublishedAt time.Time `json:"published_at"`
	Immutable   bool      `json:"immutable"`
}
type PermissionMapping struct {
	Requested       string `json:"requested"`
	LocalPermission string `json:"local_permission"`
	Resource        string `json:"resource"`
}
type Health struct {
	Publisher     string    `json:"publisher"`
	Trust         string    `json:"trust"`
	Peer          string    `json:"peer"`
	Vulnerability string    `json:"vulnerability"`
	Compatibility string    `json:"compatibility"`
	Blockers      []string  `json:"blockers"`
	ObservedAt    time.Time `json:"observed_at"`
}
type InstallInput struct {
	ComponentID   string              `json:"component_id"`
	PullRequestID string              `json:"pull_request_id"`
	PullRevision  string              `json:"pull_revision"`
	Configuration map[string]any      `json:"configuration"`
	Permissions   []PermissionMapping `json:"permissions"`
	Health        Health              `json:"health"`
	Reason        string              `json:"reason"`
}
type InstallationRevision struct {
	Number int64 `json:"number"`
	InstallInput
	Component Component `json:"component"`
	DecidedBy string    `json:"decided_by"`
	CreatedAt time.Time `json:"created_at"`
}
type Installation struct {
	ID              string                 `json:"id"`
	RepositoryID    string                 `json:"repository_id"`
	CurrentRevision int64                  `json:"current_revision"`
	Revisions       []InstallationRevision `json:"revisions"`
	State           string                 `json:"state"`
	Blockers        []string               `json:"blockers"`
	GrantsAuthority bool                   `json:"grants_authority"`
}
type Catalog struct {
	Items []Component `json:"items"`
}
type InstallationCatalog struct {
	Items []Installation `json:"items"`
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
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, now: time.Now}, e
}
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func safe(v any) bool {
	b, e := json.Marshal(v)
	if e != nil || len(b) > 64<<10 {
		return false
	}
	q := strings.ToLower(string(b))
	for _, x := range []string{"password", "private_key", "access_token", "authorization:", "secret="} {
		if strings.Contains(q, x) {
			return false
		}
	}
	return true
}
func validFields(fs []Field) bool {
	seen := map[string]bool{}
	for _, f := range fs {
		if f.Name == "" || seen[f.Name] || (f.Type != "string" && f.Type != "number" && f.Type != "boolean" && f.Type != "object" && f.Type != "array") {
			return false
		}
		seen[f.Name] = true
	}
	return true
}
func validPublish(x PublishInput) bool {
	if x.Name == "" || x.Version == "" || x.Summary == "" || x.PackageVersionID == "" || x.SourceRepositoryID == "" || x.SourceRevision == "" || x.SourcePath == "" || !strings.HasPrefix(x.ArtifactDigest, "sha256:") || x.Attestation == "" || x.PublisherSubject == "" || x.Visibility != "public" && x.Visibility != "repository" || !validFields(x.Inputs) || !validFields(x.Outputs) || len(x.RequestedCapabilities) == 0 || x.Compatibility.Engine == "" || x.Compatibility.MinimumVersion == "" || len(x.DataUse.Purposes) == 0 || x.DataUse.Retention == "" || len(x.Tests) == 0 || x.Support.Policy == "" || x.Support.Contact == "" {
		return false
	}
	for _, t := range x.Tests {
		if t.Name == "" || t.Revision != x.SourceRevision || t.Status != "passed" || t.Attestation == "" {
			return false
		}
	}
	return true
}
func (s *Store) componentPath(repo, c string) string {
	return filepath.Join(s.root, "components", repo, c+".json")
}
func (s *Store) installationPath(repo, i string) string {
	return filepath.Join(s.root, "installations", repo, i+".json")
}
func write(path string, v any) error {
	if e := os.MkdirAll(filepath.Dir(path), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e == nil {
		e = os.WriteFile(path, append(b, '\n'), 0640)
	}
	return e
}
func read(path string, v any) error {
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, v)
	}
	return e
}
func (s *Store) Publish(repo, actor string, in PublishInput) (Component, error) {
	if repo == "" || actor == "" || in.SourceRepositoryID != repo || !validPublish(in) {
		return Component{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items, _ := s.Components(repo)
	for _, x := range items.Items {
		if x.Name == in.Name && x.Version == in.Version {
			return Component{}, ErrConflict
		}
	}
	x := Component{ID: id(), PublishInput: in, PublishedBy: actor, PublishedAt: s.now().UTC(), Immutable: true}
	return x, write(s.componentPath(repo, x.ID), x)
}
func (s *Store) GetComponent(repo, c string) (Component, error) {
	var x Component
	e := read(s.componentPath(repo, c), &x)
	if e != nil || x.ID != c {
		return Component{}, ErrNotFound
	}
	return x, nil
}
func (s *Store) Components(repo string) (Catalog, error) {
	es, e := os.ReadDir(filepath.Join(s.root, "components", repo))
	if os.IsNotExist(e) {
		return Catalog{Items: []Component{}}, nil
	}
	if e != nil {
		return Catalog{}, e
	}
	out := []Component{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		var x Component
		if e = read(filepath.Join(s.root, "components", repo, f.Name()), &x); e != nil {
			return Catalog{}, e
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PublishedAt.Before(out[j].PublishedAt) })
	return Catalog{out}, nil
}
func deriveHealth(h Health) Health {
	h.Blockers = nil
	if h.Publisher != "unchanged" {
		h.Blockers = append(h.Blockers, "publisher_changed")
	}
	if h.Trust != "trusted" {
		h.Blockers = append(h.Blockers, "publisher_trust_"+h.Trust)
	}
	if h.Peer != "available" {
		h.Blockers = append(h.Blockers, "publisher_peer_"+h.Peer)
	}
	if h.Vulnerability != "clear" {
		h.Blockers = append(h.Blockers, "component_vulnerability_"+h.Vulnerability)
	}
	if h.Compatibility != "compatible" {
		h.Blockers = append(h.Blockers, "component_"+h.Compatibility)
	}
	return h
}
func validateInstall(c Component, in InstallInput) bool {
	if in.ComponentID != c.ID || in.PullRequestID == "" || in.PullRevision == "" || in.Reason == "" || !safe(in.Configuration) {
		return false
	}
	mapped := map[string]bool{}
	for _, m := range in.Permissions {
		if m.Requested == "" || m.LocalPermission == "" || m.Resource == "" {
			return false
		}
		mapped[m.Requested] = true
	}
	for _, cap := range c.RequestedCapabilities {
		if !mapped[cap] {
			return false
		}
	}
	return true
}
func refresh(i *Installation) {
	r := i.Revisions[len(i.Revisions)-1]
	i.Blockers = append([]string{}, r.Health.Blockers...)
	i.State = "installed"
	if len(i.Blockers) > 0 {
		i.State = "attention_required"
	}
	i.GrantsAuthority = false
}
func (s *Store) Install(repo, actor string, in InstallInput) (Installation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.findComponent(in.ComponentID)
	if e != nil || !validateInstall(c, in) {
		return Installation{}, ErrInvalid
	}
	in.Health = deriveHealth(in.Health)
	in.Health.ObservedAt = s.now().UTC()
	i := Installation{ID: id(), RepositoryID: repo, CurrentRevision: 1, Revisions: []InstallationRevision{{Number: 1, InstallInput: in, Component: c, DecidedBy: actor, CreatedAt: s.now().UTC()}}}
	refresh(&i)
	return i, write(s.installationPath(repo, i.ID), i)
}
func (s *Store) findComponent(cid string) (Component, error) {
	root := filepath.Join(s.root, "components")
	repos, _ := os.ReadDir(root)
	for _, r := range repos {
		if x, e := s.GetComponent(r.Name(), cid); e == nil {
			return x, nil
		}
	}
	return Component{}, ErrNotFound
}
func (s *Store) Revise(repo, iid, actor string, expected int64, in InstallInput) (Installation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var i Installation
	if e := read(s.installationPath(repo, iid), &i); e != nil {
		return i, e
	}
	if i.CurrentRevision != expected {
		return i, ErrConflict
	}
	c, e := s.findComponent(in.ComponentID)
	if e != nil || !validateInstall(c, in) {
		return i, ErrInvalid
	}
	in.Health = deriveHealth(in.Health)
	in.Health.ObservedAt = s.now().UTC()
	i.CurrentRevision++
	i.Revisions = append(i.Revisions, InstallationRevision{Number: i.CurrentRevision, InstallInput: in, Component: c, DecidedBy: actor, CreatedAt: s.now().UTC()})
	refresh(&i)
	return i, write(s.installationPath(repo, i.ID), i)
}
func (s *Store) Installations(repo string) (InstallationCatalog, error) {
	es, e := os.ReadDir(filepath.Join(s.root, "installations", repo))
	if os.IsNotExist(e) {
		return InstallationCatalog{Items: []Installation{}}, nil
	}
	if e != nil {
		return InstallationCatalog{}, e
	}
	out := []Installation{}
	for _, f := range es {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		var i Installation
		if e = read(filepath.Join(s.root, "installations", repo, f.Name()), &i); e != nil {
			return InstallationCatalog{}, e
		}
		refresh(&i)
		out = append(out, i)
	}
	return InstallationCatalog{out}, nil
}
