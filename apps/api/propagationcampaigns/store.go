// Package propagationcampaigns retains the agreed scope of cross-line changes.
package propagationcampaigns

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

var ErrNotFound = errors.New("propagation campaign not found")
var ErrInvalid = errors.New("invalid propagation campaign")
var ErrForbidden = errors.New("propagation campaign action forbidden")

type Source struct {
	Kind               string   `json:"kind"`
	RepositoryID       string   `json:"repository_id"`
	ResourceID         string   `json:"resource_id"`
	CommitIDs          []string `json:"commit_ids"`
	Revision           string   `json:"revision"`
	EvidenceReferences []string `json:"evidence_references,omitempty"`
}

type Authority struct {
	OwnerIDs   []string  `json:"owner_ids"`
	Access     string    `json:"access"`
	Basis      string    `json:"basis"`
	ObservedAt time.Time `json:"observed_at"`
}

type Target struct {
	ID                  string    `json:"id"`
	RepositoryID        string    `json:"repository_id,omitempty"`
	RepositoryReference string    `json:"repository_reference,omitempty"`
	ReleaseLine         string    `json:"release_line"`
	Revision            string    `json:"revision,omitempty"`
	PackageIDs          []string  `json:"package_ids,omitempty"`
	OwnerIDs            []string  `json:"owner_ids,omitempty"`
	Deadline            time.Time `json:"deadline"`
	DependsOn           []string  `json:"depends_on,omitempty"`
	Disposition         string    `json:"disposition"`
	DispositionReason   string    `json:"disposition_reason,omitempty"`
	Authority           Authority `json:"authority"`
}

type CompletionPolicy struct {
	Mode                   string   `json:"mode"`
	RequiredTargetIDs      []string `json:"required_target_ids,omitempty"`
	AllowEquivalent        bool     `json:"allow_already_equivalent"`
	ExceptionRequiresOwner bool     `json:"exception_requires_owner"`
}

type Input struct {
	Title              string           `json:"title"`
	Intent             string           `json:"intent"`
	AcceptanceCriteria []string         `json:"acceptance_criteria"`
	Source             Source           `json:"source"`
	Targets            []Target         `json:"targets"`
	CompletionPolicy   CompletionPolicy `json:"completion_policy"`
}

type Blocker struct {
	TargetID string `json:"target_id"`
	Kind     string `json:"kind"`
	Detail   string `json:"detail"`
}

type Citation struct {
	Kind      string `json:"kind"`
	Reference string `json:"reference"`
	Revision  string `json:"revision"`
	Path      string `json:"path,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
}

type Comparison struct {
	Kind            string     `json:"kind"`
	SourceSummary   string     `json:"source_summary"`
	TargetSummary   string     `json:"target_summary"`
	Conclusion      string     `json:"conclusion"`
	BehavioralProof bool       `json:"behavioral_proof"`
	Citations       []Citation `json:"citations"`
}

type AssessmentInput struct {
	TargetRevision       string       `json:"target_revision"`
	SourceRevision       string       `json:"source_revision"`
	Classification       string       `json:"classification"`
	Rationale            string       `json:"rationale"`
	Comparisons          []Comparison `json:"comparisons"`
	Risks                []string     `json:"risks,omitempty"`
	Uncertainty          string       `json:"uncertainty,omitempty"`
	AssumptionsStillHold bool         `json:"assumptions_still_hold"`
}

type FindingInput struct {
	ActorKind   string     `json:"actor_kind"`
	Summary     string     `json:"summary"`
	Risk        string     `json:"risk,omitempty"`
	Uncertainty string     `json:"uncertainty,omitempty"`
	Citations   []Citation `json:"citations"`
}

type Finding struct {
	ID      string `json:"id"`
	ActorID string `json:"actor_id"`
	FindingInput
	CreatedAt time.Time `json:"created_at"`
}

type Acknowledgement struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	Decision  string    `json:"decision"`
	Rationale string    `json:"rationale"`
	CreatedAt time.Time `json:"created_at"`
}

type Assessment struct {
	ID       string `json:"id"`
	TargetID string `json:"target_id"`
	AuthorID string `json:"author_id"`
	AssessmentInput
	Findings         []Finding         `json:"findings"`
	Acknowledgements []Acknowledgement `json:"acknowledgements"`
	CreatedAt        time.Time         `json:"created_at"`
	Stale            bool              `json:"stale"`
}

// Contribution turns one current applicability decision into locally owned,
// ordinary collaboration resources. Resource references are provenance, not
// authority: their native APIs remain responsible for access and publication.
type ContributionInput struct {
	AssessmentID       string             `json:"assessment_id"`
	Mode               string             `json:"mode"`
	Rationale          string             `json:"rationale"`
	SourceAuthorIDs    []string           `json:"source_author_ids"`
	RelevantCommitIDs  []string           `json:"relevant_commit_ids"`
	Constraints        []string           `json:"constraints"`
	AcceptanceCriteria []string           `json:"acceptance_criteria"`
	Deviations         []string           `json:"deviations,omitempty"`
	ContextReferences  []string           `json:"context_references,omitempty"`
	Tasks              []ContributionTask `json:"tasks"`
}

type ContributionTask struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	OwnerKind          string   `json:"owner_kind"`
	OwnerID            string   `json:"owner_id"`
	DependsOn          []string `json:"depends_on,omitempty"`
	Scope              []string `json:"scope"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	TaskID             string   `json:"task_id,omitempty"`
	SessionID          string   `json:"session_id,omitempty"`
	WorkspaceID        string   `json:"workspace_id,omitempty"`
	ForkRepositoryID   string   `json:"fork_repository_id,omitempty"`
	PullRequestID      string   `json:"pull_request_id,omitempty"`
	FederatedPullRef   string   `json:"federated_pull_reference,omitempty"`
}

type Contribution struct {
	ID        string `json:"id"`
	TargetID  string `json:"target_id"`
	CreatorID string `json:"creator_id"`
	ContributionInput
	SourceIntent        string    `json:"source_intent"`
	SourceResourceID    string    `json:"source_resource_id"`
	SourceRevision      string    `json:"source_revision"`
	AssessmentRationale string    `json:"assessment_rationale"`
	CreatedAt           time.Time `json:"created_at"`
	AuthorityGranted    []string  `json:"authority_granted"`
}
type Campaign struct {
	ID           string `json:"id"`
	RepositoryID string `json:"repository_id"`
	CreatorID    string `json:"creator_id"`
	Input
	Blockers      []Blocker      `json:"blockers"`
	Assessments   []Assessment   `json:"assessments,omitempty"`
	Contributions []Contribution `json:"contributions,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

func validContribution(in ContributionInput, c Campaign, t Target, a Assessment) bool {
	if in.AssessmentID == "" || !map[string]bool{"direct": true, "adapted": true}[in.Mode] || strings.TrimSpace(in.Rationale) == "" || !textList(in.SourceAuthorIDs, true) || !textList(in.RelevantCommitIDs, true) || !textList(in.Constraints, true) || !textList(in.AcceptanceCriteria, true) || !textList(in.ContextReferences, false) || len(in.Tasks) == 0 || len(in.Tasks) > 100 {
		return false
	}
	if a.Stale || a.TargetID != t.ID || a.SourceRevision != c.Source.Revision || (t.Revision != "" && a.TargetRevision != t.Revision) || !map[string]bool{"directly_applicable": true, "adaptation_required": true, "conflicting": true}[a.Classification] {
		return false
	}
	if (in.Mode == "direct") != (a.Classification == "directly_applicable") || in.Mode == "adapted" && !textList(in.Deviations, true) || in.Mode == "direct" && len(in.Deviations) != 0 {
		return false
	}
	commits := map[string]bool{}
	for _, x := range c.Source.CommitIDs {
		commits[x] = true
	}
	for _, x := range in.RelevantCommitIDs {
		if !commits[x] {
			return false
		}
	}
	seen := map[string]bool{}
	for _, x := range in.Tasks {
		if x.ID == "" || seen[x.ID] || strings.TrimSpace(x.Title) == "" || !map[string]bool{"human": true, "agent": true}[x.OwnerKind] || x.OwnerID == "" || !textList(x.Scope, true) || !textList(x.AcceptanceCriteria, true) {
			return false
		}
		seen[x.ID] = true
		// Independently owned targets must publish through an ordinary fork or
		// federation reference; a campaign-local PR identifier is insufficient.
		if x.PullRequestID != "" && t.RepositoryID != c.RepositoryID && x.ForkRepositoryID == "" && x.FederatedPullRef == "" {
			return false
		}
	}
	for _, x := range in.Tasks {
		for _, d := range x.DependsOn {
			if !seen[d] || d == x.ID {
				return false
			}
		}
	}
	plain := make([]Target, 0, len(in.Tasks))
	for _, x := range in.Tasks {
		plain = append(plain, Target{ID: x.ID, DependsOn: x.DependsOn})
	}
	return !cyclic(plain)
}

var comparisonKinds = map[string]bool{"history": true, "symbols": true, "dependencies": true, "interfaces": true, "schemas": true, "prior_fixes": true, "release_commitments": true}

func validCitations(v []Citation) bool {
	if len(v) == 0 || len(v) > 100 {
		return false
	}
	for _, c := range v {
		if !comparisonKinds[c.Kind] && c.Kind != "test" && c.Kind != "change" || strings.TrimSpace(c.Reference) == "" || strings.TrimSpace(c.Revision) == "" {
			return false
		}
	}
	return true
}
func validAssessment(in AssessmentInput, source string) bool {
	classes := map[string]bool{"directly_applicable": true, "already_satisfied": true, "adaptation_required": true, "conflicting": true, "not_applicable": true}
	if in.TargetRevision == "" || in.SourceRevision != source || !classes[in.Classification] || strings.TrimSpace(in.Rationale) == "" || len(in.Comparisons) != len(comparisonKinds) || !textList(in.Risks, false) {
		return false
	}
	seen, proof := map[string]bool{}, false
	for _, c := range in.Comparisons {
		if !comparisonKinds[c.Kind] || seen[c.Kind] || strings.TrimSpace(c.SourceSummary) == "" || strings.TrimSpace(c.TargetSummary) == "" || !map[string]bool{"matched": true, "different": true, "absent": true, "unknown": true}[c.Conclusion] || !validCitations(c.Citations) {
			return false
		}
		seen[c.Kind] = true
		proof = proof || c.BehavioralProof
	}
	// Similar code/history is evidence, never proof of equivalent behavior.
	return in.Classification != "already_satisfied" || proof
}
func target(c Campaign, targetID string) (Target, bool) {
	for _, t := range c.Targets {
		if t.ID == targetID {
			return t, true
		}
	}
	return Target{}, false
}
func currentAssessments(v []Assessment) []Assessment {
	latest := map[string]int{}
	for i := range v {
		latest[v[i].TargetID] = i
	}
	out := append([]Assessment(nil), v...)
	for i := range out {
		out[i].Stale = latest[out[i].TargetID] != i
	}
	return out
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
func id() string { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func textList(v []string, required bool) bool {
	if required && len(v) == 0 || len(v) > 100 {
		return false
	}
	for _, x := range v {
		if strings.TrimSpace(x) == "" {
			return false
		}
	}
	return true
}

func validate(in Input) bool {
	kinds := map[string]bool{"merged_pull_request": true, "security_repair": true, "regression_correction": true, "policy_change": true, "package_release": true, "interface_evolution": true}
	dispositions := map[string]bool{"pending": true, "unknown": true, "unsupported": true, "inaccessible": true, "already_equivalent": true}
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Intent) == "" || !textList(in.AcceptanceCriteria, true) || !kinds[in.Source.Kind] || in.Source.RepositoryID == "" || in.Source.ResourceID == "" || in.Source.Revision == "" || !textList(in.Source.CommitIDs, true) || len(in.Targets) == 0 || !map[string]bool{"all_supported": true, "required_targets": true}[in.CompletionPolicy.Mode] {
		return false
	}
	seen := map[string]bool{}
	for _, t := range in.Targets {
		if t.ID == "" || seen[t.ID] || (t.RepositoryID == "" && t.RepositoryReference == "") || t.ReleaseLine == "" || t.Deadline.IsZero() || !dispositions[t.Disposition] || t.Authority.Access == "" || t.Authority.Basis == "" || t.Authority.ObservedAt.IsZero() || !textList(t.OwnerIDs, false) || !textList(t.Authority.OwnerIDs, false) || (t.Disposition != "pending" && t.DispositionReason == "") {
			return false
		}
		seen[t.ID] = true
	}
	for _, t := range in.Targets {
		for _, d := range t.DependsOn {
			if !seen[d] || d == t.ID {
				return false
			}
		}
	}
	if cyclic(in.Targets) {
		return false
	}
	if in.CompletionPolicy.Mode == "required_targets" {
		if !textList(in.CompletionPolicy.RequiredTargetIDs, true) {
			return false
		}
		for _, x := range in.CompletionPolicy.RequiredTargetIDs {
			if !seen[x] {
				return false
			}
		}
	}
	return true
}
func cyclic(ts []Target) bool {
	deps := map[string][]string{}
	for _, t := range ts {
		deps[t.ID] = t.DependsOn
	}
	state := map[string]byte{}
	var visit func(string) bool
	visit = func(x string) bool {
		if state[x] == 1 {
			return true
		}
		if state[x] == 2 {
			return false
		}
		state[x] = 1
		for _, d := range deps[x] {
			if visit(d) {
				return true
			}
		}
		state[x] = 2
		return false
	}
	for x := range deps {
		if visit(x) {
			return true
		}
	}
	return false
}
func blockers(in Input) []Blocker {
	var out []Blocker
	for _, t := range in.Targets {
		if t.Disposition != "pending" && !(t.Disposition == "already_equivalent" && in.CompletionPolicy.AllowEquivalent) {
			out = append(out, Blocker{t.ID, t.Disposition, t.DispositionReason})
		}
	}
	return out
}
func (s *Store) path(repo, campaign string) string {
	return filepath.Join(s.root, repo, campaign+".json")
}
func (s *Store) Create(repo, actor string, in Input) (Campaign, error) {
	if repo == "" || actor == "" || !validate(in) {
		return Campaign{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	x := Campaign{ID: id(), RepositoryID: repo, CreatorID: actor, Input: in, Blockers: blockers(in), CreatedAt: s.now().UTC()}
	dir := filepath.Dir(s.path(repo, x.ID))
	if e := os.MkdirAll(dir, 0750); e != nil {
		return Campaign{}, e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(repo, x.ID), b, 0640)
	}
	return x, e
}
func (s *Store) Get(repo, campaign string) (Campaign, error) {
	var x Campaign
	b, e := os.ReadFile(s.path(repo, campaign))
	if os.IsNotExist(e) {
		return x, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &x)
		x.Assessments = currentAssessments(x.Assessments)
	}
	return x, e
}

func (s *Store) save(x Campaign) error {
	x.Assessments = currentAssessments(x.Assessments)
	b, err := json.MarshalIndent(x, "", "  ")
	if err == nil {
		err = os.WriteFile(s.path(x.RepositoryID, x.ID), b, 0640)
	}
	return err
}

func (s *Store) Assess(repo, campaign, targetID, actor string, in AssessmentInput) (Campaign, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, err := s.Get(repo, campaign)
	if err != nil {
		return x, err
	}
	t, ok := target(x, targetID)
	if !ok || t.Authority.Access == "inaccessible" || !validAssessment(in, x.Source.Revision) {
		return Campaign{}, ErrInvalid
	}
	x.Assessments = append(x.Assessments, Assessment{ID: id(), TargetID: targetID, AuthorID: actor, AssessmentInput: in, Findings: []Finding{}, Acknowledgements: []Acknowledgement{}, CreatedAt: s.now().UTC()})
	x.Assessments = currentAssessments(x.Assessments)
	err = s.save(x)
	return x, err
}

func (s *Store) AddFinding(repo, campaign, targetID, assessmentID, actor string, in FindingInput) (Campaign, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, err := s.Get(repo, campaign)
	if err != nil {
		return x, err
	}
	if !map[string]bool{"human": true, "read_only_agent": true}[in.ActorKind] || strings.TrimSpace(in.Summary) == "" || !validCitations(in.Citations) {
		return Campaign{}, ErrInvalid
	}
	for i := range x.Assessments {
		if x.Assessments[i].ID == assessmentID && x.Assessments[i].TargetID == targetID {
			x.Assessments[i].Findings = append(x.Assessments[i].Findings, Finding{ID: id(), ActorID: actor, FindingInput: in, CreatedAt: s.now().UTC()})
			err = s.save(x)
			return x, err
		}
	}
	return Campaign{}, ErrNotFound
}

func (s *Store) Acknowledge(repo, campaign, targetID, assessmentID, actor, decision, rationale string) (Campaign, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, err := s.Get(repo, campaign)
	if err != nil {
		return x, err
	}
	t, ok := target(x, targetID)
	if !ok {
		return Campaign{}, ErrNotFound
	}
	owner := false
	for _, v := range append(append([]string{}, t.OwnerIDs...), t.Authority.OwnerIDs...) {
		owner = owner || v == actor
	}
	if !owner {
		return Campaign{}, ErrForbidden
	}
	if !map[string]bool{"acknowledged": true, "changes_requested": true}[decision] || strings.TrimSpace(rationale) == "" {
		return Campaign{}, ErrInvalid
	}
	for i := range x.Assessments {
		if x.Assessments[i].ID == assessmentID && x.Assessments[i].TargetID == targetID {
			x.Assessments[i].Acknowledgements = append(x.Assessments[i].Acknowledgements, Acknowledgement{ID: id(), OwnerID: actor, Decision: decision, Rationale: strings.TrimSpace(rationale), CreatedAt: s.now().UTC()})
			err = s.save(x)
			return x, err
		}
	}
	return Campaign{}, ErrNotFound
}

func (s *Store) CreateContribution(repo, campaign, targetID, actor string, in ContributionInput) (Campaign, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, err := s.Get(repo, campaign)
	if err != nil {
		return x, err
	}
	t, ok := target(x, targetID)
	if !ok {
		return Campaign{}, ErrNotFound
	}
	if t.Disposition != "pending" || t.Authority.Access == "inaccessible" || t.Authority.Access == "unknown" {
		return Campaign{}, ErrForbidden
	}
	var assessment Assessment
	found := false
	for _, a := range x.Assessments {
		if a.ID == in.AssessmentID && a.TargetID == targetID {
			assessment, found = a, true
			break
		}
	}
	if !found {
		return Campaign{}, ErrNotFound
	}
	if !validContribution(in, x, t, assessment) {
		return Campaign{}, ErrInvalid
	}
	x.Contributions = append(x.Contributions, Contribution{ID: id(), TargetID: targetID, CreatorID: actor, ContributionInput: in, SourceIntent: x.Intent, SourceResourceID: x.Source.ResourceID, SourceRevision: x.Source.Revision, AssessmentRationale: assessment.Rationale, CreatedAt: s.now().UTC(), AuthorityGranted: []string{}})
	err = s.save(x)
	return x, err
}
func (s *Store) List(repo string) ([]Campaign, error) {
	entries, e := os.ReadDir(filepath.Join(s.root, repo))
	if os.IsNotExist(e) {
		return []Campaign{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Campaign{}
	for _, f := range entries {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}
		x, e := s.Get(repo, strings.TrimSuffix(f.Name(), ".json"))
		if e != nil {
			return nil, e
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
