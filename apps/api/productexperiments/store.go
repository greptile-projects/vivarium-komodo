// Package productexperiments owns pre-exposure product experiment contracts.
package productexperiments

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
)

var (
	ErrNotFound = errors.New("product experiment not found")
	ErrInvalid  = errors.New("invalid product experiment")
	ErrConflict = errors.New("product experiment version conflict")
)

type SignalVersion struct {
	Version            int64     `json:"version"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Unit               string    `json:"unit"`
	Event              string    `json:"event"`
	Properties         []string  `json:"properties"`
	PermittedAudiences []string  `json:"permitted_audiences"`
	Instrumented       bool      `json:"instrumented"`
	AuthorID           string    `json:"author_id"`
	ChangeReason       string    `json:"change_reason"`
	CreatedAt          time.Time `json:"created_at"`
}
type Signal struct {
	ID             string          `json:"id"`
	RepositoryID   string          `json:"repository_id"`
	CurrentVersion int64           `json:"current_version"`
	Versions       []SignalVersion `json:"versions"`
}
type Source struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type Variant struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Control     bool   `json:"control"`
}
type Audience struct {
	Description   string   `json:"description"`
	Eligibility   []string `json:"eligibility"`
	Exclusions    []string `json:"exclusions"`
	Consent       string   `json:"consent"`
	EstimatedSize int      `json:"estimated_size"`
}
type Measure struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	SignalID      string `json:"signal_id"`
	SignalVersion int64  `json:"signal_version"`
	Aggregation   string `json:"aggregation"`
	Threshold     string `json:"threshold"`
}
type PlanInput struct {
	Title           string    `json:"title"`
	Source          Source    `json:"source"`
	Hypothesis      string    `json:"hypothesis"`
	Variants        []Variant `json:"variants"`
	Audience        Audience  `json:"target_audience"`
	Measures        []Measure `json:"measures"`
	MinimumEvidence string    `json:"minimum_evidence"`
	DurationHours   int       `json:"duration_hours"`
	OwnerIDs        []string  `json:"owner_ids"`
	ParticipantIDs  []string  `json:"participant_ids"`
	StopConditions  []string  `json:"stop_conditions"`
	Assumptions     []string  `json:"assumptions"`
	OverlapKeys     []string  `json:"overlap_keys"`
	ChangeReason    string    `json:"change_reason"`
}
type PlanVersion struct {
	Number int64 `json:"number"`
	PlanInput
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Comment struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`
	Body      string    `json:"body"`
	AuthorID  string    `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
}
type Approval struct {
	ID        string    `json:"id"`
	Version   int64     `json:"version"`
	ActorID   string    `json:"actor_id"`
	Decision  string    `json:"decision"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}
type AssumptionChange struct {
	ID         string    `json:"id"`
	Version    int64     `json:"version"`
	Assumption string    `json:"assumption"`
	Detail     string    `json:"detail"`
	ActorID    string    `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Blocker struct {
	Kind       string `json:"kind"`
	Detail     string `json:"detail"`
	ResourceID string `json:"resource_id,omitempty"`
}
type WorkItem struct {
	ID          string    `json:"id"`
	PlanVersion int64     `json:"plan_version"`
	Kind        string    `json:"kind"`
	OwnerKind   string    `json:"owner_kind"`
	OwnerID     string    `json:"owner_id"`
	VariantIDs  []string  `json:"variant_ids"`
	ResourceID  string    `json:"resource_id"`
	Revision    string    `json:"revision"`
	CreatedByID string    `json:"created_by_id"`
	CreatedAt   time.Time `json:"created_at"`
}
type EventDefinition struct {
	SignalID      string   `json:"signal_id"`
	SignalVersion int64    `json:"signal_version"`
	Event         string   `json:"event"`
	Properties    []string `json:"properties"`
}
type ImplementationInput struct {
	PullRequestID         string            `json:"pull_request_id"`
	VariantIDs            []string          `json:"variant_ids"`
	EventDefinitions      []EventDefinition `json:"event_definitions"`
	ExposureRules         []string          `json:"exposure_rules"`
	PrivacyClassification string            `json:"privacy_classification"`
	RemovalPlan           string            `json:"removal_plan"`
	CheckNames            map[string]string `json:"check_names"`
}
type Implementation struct {
	ID          string `json:"id"`
	PlanVersion int64  `json:"plan_version"`
	ImplementationInput
	SourceCommitID string    `json:"source_commit_id"`
	AuthorID       string    `json:"author_id"`
	CreatedAt      time.Time `json:"created_at"`
	Current        bool      `json:"current"`
}
type Experiment struct {
	ID                string             `json:"id"`
	RepositoryID      string             `json:"repository_id"`
	CurrentVersion    int64              `json:"current_version"`
	Versions          []PlanVersion      `json:"versions"`
	Comments          []Comment          `json:"comments"`
	Approvals         []Approval         `json:"approvals"`
	AssumptionChanges []AssumptionChange `json:"assumption_changes"`
	WorkItems         []WorkItem         `json:"work_items"`
	Implementations   []Implementation   `json:"implementations"`
	AudiencePolicies  []AudiencePolicy   `json:"audience_policies"`
	Runs              []Run              `json:"runs"`
	Blockers          []Blocker          `json:"blockers"`
	Ready             bool               `json:"ready"`
}

type Allocation struct {
	VariantID   string `json:"variant_id"`
	BasisPoints int    `json:"basis_points"`
}
type EligibilityPolicy struct {
	ConsentClass       string   `json:"consent_class"`
	Regions            []string `json:"regions"`
	OrganizationIDs    []string `json:"organization_ids"`
	RequiredAttributes []string `json:"required_attributes"`
	ExcludedAttributes []string `json:"excluded_attributes"`
}
type CollectionField struct {
	SignalID      string   `json:"signal_id"`
	SignalVersion int64    `json:"signal_version"`
	Properties    []string `json:"properties"`
}
type AudiencePolicyInput struct {
	ExpectedPlanVersion  int64             `json:"expected_plan_version"`
	ReleaseID            string            `json:"release_id"`
	VariantIDs           []string          `json:"variant_ids"`
	MutualExclusionGroup string            `json:"mutual_exclusion_group"`
	Eligibility          EligibilityPolicy `json:"eligibility"`
	Allocation           []Allocation      `json:"allocation"`
	Collection           []CollectionField `json:"collection"`
	RetentionDays        int               `json:"retention_days"`
	ApproverIDs          []string          `json:"approver_ids"`
	ChangeReason         string            `json:"change_reason"`
}
type AudiencePolicyApproval struct {
	ActorID   string    `json:"actor_id"`
	Decision  string    `json:"decision"`
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}
type AssignmentAudit struct {
	ID            string    `json:"id"`
	SubjectDigest string    `json:"subject_digest"`
	VariantID     string    `json:"variant_id,omitempty"`
	Decision      string    `json:"decision"`
	Reason        string    `json:"reason"`
	PolicyVersion int64     `json:"policy_version"`
	CreatedAt     time.Time `json:"created_at"`
}
type AudiencePolicy struct {
	ID                   string                   `json:"id"`
	Version              int64                    `json:"version"`
	PlanVersion          int64                    `json:"plan_version"`
	ReleaseID            string                   `json:"release_id"`
	ReleaseCommitID      string                   `json:"release_commit_id"`
	VariantIDs           []string                 `json:"variant_ids"`
	MutualExclusionGroup string                   `json:"mutual_exclusion_group"`
	Eligibility          EligibilityPolicy        `json:"eligibility"`
	Allocation           []Allocation             `json:"allocation"`
	Collection           []CollectionField        `json:"collection"`
	RetentionDays        int                      `json:"retention_days"`
	ApproverIDs          []string                 `json:"approver_ids"`
	ChangeReason         string                   `json:"change_reason"`
	AuthorID             string                   `json:"author_id"`
	CreatedAt            time.Time                `json:"created_at"`
	Approvals            []AudiencePolicyApproval `json:"approvals"`
	Assignments          []AssignmentAudit        `json:"assignment_audits"`
	Blockers             []Blocker                `json:"blockers"`
	Ready                bool                     `json:"ready"`
	AssignmentAuthority  bool                     `json:"assignment_authority"`
}
type AssignmentInput struct {
	Subject        string   `json:"subject"`
	Region         string   `json:"region"`
	OrganizationID string   `json:"organization_id"`
	ConsentClasses []string `json:"consent_classes"`
	Attributes     []string `json:"attributes"`
	ExistingGroups []string `json:"existing_groups"`
}

func validAudiencePolicy(p PlanVersion, in AudiencePolicyInput) bool {
	if in.ExpectedPlanVersion != p.Number || in.ReleaseID == "" || in.MutualExclusionGroup == "" || in.Eligibility.ConsentClass == "" || in.RetentionDays < 1 || in.RetentionDays > 730 || len(in.ApproverIDs) == 0 || in.ChangeReason == "" || !declaredVariants(p, in.VariantIDs) || len(in.VariantIDs) != len(p.Variants) || len(in.Allocation) != len(in.VariantIDs) || len(in.Collection) == 0 {
		return false
	}
	total, weights := 0, map[string]bool{}
	for _, a := range in.Allocation {
		if a.BasisPoints < 0 || weights[a.VariantID] || !contains(in.VariantIDs, a.VariantID) {
			return false
		}
		weights[a.VariantID] = true
		total += a.BasisPoints
	}
	if total != 10000 {
		return false
	}
	for _, c := range in.Collection {
		if c.SignalID == "" || c.SignalVersion < 1 || len(c.Properties) == 0 {
			return false
		}
	}
	return true
}

func (s *Store) PutAudiencePolicy(repo, eid, actor, release, commit string, in AudiencePolicyInput) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		p := v.Versions[len(v.Versions)-1]
		if release == "" || commit == "" || !validAudiencePolicy(p, in) {
			return ErrInvalid
		}
		version := int64(1)
		if len(v.AudiencePolicies) > 0 {
			version = v.AudiencePolicies[len(v.AudiencePolicies)-1].Version + 1
		}
		v.AudiencePolicies = append(v.AudiencePolicies, AudiencePolicy{ID: id("aud_"), Version: version, PlanVersion: p.Number, ReleaseID: release, ReleaseCommitID: commit, VariantIDs: in.VariantIDs, MutualExclusionGroup: in.MutualExclusionGroup, Eligibility: in.Eligibility, Allocation: in.Allocation, Collection: in.Collection, RetentionDays: in.RetentionDays, ApproverIDs: in.ApproverIDs, ChangeReason: in.ChangeReason, AuthorID: actor, CreatedAt: s.now()})
		return nil
	})
}
func (s *Store) ApproveAudiencePolicy(repo, eid, actor, decision, note string) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		if len(v.AudiencePolicies) == 0 || !one(decision, "approved", "changes_requested") {
			return ErrInvalid
		}
		p := &v.AudiencePolicies[len(v.AudiencePolicies)-1]
		if !contains(p.ApproverIDs, actor) {
			return ErrInvalid
		}
		p.Approvals = append(p.Approvals, AudiencePolicyApproval{ActorID: actor, Decision: decision, Note: note, CreatedAt: s.now()})
		return nil
	})
}
func (s *Store) Assign(repo, eid, actor string, in AssignmentInput) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		if len(v.AudiencePolicies) == 0 || in.Subject == "" {
			return ErrInvalid
		}
		p := &v.AudiencePolicies[len(v.AudiencePolicies)-1]
		resolved := s.resolve(repo, *v)
		current := resolved.AudiencePolicies[len(resolved.AudiencePolicies)-1]
		if !current.Ready {
			return ErrConflict
		}
		digest := sha256.Sum256([]byte(repo + ":" + eid + ":" + in.Subject))
		subject := hex.EncodeToString(digest[:])
		for _, a := range p.Assignments {
			if a.SubjectDigest == subject {
				return nil
			}
		}
		decision, reason, variant := "assigned", "eligible and consented", ""
		if !contains(in.ConsentClasses, p.Eligibility.ConsentClass) {
			decision, reason = "excluded", "required consent is absent"
		}
		if decision == "assigned" && len(p.Eligibility.Regions) > 0 && !contains(p.Eligibility.Regions, in.Region) {
			decision, reason = "excluded", "region is not eligible"
		}
		if decision == "assigned" && len(p.Eligibility.OrganizationIDs) > 0 && !contains(p.Eligibility.OrganizationIDs, in.OrganizationID) {
			decision, reason = "excluded", "organization is not eligible"
		}
		for _, x := range p.Eligibility.RequiredAttributes {
			if decision == "assigned" && !contains(in.Attributes, x) {
				decision, reason = "excluded", "required eligibility attribute is absent"
			}
		}
		for _, x := range p.Eligibility.ExcludedAttributes {
			if decision == "assigned" && contains(in.Attributes, x) {
				decision, reason = "excluded", "exclusion applies"
			}
		}
		if decision == "assigned" && contains(in.ExistingGroups, p.MutualExclusionGroup) {
			decision, reason = "excluded", "mutually exclusive assignment already exists"
		}
		if decision == "assigned" {
			bucket := int(digest[0])<<8 | int(digest[1])
			point := bucket * 10000 / 65536
			n := 0
			for _, a := range p.Allocation {
				n += a.BasisPoints
				if point < n {
					variant = a.VariantID
					break
				}
			}
		}
		p.Assignments = append(p.Assignments, AssignmentAudit{ID: id("asn_"), SubjectDigest: subject, VariantID: variant, Decision: decision, Reason: reason, PolicyVersion: p.Version, CreatedAt: s.now()})
		_ = actor
		return nil
	})
}

func (s *Store) AddWorkItem(repo, eid, actor string, item WorkItem) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		p := v.Versions[len(v.Versions)-1]
		if !one(item.Kind, "task", "session", "workspace") || !one(item.OwnerKind, "human", "agent") || strings.TrimSpace(item.OwnerID) == "" || strings.TrimSpace(item.ResourceID) == "" || strings.TrimSpace(item.Revision) == "" || !declaredVariants(p, item.VariantIDs) {
			return ErrInvalid
		}
		item.ID, item.PlanVersion, item.CreatedByID, item.CreatedAt = id("work_"), v.CurrentVersion, actor, s.now()
		v.WorkItems = append(v.WorkItems, item)
		return nil
	})
}

func (s *Store) AddImplementation(repo, eid, actor, commit string, in ImplementationInput) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		p := v.Versions[len(v.Versions)-1]
		if strings.TrimSpace(in.PullRequestID) == "" || strings.TrimSpace(commit) == "" || !declaredVariants(p, in.VariantIDs) || len(in.ExposureRules) == 0 || strings.TrimSpace(in.PrivacyClassification) == "" || strings.TrimSpace(in.RemovalPlan) == "" {
			return ErrInvalid
		}
		for _, kind := range []string{"assignment", "metric_capture", "variant_isolation", "fallback"} {
			if strings.TrimSpace(in.CheckNames[kind]) == "" {
				return ErrInvalid
			}
		}
		if len(in.EventDefinitions) != len(p.Measures) {
			return ErrInvalid
		}
		for _, m := range p.Measures {
			matched := false
			for _, d := range in.EventDefinitions {
				if d.SignalID == m.SignalID && d.SignalVersion == m.SignalVersion && strings.TrimSpace(d.Event) != "" {
					matched = true
				}
			}
			if !matched {
				return ErrInvalid
			}
		}
		v.Implementations = append(v.Implementations, Implementation{ID: id("impl_"), PlanVersion: v.CurrentVersion, ImplementationInput: in, SourceCommitID: commit, AuthorID: actor, CreatedAt: s.now(), Current: true})
		return nil
	})
}

func declaredVariants(p PlanVersion, ids []string) bool {
	if len(ids) == 0 {
		return false
	}
	for _, wanted := range ids {
		found := false
		for _, v := range p.Variants {
			if v.ID == wanted {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
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
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Store{root: root, now: func() time.Time { return time.Now().UTC() }}, nil
}
func id(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}
func (s *Store) path(repo, kind, item string) string {
	return filepath.Join(s.root, repo, kind, item+".json")
}
func write(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, b, 0600)
}
func read[T any](path string) (T, error) {
	var v T
	b, e := os.ReadFile(path)
	if os.IsNotExist(e) {
		return v, ErrNotFound
	}
	if e != nil {
		return v, e
	}
	e = json.Unmarshal(b, &v)
	return v, e
}
func validSignal(in SignalVersion) bool {
	return strings.TrimSpace(in.Name) != "" && strings.TrimSpace(in.Description) != "" && strings.TrimSpace(in.Unit) != "" && strings.TrimSpace(in.Event) != "" && len(in.PermittedAudiences) > 0 && strings.TrimSpace(in.ChangeReason) != ""
}
func (s *Store) CreateSignal(repo, actor string, in SignalVersion) (Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validSignal(in) {
		return Signal{}, ErrInvalid
	}
	in.Version = 1
	in.AuthorID = actor
	in.CreatedAt = s.now()
	v := Signal{ID: id("sig_"), RepositoryID: repo, CurrentVersion: 1, Versions: []SignalVersion{in}}
	return v, write(s.path(repo, "signals", v.ID), v)
}
func (s *Store) ReviseSignal(repo, sid, actor string, expected int64, in SignalVersion) (Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := read[Signal](s.path(repo, "signals", sid))
	if e != nil {
		return v, e
	}
	if v.CurrentVersion != expected {
		return v, ErrConflict
	}
	if !validSignal(in) {
		return v, ErrInvalid
	}
	in.Version = expected + 1
	in.AuthorID = actor
	in.CreatedAt = s.now()
	v.CurrentVersion++
	v.Versions = append(v.Versions, in)
	return v, write(s.path(repo, "signals", sid), v)
}
func (s *Store) Signals(repo string) ([]Signal, error) {
	paths, e := filepath.Glob(filepath.Join(s.root, repo, "signals", "*.json"))
	if e != nil {
		return nil, e
	}
	out := []Signal{}
	for _, p := range paths {
		v, x := read[Signal](p)
		if x != nil {
			return nil, x
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
func validPlan(in PlanInput) bool {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Hypothesis) == "" || !one(in.Source.Kind, "proposal", "issue", "decision", "pull_request", "preview", "release") || in.Source.ID == "" || len(in.Variants) < 2 || len(in.Measures) == 0 || in.Audience.Description == "" || len(in.Audience.Eligibility) == 0 || in.MinimumEvidence == "" || in.DurationHours < 1 || len(in.OwnerIDs) == 0 || len(in.ParticipantIDs) == 0 || len(in.StopConditions) == 0 || in.ChangeReason == "" {
		return false
	}
	control := 0
	for _, v := range in.Variants {
		if v.ID == "" || v.Name == "" {
			return false
		}
		if v.Control {
			control++
		}
	}
	if control != 1 {
		return false
	}
	for _, m := range in.Measures {
		if m.ID == "" || m.Name == "" || !one(m.Kind, "success", "guardrail") || m.SignalID == "" || m.SignalVersion < 1 || m.Aggregation == "" || m.Threshold == "" {
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
func (s *Store) Create(repo, actor string, in PlanInput) (Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validPlan(in) {
		return Experiment{}, ErrInvalid
	}
	p := PlanVersion{Number: 1, PlanInput: in, AuthorID: actor, CreatedAt: s.now()}
	v := Experiment{ID: id("exp_"), RepositoryID: repo, CurrentVersion: 1, Versions: []PlanVersion{p}}
	if e := write(s.path(repo, "experiments", v.ID), v); e != nil {
		return v, e
	}
	return s.resolve(repo, v), nil
}
func (s *Store) Revise(repo, eid, actor string, expected int64, in PlanInput) (Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := read[Experiment](s.path(repo, "experiments", eid))
	if e != nil {
		return v, e
	}
	if v.CurrentVersion != expected {
		return v, ErrConflict
	}
	if !validPlan(in) {
		return v, ErrInvalid
	}
	v.CurrentVersion++
	v.Versions = append(v.Versions, PlanVersion{Number: v.CurrentVersion, PlanInput: in, AuthorID: actor, CreatedAt: s.now()})
	e = write(s.path(repo, "experiments", eid), v)
	return s.resolve(repo, v), e
}
func (s *Store) mutate(repo, eid string, fn func(*Experiment) error) (Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := read[Experiment](s.path(repo, "experiments", eid))
	if e != nil {
		return v, e
	}
	if e = fn(&v); e != nil {
		return v, e
	}
	e = write(s.path(repo, "experiments", eid), v)
	return s.resolve(repo, v), e
}
func (s *Store) Comment(repo, eid, actor, body string) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		if strings.TrimSpace(body) == "" || len(body) > 10000 {
			return ErrInvalid
		}
		v.Comments = append(v.Comments, Comment{ID: id("com_"), Version: v.CurrentVersion, Body: body, AuthorID: actor, CreatedAt: s.now()})
		return nil
	})
}
func (s *Store) Approve(repo, eid, actor, decision, note string) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		p := v.Versions[len(v.Versions)-1]
		found := false
		for _, x := range p.ParticipantIDs {
			if x == actor {
				found = true
			}
		}
		if !found || !one(decision, "approved", "changes_requested") {
			return ErrInvalid
		}
		v.Approvals = append(v.Approvals, Approval{ID: id("apr_"), Version: v.CurrentVersion, ActorID: actor, Decision: decision, Note: note, CreatedAt: s.now()})
		return nil
	})
}
func (s *Store) ChangeAssumption(repo, eid, actor, assumption, detail string) (Experiment, error) {
	return s.mutate(repo, eid, func(v *Experiment) error {
		if assumption == "" || detail == "" {
			return ErrInvalid
		}
		v.AssumptionChanges = append(v.AssumptionChanges, AssumptionChange{ID: id("chg_"), Version: v.CurrentVersion, Assumption: assumption, Detail: detail, ActorID: actor, CreatedAt: s.now()})
		return nil
	})
}
func (s *Store) Get(repo, eid string) (Experiment, error) {
	v, e := read[Experiment](s.path(repo, "experiments", eid))
	return s.resolve(repo, v), e
}
func (s *Store) List(repo string) ([]Experiment, error) {
	paths, e := filepath.Glob(filepath.Join(s.root, repo, "experiments", "*.json"))
	if e != nil {
		return nil, e
	}
	out := []Experiment{}
	for _, p := range paths {
		v, x := read[Experiment](p)
		if x != nil {
			return nil, x
		}
		out = append(out, s.resolve(repo, v))
	}
	return out, nil
}
func (s *Store) resolve(repo string, v Experiment) Experiment {
	v.Blockers = nil
	if len(v.Versions) == 0 {
		return v
	}
	p := v.Versions[len(v.Versions)-1]
	for i := range v.AudiencePolicies {
		a := &v.AudiencePolicies[i]
		a.Blockers = nil
		a.Ready = false
		a.AssignmentAuthority = false
		if a.PlanVersion != v.CurrentVersion {
			a.Blockers = append(a.Blockers, Blocker{Kind: "stale_plan", Detail: "audience policy does not bind the current experiment plan"})
		}
		implementation := false
		for _, x := range v.Implementations {
			if x.PlanVersion == a.PlanVersion && x.SourceCommitID == a.ReleaseCommitID {
				implementation = true
			}
		}
		if !implementation {
			a.Blockers = append(a.Blockers, Blocker{Kind: "stale_release", Detail: "released commit is not a current exact implementation"})
		}
		for _, approver := range a.ApproverIDs {
			decision := ""
			for j := len(a.Approvals) - 1; j >= 0; j-- {
				if a.Approvals[j].ActorID == approver {
					decision = a.Approvals[j].Decision
					break
				}
			}
			if decision != "approved" {
				a.Blockers = append(a.Blockers, Blocker{Kind: "missing_audience_approval", Detail: approver + " has not approved this audience policy"})
			}
		}
		for _, c := range a.Collection {
			found := false
			for _, sig := range mustSignals(s, repo) {
				if sig.ID == c.SignalID && c.SignalVersion <= sig.CurrentVersion {
					sv := sig.Versions[c.SignalVersion-1]
					found = contains(sv.PermittedAudiences, a.Eligibility.ConsentClass)
					for _, prop := range c.Properties {
						if !contains(sv.Properties, prop) {
							found = false
						}
					}
				}
			}
			if !found {
				a.Blockers = append(a.Blockers, Blocker{Kind: "unauthorized_collection", Detail: "collection exceeds the exact signal consent or property policy", ResourceID: c.SignalID})
			}
		}
		for j := range v.AudiencePolicies {
			other := v.AudiencePolicies[j]
			if j != i && other.Version > a.Version && other.MutualExclusionGroup == a.MutualExclusionGroup {
				a.Blockers = append(a.Blockers, Blocker{Kind: "conflicting_allocation", Detail: "a newer allocation uses the mutually exclusive group", ResourceID: other.ID})
			}
		}
		a.Ready = len(a.Blockers) == 0
	}
	for i := range v.Implementations {
		v.Implementations[i].Current = v.Implementations[i].PlanVersion == v.CurrentVersion
	}
	for i := range v.Runs {
		r := &v.Runs[i]
		r.ExposureAuthority = (r.Status == "running") && r.PlanVersion == v.CurrentVersion
		if len(v.AudiencePolicies) == 0 || r.AudiencePolicyVersion != v.AudiencePolicies[len(v.AudiencePolicies)-1].Version {
			r.ExposureAuthority = false
			if r.Status == "running" {
				r.Status = "contained"
				r.ContainmentReason = "audience policy changed"
			}
		}
	}
	signals, _ := s.Signals(repo)
	sm := map[string]Signal{}
	for _, x := range signals {
		sm[x.ID] = x
	}
	for _, m := range p.Measures {
		x, ok := sm[m.SignalID]
		if !ok || m.SignalVersion > x.CurrentVersion {
			v.Blockers = append(v.Blockers, Blocker{Kind: "missing_instrumentation", Detail: m.Name + " references an unavailable signal version", ResourceID: m.SignalID})
			continue
		}
		sv := x.Versions[m.SignalVersion-1]
		if !sv.Instrumented {
			v.Blockers = append(v.Blockers, Blocker{Kind: "missing_instrumentation", Detail: sv.Name + " is not instrumented", ResourceID: x.ID})
		}
		if !contains(sv.PermittedAudiences, p.Audience.Consent) {
			v.Blockers = append(v.Blockers, Blocker{Kind: "ineligible_audience", Detail: sv.Name + " does not permit audience consent class " + p.Audience.Consent, ResourceID: x.ID})
		}
	}
	all, _ := s.raw(repo)
	for _, x := range all {
		if x.ID == v.ID || len(x.Versions) == 0 {
			continue
		}
		q := x.Versions[len(x.Versions)-1]
		for _, a := range p.OverlapKeys {
			if contains(q.OverlapKeys, a) {
				v.Blockers = append(v.Blockers, Blocker{Kind: "overlapping_experiment", Detail: "audience or surface overlaps " + x.ID + " at " + a, ResourceID: x.ID})
			}
		}
	}
	for _, c := range v.AssumptionChanges {
		if c.Version == v.CurrentVersion {
			v.Blockers = append(v.Blockers, Blocker{Kind: "changed_assumption", Detail: c.Assumption + ": " + c.Detail, ResourceID: c.ID})
		}
	}
	for _, participant := range p.ParticipantIDs {
		decision := ""
		for i := len(v.Approvals) - 1; i >= 0; i-- {
			a := v.Approvals[i]
			if a.Version == v.CurrentVersion && a.ActorID == participant {
				decision = a.Decision
				break
			}
		}
		if decision != "approved" {
			v.Blockers = append(v.Blockers, Blocker{Kind: "missing_approval", Detail: participant + " has not approved the current plan"})
		}
	}
	v.Ready = len(v.Blockers) == 0
	return v
}
func mustSignals(s *Store, repo string) []Signal { v, _ := s.Signals(repo); return v }
func (s *Store) raw(repo string) ([]Experiment, error) {
	paths, e := filepath.Glob(filepath.Join(s.root, repo, "experiments", "*.json"))
	out := []Experiment{}
	for _, p := range paths {
		v, x := read[Experiment](p)
		if x != nil {
			return nil, x
		}
		out = append(out, v)
	}
	return out, e
}
func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
