package deliveryteams

import (
	"sort"
	"strings"
	"time"
)

// Integration is an immutable review contract over the exact outputs a team
// intends to offer through ordinary pull requests.
type Integration struct {
	ID             string               `json:"id"`
	PlanVersion    int64                `json:"plan_version"`
	Status         string               `json:"status"`
	Contributions  []Contribution       `json:"contributions"`
	Blockers       []IntegrationBlocker `json:"blockers"`
	ReconciledByID string               `json:"reconciled_by_id"`
	ReconciledAt   time.Time            `json:"reconciled_at"`
}

type Contribution struct {
	StreamID           string      `json:"stream_id"`
	SourceRepositoryID string      `json:"source_repository_id"`
	SourceBranch       string      `json:"source_branch"`
	SourceCommitID     string      `json:"source_commit_id"`
	TargetBranch       string      `json:"target_branch"`
	TargetCommitID     string      `json:"target_commit_id"`
	Title              string      `json:"title"`
	Summary            string      `json:"summary"`
	EvidenceEntryIDs   []string    `json:"evidence_entry_ids"`
	HandoffIDs         []string    `json:"handoff_ids"`
	Criteria           []Criterion `json:"criteria"`
	ResidualRisks      []string    `json:"residual_risks"`
	Conflict           bool        `json:"conflict"`
	PullRequestID      string      `json:"pull_request_id,omitempty"`
	PublishedByID      string      `json:"published_by_id,omitempty"`
	PublishedAt        *time.Time  `json:"published_at,omitempty"`
}

type Criterion struct {
	Criterion        string   `json:"criterion"`
	Status           string   `json:"status"`
	EvidenceEntryIDs []string `json:"evidence_entry_ids,omitempty"`
}
type IntegrationBlocker struct {
	Kind     string `json:"kind"`
	StreamID string `json:"stream_id,omitempty"`
	Detail   string `json:"detail"`
}
type IntegrationInput struct {
	Contributions []Contribution `json:"contributions"`
}

func (s *Store) Reconcile(repo, team, actor string, version int64, in IntegrationInput) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, team)
	if err != nil {
		return v, err
	}
	if version != v.Version {
		return v, ErrConflict
	}
	if v.Plan == nil || v.Plan.Current.Status != "accepted" || len(in.Contributions) == 0 {
		return v, ErrInvalid
	}
	byStream := map[string]WorkStream{}
	order := map[string]int{}
	for _, stream := range v.Plan.Current.Streams {
		byStream[stream.ID] = stream
		order[stream.ID] = stream.IntegrationOrder
	}
	seen := map[string]bool{}
	blockers := []IntegrationBlocker{}
	for i := range in.Contributions {
		c := &in.Contributions[i]
		c.StreamID = strings.TrimSpace(c.StreamID)
		c.SourceRepositoryID = strings.TrimSpace(c.SourceRepositoryID)
		c.SourceBranch = strings.TrimSpace(c.SourceBranch)
		c.TargetBranch = strings.TrimSpace(c.TargetBranch)
		c.Title = strings.TrimSpace(c.Title)
		c.Summary = strings.TrimSpace(c.Summary)
		stream, ok := byStream[c.StreamID]
		if !ok || seen[c.StreamID] || c.SourceBranch == "" || c.TargetBranch == "" || c.Title == "" || c.Summary == "" || !fullCommit(c.SourceCommitID) || !fullCommit(c.TargetCommitID) {
			return v, ErrInvalid
		}
		seen[c.StreamID] = true
		if c.SourceRepositoryID == "" {
			c.SourceRepositoryID = repo
		}
		scoped := false
		for _, scope := range stream.RepositoryScope {
			scoped = scoped || scope.RepositoryID == c.SourceRepositoryID
		}
		if !scoped {
			return v, ErrForbidden
		}
		var valid bool
		c.EvidenceEntryIDs, valid = clean(c.EvidenceEntryIDs, 100)
		if !valid || len(c.EvidenceEntryIDs) == 0 {
			return v, ErrInvalid
		}
		entries := map[string]TimelineEntry{}
		for _, e := range v.Timeline {
			entries[e.ID] = e
		}
		for _, eid := range c.EvidenceEntryIDs {
			e, ok := entries[eid]
			if !ok || e.StreamID != c.StreamID || e.StreamRevision != stream.Revision {
				blockers = append(blockers, IntegrationBlocker{Kind: "missing_evidence", StreamID: c.StreamID, Detail: "evidence entry is absent or stale: " + eid})
			}
		}
		criteria := map[string]Criterion{}
		for _, x := range c.Criteria {
			x.Criterion = strings.TrimSpace(x.Criterion)
			if x.Status != "met" && x.Status != "unmet" {
				return v, ErrInvalid
			}
			criteria[x.Criterion] = x
			for _, eid := range x.EvidenceEntryIDs {
				e, ok := entries[eid]
				if !ok || e.StreamID != c.StreamID || e.StreamRevision != stream.Revision {
					blockers = append(blockers, IntegrationBlocker{Kind: "missing_evidence", StreamID: c.StreamID, Detail: "criterion evidence is absent or stale: " + eid})
				}
			}
			for _, eid := range x.EvidenceEntryIDs {
				e, ok := entries[eid]
				if !ok || e.StreamID != c.StreamID || e.StreamRevision != stream.Revision {
					blockers = append(blockers, IntegrationBlocker{Kind: "missing_evidence", StreamID: c.StreamID, Detail: "criterion evidence is absent or stale: " + eid})
				}
			}
		}
		for _, expected := range stream.AcceptanceCriteria {
			if x, ok := criteria[expected]; !ok || x.Status != "met" || len(x.EvidenceEntryIDs) == 0 {
				blockers = append(blockers, IntegrationBlocker{Kind: "acceptance_criterion_missing", StreamID: c.StreamID, Detail: expected})
			}
		}
		for _, hid := range c.HandoffIDs {
			found := false
			for _, h := range v.Handoffs {
				found = found || h.ID == hid && h.Status == "accepted" && h.StreamID == c.StreamID
			}
			if !found {
				blockers = append(blockers, IntegrationBlocker{Kind: "handoff_unaccepted", StreamID: c.StreamID, Detail: hid})
			}
		}
		if c.Conflict {
			blockers = append(blockers, IntegrationBlocker{Kind: "merge_conflict", StreamID: c.StreamID, Detail: "source conflicts with the declared target revision"})
		}
	}
	for _, stream := range v.Plan.Current.Streams {
		if !seen[stream.ID] {
			blockers = append(blockers, IntegrationBlocker{Kind: "stream_missing", StreamID: stream.ID, Detail: "accepted stream has no contribution"})
		}
	}
	sort.SliceStable(in.Contributions, func(i, j int) bool { return order[in.Contributions[i].StreamID] < order[in.Contributions[j].StreamID] })
	status := "ready"
	if len(blockers) > 0 {
		status = "blocked"
	}
	now := s.now().UTC()
	integration := Integration{ID: id(), PlanVersion: v.Plan.Current.Version, Status: status, Contributions: in.Contributions, Blockers: blockers, ReconciledByID: actor, ReconciledAt: now}
	v.Integrations = append(v.Integrations, integration)
	addEvent(&v, "integration.reconciled", actor, "", integration.ID+":"+status, now)
	return v, s.write(v)
}

func (s *Store) LinkPullRequest(repo, team, integrationID, streamID, actor, pullID string, version int64) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, team)
	if err != nil {
		return v, err
	}
	if version != v.Version {
		return v, ErrConflict
	}
	for i := range v.Integrations {
		x := &v.Integrations[i]
		if x.ID != integrationID || x.Status != "ready" {
			continue
		}
		for j := range x.Contributions {
			c := &x.Contributions[j]
			if c.StreamID == streamID && c.PullRequestID == "" {
				now := s.now().UTC()
				c.PullRequestID = pullID
				c.PublishedByID = actor
				c.PublishedAt = &now
				addEvent(&v, "integration.pull_request_published", actor, "", streamID+":"+pullID, now)
				return v, s.write(v)
			}
		}
		return v, ErrInvalid
	}
	return v, ErrNotFound
}
