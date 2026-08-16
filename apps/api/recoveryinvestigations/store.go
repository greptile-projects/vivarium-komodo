// Package recoveryinvestigations retains bounded, cited diagnosis of recovery exercises.
package recoveryinvestigations

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

var ErrNotFound = errors.New("recovery investigation not found")
var ErrInvalid = errors.New("invalid recovery investigation")

type Evidence struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	ResourceID  string `json:"resource_id"`
	Revision    string `json:"revision"`
	Path        string `json:"path,omitempty"`
	Summary     string `json:"summary"`
	Audience    string `json:"audience"`
	Uncertainty string `json:"uncertainty,omitempty"`
}
type Citation struct {
	EvidenceID string `json:"evidence_id"`
	Label      string `json:"label,omitempty"`
}
type Finding struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	Statement   string     `json:"statement"`
	Citations   []Citation `json:"citations"`
	Uncertainty string     `json:"uncertainty,omitempty"`
	Challenges  string     `json:"challenges,omitempty"`
	Verdict     string     `json:"verdict,omitempty"`
	ActorID     string     `json:"actor_id"`
	CreatedAt   time.Time  `json:"created_at"`
}
type Investigation struct {
	ID               string     `json:"id"`
	RepositoryID     string     `json:"repository_id"`
	ExerciseID       string     `json:"exercise_id"`
	ExerciseRevision int64      `json:"exercise_revision"`
	PlanID           string     `json:"plan_id"`
	PlanVersion      int64      `json:"plan_version"`
	Title            string     `json:"title"`
	Question         string     `json:"question"`
	ResourceIDs      []string   `json:"resource_ids"`
	Participants     []string   `json:"participants"`
	Evidence         []Evidence `json:"evidence"`
	Findings         []Finding  `json:"findings"`
	CreatorID        string     `json:"creator_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Current          bool       `json:"current"`
	Blockers         []string   `json:"blockers"`
}
type CreateInput struct {
	ExerciseID       string     `json:"exercise_id"`
	ExerciseRevision int64      `json:"exercise_revision"`
	Title            string     `json:"title"`
	Question         string     `json:"question"`
	ResourceIDs      []string   `json:"resource_ids"`
	Evidence         []Evidence `json:"evidence"`
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func newid() string { var b [16]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func uniq(xs []string) []string {
	m := map[string]bool{}
	out := []string{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !m[x] {
			m[x] = true
			out = append(out, x)
		}
	}
	sort.Strings(out)
	return out
}
func validEvidence(es []Evidence) bool {
	if len(es) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, e := range es {
		if !map[string]bool{"exercise": true, "code": true, "dependency": true, "release": true, "configuration": true, "ownership": true, "protection_plan": true}[e.Kind] || e.ResourceID == "" || e.Revision == "" || strings.TrimSpace(e.Summary) == "" || !map[string]bool{"repository": true, "participants": true}[e.Audience] || seen[e.Kind+":"+e.ResourceID+":"+e.Revision+":"+e.Path] {
			return false
		}
		seen[e.Kind+":"+e.ResourceID+":"+e.Revision+":"+e.Path] = true
	}
	return true
}
func (s *Store) Create(repo, actor, plan string, planVersion int64, status string, in CreateInput) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || actor == "" || plan == "" || planVersion < 1 || in.ExerciseID == "" || in.ExerciseRevision < 1 || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Question) == "" || len(in.ResourceIDs) == 0 || !validEvidence(in.Evidence) || (status != "failed" && status != "passed") {
		return Investigation{}, ErrInvalid
	}
	found := false
	uncertain := false
	for _, e := range in.Evidence {
		found = found || (e.Kind == "exercise" && e.ResourceID == in.ExerciseID)
		uncertain = uncertain || strings.TrimSpace(e.Uncertainty) != ""
	}
	if !found || (status == "passed" && !uncertain) {
		return Investigation{}, ErrInvalid
	}
	for i := range in.Evidence {
		in.Evidence[i].ID = newid()
	}
	now := s.now().UTC()
	v := Investigation{ID: newid(), RepositoryID: repo, ExerciseID: in.ExerciseID, ExerciseRevision: in.ExerciseRevision, PlanID: plan, PlanVersion: planVersion, Title: strings.TrimSpace(in.Title), Question: strings.TrimSpace(in.Question), ResourceIDs: uniq(in.ResourceIDs), Participants: []string{actor}, Evidence: in.Evidence, CreatorID: actor, CreatedAt: now, UpdatedAt: now, Current: true}
	return v, s.write(v)
}
func (s *Store) Get(repo, id string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) List(repo string) ([]Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(s.root)
	if e != nil {
		return nil, e
	}
	out := []Investigation{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, er := s.read(strings.TrimSuffix(x.Name(), ".json"))
		if er == nil && v.RepositoryID == repo {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Invite(repo, id, actor, participant string) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if !contains(v.Participants, actor) || participant == "" {
		return Investigation{}, ErrInvalid
	}
	v.Participants = uniq(append(v.Participants, participant))
	v.UpdatedAt = s.now().UTC()
	return v, s.write(v)
}
func (s *Store) AddFinding(repo, id, actor string, in Finding) (Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(id)
	if e != nil || v.RepositoryID != repo {
		return Investigation{}, ErrNotFound
	}
	if !contains(v.Participants, actor) || !map[string]bool{"observation": true, "hypothesis": true, "challenge": true, "conclusion": true}[in.Kind] || strings.TrimSpace(in.Statement) == "" || len(in.Citations) == 0 {
		return Investigation{}, ErrInvalid
	}
	for _, c := range in.Citations {
		if !hasEvidence(v, c.EvidenceID) {
			return Investigation{}, ErrInvalid
		}
	}
	if in.Challenges != "" && !hasFinding(v, in.Challenges) {
		return Investigation{}, ErrInvalid
	}
	if in.Kind == "conclusion" && !map[string]bool{"supported": true, "disputed": true, "inconclusive": true}[in.Verdict] {
		return Investigation{}, ErrInvalid
	}
	in.ID = newid()
	in.ActorID = actor
	in.CreatedAt = s.now().UTC()
	v.Findings = append(v.Findings, in)
	v.UpdatedAt = in.CreatedAt
	return v, s.write(v)
}
func Resolve(v Investigation, exerciseRevision int64, current bool) Investigation {
	v.Current = exerciseRevision == v.ExerciseRevision && current
	v.Blockers = nil
	if exerciseRevision != v.ExerciseRevision {
		v.Blockers = append(v.Blockers, "exercise_changed")
	}
	if !current {
		v.Blockers = append(v.Blockers, "exercise_evidence_non_current")
	}
	for _, e := range v.Evidence {
		if e.Uncertainty != "" {
			v.Blockers = append(v.Blockers, "uncertain_evidence")
		}
	}
	v.Blockers = uniq(v.Blockers)
	return v
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
func hasEvidence(v Investigation, id string) bool {
	for _, e := range v.Evidence {
		if e.ID == id {
			return true
		}
	}
	return false
}
func hasFinding(v Investigation, id string) bool {
	for _, e := range v.Findings {
		if e.ID == id {
			return true
		}
	}
	return false
}
func (s *Store) read(id string) (Investigation, error) {
	b, e := os.ReadFile(filepath.Join(s.root, id+".json"))
	if e != nil {
		return Investigation{}, e
	}
	var v Investigation
	if json.Unmarshal(b, &v) != nil || v.ID != id {
		return Investigation{}, ErrInvalid
	}
	return v, nil
}
func (s *Store) write(v Investigation) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(s.root, ".investigation-")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if e = tmp.Chmod(0640); e == nil {
		_, e = tmp.Write(append(b, '\n'))
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(name, filepath.Join(s.root, v.ID+".json"))
	}
	return e
}
