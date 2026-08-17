// Package supportquestions owns durable, audience-scoped developer support threads.
package supportquestions

import (
	"crypto/rand"
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

var (
	ErrNotFound = errors.New("support question not found")
	ErrInvalid  = errors.New("invalid support question")
)

type Subject struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Label      string `json:"label,omitempty"`
}
type Contact struct {
	Preference string `json:"preference"`
	Value      string `json:"value,omitempty"`
}
type Evidence struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	MediaType  string `json:"media_type"`
	Content    string `json:"content,omitempty"`
	Visibility string `json:"visibility"`
}
type Comment struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"author_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
type Event struct {
	Sequence  int64     `json:"sequence"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
type Related struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	Title      string `json:"title"`
	Status     string `json:"status"`
}

// Guidance is intentionally claim-addressable: a reader can tell which parts
// are supported, inferred, or still uncertain without treating an answer as a
// single verified blob.
type Citation struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Path       string `json:"path,omitempty"`
	Symbol     string `json:"symbol,omitempty"`
	LineStart  int    `json:"line_start,omitempty"`
	LineEnd    int    `json:"line_end,omitempty"`
	Label      string `json:"label,omitempty"`
	Visibility string `json:"visibility"`
}
type Claim struct {
	ID          string     `json:"id"`
	Text        string     `json:"text"`
	Mode        string     `json:"mode"` // verified, inference, or uncertainty
	Citations   []Citation `json:"citations"`
	Uncertainty string     `json:"uncertainty,omitempty"`
}
type GuidanceComment struct {
	ID         string    `json:"id"`
	RevisionID string    `json:"revision_id"`
	ClaimID    string    `json:"claim_id,omitempty"`
	Kind       string    `json:"kind"` // comment, clarification, challenge, endorsement
	Body       string    `json:"body,omitempty"`
	ActorID    string    `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type AnswerRevision struct {
	ID                 string    `json:"id"`
	AnswerID           string    `json:"answer_id"`
	Revision           int64     `json:"revision"`
	SupersedesID       string    `json:"supersedes_id,omitempty"`
	AuthorID           string    `json:"author_id"`
	AuthorKind         string    `json:"author_kind"`
	Summary            string    `json:"summary"`
	Instructions       []string  `json:"instructions"`
	ApplicableVersions []string  `json:"applicable_versions"`
	Claims             []Claim   `json:"claims"`
	Uncertainty        string    `json:"uncertainty,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}
type Answer struct {
	ID        string            `json:"id"`
	CurrentID string            `json:"current_revision_id"`
	Revisions []AnswerRevision  `json:"revisions"`
	Feedback  []GuidanceComment `json:"feedback"`
}
type AnswerInput struct {
	AnswerID           string   `json:"answer_id,omitempty"`
	SupersedesID       string   `json:"supersedes_id,omitempty"`
	AuthorKind         string   `json:"author_kind"`
	Summary            string   `json:"summary"`
	Instructions       []string `json:"instructions"`
	ApplicableVersions []string `json:"applicable_versions"`
	Claims             []Claim  `json:"claims"`
	Uncertainty        string   `json:"uncertainty,omitempty"`
}

type SolutionLink struct {
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id,omitempty"`
	Label      string `json:"label,omitempty"`
}
type SolutionCredit struct {
	ActorID string   `json:"actor_id"`
	Roles   []string `json:"roles"`
}
type SolutionNotification struct {
	ID        string    `json:"id"`
	Recipient string    `json:"recipient_id"`
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	CreatedAt time.Time `json:"created_at"`
}
type SolutionEvent struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	ActorID        string    `json:"actor_id"`
	Reason         string    `json:"reason,omitempty"`
	TargetQuestion string    `json:"target_question_id,omitempty"`
	TargetSolution string    `json:"target_solution_id,omitempty"`
	Version        string    `json:"version,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}
type Solution struct {
	ID                 string                 `json:"id"`
	AnswerID           string                 `json:"answer_id"`
	AnswerRevisionID   string                 `json:"answer_revision_id"`
	VerificationID     string                 `json:"verification_id"`
	Title              string                 `json:"title"`
	Summary            string                 `json:"summary"`
	ApplicableVersions []string               `json:"applicable_versions"`
	Limitations        []string               `json:"limitations"`
	Audience           string                 `json:"audience"`
	Links              []SolutionLink         `json:"links"`
	Status             string                 `json:"status"`
	Credits            []SolutionCredit       `json:"credits"`
	Notifications      []SolutionNotification `json:"notifications"`
	Events             []SolutionEvent        `json:"events"`
	PublishedBy        string                 `json:"published_by"`
	PublishedAt        time.Time              `json:"published_at"`
}
type ResolutionInput struct {
	AnswerID           string         `json:"answer_id"`
	AnswerRevisionID   string         `json:"answer_revision_id"`
	VerificationID     string         `json:"verification_id"`
	Title              string         `json:"title"`
	Summary            string         `json:"summary"`
	ApplicableVersions []string       `json:"applicable_versions"`
	Limitations        []string       `json:"limitations"`
	Audience           string         `json:"audience"`
	Links              []SolutionLink `json:"links"`
}

type ImprovementContext struct {
	Question        string    `json:"question"`
	Goal            string    `json:"goal"`
	AttemptedSteps  []string  `json:"attempted_steps"`
	SoftwareVersion string    `json:"software_version,omitempty"`
	Environment     string    `json:"environment,omitempty"`
	Discussion      []Comment `json:"discussion"`
}
type ImprovementLink struct {
	Kind       string    `json:"kind"`
	ResourceID string    `json:"resource_id"`
	State      string    `json:"state"`
	Revision   string    `json:"revision,omitempty"`
	Summary    string    `json:"summary,omitempty"`
	AddedByID  string    `json:"added_by_id"`
	CreatedAt  time.Time `json:"created_at"`
}
type Improvement struct {
	ID                 string             `json:"id"`
	Classification     string             `json:"classification"`
	AcceptanceCriteria []string           `json:"acceptance_criteria"`
	Context            ImprovementContext `json:"context"`
	TargetKind         string             `json:"target_kind"`
	TargetID           string             `json:"target_id"`
	Links              []ImprovementLink  `json:"links"`
	CreatedByID        string             `json:"created_by_id"`
	CreatedAt          time.Time          `json:"created_at"`
}

type Question struct {
	ID              string        `json:"id"`
	RepositoryID    string        `json:"repository_id"`
	AuthorID        string        `json:"author_id"`
	Title           string        `json:"title"`
	Question        string        `json:"question"`
	Subject         Subject       `json:"subject"`
	SoftwareVersion string        `json:"software_version,omitempty"`
	Environment     string        `json:"environment,omitempty"`
	Goal            string        `json:"goal"`
	AttemptedSteps  []string      `json:"attempted_steps"`
	Urgency         string        `json:"urgency"`
	Audience        string        `json:"audience"`
	Contact         Contact       `json:"contact"`
	Status          string        `json:"status"`
	MissingContext  []string      `json:"missing_context"`
	Evidence        []Evidence    `json:"evidence"`
	Discussion      []Comment     `json:"discussion"`
	History         []Event       `json:"history"`
	Related         []Related     `json:"related"`
	Answers         []Answer      `json:"answers"`
	Solutions       []Solution    `json:"solutions"`
	Improvements    []Improvement `json:"improvements"`
	Version         int64         `json:"version"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

func (s *Store) CreateImprovement(repo, question, actor, classification, targetKind, targetID string, criteria []string, discussionIDs []string) (Question, Improvement, error) {
	classification, targetKind, targetID = strings.TrimSpace(classification), strings.TrimSpace(targetKind), strings.TrimSpace(targetID)
	if actor == "" || targetID == "" || !map[string]bool{"defect": true, "documentation_gap": true, "missing_example": true, "compatibility_problem": true, "product_opportunity": true}[classification] || !map[string]bool{"issue": true, "documentation_task": true, "proposal": true}[targetKind] || len(criteria) == 0 || len(criteria) > 20 {
		return Question{}, Improvement{}, ErrInvalid
	}
	var made Improvement
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, question)
	if err != nil {
		return v, made, err
	}
	wanted := map[string]bool{}
	for _, id := range discussionIDs {
		wanted[id] = true
	}
	thread := []Comment{}
	for _, comment := range v.Discussion {
		if wanted[comment.ID] {
			thread = append(thread, comment)
		}
	}
	if len(thread) != len(wanted) {
		return v, made, ErrInvalid
	}
	clean := make([]string, 0, len(criteria))
	for _, item := range criteria {
		item = strings.TrimSpace(item)
		if item == "" || len(item) > 2000 {
			return v, made, ErrInvalid
		}
		clean = append(clean, item)
	}
	now := s.now().UTC()
	id, err := newID()
	if err != nil {
		return v, made, err
	}
	made = Improvement{ID: id, Classification: classification, AcceptanceCriteria: clean, Context: ImprovementContext{Question: v.Question, Goal: v.Goal, AttemptedSteps: append([]string{}, v.AttemptedSteps...), SoftwareVersion: v.SoftwareVersion, Environment: v.Environment, Discussion: thread}, TargetKind: targetKind, TargetID: targetID, Links: []ImprovementLink{}, CreatedByID: actor, CreatedAt: now}
	v.Improvements = append(v.Improvements, made)
	v.Version++
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "improvement.created", ActorID: actor, Detail: id, CreatedAt: now})
	return v, made, s.write(v)
}

func (s *Store) AddImprovementLink(repo, question, improvement, actor string, link ImprovementLink) (Question, error) {
	if actor == "" || link.ResourceID == "" || !map[string]bool{"pull_request": true, "check": true, "preview": true, "release": true, "documentation_publication": true}[link.Kind] || !map[string]bool{"queued": true, "in_progress": true, "succeeded": true, "failed": true, "published": true}[link.State] {
		return Question{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, question)
	if err != nil {
		return v, err
	}
	for i := range v.Improvements {
		if v.Improvements[i].ID == improvement {
			link.AddedByID = actor
			link.CreatedAt = s.now().UTC()
			v.Improvements[i].Links = append(v.Improvements[i].Links, link)
			v.Version++
			v.UpdatedAt = link.CreatedAt
			v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "improvement.progress", ActorID: actor, Detail: link.ResourceID, CreatedAt: link.CreatedAt})
			return v, s.write(v)
		}
	}
	return v, ErrNotFound
}

type Input struct {
	Title           string     `json:"title"`
	Question        string     `json:"question"`
	Subject         Subject    `json:"subject"`
	SoftwareVersion string     `json:"software_version"`
	Environment     string     `json:"environment"`
	Goal            string     `json:"goal"`
	AttemptedSteps  []string   `json:"attempted_steps"`
	Urgency         string     `json:"urgency"`
	Audience        string     `json:"audience"`
	Contact         Contact    `json:"contact"`
	Evidence        []Evidence `json:"evidence"`
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
	root, _ = filepath.Abs(root)
	if err := os.MkdirAll(root, 0750); err != nil {
		return nil, err
	}
	return &Store{root: root, now: time.Now}, nil
}

func missing(in Input) []string {
	out := []string{}
	if strings.TrimSpace(in.SoftwareVersion) == "" {
		out = append(out, "software_version")
	}
	if strings.TrimSpace(in.Environment) == "" {
		out = append(out, "environment")
	}
	if len(cleanSteps(in.AttemptedSteps)) == 0 {
		out = append(out, "attempted_steps")
	}
	return out
}
func cleanSteps(in []string) []string {
	out := []string{}
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
func valid(in Input) bool {
	if strings.TrimSpace(in.Title) == "" || len(in.Title) > 200 || strings.TrimSpace(in.Question) == "" || strings.TrimSpace(in.Goal) == "" || len(in.AttemptedSteps) > 50 {
		return false
	}
	if !map[string]bool{"repository": true, "package": true, "release": true, "api": true, "journey": true, "error": true}[in.Subject.Kind] || (in.Subject.Kind != "repository" && strings.TrimSpace(in.Subject.ResourceID) == "") {
		return false
	}
	if !map[string]bool{"low": true, "normal": true, "high": true, "urgent": true}[in.Urgency] || !map[string]bool{"public": true, "repository": true}[in.Audience] {
		return false
	}
	if !map[string]bool{"none": true, "thread": true, "email": true}[in.Contact.Preference] || (in.Contact.Preference == "email" && !strings.Contains(in.Contact.Value, "@")) {
		return false
	}
	if len(in.Evidence) > 10 {
		return false
	}
	total := 0
	for _, e := range in.Evidence {
		b, err := base64.StdEncoding.DecodeString(e.Content)
		total += len(b)
		if err != nil || len(b) == 0 || len(b) > 1<<20 || strings.TrimSpace(e.Name) == "" || !map[string]bool{"log": true, "configuration": true, "sample_code": true}[e.Kind] || !map[string]bool{"text/plain": true, "application/json": true, "application/yaml": true, "text/yaml": true, "text/x-go": true, "text/javascript": true, "text/typescript": true}[e.MediaType] || !map[string]bool{"audience": true, "maintainers": true}[e.Visibility] {
			return false
		}
	}
	return total <= 5<<20
}
func (s *Store) Create(repo, actor string, in Input) (Question, error) {
	if repo == "" || actor == "" || !valid(in) {
		return Question{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, e := newID()
	if e != nil {
		return Question{}, e
	}
	now := s.now().UTC()
	gaps := missing(in)
	status := "open"
	if len(gaps) > 0 {
		status = "needs_context"
	}
	for i := range in.Evidence {
		in.Evidence[i].ID, _ = newID()
	}
	v := Question{ID: id, RepositoryID: repo, AuthorID: actor, Title: strings.TrimSpace(in.Title), Question: strings.TrimSpace(in.Question), Subject: in.Subject, SoftwareVersion: strings.TrimSpace(in.SoftwareVersion), Environment: strings.TrimSpace(in.Environment), Goal: strings.TrimSpace(in.Goal), AttemptedSteps: cleanSteps(in.AttemptedSteps), Urgency: in.Urgency, Audience: in.Audience, Contact: in.Contact, Status: status, MissingContext: gaps, Evidence: in.Evidence, Discussion: []Comment{}, History: []Event{{Sequence: 1, Type: "question.opened", ActorID: actor, CreatedAt: now}}, Related: []Related{}, Answers: []Answer{}, Solutions: []Solution{}, Version: 1, CreatedAt: now, UpdatedAt: now}
	return v, s.write(v)
}

func validAnswer(in AnswerInput) bool {
	if strings.TrimSpace(in.Summary) == "" || len(in.Summary) > 65536 || len(in.Instructions) == 0 || len(in.Instructions) > 100 || len(in.Claims) == 0 || len(in.Claims) > 100 || len(in.ApplicableVersions) == 0 || len(in.ApplicableVersions) > 50 {
		return false
	}
	if len(cleanSteps(in.Instructions)) == 0 || len(cleanSteps(in.ApplicableVersions)) == 0 {
		return false
	}
	if !map[string]bool{"human": true, "agent": true}[in.AuthorKind] || (in.AuthorKind == "agent" && strings.TrimSpace(in.Uncertainty) == "") {
		return false
	}
	for _, c := range in.Claims {
		if strings.TrimSpace(c.Text) == "" || !map[string]bool{"verified": true, "inference": true, "uncertainty": true}[c.Mode] || len(c.Citations) == 0 || (c.Mode != "verified" && strings.TrimSpace(c.Uncertainty) == "") {
			return false
		}
	}
	return true
}

func (s *Store) ReviseAnswer(repo, question, actor string, in AnswerInput) (Question, error) {
	if actor == "" || !validAnswer(in) {
		return Question{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, question)
	if e != nil {
		return v, e
	}
	now := s.now().UTC()
	answerIndex := -1
	if in.AnswerID != "" {
		for i := range v.Answers {
			if v.Answers[i].ID == in.AnswerID {
				answerIndex = i
				break
			}
		}
		if answerIndex < 0 {
			return Question{}, ErrInvalid
		}
	}
	if answerIndex < 0 {
		aid, _ := newID()
		v.Answers = append(v.Answers, Answer{ID: aid, Revisions: []AnswerRevision{}, Feedback: []GuidanceComment{}})
		answerIndex = len(v.Answers) - 1
	}
	a := &v.Answers[answerIndex]
	if len(a.Revisions) > 0 && (in.SupersedesID == "" || in.SupersedesID != a.CurrentID) {
		return Question{}, ErrInvalid
	}
	rid, _ := newID()
	for i := range in.Claims {
		in.Claims[i].ID, _ = newID()
	}
	r := AnswerRevision{ID: rid, AnswerID: a.ID, Revision: int64(len(a.Revisions) + 1), SupersedesID: in.SupersedesID, AuthorID: actor, AuthorKind: in.AuthorKind, Summary: strings.TrimSpace(in.Summary), Instructions: cleanSteps(in.Instructions), ApplicableVersions: cleanSteps(in.ApplicableVersions), Claims: in.Claims, Uncertainty: strings.TrimSpace(in.Uncertainty), CreatedAt: now}
	a.Revisions = append(a.Revisions, r)
	a.CurrentID = rid
	v.Version++
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "answer.revised", ActorID: actor, Detail: rid, CreatedAt: now})
	return v, s.write(v)
}

func (s *Store) Feedback(repo, question, answer, revision, claim, actor, kind, body string) (Question, error) {
	if actor == "" || !map[string]bool{"comment": true, "clarification": true, "challenge": true, "endorsement": true}[kind] || (kind != "endorsement" && strings.TrimSpace(body) == "") {
		return Question{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, question)
	if e != nil {
		return v, e
	}
	var target *Answer
	for i := range v.Answers {
		if v.Answers[i].ID == answer {
			target = &v.Answers[i]
			break
		}
	}
	if target == nil {
		return Question{}, ErrInvalid
	}
	validRevision, validClaim := false, claim == ""
	for _, r := range target.Revisions {
		if r.ID == revision {
			validRevision = true
			for _, c := range r.Claims {
				if c.ID == claim {
					validClaim = true
				}
			}
			break
		}
	}
	if !validRevision || !validClaim {
		return Question{}, ErrInvalid
	}
	id, _ := newID()
	now := s.now().UTC()
	target.Feedback = append(target.Feedback, GuidanceComment{ID: id, RevisionID: revision, ClaimID: claim, Kind: kind, Body: strings.TrimSpace(body), ActorID: actor, CreatedAt: now})
	v.Version++
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "answer." + kind, ActorID: actor, Detail: revision, CreatedAt: now})
	return v, s.write(v)
}

func validResolution(q Question, in ResolutionInput, verification VerificationAttempt) (*AnswerRevision, bool) {
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Summary) == "" || len(in.Title) > 200 || len(in.Summary) > 65536 || len(cleanSteps(in.ApplicableVersions)) == 0 || len(in.ApplicableVersions) > 50 || len(in.Limitations) > 50 || len(in.Links) > 50 || !map[string]bool{"public": true, "repository": true}[in.Audience] || (q.Audience == "repository" && in.Audience == "public") || verification.State != "passed" || verification.RepositoryID != q.RepositoryID || verification.QuestionID != q.ID || verification.AnswerID != in.AnswerID || verification.AnswerRevisionID != in.AnswerRevisionID || verification.ID != in.VerificationID {
		return nil, false
	}
	for _, link := range in.Links {
		if !map[string]bool{"documentation": true, "package": true, "release": true, "contributor_guidance": true}[link.Kind] || strings.TrimSpace(link.ResourceID) == "" {
			return nil, false
		}
	}
	for i := range q.Answers {
		if q.Answers[i].ID != in.AnswerID {
			continue
		}
		if q.Answers[i].CurrentID != in.AnswerRevisionID {
			return nil, false
		}
		for j := range q.Answers[i].Revisions {
			r := &q.Answers[i].Revisions[j]
			if r.ID == in.AnswerRevisionID {
				allowed := map[string]bool{}
				for _, version := range r.ApplicableVersions {
					allowed[version] = true
				}
				for _, version := range cleanSteps(in.ApplicableVersions) {
					if !allowed[version] {
						return nil, false
					}
				}
				return r, true
			}
		}
	}
	return nil, false
}

// Resolve publishes a new immutable solution from one exact, successfully
// verified answer revision. Later lifecycle events never rewrite this record.
func (s *Store) Resolve(repo, question, actor string, in ResolutionInput, verification VerificationAttempt) (Question, error) {
	if actor == "" {
		return Question{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, question)
	if e != nil {
		return v, e
	}
	revision, ok := validResolution(v, in, verification)
	if !ok {
		return Question{}, ErrInvalid
	}
	for _, solution := range v.Solutions {
		if solution.AnswerRevisionID == in.AnswerRevisionID && solution.Status != "archived" && solution.Status != "merged" {
			return Question{}, ErrInvalid
		}
	}
	now := s.now().UTC()
	id, _ := newID()
	roles := map[string]map[string]bool{}
	addRole := func(id, role string) {
		if id == "" {
			return
		}
		if roles[id] == nil {
			roles[id] = map[string]bool{}
		}
		roles[id][role] = true
	}
	addRole(v.AuthorID, "asker")
	addRole(revision.AuthorID, "answer_author")
	addRole(verification.CreatedByID, "verifier")
	for _, a := range v.Answers {
		for _, f := range a.Feedback {
			addRole(f.ActorID, "reviewer")
		}
	}
	for _, c := range v.Discussion {
		addRole(c.AuthorID, "participant")
	}
	credits, notices := []SolutionCredit{}, []SolutionNotification{}
	for id, set := range roles {
		r := []string{}
		for role := range set {
			r = append(r, role)
		}
		sort.Strings(r)
		credits = append(credits, SolutionCredit{ActorID: id, Roles: r})
		nid, _ := newID()
		notices = append(notices, SolutionNotification{ID: nid, Recipient: id, Type: "solution.published", ActorID: actor, CreatedAt: now})
	}
	sort.Slice(credits, func(i, j int) bool { return credits[i].ActorID < credits[j].ActorID })
	sort.Slice(notices, func(i, j int) bool { return notices[i].Recipient < notices[j].Recipient })
	eid, _ := newID()
	solution := Solution{ID: id, AnswerID: in.AnswerID, AnswerRevisionID: in.AnswerRevisionID, VerificationID: in.VerificationID, Title: strings.TrimSpace(in.Title), Summary: strings.TrimSpace(in.Summary), ApplicableVersions: cleanSteps(in.ApplicableVersions), Limitations: cleanSteps(in.Limitations), Audience: in.Audience, Links: in.Links, Status: "published", Credits: credits, Notifications: notices, Events: []SolutionEvent{{ID: eid, Type: "published", ActorID: actor, CreatedAt: now}}, PublishedBy: actor, PublishedAt: now}
	v.Solutions = append(v.Solutions, solution)
	v.Status, v.Version, v.UpdatedAt = "resolved", v.Version+1, now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "solution.published", ActorID: actor, Detail: id, CreatedAt: now})
	return v, s.write(v)
}

func (s *Store) SolutionEvent(repo, question, solution, actor, kind, reason, targetQuestion, targetSolution, version string) (Question, error) {
	if actor == "" || strings.TrimSpace(reason) == "" || len(reason) > 4000 || !map[string]bool{"archive": true, "merge": true, "request_revalidation": true}[kind] {
		return Question{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, question)
	if e != nil {
		return v, e
	}
	index := -1
	for i := range v.Solutions {
		if v.Solutions[i].ID == solution {
			index = i
		}
	}
	if index < 0 || (kind == "merge" && (targetQuestion == "" || targetSolution == "" || (targetQuestion == question && targetSolution == solution))) || (kind == "request_revalidation" && strings.TrimSpace(version) == "") {
		return Question{}, ErrInvalid
	}
	if v.Solutions[index].Status == "archived" || v.Solutions[index].Status == "merged" {
		return Question{}, ErrInvalid
	}
	now := s.now().UTC()
	eid, _ := newID()
	typeName := map[string]string{"archive": "archived", "merge": "merged", "request_revalidation": "revalidation_requested"}[kind]
	ev := SolutionEvent{ID: eid, Type: typeName, ActorID: actor, Reason: strings.TrimSpace(reason), TargetQuestion: targetQuestion, TargetSolution: targetSolution, Version: strings.TrimSpace(version), CreatedAt: now}
	v.Solutions[index].Events = append(v.Solutions[index].Events, ev)
	if kind == "archive" || kind == "merge" {
		v.Solutions[index].Status = typeName
	} else {
		v.Solutions[index].Status = "revalidation_requested"
	}
	seen := map[string]bool{}
	for _, c := range v.Solutions[index].Credits {
		seen[c.ActorID] = true
	}
	for recipient := range seen {
		nid, _ := newID()
		v.Solutions[index].Notifications = append(v.Solutions[index].Notifications, SolutionNotification{ID: nid, Recipient: recipient, Type: "solution." + typeName, ActorID: actor, CreatedAt: now})
	}
	v.Version++
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "solution." + typeName, ActorID: actor, Detail: solution, CreatedAt: now})
	return v, s.write(v)
}

func (s *Store) Solutions(repo, query, actor string, publicOnly bool) ([]Solution, error) {
	questions, e := s.List(repo)
	if e != nil {
		return nil, e
	}
	terms := strings.Fields(strings.ToLower(query))
	out := []Solution{}
	for _, q := range questions {
		for _, solution := range q.Solutions {
			if solution.Status == "archived" || solution.Status == "merged" || (publicOnly && solution.Audience != "public") {
				continue
			}
			hay := strings.ToLower(solution.Title + " " + solution.Summary + " " + strings.Join(solution.ApplicableVersions, " ") + " " + strings.Join(solution.Limitations, " "))
			match := true
			for _, term := range terms {
				if !strings.Contains(hay, term) {
					match = false
				}
			}
			if match {
				solution.Notifications = nil
				out = append(out, solution)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PublishedAt.After(out[j].PublishedAt) })
	return out, nil
}
func (s *Store) Get(repo, id string) (Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.read(repo, id)
}
func (s *Store) List(repo string) ([]Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	es, e := os.ReadDir(filepath.Join(s.root, repo))
	if errors.Is(e, fs.ErrNotExist) {
		return []Question{}, nil
	}
	if e != nil {
		return nil, e
	}
	out := []Question{}
	for _, x := range es {
		if x.IsDir() || filepath.Ext(x.Name()) != ".json" {
			continue
		}
		v, er := s.read(repo, strings.TrimSuffix(x.Name(), ".json"))
		if er != nil {
			return nil, er
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Comment(repo, id, actor, body string) (Question, error) {
	body = strings.TrimSpace(body)
	if actor == "" || body == "" || len(body) > 65536 {
		return Question{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return v, e
	}
	cid, _ := newID()
	now := s.now().UTC()
	v.Discussion = append(v.Discussion, Comment{ID: cid, AuthorID: actor, Body: body, CreatedAt: now})
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "comment.added", ActorID: actor, CreatedAt: now})
	v.Version++
	v.UpdatedAt = now
	return v, s.write(v)
}
func (s *Store) Status(repo, id, actor, status string) (Question, error) {
	if !map[string]bool{"open": true, "needs_context": true, "answered": true, "resolved": true, "closed": true}[status] {
		return Question{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return v, e
	}
	if v.Status == status {
		return v, nil
	}
	now := s.now().UTC()
	v.Status = status
	v.Version++
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "status." + status, ActorID: actor, CreatedAt: now})
	return v, s.write(v)
}
func (s *Store) SetRelated(repo, id string, related []Related) (Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, e := s.read(repo, id)
	if e != nil {
		return v, e
	}
	v.Related = related
	return v, s.write(v)
}
func (s *Store) read(repo, id string) (Question, error) {
	b, e := os.ReadFile(filepath.Join(s.root, repo, id+".json"))
	if errors.Is(e, fs.ErrNotExist) {
		return Question{}, ErrNotFound
	}
	if e != nil {
		return Question{}, e
	}
	var v Question
	if json.Unmarshal(b, &v) != nil || v.RepositoryID != repo || v.ID != id {
		return Question{}, ErrNotFound
	}
	return v, nil
}
func (s *Store) write(v Question) error {
	dir := filepath.Join(s.root, v.RepositoryID)
	if e := os.MkdirAll(dir, 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp, e := os.CreateTemp(dir, ".support-*")
	if e != nil {
		return e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Chmod(0640)
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, filepath.Join(dir, v.ID+".json"))
}
func newID() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	return hex.EncodeToString(b[:]), nil
}
