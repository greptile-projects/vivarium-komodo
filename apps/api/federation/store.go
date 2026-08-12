// Package federation owns instance identity, signed discovery, and local peer trust.
package federation

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("federation resource not found")
var ErrInvalid = errors.New("invalid federation resource")
var ErrConflict = errors.New("federation version conflict")

type Operator struct {
	Name    string `json:"name"`
	Contact string `json:"contact"`
}
type Endpoints struct {
	Discovery string `json:"discovery"`
	Actors    string `json:"actors"`
	Events    string `json:"events,omitempty"`
}
type Key struct {
	ID        string     `json:"id"`
	Algorithm string     `json:"algorithm"`
	PublicKey string     `json:"public_key"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}
type Actor struct {
	Subject     string   `json:"subject"`
	Kind        string   `json:"kind"`
	LocalID     string   `json:"local_id"`
	DisplayName string   `json:"display_name"`
	ProfileURL  string   `json:"profile_url,omitempty"`
	Status      string   `json:"status"`
	KeyIDs      []string `json:"key_ids,omitempty"`
}
type Document struct {
	SchemaVersion  int        `json:"schema_version"`
	Instance       string     `json:"instance"`
	Version        int64      `json:"version"`
	IssuedAt       time.Time  `json:"issued_at"`
	Endpoints      Endpoints  `json:"endpoints"`
	Capabilities   []string   `json:"capabilities"`
	Operators      []Operator `json:"operators"`
	Keys           []Key      `json:"keys"`
	Actors         []Actor    `json:"actors"`
	PreviousDigest string     `json:"previous_digest,omitempty"`
	KeyID          string     `json:"key_id"`
	Signature      string     `json:"signature,omitempty"`
}
type Peer struct {
	Instance        string     `json:"instance"`
	DiscoveryURL    string     `json:"discovery_url"`
	Status          string     `json:"status"`
	Trust           string     `json:"trust"`
	LastDocument    *Document  `json:"document,omitempty"`
	LastDigest      string     `json:"last_digest,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	IdentityChanged bool       `json:"identity_changed"`
	FirstSeenAt     time.Time  `json:"first_seen_at"`
	LastCheckedAt   time.Time  `json:"last_checked_at"`
	TrustedAt       *time.Time `json:"trusted_at,omitempty"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
}
type Config struct {
	Instance     string     `json:"instance"`
	Operators    []Operator `json:"operators"`
	Capabilities []string   `json:"capabilities"`
	Endpoints    Endpoints  `json:"endpoints"`
}
type data struct {
	Config         Config    `json:"config"`
	Version        int64     `json:"version"`
	PreviousDigest string    `json:"previous_digest,omitempty"`
	PublicKey      string    `json:"public_key"`
	PrivateKey     string    `json:"private_key"`
	KeyID          string    `json:"key_id"`
	KeyCreatedAt   time.Time `json:"key_created_at"`
	IssuedAt       time.Time `json:"issued_at"`
	RetiredKeys    []Key     `json:"retired_keys,omitempty"`
	Actors         []Actor   `json:"actors"`
	Peers          []Peer    `json:"peers"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string, config Config) (*Store, error) {
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
	s := &Store{root: a, now: time.Now}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return nil, e
	}
	if d.PrivateKey == "" {
		if !validConfig(config) {
			return nil, ErrInvalid
		}
		pub, priv, _ := ed25519.GenerateKey(rand.Reader)
		now := s.now().UTC()
		d = data{Config: config, Version: 1, PublicKey: base64.RawURLEncoding.EncodeToString(pub), PrivateKey: base64.RawURLEncoding.EncodeToString(priv), KeyID: keyID(pub), KeyCreatedAt: now, IssuedAt: now, Actors: []Actor{}, Peers: []Peer{}}
		e = s.save(d)
	}
	return s, e
}
func validConfig(c Config) bool {
	u, e := url.Parse(c.Instance)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.Path == "" && len(c.Operators) > 0 && len(c.Capabilities) > 0 && strings.HasPrefix(c.Endpoints.Discovery, c.Instance)
}
func keyID(pub []byte) string { h := sha256.Sum256(pub); return "fed_" + hex.EncodeToString(h[:8]) }
func (s *Store) load() (data, error) {
	var d data
	b, e := os.ReadFile(filepath.Join(s.root, "federation.json"))
	if errors.Is(e, os.ErrNotExist) {
		return d, nil
	}
	if e == nil {
		e = json.Unmarshal(b, &d)
	}
	return d, e
}
func (s *Store) save(d data) error {
	b, e := json.MarshalIndent(d, "", "  ")
	if e != nil {
		return e
	}
	t, e := os.CreateTemp(s.root, "federation-*.tmp")
	if e != nil {
		return e
	}
	n := t.Name()
	defer os.Remove(n)
	_, e = t.Write(b)
	if e == nil {
		e = t.Sync()
	}
	if ce := t.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(n, filepath.Join(s.root, "federation.json"))
	}
	return e
}
func unsigned(doc Document) []byte { doc.Signature = ""; b, _ := json.Marshal(doc); return b }
func Digest(doc Document) string {
	h := sha256.Sum256(unsigned(doc))
	return "sha256:" + hex.EncodeToString(h[:])
}
func Verify(doc Document) error {
	if doc.SchemaVersion != 1 || doc.Instance == "" || doc.KeyID == "" || doc.Signature == "" {
		return ErrInvalid
	}
	var selected *Key
	for i := range doc.Keys {
		if doc.Keys[i].ID == doc.KeyID && doc.Keys[i].Status == "active" {
			selected = &doc.Keys[i]
		}
	}
	if selected == nil {
		return ErrInvalid
	}
	pub, e := base64.RawURLEncoding.DecodeString(selected.PublicKey)
	if e != nil || len(pub) != ed25519.PublicKeySize {
		return ErrInvalid
	}
	sig, e := base64.RawURLEncoding.DecodeString(doc.Signature)
	if e != nil || !ed25519.Verify(pub, unsigned(doc), sig) {
		return ErrInvalid
	}
	for _, a := range doc.Actors {
		if a.Subject != a.Kind+":"+a.LocalID+"@"+doc.Instance {
			return ErrInvalid
		}
	}
	return nil
}
func (s *Store) Document() (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Document{}, e
	}
	return signed(d, s.now().UTC())
}
func signed(d data, now time.Time) (Document, error) {
	if d.IssuedAt.IsZero() {
		d.IssuedAt = now
	}
	doc := Document{SchemaVersion: 1, Instance: d.Config.Instance, Version: d.Version, IssuedAt: d.IssuedAt, Endpoints: d.Config.Endpoints, Capabilities: d.Config.Capabilities, Operators: d.Config.Operators, Keys: append([]Key{{ID: d.KeyID, Algorithm: "Ed25519", PublicKey: d.PublicKey, Status: "active", CreatedAt: d.KeyCreatedAt}}, d.RetiredKeys...), Actors: d.Actors, PreviousDigest: d.PreviousDigest, KeyID: d.KeyID}
	priv, e := base64.RawURLEncoding.DecodeString(d.PrivateKey)
	if e != nil {
		return Document{}, e
	}
	doc.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, unsigned(doc)))
	return doc, nil
}
func (s *Store) PublishActor(kind, id, name, profile string) (Actor, error) {
	if !oneOf(kind, "user", "agent", "installation") || strings.TrimSpace(id) == "" || strings.ContainsAny(id, "@:") || name == "" {
		return Actor{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Actor{}, e
	}
	for _, a := range d.Actors {
		if a.Kind == kind && a.LocalID == id {
			return Actor{}, ErrConflict
		}
	}
	a := Actor{Kind: kind, LocalID: id, Subject: kind + ":" + id + "@" + d.Config.Instance, DisplayName: name, ProfileURL: profile, Status: "active", KeyIDs: []string{d.KeyID}}
	old, _ := signed(d, s.now().UTC())
	d.PreviousDigest = Digest(old)
	d.Version++
	d.IssuedAt = s.now().UTC()
	d.Actors = append(d.Actors, a)
	return a, s.save(d)
}
func (s *Store) Rotate() (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Document{}, e
	}
	old, _ := signed(d, s.now().UTC())
	now := s.now().UTC()
	revoked := now
	d.RetiredKeys = append(d.RetiredKeys, Key{ID: d.KeyID, Algorithm: "Ed25519", PublicKey: d.PublicKey, Status: "retired", CreatedAt: d.KeyCreatedAt, RevokedAt: &revoked})
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	d.PublicKey = base64.RawURLEncoding.EncodeToString(pub)
	d.PrivateKey = base64.RawURLEncoding.EncodeToString(priv)
	d.KeyID = keyID(pub)
	d.KeyCreatedAt = now
	d.IssuedAt = now
	d.PreviousDigest = Digest(old)
	d.Version++
	for i := range d.Actors {
		d.Actors[i].KeyIDs = []string{d.KeyID}
	}
	if e = s.save(d); e != nil {
		return Document{}, e
	}
	return signed(d, now)
}
func (s *Store) Observe(discovery string, doc Document, fetchErr error) (Peer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Peer{}, e
	}
	now := s.now().UTC()
	idx := -1
	for i := range d.Peers {
		if d.Peers[i].DiscoveryURL == discovery {
			idx = i
		}
	}
	p := Peer{DiscoveryURL: discovery, Status: "unreachable", Trust: "untrusted", FirstSeenAt: now, LastCheckedAt: now}
	if idx >= 0 {
		p = d.Peers[idx]
		p.LastCheckedAt = now
	}
	if fetchErr != nil {
		p.Status = "unreachable"
		p.LastError = fetchErr.Error()
	} else if e = Verify(doc); e != nil {
		return Peer{}, e
	} else {
		p.Instance = doc.Instance
		p.Status = "reachable"
		p.LastError = ""
		digest := Digest(doc)
		p.IdentityChanged = p.LastDigest != "" && p.LastDigest != digest && doc.PreviousDigest != p.LastDigest
		p.LastDigest = digest
		p.LastDocument = &doc
	}
	if idx < 0 {
		d.Peers = append(d.Peers, p)
	} else {
		d.Peers[idx] = p
	}
	return p, s.save(d)
}
func (s *Store) Trust(instance, action string) (Peer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return Peer{}, e
	}
	for i := range d.Peers {
		p := &d.Peers[i]
		if p.Instance == instance {
			now := s.now().UTC()
			switch action {
			case "trust":
				if p.Status != "reachable" || p.IdentityChanged {
					return Peer{}, ErrConflict
				}
				p.Trust = "trusted"
				p.TrustedAt = &now
				p.RevokedAt = nil
			case "revoke":
				p.Trust = "revoked"
				p.RevokedAt = &now
			default:
				return Peer{}, ErrInvalid
			}
			return *p, s.save(d)
		}
	}
	return Peer{}, ErrNotFound
}
func (s *Store) Peers() ([]Peer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	return d.Peers, e
}
func oneOf(v string, xs ...string) bool {
	for _, x := range xs {
		if v == x {
			return true
		}
	}
	return false
}
