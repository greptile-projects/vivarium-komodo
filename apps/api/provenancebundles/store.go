// Package provenancebundles persists immutable signed release claims and append-only trust observations.
package provenancebundles

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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

var ErrInvalid = errors.New("invalid provenance bundle")
var ErrNotFound = errors.New("provenance bundle not found")
var ErrConflict = errors.New("provenance bundle already exists")

type Artifact struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SHA256     string `json:"sha256"`
	Size       int64  `json:"size"`
	MediaType  string `json:"media_type"`
	BuildRunID string `json:"build_run_id"`
}
type Component struct {
	Kind            string   `json:"kind"`
	Name            string   `json:"name"`
	Version         string   `json:"version,omitempty"`
	SHA256          string   `json:"sha256,omitempty"`
	License         string   `json:"license,omitempty"`
	Origin          string   `json:"origin"`
	Dependencies    []string `json:"dependencies"`
	Notices         []string `json:"notices"`
	SourceReference string   `json:"source_reference,omitempty"`
	AttestationIDs  []string `json:"attestation_ids"`
}
type Attestation struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	SubjectSHA256 string `json:"subject_sha256"`
	Issuer        string `json:"issuer"`
	Reference     string `json:"reference"`
}
type Omission struct {
	Subject string `json:"subject"`
	Reason  string `json:"reason"`
	Impact  string `json:"impact"`
}
type Verification struct {
	Algorithm     string   `json:"algorithm"`
	PublicKey     string   `json:"public_key"`
	PayloadSHA256 string   `json:"payload_sha256"`
	Instructions  []string `json:"instructions"`
}
type Notice struct {
	ID                string    `json:"id"`
	Kind              string    `json:"kind"`
	Subject           string    `json:"subject"`
	Detail            string    `json:"detail"`
	EvidenceReference string    `json:"evidence_reference"`
	Action            string    `json:"action"`
	CampaignID        string    `json:"campaign_id,omitempty"`
	CreatedByID       string    `json:"created_by_id"`
	CreatedAt         time.Time `json:"created_at"`
}
type Bundle struct {
	SchemaVersion      int           `json:"schema_version"`
	ID                 string        `json:"id"`
	RepositoryID       string        `json:"repository_id"`
	ReleaseID          string        `json:"release_id"`
	ReleaseVersion     string        `json:"release_version"`
	Revision           string        `json:"revision"`
	Audience           string        `json:"audience"`
	GraphID            string        `json:"graph_id"`
	AssessmentID       string        `json:"assessment_id"`
	PolicyVersion      int           `json:"policy_version"`
	Artifacts          []Artifact    `json:"artifacts"`
	Components         []Component   `json:"components"`
	Licenses           []string      `json:"licenses"`
	Notices            []string      `json:"notices"`
	SourceAttestations []Attestation `json:"source_attestations"`
	BuildAttestations  []Attestation `json:"build_attestations"`
	Omissions          []Omission    `json:"omissions"`
	Verification       Verification  `json:"verification"`
	Signature          string        `json:"signature"`
	PublishedByID      string        `json:"published_by_id"`
	PublishedAt        time.Time     `json:"published_at"`
	TrustStatus        string        `json:"trust_status"`
	TrustNotices       []Notice      `json:"trust_notices"`
	AuthorityGranted   bool          `json:"authority_granted"`
}
type PublishInput struct {
	RepositoryID, ReleaseID, ReleaseVersion, Revision, Audience, GraphID, AssessmentID, PublishedByID string
	PolicyVersion                                                                                     int
	Artifacts                                                                                         []Artifact
	Components                                                                                        []Component
	Licenses, Notices                                                                                 []string
	SourceAttestations, BuildAttestations                                                             []Attestation
	Omissions                                                                                         []Omission
}
type Store struct {
	root    string
	mu      sync.Mutex
	now     func() time.Time
	public  ed25519.PublicKey
	private ed25519.PrivateKey
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(a, 0750); e != nil {
		return nil, e
	}
	pub, priv, e := key(a)
	if e != nil {
		return nil, e
	}
	return &Store{root: a, now: time.Now, public: pub, private: priv}, nil
}
func key(root string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	p := filepath.Join(root, "signing.key")
	if b, e := os.ReadFile(p); e == nil {
		raw, x := base64.RawURLEncoding.DecodeString(strings.TrimSpace(string(b)))
		if x == nil && len(raw) == ed25519.PrivateKeySize {
			return raw[32:], raw, nil
		}
	}
	pub, priv, e := ed25519.GenerateKey(rand.Reader)
	if e == nil {
		e = os.WriteFile(p, []byte(base64.RawURLEncoding.EncodeToString(priv)+"\n"), 0600)
	}
	return pub, priv, e
}
func payload(v Bundle) []byte {
	v.Signature = ""
	v.TrustStatus = ""
	v.TrustNotices = nil
	v.AuthorityGranted = false
	v.Verification.PayloadSHA256 = ""
	b, _ := json.Marshal(v)
	return b
}
func (s *Store) Publish(in PublishInput) (Bundle, error) {
	if in.RepositoryID == "" || in.ReleaseID == "" || in.ReleaseVersion == "" || in.Revision == "" || in.PublishedByID == "" || in.GraphID == "" || in.AssessmentID == "" || in.PolicyVersion < 1 || len(in.Artifacts) == 0 || len(in.SourceAttestations) == 0 || len(in.BuildAttestations) == 0 || !one(in.Audience, "public", "repository") {
		return Bundle{}, ErrInvalid
	}
	attestations := map[string]bool{}
	for _, a := range append(append([]Attestation{}, in.SourceAttestations...), in.BuildAttestations...) {
		if a.ID == "" || a.Kind == "" || len(a.SubjectSHA256) != 64 || a.Issuer == "" || a.Reference == "" || attestations[a.ID] {
			return Bundle{}, ErrInvalid
		}
		attestations[a.ID] = true
	}
	buildSubjects := map[string]bool{}
	for _, a := range in.BuildAttestations {
		buildSubjects[a.SubjectSHA256] = true
	}
	for _, a := range in.Artifacts {
		if a.ID == "" || a.Name == "" || len(a.SHA256) != 64 || a.Size < 0 || a.BuildRunID == "" || !buildSubjects[a.SHA256] {
			return Bundle{}, ErrInvalid
		}
	}
	for _, c := range in.Components {
		if c.Kind == "" || c.Name == "" || c.Origin == "" || c.License == "" || len(c.AttestationIDs) == 0 {
			return Bundle{}, ErrInvalid
		}
		for _, id := range c.AttestationIDs {
			if !attestations[id] {
				return Bundle{}, ErrInvalid
			}
		}
	}
	for _, o := range in.Omissions {
		if o.Subject == "" || o.Reason == "" || o.Impact == "" {
			return Bundle{}, ErrInvalid
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, e := s.list()
	if e != nil {
		return Bundle{}, e
	}
	for _, x := range all {
		if x.RepositoryID == in.RepositoryID && x.ReleaseID == in.ReleaseID && x.Audience == in.Audience {
			return Bundle{}, ErrConflict
		}
	}
	idb := make([]byte, 16)
	if _, e = rand.Read(idb); e != nil {
		return Bundle{}, e
	}
	now := s.now().UTC()
	v := Bundle{SchemaVersion: 1, ID: hex.EncodeToString(idb), RepositoryID: in.RepositoryID, ReleaseID: in.ReleaseID, ReleaseVersion: in.ReleaseVersion, Revision: in.Revision, Audience: in.Audience, GraphID: in.GraphID, AssessmentID: in.AssessmentID, PolicyVersion: in.PolicyVersion, Artifacts: in.Artifacts, Components: in.Components, Licenses: clean(in.Licenses), Notices: clean(in.Notices), SourceAttestations: in.SourceAttestations, BuildAttestations: in.BuildAttestations, Omissions: in.Omissions, PublishedByID: in.PublishedByID, PublishedAt: now, TrustStatus: "current", TrustNotices: []Notice{}, Verification: Verification{Algorithm: "Ed25519", PublicKey: base64.RawURLEncoding.EncodeToString(s.public), Instructions: []string{"Download the JSON bundle.", "Remove signature, trust_status, trust_notices, authority_granted, and verification.payload_sha256; encode compact JSON in field order.", "Verify the Ed25519 signature with verification.public_key and compare every artifact SHA-256 before use."}}}
	p := payload(v)
	sum := sha256.Sum256(p)
	v.Verification.PayloadSHA256 = hex.EncodeToString(sum[:])
	v.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(s.private, p))
	if e = s.write(v); e != nil {
		return Bundle{}, e
	}
	return v, nil
}
func (s *Store) Observe(repo, id, actor string, n Notice) (Bundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.get(repo, id)
	if e != nil {
		return Bundle{}, e
	}
	if actor == "" || !one(n.Kind, "license_changed", "attestation_revoked", "package_quarantined", "provenance_drift", "origin_gap") || n.Subject == "" || n.Detail == "" || n.EvidenceReference == "" || n.Action == "" {
		return Bundle{}, ErrInvalid
	}
	b := make([]byte, 8)
	rand.Read(b)
	n.ID = hex.EncodeToString(b)
	n.CreatedByID = actor
	n.CreatedAt = s.now().UTC()
	v.TrustNotices = append(v.TrustNotices, n)
	v.TrustStatus = "attention_required"
	e = s.write(v)
	return v, e
}
func (s *Store) Get(repo, id string) (Bundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.get(repo, id)
}
func (s *Store) FindRelease(repo, release string) ([]Bundle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.list()
	o := []Bundle{}
	for _, v := range a {
		if v.RepositoryID == repo && v.ReleaseID == release {
			o = append(o, v)
		}
	}
	return o, e
}
func (s *Store) Verify(v Bundle) bool {
	sig, e := base64.RawURLEncoding.DecodeString(v.Signature)
	if e != nil {
		return false
	}
	pub, e := base64.RawURLEncoding.DecodeString(v.Verification.PublicKey)
	if e != nil {
		return false
	}
	p := payload(v)
	sum := sha256.Sum256(p)
	return hex.EncodeToString(sum[:]) == v.Verification.PayloadSHA256 && ed25519.Verify(pub, p, sig)
}
func (s *Store) get(repo, id string) (Bundle, error) {
	a, e := s.list()
	if e != nil {
		return Bundle{}, e
	}
	for _, v := range a {
		if v.ID == id && (repo == "" || v.RepositoryID == repo) {
			return v, nil
		}
	}
	return Bundle{}, ErrNotFound
}
func (s *Store) list() ([]Bundle, error) {
	es, e := os.ReadDir(s.root)
	if errors.Is(e, fs.ErrNotExist) {
		return []Bundle{}, nil
	}
	if e != nil {
		return nil, e
	}
	o := []Bundle{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		b, er := os.ReadFile(filepath.Join(s.root, x.Name()))
		var v Bundle
		if er != nil || json.Unmarshal(b, &v) != nil {
			return nil, ErrInvalid
		}
		o = append(o, v)
	}
	sort.Slice(o, func(i, j int) bool { return o[i].PublishedAt.After(o[j].PublishedAt) })
	return o, nil
}
func (s *Store) write(v Bundle) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(s.root, v.ID+".json"), append(b, '\n'), 0600)
}
func one(v string, a ...string) bool {
	for _, x := range a {
		if v == x {
			return true
		}
	}
	return false
}
func clean(a []string) []string {
	o := []string{}
	seen := map[string]bool{}
	for _, x := range a {
		x = strings.TrimSpace(x)
		if x != "" && !seen[x] {
			seen[x] = true
			o = append(o, x)
		}
	}
	return o
}
