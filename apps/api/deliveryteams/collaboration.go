package deliveryteams

import (
	"sort"
	"strings"
	"time"
)

type ExecutionContext struct {
	Kind         string `json:"kind"`
	ID           string `json:"id"`
	ParentID     string `json:"parent_id,omitempty"`
	RepositoryID string `json:"repository_id"`
	Revision     string `json:"revision"`
}

type Execution struct {
	ID                 string           `json:"id"`
	StreamID           string           `json:"stream_id"`
	StreamRevision     int64            `json:"stream_revision"`
	OwnerParticipantID string           `json:"owner_participant_id"`
	Context            ExecutionContext `json:"context"`
	AttachedByID       string           `json:"attached_by_id"`
	AttachedAt         time.Time        `json:"attached_at"`
}

type Citation struct {
	RepositoryID string `json:"repository_id"`
	Revision     string `json:"revision"`
	Path         string `json:"path,omitempty"`
	ResourceKind string `json:"resource_kind"`
	ResourceID   string `json:"resource_id"`
	URL          string `json:"url,omitempty"`
}

type TimelineEntry struct {
	ID                  string           `json:"id"`
	Sequence            int64            `json:"sequence"`
	StreamID            string           `json:"stream_id"`
	StreamRevision      int64            `json:"stream_revision"`
	Kind                string           `json:"kind"`
	Summary             string           `json:"summary"`
	AuthorParticipantID string           `json:"author_participant_id"`
	AuthorID            string           `json:"author_id"`
	Context             ExecutionContext `json:"context"`
	Citations           []Citation       `json:"citations"`
	CreatedAt           time.Time        `json:"created_at"`
}

type TimelineInput struct {
	StreamID  string           `json:"stream_id"`
	Kind      string           `json:"kind"`
	Summary   string           `json:"summary"`
	Context   ExecutionContext `json:"context"`
	Citations []Citation       `json:"citations"`
}

type HandoffAcceptance struct {
	ActorID    string    `json:"actor_id"`
	Note       string    `json:"note"`
	AcceptedAt time.Time `json:"accepted_at"`
}

type Handoff struct {
	ID                  string             `json:"id"`
	StreamID            string             `json:"stream_id"`
	StreamRevision      int64              `json:"stream_revision"`
	FromParticipantID   string             `json:"from_participant_id"`
	ToParticipantID     string             `json:"to_participant_id"`
	InputEntryIDs       []string           `json:"input_entry_ids"`
	InputRevisions      []string           `json:"input_revisions"`
	Context             ExecutionContext   `json:"context"`
	AcceptanceCriteria  []string           `json:"acceptance_criteria"`
	ResidualUncertainty []string           `json:"residual_uncertainty"`
	Status              string             `json:"status"`
	RequestedByID       string             `json:"requested_by_id"`
	RequestedAt         time.Time          `json:"requested_at"`
	Acceptance          *HandoffAcceptance `json:"acceptance,omitempty"`
}

type HandoffInput struct {
	StreamID            string           `json:"stream_id"`
	ToParticipantID     string           `json:"to_participant_id"`
	InputEntryIDs       []string         `json:"input_entry_ids"`
	Context             ExecutionContext `json:"context"`
	AcceptanceCriteria  []string         `json:"acceptance_criteria"`
	ResidualUncertainty []string         `json:"residual_uncertainty"`
}

func stream(v Team, streamID string) (WorkStream, bool) {
	if v.Plan == nil || v.Plan.Current.Status != "accepted" {
		return WorkStream{}, false
	}
	for _, item := range v.Plan.Current.Streams {
		if item.ID == streamID {
			return item, true
		}
	}
	return WorkStream{}, false
}

func contextAllowed(item WorkStream, context ExecutionContext) bool {
	context.Kind = strings.TrimSpace(context.Kind)
	context.ID = strings.TrimSpace(context.ID)
	validKind := context.Kind == "change_session" || context.Kind == "investigation" || context.Kind == "experiment" || context.Kind == "workspace"
	if !validKind || context.ID == "" || len(context.ID) > 300 || !fullCommit(context.Revision) {
		return false
	}
	for _, scope := range item.RepositoryScope {
		if scope.RepositoryID == context.RepositoryID && scope.CommitID == context.Revision {
			return true
		}
	}
	return false
}

func actorOwns(v Team, item WorkStream, participantID, actor string) bool {
	p, ok := participant(v, participantID)
	return ok && participantID == item.OwnerParticipantID && (p.Kind == "agent" || p.PrincipalID == actor)
}

func citationAllowed(item WorkStream, citation Citation) bool {
	citation.Path = strings.Trim(strings.TrimSpace(citation.Path), "/")
	if citation.ResourceKind == "" || citation.ResourceID == "" || !fullCommit(citation.Revision) {
		return false
	}
	for _, scope := range item.RepositoryScope {
		if scope.RepositoryID != citation.RepositoryID || scope.CommitID != citation.Revision {
			continue
		}
		if citation.Path == "" {
			return true
		}
		for _, path := range scope.Paths {
			path = strings.Trim(path, "/")
			if citation.Path == path || strings.HasPrefix(citation.Path, path+"/") {
				return true
			}
		}
	}
	return false
}

func executionAttached(v Team, item WorkStream, context ExecutionContext) bool {
	for _, execution := range v.Executions {
		if execution.StreamID == item.ID && execution.StreamRevision == item.Revision && execution.Context == context {
			return true
		}
	}
	return false
}

func (s *Store) AttachExecution(repo, team, actor, participantID string, version int64, context ExecutionContext, streamID string) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, team)
	if err != nil {
		return v, err
	}
	if version != v.Version {
		return v, ErrConflict
	}
	item, ok := stream(v, strings.TrimSpace(streamID))
	if !ok || !actorOwns(v, item, participantID, actor) {
		return v, ErrForbidden
	}
	if !contextAllowed(item, context) {
		return v, ErrInvalid
	}
	if context.RepositoryID != v.RepositoryID {
		return v, ErrForbidden
	}
	for _, execution := range v.Executions {
		if execution.StreamID == item.ID && execution.Context.Kind == context.Kind && execution.Context.ID == context.ID {
			return v, ErrConflict
		}
	}
	now := s.now().UTC()
	v.Executions = append(v.Executions, Execution{ID: id(), StreamID: item.ID, StreamRevision: item.Revision, OwnerParticipantID: participantID, Context: context, AttachedByID: actor, AttachedAt: now})
	addEvent(&v, "stream.context_attached", actor, participantID, item.ID+":"+context.Kind+":"+context.ID, now)
	return v, s.write(v)
}

func (s *Store) PublishTimeline(repo, team, actor, participantID string, version int64, in TimelineInput) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, team)
	if err != nil {
		return v, err
	}
	if version != v.Version {
		return v, ErrConflict
	}
	item, ok := stream(v, strings.TrimSpace(in.StreamID))
	if !ok || !actorOwns(v, item, participantID, actor) {
		return v, ErrForbidden
	}
	in.Summary = strings.TrimSpace(in.Summary)
	validKind := map[string]bool{"finding": true, "question": true, "checkpoint": true, "artifact": true, "decision": true, "uncertainty": true}
	if !validKind[in.Kind] || in.Summary == "" || len(in.Summary) > 10000 || !contextAllowed(item, in.Context) || !executionAttached(v, item, in.Context) || len(in.Citations) == 0 || len(in.Citations) > 100 {
		return v, ErrInvalid
	}
	for i := range in.Citations {
		in.Citations[i].Path = strings.Trim(strings.TrimSpace(in.Citations[i].Path), "/")
		if in.Citations[i].RepositoryID != v.RepositoryID {
			return v, ErrForbidden
		}
		if !citationAllowed(item, in.Citations[i]) {
			return v, ErrInvalid
		}
	}
	now := s.now().UTC()
	entry := TimelineEntry{ID: id(), Sequence: int64(len(v.Timeline) + 1), StreamID: item.ID, StreamRevision: item.Revision, Kind: in.Kind, Summary: in.Summary, AuthorParticipantID: participantID, AuthorID: actor, Context: in.Context, Citations: in.Citations, CreatedAt: now}
	v.Timeline = append(v.Timeline, entry)
	addEvent(&v, "timeline."+in.Kind, actor, participantID, entry.ID, now)
	return v, s.write(v)
}

func (s *Store) RequestHandoff(repo, team, actor, participantID string, version int64, in HandoffInput) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, team)
	if err != nil {
		return v, err
	}
	if version != v.Version {
		return v, ErrConflict
	}
	item, ok := stream(v, strings.TrimSpace(in.StreamID))
	if !ok || !actorOwns(v, item, participantID, actor) {
		return v, ErrForbidden
	}
	if _, ok = participant(v, in.ToParticipantID); !ok || in.ToParticipantID == participantID || !contextAllowed(item, in.Context) || !executionAttached(v, item, in.Context) {
		return v, ErrInvalid
	}
	criteria, ok := clean(in.AcceptanceCriteria, 1000)
	if !ok || len(criteria) == 0 {
		return v, ErrInvalid
	}
	uncertainty, ok := clean(in.ResidualUncertainty, 2000)
	if !ok {
		return v, ErrInvalid
	}
	inputIDs, ok := clean(in.InputEntryIDs, 100)
	if !ok || len(inputIDs) == 0 {
		return v, ErrInvalid
	}
	revisions := map[string]bool{}
	for _, inputID := range inputIDs {
		found := false
		for _, entry := range v.Timeline {
			if entry.ID == inputID && entry.StreamID == item.ID {
				if entry.Context != in.Context {
					return v, ErrInvalid
				}
				found = true
				revisions[entry.Context.Revision] = true
			}
		}
		if !found {
			return v, ErrInvalid
		}
	}
	exact := make([]string, 0, len(revisions))
	for revision := range revisions {
		exact = append(exact, revision)
	}
	sort.Strings(exact)
	now := s.now().UTC()
	h := Handoff{ID: id(), StreamID: item.ID, StreamRevision: item.Revision, FromParticipantID: participantID, ToParticipantID: in.ToParticipantID, InputEntryIDs: inputIDs, InputRevisions: exact, Context: in.Context, AcceptanceCriteria: criteria, ResidualUncertainty: uncertainty, Status: "requested", RequestedByID: actor, RequestedAt: now}
	v.Handoffs = append(v.Handoffs, h)
	addEvent(&v, "handoff.requested", actor, participantID, h.ID, now)
	return v, s.write(v)
}

func (s *Store) AcceptHandoff(repo, team, handoff, actor, participantID, note string, version int64) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, team)
	if err != nil {
		return v, err
	}
	if version != v.Version {
		return v, ErrConflict
	}
	p, ok := participant(v, participantID)
	if !ok || (p.Kind == "human" && p.PrincipalID != actor) {
		return v, ErrForbidden
	}
	for i := range v.Handoffs {
		h := &v.Handoffs[i]
		if h.ID != handoff {
			continue
		}
		if h.Status != "requested" || h.ToParticipantID != participantID {
			return v, ErrForbidden
		}
		note = strings.TrimSpace(note)
		if len(note) > 4000 {
			return v, ErrInvalid
		}
		now := s.now().UTC()
		h.Status = "accepted"
		h.Acceptance = &HandoffAcceptance{ActorID: actor, Note: note, AcceptedAt: now}
		addEvent(&v, "handoff.accepted", actor, participantID, h.ID, now)
		return v, s.write(v)
	}
	return v, ErrNotFound
}
