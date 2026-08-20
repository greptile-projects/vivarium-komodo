// Package assuranceevidence assembles permission-aware evidence for exact assurance controls.
package assuranceevidence

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/assuranceprograms"
)

var ErrNotFound = errors.New("assurance evidence not found")
var ErrInvalid = errors.New("invalid assurance evidence")
var ErrConflict = errors.New("assurance evidence conflict")

var kinds = map[string]bool{"review": true, "check": true, "access": true, "dependency": true, "build": true, "release": true, "deployment": true, "incident": true, "continuity": true, "security": true, "privacy": true, "governance": true}
var audiences = map[string]bool{"public": true, "repository": true, "owners": true}

type QueryInput struct {
	ControlVersion  int64             `json:"control_version"`
	ControlID       string            `json:"control_id"`
	Name            string            `json:"name"`
	Kind            string            `json:"kind"`
	Source          string            `json:"source"`
	Selector        map[string]string `json:"selector"`
	Schedule        string            `json:"schedule"`
	FreshnessHours  int               `json:"freshness_hours"`
	Audience        string            `json:"audience"`
	Transformations []string          `json:"transformations"`
}
type Query struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	ProgramID    string `json:"program_id"`
	QueryInput
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Record is deliberately metadata-only. Source content, credentials, and personal data are never accepted.
type Record struct {
	QueryID              string    `json:"query_id"`
	SourceRecordID       string    `json:"source_record_id"`
	SourceRevision       string    `json:"source_revision"`
	ObservedAt           time.Time `json:"observed_at"`
	SourceDigest         string    `json:"source_digest"`
	SourceAttestation    string    `json:"source_attestation"`
	Audience             string    `json:"audience"`
	Accessible           bool      `json:"accessible"`
	Embargoed            bool      `json:"embargoed"`
	ContainsCredentials  bool      `json:"contains_credentials"`
	ContainsPersonalData bool      `json:"contains_personal_data"`
	Result               string    `json:"result"`
	ContradictsRecordID  string    `json:"contradicts_record_id,omitempty"`
}
type CollectInput struct {
	ControlVersion int64     `json:"control_version"`
	ControlID      string    `json:"control_id"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	Records        []Record  `json:"records"`
}
type Gap struct {
	Kind    string `json:"kind"`
	QueryID string `json:"query_id,omitempty"`
	Detail  string `json:"detail"`
}
type Package struct {
	ID             string            `json:"id"`
	RepositoryID   string            `json:"repository_id"`
	ProgramID      string            `json:"program_id"`
	ControlVersion int64             `json:"control_version"`
	ControlID      string            `json:"control_id"`
	PeriodStart    time.Time         `json:"period_start"`
	PeriodEnd      time.Time         `json:"period_end"`
	CollectedAt    time.Time         `json:"collected_at"`
	CollectorID    string            `json:"collector_id"`
	Records        []Record          `json:"records"`
	Coverage       map[string]string `json:"coverage"`
	Gaps           []Gap             `json:"gaps"`
	Fresh          bool              `json:"fresh"`
	PackageHash    string            `json:"package_hash"`
	Attestation    string            `json:"attestation"`
	Immutable      bool              `json:"immutable"`
}
type Catalog struct {
	Queries  []Query   `json:"queries"`
	Packages []Package `json:"packages"`
}
type programStore interface {
	Get(string, string) (assuranceprograms.Program, error)
}
type Store struct {
	root     string
	programs programStore
	mu       sync.Mutex
	now      func() time.Time
}

func New(root string, programs programStore) (*Store, error) {
	if strings.TrimSpace(root) == "" || programs == nil {
		return nil, ErrInvalid
	}
	p, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(p, 0750)
	}
	return &Store{root: p, programs: programs, now: time.Now}, e
}
func newID() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, program string) string {
	return filepath.Join(s.root, repo, program+".json")
}
func validControl(p assuranceprograms.Program, version int64, control string) bool {
	for _, v := range p.Versions {
		if v.Number == version {
			for _, c := range v.Controls {
				if c.ID == control {
					return true
				}
			}
		}
	}
	return false
}
func ownsControl(p assuranceprograms.Program, version int64, control, actor string) bool {
	for _, v := range p.Versions {
		if v.Number == version {
			for _, c := range v.Controls {
				if c.ID == control {
					for _, owner := range c.OwnerIDs {
						if owner == actor {
							return true
						}
					}
				}
			}
		}
	}
	return false
}
func validQuery(in QueryInput) bool {
	return in.ControlVersion > 0 && strings.TrimSpace(in.ControlID) != "" && strings.TrimSpace(in.Name) != "" && kinds[in.Kind] && strings.TrimSpace(in.Source) != "" && strings.TrimSpace(in.Schedule) != "" && in.FreshnessHours > 0 && audiences[in.Audience]
}
func (s *Store) read(repo, program string) (Catalog, error) {
	b, e := os.ReadFile(s.path(repo, program))
	if errors.Is(e, os.ErrNotExist) {
		return Catalog{Queries: []Query{}, Packages: []Package{}}, nil
	}
	var x Catalog
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) save(repo, program string, x Catalog) error {
	if e := os.MkdirAll(filepath.Dir(s.path(repo, program)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(repo, program), append(b, '\n'), 0640)
	}
	return e
}
func (s *Store) CreateQuery(repo, program, actor string, in QueryInput) (Query, error) {
	if !validQuery(in) || strings.TrimSpace(actor) == "" {
		return Query{}, ErrInvalid
	}
	p, e := s.programs.Get(repo, program)
	if e != nil {
		return Query{}, ErrNotFound
	}
	if !validControl(p, in.ControlVersion, in.ControlID) {
		return Query{}, ErrInvalid
	}
	if !ownsControl(p, in.ControlVersion, in.ControlID, actor) {
		return Query{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, program)
	if e != nil {
		return Query{}, e
	}
	for _, q := range x.Queries {
		if q.ControlVersion == in.ControlVersion && q.ControlID == in.ControlID && strings.EqualFold(q.Name, in.Name) {
			return Query{}, ErrConflict
		}
	}
	q := Query{newID(), repo, program, in, actor, s.now().UTC()}
	x.Queries = append(x.Queries, q)
	return q, s.save(repo, program, x)
}
func validRecord(r Record) bool {
	return strings.TrimSpace(r.QueryID) != "" && strings.TrimSpace(r.SourceRecordID) != "" && strings.TrimSpace(r.SourceRevision) != "" && !r.ObservedAt.IsZero() && len(r.SourceDigest) == 64 && strings.TrimSpace(r.SourceAttestation) != "" && audiences[r.Audience] && strings.TrimSpace(r.Result) != ""
}
func digestPackage(x Package) string {
	y := x
	y.PackageHash = ""
	y.Attestation = ""
	b, _ := json.Marshal(y)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s *Store) Collect(repo, program, actor string, in CollectInput) (Package, error) {
	if strings.TrimSpace(actor) == "" || in.ControlVersion < 1 || strings.TrimSpace(in.ControlID) == "" || in.PeriodStart.IsZero() || !in.PeriodEnd.After(in.PeriodStart) {
		return Package{}, ErrInvalid
	}
	p, e := s.programs.Get(repo, program)
	if e != nil {
		return Package{}, ErrNotFound
	}
	if !validControl(p, in.ControlVersion, in.ControlID) {
		return Package{}, ErrInvalid
	}
	if !ownsControl(p, in.ControlVersion, in.ControlID, actor) {
		return Package{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cat, e := s.read(repo, program)
	if e != nil {
		return Package{}, e
	}
	qs := map[string]Query{}
	for _, q := range cat.Queries {
		if q.ControlVersion == in.ControlVersion && q.ControlID == in.ControlID {
			qs[q.ID] = q
		}
	}
	seen := map[string]bool{}
	records := append([]Record(nil), in.Records...)
	for _, r := range records {
		q, ok := qs[r.QueryID]
		if !ok || !validRecord(r) || r.ContainsCredentials || r.ContainsPersonalData || r.ObservedAt.Before(in.PeriodStart) || r.ObservedAt.After(in.PeriodEnd) || audienceRank(r.Audience) > audienceRank(q.Audience) {
			return Package{}, ErrInvalid
		}
		if seen[r.QueryID+"\x00"+r.SourceRecordID] {
			return Package{}, ErrConflict
		}
		seen[r.QueryID+"\x00"+r.SourceRecordID] = true
	}
	now := s.now().UTC()
	coverage := map[string]string{}
	gaps := []Gap{}
	fresh := true
	for id, q := range qs {
		found := false
		latest := time.Time{}
		for _, r := range records {
			if r.QueryID == id && r.Accessible && !r.Embargoed {
				found = true
				if r.ObservedAt.After(latest) {
					latest = r.ObservedAt
				}
				if r.ContradictsRecordID != "" {
					gaps = append(gaps, Gap{"contradiction", id, "source records disagree"})
				}
			}
		}
		if !found {
			coverage[id] = "gap"
			gaps = append(gaps, Gap{"missing_or_inaccessible", id, "no audience-permitted source record covers the query"})
			fresh = false
		} else if now.Sub(latest) > time.Duration(q.FreshnessHours)*time.Hour {
			coverage[id] = "stale"
			gaps = append(gaps, Gap{"stale", id, "latest permitted source record exceeds the freshness window"})
			fresh = false
		} else {
			coverage[id] = "covered"
		}
	}
	if len(qs) == 0 {
		gaps = append(gaps, Gap{"missing_query", "", "control version has no evidence queries"})
		fresh = false
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].QueryID == records[j].QueryID {
			return records[i].SourceRecordID < records[j].SourceRecordID
		}
		return records[i].QueryID < records[j].QueryID
	})
	x := Package{ID: newID(), RepositoryID: repo, ProgramID: program, ControlVersion: in.ControlVersion, ControlID: in.ControlID, PeriodStart: in.PeriodStart.UTC(), PeriodEnd: in.PeriodEnd.UTC(), CollectedAt: now, CollectorID: actor, Records: records, Coverage: coverage, Gaps: gaps, Fresh: fresh, Immutable: true}
	x.PackageHash = digestPackage(x)
	x.Attestation = "sha256:" + x.PackageHash + ":collector:" + actor
	cat.Packages = append(cat.Packages, x)
	return x, s.save(repo, program, cat)
}
func audienceRank(a string) int {
	switch a {
	case "public":
		return 0
	case "repository":
		return 1
	default:
		return 2
	}
}
func project(p Package, audience string) Package {
	out := p
	out.Records = []Record{}
	for _, r := range p.Records {
		if r.Accessible && !r.Embargoed && !r.ContainsCredentials && !r.ContainsPersonalData && audienceRank(r.Audience) <= audienceRank(audience) {
			out.Records = append(out.Records, r)
		}
	}
	return out
}
func (s *Store) Catalog(repo, program, audience string) (Catalog, error) {
	if !audiences[audience] {
		return Catalog{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.read(repo, program)
	if e != nil {
		return Catalog{}, e
	}
	out := Catalog{Queries: []Query{}, Packages: []Package{}}
	for _, q := range x.Queries {
		if audienceRank(q.Audience) <= audienceRank(audience) {
			out.Queries = append(out.Queries, q)
		}
	}
	for _, p := range x.Packages {
		out.Packages = append(out.Packages, project(p, audience))
	}
	return out, nil
}
