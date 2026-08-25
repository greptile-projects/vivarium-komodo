// Package learningassessments retains revision-exact practical assessment evidence.
package learningassessments

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound  = errors.New("learning assessment not found")
	ErrInvalid   = errors.New("invalid learning assessment")
	ErrForbidden = errors.New("learning assessment forbidden")
	ErrConflict  = errors.New("learning assessment conflict")
)

type Criterion struct {
	ID                    string `json:"id"`
	Title                 string `json:"title"`
	Description           string `json:"description"`
	Required              bool   `json:"required"`
	HumanJudgmentRequired bool   `json:"human_judgment_required"`
}
type ProtectedCase struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Digest   string `json:"digest"`
	Material string `json:"material,omitempty"`
}
type Check struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}
type Definition struct {
	Title           string          `json:"title"`
	Summary         string          `json:"summary"`
	PathwayVersion  int64           `json:"pathway_version"`
	Revision        string          `json:"revision"`
	Criteria        []Criterion     `json:"criteria"`
	ProtectedCases  []ProtectedCase `json:"protected_cases"`
	Checks          []Check         `json:"checks"`
	OwnerIDs        []string        `json:"owner_ids"`
	ReviewerIDs     []string        `json:"reviewer_ids"`
	MaximumAttempts int             `json:"maximum_attempts"`
	AppealOwnerIDs  []string        `json:"appeal_owner_ids"`
}
type PublicCase struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Digest string `json:"digest"`
}
type Evidence struct {
	Number      int       `json:"number"`
	Kind        string    `json:"kind"`
	Summary     string    `json:"summary"`
	Reference   string    `json:"reference"`
	Digest      string    `json:"digest"`
	CheckName   string    `json:"check_name,omitempty"`
	CheckStatus string    `json:"check_status,omitempty"`
	Flaky       bool      `json:"flaky,omitempty"`
	AuthorID    string    `json:"author_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type Integrity struct {
	CopiedSolution bool   `json:"copied_solution"`
	AgentOverreach bool   `json:"agent_overreach"`
	Detail         string `json:"detail"`
}
type RubricDecision struct {
	CriterionID     string `json:"criterion_id"`
	Decision        string `json:"decision"`
	Rationale       string `json:"rationale"`
	EvidenceNumbers []int  `json:"evidence_numbers"`
}
type Judgment struct {
	ReviewerID  string           `json:"reviewer_id"`
	Outcome     string           `json:"outcome"`
	Feedback    string           `json:"feedback"`
	Uncertainty string           `json:"uncertainty"`
	Rubric      []RubricDecision `json:"rubric"`
	Integrity   Integrity        `json:"integrity"`
	CreatedAt   time.Time        `json:"created_at"`
}
type Appeal struct {
	ID                 string    `json:"id"`
	LearnerID          string    `json:"learner_id"`
	Reason             string    `json:"reason"`
	EvidenceReferences []string  `json:"evidence_references"`
	Status             string    `json:"status"`
	Decision           string    `json:"decision,omitempty"`
	Rationale          string    `json:"rationale,omitempty"`
	DecidedBy          string    `json:"decided_by,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}
type Accommodation struct {
	Request    string `json:"request"`
	Status     string `json:"status"`
	DecisionBy string `json:"decision_by,omitempty"`
	Rationale  string `json:"rationale,omitempty"`
}
type Attempt struct {
	ID                   string        `json:"id"`
	Number               int           `json:"number"`
	LearnerID            string        `json:"learner_id"`
	Revision             string        `json:"revision"`
	PathwayVersion       int64         `json:"pathway_version"`
	Status               string        `json:"status"`
	WorkspaceDigest      string        `json:"workspace_digest"`
	ReproductionCommands []string      `json:"reproduction_commands"`
	Assistance           []string      `json:"assistance"`
	Accommodation        Accommodation `json:"accommodation"`
	Evidence             []Evidence    `json:"evidence"`
	Judgments            []Judgment    `json:"judgments"`
	Appeals              []Appeal      `json:"appeals"`
	Blockers             []string      `json:"blockers"`
	CompletionSupported  bool          `json:"completion_supported"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
}
type Assessment struct {
	RepositoryID          string       `json:"repository_id"`
	PathwayID             string       `json:"pathway_id"`
	ID                    string       `json:"id"`
	Definition            Definition   `json:"definition"`
	ProtectedCaseMetadata []PublicCase `json:"protected_case_metadata"`
	AuthorID              string       `json:"author_id"`
	Attempts              []Attempt    `json:"attempts"`
	CreatedAt             time.Time    `json:"created_at"`
}
type AttemptInput struct {
	Revision             string   `json:"revision"`
	WorkspaceDigest      string   `json:"workspace_digest"`
	ReproductionCommands []string `json:"reproduction_commands"`
	Assistance           []string `json:"assistance"`
	AccommodationRequest string   `json:"accommodation_request"`
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
func randomID() string { b := make([]byte, 12); _, _ = rand.Read(b); return hex.EncodeToString(b) }

var credential = regexp.MustCompile(`(?i)(authorization:\s*bearer|-----BEGIN [A-Z ]*PRIVATE KEY-----|\b(api[_-]?key|password|secret|token)\s*[=:]\s*\S+)`)

func safe(s string) bool { return len(s) <= 16000 && !credential.MatchString(s) }
func validID(s string) bool {
	if s == "" || len(s) > 100 {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}
func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func validate(d Definition) bool {
	if strings.TrimSpace(d.Title) == "" || strings.TrimSpace(d.Summary) == "" || d.PathwayVersion < 1 || d.Revision == "" || len(d.Criteria) == 0 || len(d.OwnerIDs) == 0 || len(d.ReviewerIDs) == 0 || d.MaximumAttempts < 1 || d.MaximumAttempts > 20 || len(d.AppealOwnerIDs) == 0 {
		return false
	}
	ids := map[string]bool{}
	for _, c := range d.Criteria {
		if !validID(c.ID) || ids[c.ID] || c.Title == "" || c.Description == "" || !safe(c.Description) {
			return false
		}
		ids[c.ID] = true
	}
	ids = map[string]bool{}
	for _, c := range d.ProtectedCases {
		if !validID(c.ID) || ids[c.ID] || c.Title == "" || c.Digest == "" || c.Material == "" || !safe(c.Material) {
			return false
		}
		ids[c.ID] = true
	}
	for _, c := range d.Checks {
		if c.Name == "" {
			return false
		}
	}
	return true
}
func (s *Store) Publish(repo, pathway, id, author string, d Definition) (Assessment, error) {
	if repo == "" || pathway == "" || !validID(id) || author == "" || !contains(d.OwnerIDs, author) || !validate(d) {
		return Assessment{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, e := s.read(repo, pathway, id); e == nil {
		return Assessment{}, ErrConflict
	} else if !errors.Is(e, ErrNotFound) {
		return Assessment{}, e
	}
	n := s.now().UTC()
	a := Assessment{RepositoryID: repo, PathwayID: pathway, ID: id, Definition: d, AuthorID: author, Attempts: []Attempt{}, CreatedAt: n}
	for _, c := range d.ProtectedCases {
		a.ProtectedCaseMetadata = append(a.ProtectedCaseMetadata, PublicCase{ID: c.ID, Title: c.Title, Digest: c.Digest})
	}
	return a, s.write(a)
}
func (s *Store) Start(repo, pathway, id, learner string, in AttemptInput, currentPathway int64, currentRevision string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pathway, id)
	if e != nil {
		return a, e
	}
	if learner == "" || in.Revision != a.Definition.Revision || in.Revision != currentRevision || a.Definition.PathwayVersion != currentPathway || in.WorkspaceDigest == "" || len(in.ReproductionCommands) == 0 {
		return a, ErrInvalid
	}
	for _, v := range append(append([]string{}, in.ReproductionCommands...), in.Assistance...) {
		if !safe(v) {
			return a, ErrInvalid
		}
	}
	count := 0
	for _, v := range a.Attempts {
		if v.LearnerID == learner {
			count++
		}
	}
	if count >= a.Definition.MaximumAttempts {
		return a, ErrConflict
	}
	n := s.now().UTC()
	ac := Accommodation{Request: strings.TrimSpace(in.AccommodationRequest), Status: "none"}
	if ac.Request != "" {
		ac.Status = "requested"
	}
	a.Attempts = append(a.Attempts, Attempt{ID: randomID(), Number: count + 1, LearnerID: learner, Revision: in.Revision, PathwayVersion: a.Definition.PathwayVersion, Status: "active", WorkspaceDigest: in.WorkspaceDigest, ReproductionCommands: in.ReproductionCommands, Assistance: in.Assistance, Accommodation: ac, Evidence: []Evidence{}, Judgments: []Judgment{}, Appeals: []Appeal{}, Blockers: []string{}, CreatedAt: n, UpdatedAt: n})
	return a, s.write(a)
}
func findAttempt(a *Assessment, id string) (*Attempt, error) {
	for i := range a.Attempts {
		if a.Attempts[i].ID == id {
			return &a.Attempts[i], nil
		}
	}
	return nil, ErrNotFound
}
func (s *Store) AddEvidence(repo, pathway, id, attempt, actor string, in Evidence) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pathway, id)
	if e != nil {
		return a, e
	}
	v, e := findAttempt(&a, attempt)
	if e != nil {
		return a, e
	}
	if v.LearnerID != actor {
		return a, ErrForbidden
	}
	if v.Status != "active" && v.Status != "in_review" {
		return a, ErrConflict
	}
	if !safe(in.Summary) || !safe(in.Reference) || in.Summary == "" || in.Reference == "" || in.Digest == "" || !contains([]string{"repository_check", "checkpoint", "artifact", "explanation", "authorship"}, in.Kind) {
		return a, ErrInvalid
	}
	if in.Kind == "repository_check" && !contains([]string{"pass", "fail", "error"}, in.CheckStatus) {
		return a, ErrInvalid
	}
	in.Number = len(v.Evidence) + 1
	in.AuthorID = actor
	in.CreatedAt = s.now().UTC()
	v.Evidence = append(v.Evidence, in)
	v.Status = "in_review"
	v.UpdatedAt = in.CreatedAt
	return a, s.write(a)
}
func (s *Store) Judge(repo, pathway, id, attempt, actor string, j Judgment, currentPathway int64, currentRevision string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pathway, id)
	if e != nil {
		return a, e
	}
	if !contains(a.Definition.ReviewerIDs, actor) {
		return a, ErrForbidden
	}
	v, e := findAttempt(&a, attempt)
	if e != nil {
		return a, e
	}
	if !contains([]string{"pass", "fail", "uncertain"}, j.Outcome) || !safe(j.Feedback) || !safe(j.Uncertainty) || len(j.Rubric) != len(a.Definition.Criteria) {
		return a, ErrInvalid
	}
	seen := map[string]bool{}
	for _, r := range j.Rubric {
		if seen[r.CriterionID] || !contains([]string{"pass", "fail", "uncertain"}, r.Decision) || !safe(r.Rationale) {
			return a, ErrInvalid
		}
		seen[r.CriterionID] = true
		for _, n := range r.EvidenceNumbers {
			if n < 1 || n > len(v.Evidence) {
				return a, ErrInvalid
			}
		}
	}
	for _, c := range a.Definition.Criteria {
		if !seen[c.ID] {
			return a, ErrInvalid
		}
	}
	j.ReviewerID = actor
	j.CreatedAt = s.now().UTC()
	v.Judgments = append(v.Judgments, j)
	derive(&a, v, currentPathway, currentRevision)
	v.UpdatedAt = j.CreatedAt
	return a, s.write(a)
}
func derive(a *Assessment, v *Attempt, currentPathway int64, currentRevision string) {
	b := []string{}
	if currentPathway != a.Definition.PathwayVersion {
		b = append(b, "criteria_changed")
	}
	if currentRevision != a.Definition.Revision || v.Revision != a.Definition.Revision {
		b = append(b, "stale_project_revision")
	}
	checks := map[string]Evidence{}
	for _, e := range v.Evidence {
		if e.Kind == "repository_check" {
			checks[e.CheckName] = e
		}
	}
	for _, c := range a.Definition.Checks {
		if !c.Required {
			continue
		}
		e, ok := checks[c.Name]
		if !ok || e.CheckStatus != "pass" {
			b = append(b, "required_check_not_passing:"+c.Name)
		} else if e.Flaky {
			b = append(b, "flaky_check:"+c.Name)
		}
	}
	if len(v.Judgments) == 0 {
		b = append(b, "human_judgment_required")
	} else {
		j := v.Judgments[len(v.Judgments)-1]
		if j.Integrity.CopiedSolution {
			b = append(b, "copied_solution")
		}
		if j.Integrity.AgentOverreach {
			b = append(b, "agent_overreach")
		}
		if j.Outcome != "pass" || j.Uncertainty != "" {
			b = append(b, "review_not_conclusive")
		}
		for _, r := range j.Rubric {
			if r.Decision != "pass" {
				b = append(b, "rubric_not_met:"+r.CriterionID)
			}
		}
	}
	v.Blockers = b
	v.CompletionSupported = len(b) == 0
	if v.CompletionSupported {
		v.Status = "passed"
	} else {
		v.Status = "not_demonstrated"
	}
}
func (s *Store) DecideAccommodation(repo, pathway, id, attempt, actor, status, rationale string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pathway, id)
	if e != nil {
		return a, e
	}
	if !contains(a.Definition.OwnerIDs, actor) {
		return a, ErrForbidden
	}
	v, e := findAttempt(&a, attempt)
	if e != nil {
		return a, e
	}
	if v.Accommodation.Status != "requested" || !contains([]string{"approved", "denied"}, status) || !safe(rationale) {
		return a, ErrInvalid
	}
	v.Accommodation.Status = status
	v.Accommodation.DecisionBy = actor
	v.Accommodation.Rationale = rationale
	v.UpdatedAt = s.now().UTC()
	return a, s.write(a)
}
func (s *Store) Appeal(repo, pathway, id, attempt, actor, reason string, refs []string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pathway, id)
	if e != nil {
		return a, e
	}
	v, e := findAttempt(&a, attempt)
	if e != nil {
		return a, e
	}
	if v.LearnerID != actor {
		return a, ErrForbidden
	}
	if reason == "" || !safe(reason) {
		return a, ErrInvalid
	}
	for _, x := range refs {
		if !safe(x) {
			return a, ErrInvalid
		}
	}
	v.Appeals = append(v.Appeals, Appeal{ID: randomID(), LearnerID: actor, Reason: reason, EvidenceReferences: refs, Status: "open", CreatedAt: s.now().UTC()})
	return a, s.write(a)
}
func (s *Store) DecideAppeal(repo, pathway, id, attempt, appeal, actor, decision, rationale string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, e := s.read(repo, pathway, id)
	if e != nil {
		return a, e
	}
	if !contains(a.Definition.AppealOwnerIDs, actor) {
		return a, ErrForbidden
	}
	v, e := findAttempt(&a, attempt)
	if e != nil {
		return a, e
	}
	for i := range v.Appeals {
		p := &v.Appeals[i]
		if p.ID == appeal {
			if p.Status != "open" || !contains([]string{"upheld", "denied", "reassessment"}, decision) || !safe(rationale) {
				return a, ErrInvalid
			}
			p.Status = "decided"
			p.Decision = decision
			p.Rationale = rationale
			p.DecidedBy = actor
			if decision == "reassessment" {
				v.Status = "in_review"
				v.CompletionSupported = false
			}
			return a, s.write(a)
		}
	}
	return a, ErrNotFound
}
func (s *Store) Get(repo, pathway, id string) (Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, pathway, id)
}
func (s *Store) List(repo, pathway string) ([]Assessment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := filepath.Join(s.root, repo, pathway)
	es, e := os.ReadDir(d)
	if errors.Is(e, fs.ErrNotExist) {
		return []Assessment{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Assessment{}
	for _, x := range es {
		if filepath.Ext(x.Name()) != ".json" {
			continue
		}
		a, e := s.read(repo, pathway, strings.TrimSuffix(x.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}
func Project(a Assessment, actor string, writer bool) Assessment {
	for i := range a.Definition.ProtectedCases {
		a.Definition.ProtectedCases[i].Material = ""
	}
	visible := []Attempt{}
	for _, v := range a.Attempts {
		if writer || v.LearnerID == actor || contains(a.Definition.ReviewerIDs, actor) || contains(a.Definition.AppealOwnerIDs, actor) {
			visible = append(visible, v)
		}
	}
	a.Attempts = visible
	return a
}
func (s *Store) read(repo, pathway, id string) (Assessment, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, pathway, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Assessment{}, ErrNotFound
	}
	var a Assessment
	if e == nil {
		e = json.Unmarshal(b, &a)
	}
	return a, e
}
func (s *Store) write(a Assessment) error {
	d := filepath.Join(s.root, a.RepositoryID, a.PathwayID)
	if e := os.MkdirAll(d, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(a, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(d, "assessment-*.tmp")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, filepath.Join(d, a.ID+".json"))
}
