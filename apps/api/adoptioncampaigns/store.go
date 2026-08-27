// Package adoptioncampaigns persists versioned, release-exact adoption definitions.
package adoptioncampaigns

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

var ErrNotFound = errors.New("adoption campaign not found")
var ErrInvalid = errors.New("invalid adoption campaign")
var ErrConflict = errors.New("adoption campaign changed")

type Audience struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Kind            string  `json:"kind"`
	DesiredCoverage float64 `json:"desired_coverage"`
	OwnerID         string  `json:"owner_id"`
}
type StartingVersion struct {
	Version     string `json:"version"`
	Supported   bool   `json:"supported"`
	UpgradePath string `json:"upgrade_path,omitempty"`
	OwnerID     string `json:"owner_id"`
}
type Measure struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Target      float64 `json:"target"`
	Unit        string  `json:"unit"`
	Evidence    string  `json:"evidence"`
	OwnerID     string  `json:"owner_id"`
}
type Link struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Revision   string `json:"revision"`
	Summary    string `json:"summary"`
	OwnerID    string `json:"owner_id"`
}
type Compatibility struct {
	Subject          string   `json:"subject"`
	Requirement      string   `json:"requirement"`
	StartingVersions []string `json:"starting_versions"`
	OwnerID          string   `json:"owner_id"`
}
type VersionInput struct {
	ReleaseID        string            `json:"release_id"`
	ReleaseVersion   string            `json:"release_version"`
	ReleaseRevision  string            `json:"release_revision"`
	BundleID         string            `json:"provenance_bundle_id"`
	BundleDigest     string            `json:"provenance_bundle_digest"`
	Title            string            `json:"title"`
	Why              string            `json:"why"`
	Audiences        []Audience        `json:"target_audiences"`
	StartingVersions []StartingVersion `json:"supported_starting_versions"`
	Deadline         time.Time         `json:"deadline"`
	Measures         []Measure         `json:"success_measures"`
	SupportPolicy    string            `json:"support_policy"`
	RollbackPolicy   string            `json:"rollback_policy"`
	OwnerIDs         []string          `json:"owner_ids"`
	Links            []Link            `json:"links"`
	Compatibility    []Compatibility   `json:"compatibility_requirements"`
	ChangeReason     string            `json:"change_reason"`
}
type Version struct {
	Number int64 `json:"number"`
	VersionInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Finding struct {
	Kind      string `json:"kind"`
	Detail    string `json:"detail"`
	OwnerID   string `json:"owner_id,omitempty"`
	Reference string `json:"reference,omitempty"`
	Blocking  bool   `json:"blocking"`
}
type Campaign struct {
	ID               string    `json:"id"`
	RepositoryID     string    `json:"repository_id"`
	CurrentVersion   int64     `json:"current_version"`
	Versions         []Version `json:"versions"`
	Findings         []Finding `json:"findings"`
	AuthorityGranted bool      `json:"authority_granted"`
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
func newID() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func valid(in VersionInput) bool {
	if in.ReleaseID == "" || in.ReleaseVersion == "" || in.ReleaseRevision == "" || in.BundleID == "" || len(in.BundleDigest) != 64 || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Why) == "" || in.Deadline.IsZero() || len(in.Audiences) == 0 || len(in.StartingVersions) == 0 || len(in.Measures) == 0 || strings.TrimSpace(in.SupportPolicy) == "" || strings.TrimSpace(in.RollbackPolicy) == "" || len(in.OwnerIDs) == 0 || len(in.Links) == 0 || strings.TrimSpace(in.ChangeReason) == "" {
		return false
	}
	seen := map[string]bool{}
	for _, x := range in.Audiences {
		if x.ID == "" || seen[x.ID] || x.Name == "" || !one(x.Kind, "package_consumers", "api_consumers", "deployments", "forks", "federated_projects", "users") || x.DesiredCoverage <= 0 || x.DesiredCoverage > 100 {
			return false
		}
		seen[x.ID] = true
	}
	seen = map[string]bool{}
	for _, x := range in.StartingVersions {
		if x.Version == "" || seen[x.Version] {
			return false
		}
		seen[x.Version] = true
		if x.Supported && x.UpgradePath == "" {
			return false
		}
	}
	seen = map[string]bool{}
	for _, x := range in.Measures {
		if x.ID == "" || seen[x.ID] || x.Description == "" || x.Target < 0 || x.Unit == "" || x.Evidence == "" {
			return false
		}
		seen[x.ID] = true
	}
	for _, x := range in.Links {
		if !one(x.Kind, "change", "decision", "documentation", "package", "api", "schema") || x.ResourceID == "" || x.Revision == "" || x.Summary == "" {
			return false
		}
	}
	for _, x := range in.Compatibility {
		if x.Subject == "" || x.Requirement == "" || len(x.StartingVersions) == 0 {
			return false
		}
	}
	return true
}
func one(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func (s *Store) Create(repo, actor string, in VersionInput) (Campaign, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Campaign{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.publish(Campaign{ID: newID(), RepositoryID: repo}, actor, 0, in)
}
func (s *Store) Revise(repo, id, actor string, expected int64, in VersionInput) (Campaign, error) {
	if actor == "" || !valid(in) {
		return Campaign{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, e := s.read(repo, id)
	if e != nil {
		return c, e
	}
	first := c.Versions[0]
	if in.ReleaseID != first.ReleaseID || in.ReleaseRevision != first.ReleaseRevision || in.BundleID != first.BundleID || in.BundleDigest != first.BundleDigest {
		return c, ErrInvalid
	}
	return s.publish(c, actor, expected, in)
}
func (s *Store) publish(c Campaign, actor string, expected int64, in VersionInput) (Campaign, error) {
	if c.CurrentVersion != expected {
		return c, ErrConflict
	}
	c.CurrentVersion++
	c.Versions = append(c.Versions, Version{Number: c.CurrentVersion, VersionInput: in, AuthorID: actor, CreatedAt: s.now().UTC()})
	c.Findings = IntrinsicFindings(c)
	return c, s.write(c)
}
func IntrinsicFindings(c Campaign) []Finding {
	v := c.Versions[len(c.Versions)-1]
	out := []Finding{}
	owners := map[string]bool{}
	for _, x := range v.OwnerIDs {
		if strings.TrimSpace(x) != "" {
			owners[x] = true
		}
	}
	if len(owners) == 0 {
		out = append(out, Finding{Kind: "missing_owner", Detail: "campaign has no accountable owner", Blocking: true})
	}
	for _, x := range v.Audiences {
		if x.OwnerID == "" {
			out = append(out, Finding{Kind: "missing_owner", Detail: "target audience has no accountable owner", Reference: x.ID, Blocking: true})
		}
	}
	for _, x := range v.StartingVersions {
		if x.OwnerID == "" {
			out = append(out, Finding{Kind: "missing_owner", Detail: "starting version has no accountable owner", Reference: x.Version, Blocking: true})
		}
		if !x.Supported {
			out = append(out, Finding{Kind: "unsupported_upgrade_path", Detail: "starting version is explicitly unsupported", OwnerID: x.OwnerID, Reference: x.Version, Blocking: true})
		}
	}
	for _, x := range v.Measures {
		if x.OwnerID == "" {
			out = append(out, Finding{Kind: "missing_owner", Detail: "success measure has no accountable owner", Reference: x.ID, Blocking: true})
		}
	}
	for _, x := range v.Links {
		if x.OwnerID == "" {
			out = append(out, Finding{Kind: "missing_owner", Detail: "adopter context has no accountable owner", Reference: x.Kind + ":" + x.ResourceID, Blocking: true})
		}
	}
	for _, x := range v.Compatibility {
		if x.OwnerID == "" {
			out = append(out, Finding{Kind: "missing_owner", Detail: "compatibility requirement has no accountable owner", Reference: x.Subject, Blocking: true})
		}
	}
	if len(c.Versions) > 1 {
		p := c.Versions[len(c.Versions)-2]
		if commitments(p.VersionInput) != commitments(v.VersionInput) {
			out = append(out, Finding{Kind: "changed_commitment", Detail: "audience coverage, deadline, success, support, rollback, or compatibility commitments changed", OwnerID: v.AuthorID, Reference: "version:" + itoa(v.Number), Blocking: false})
		}
	}
	return out
}
func commitments(v VersionInput) string {
	b, _ := json.Marshal([]any{v.Audiences, v.StartingVersions, v.Deadline, v.Measures, v.SupportPolicy, v.RollbackPolicy, v.Compatibility})
	return string(b)
}
func itoa(n int64) string {
	const d = "0123456789"
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{d[n%10]}, b...)
		n /= 10
	}
	return string(b)
}
func (s *Store) Get(repo, id string) (Campaign, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Campaign, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	sort.Strings(files)
	out := []Campaign{}
	for _, f := range files {
		b, x := os.ReadFile(f)
		var c Campaign
		if x == nil {
			x = json.Unmarshal(b, &c)
		}
		if x != nil {
			return nil, x
		}
		out = append(out, c)
	}
	return out, e
}
func (s *Store) read(repo, id string) (Campaign, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Campaign{}, ErrNotFound
	}
	var c Campaign
	if e == nil {
		e = json.Unmarshal(b, &c)
	}
	if e == nil && (c.RepositoryID != repo || c.ID != id) {
		e = ErrNotFound
	}
	return c, e
}
func (s *Store) write(c Campaign) error {
	d := filepath.Join(s.root, c.RepositoryID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(d, ".campaign-*.tmp")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if e = tmp.Chmod(0600); e == nil {
		_, e = tmp.Write(append(b, '\n'))
	}
	if x := tmp.Close(); e == nil {
		e = x
	}
	if e == nil {
		e = os.Rename(name, filepath.Join(d, c.ID+".json"))
	}
	return e
}
