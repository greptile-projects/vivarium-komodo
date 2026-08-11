package deliveryteams

import (
	"sort"
	"strings"
	"time"
)

// ResourceUse is reported against, but never changes, an accepted budget.
type ResourceUse struct {
	Hours     int `json:"hours"`
	CostUnits int `json:"cost_units"`
	AgentRuns int `json:"agent_runs"`
}

// StreamRun is the latest public execution projection supplied by a stream
// owner. It intentionally contains no raw logs, prompts, or credentials.
type StreamRun struct {
	StreamID            string      `json:"stream_id"`
	StreamRevision      int64       `json:"stream_revision"`
	ReportedRevision    string      `json:"reported_revision"`
	Status              string      `json:"status"`
	ActiveAction        string      `json:"active_action,omitempty"`
	Question            string      `json:"question,omitempty"`
	PredictedNextAction string      `json:"predicted_next_action,omitempty"`
	ResourceUse         ResourceUse `json:"resource_use"`
	AccessState         string      `json:"access_state"`
	OutputState         string      `json:"output_state"`
	OperationalOwnerID  string      `json:"operational_owner_id"`
	RecoveryAttempts    int         `json:"recovery_attempts"`
	ReportedByID        string      `json:"reported_by_id"`
	ReportedAt          time.Time   `json:"reported_at"`
}

type StreamStatusInput struct {
	ReportedRevision    string      `json:"reported_revision"`
	Status              string      `json:"status"`
	ActiveAction        string      `json:"active_action"`
	Question            string      `json:"question"`
	PredictedNextAction string      `json:"predicted_next_action"`
	ResourceUse         ResourceUse `json:"resource_use"`
	AccessState         string      `json:"access_state"`
	OutputState         string      `json:"output_state"`
}

type Control struct {
	ID                    string    `json:"id"`
	StreamID              string    `json:"stream_id,omitempty"`
	StreamRevision        int64     `json:"stream_revision,omitempty"`
	Action                string    `json:"action"`
	Instruction           string    `json:"instruction"`
	TargetParticipantID   string    `json:"target_participant_id,omitempty"`
	NarrowedPaths         []string  `json:"narrowed_paths,omitempty"`
	PreservesAcceptedWork bool      `json:"preserves_accepted_work"`
	ExpandsAuthority      bool      `json:"expands_authority"`
	ActorID               string    `json:"actor_id"`
	CreatedAt             time.Time `json:"created_at"`
}

type ControlInput struct {
	StreamID            string   `json:"stream_id"`
	Action              string   `json:"action"`
	Instruction         string   `json:"instruction"`
	TargetParticipantID string   `json:"target_participant_id"`
	NarrowedPaths       []string `json:"narrowed_paths"`
}

type RuntimeBlocker struct {
	Kind      string `json:"kind"`
	StreamID  string `json:"stream_id,omitempty"`
	Detail    string `json:"detail"`
	Recovery  string `json:"recovery"`
	Escalates bool   `json:"escalates"`
}
type RuntimeStream struct {
	StreamID             string           `json:"stream_id"`
	Title                string           `json:"title"`
	OwnerParticipantID   string           `json:"owner_participant_id"`
	OperationalOwnerID   string           `json:"operational_owner_id"`
	DependsOn            []string         `json:"depends_on"`
	Status               string           `json:"status"`
	ActiveControl        *Control         `json:"active_control,omitempty"`
	Run                  *StreamRun       `json:"run,omitempty"`
	Blockers             []RuntimeBlocker `json:"blockers"`
	PredictedNextActions []string         `json:"predicted_next_actions"`
}
type RuntimeView struct {
	Status      string           `json:"status"`
	ResourceUse ResourceUse      `json:"resource_use"`
	Streams     []RuntimeStream  `json:"streams"`
	Blockers    []RuntimeBlocker `json:"blockers"`
	Questions   []string         `json:"questions"`
	UpdatedAt   *time.Time       `json:"updated_at,omitempty"`
}

func (s *Store) ReportStream(repo, team, streamID, actor, participantID string, version int64, in StreamStatusInput) (Team, error) {
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
	if !ok || !operationalOwner(v, item, participantID, actor) {
		return v, ErrForbidden
	}
	validStatus := map[string]bool{"queued": true, "running": true, "paused": true, "failed": true, "completed": true, "canceled": true}
	validAccess := map[string]bool{"active": true, "revoked": true, "expired": true}
	validOutput := map[string]bool{"clean": true, "conflicting": true}
	in.ActiveAction, in.Question, in.PredictedNextAction = strings.TrimSpace(in.ActiveAction), strings.TrimSpace(in.Question), strings.TrimSpace(in.PredictedNextAction)
	if !validStatus[in.Status] || !validAccess[in.AccessState] || !validOutput[in.OutputState] || !validBudget(Budget(in.ResourceUse)) || len(in.ActiveAction) > 2000 || len(in.Question) > 4000 || len(in.PredictedNextAction) > 2000 || !fullCommit(in.ReportedRevision) {
		return v, ErrInvalid
	}
	now := s.now().UTC()
	run := StreamRun{StreamID: item.ID, StreamRevision: item.Revision, ReportedRevision: in.ReportedRevision, Status: in.Status, ActiveAction: in.ActiveAction, Question: in.Question, PredictedNextAction: in.PredictedNextAction, ResourceUse: in.ResourceUse, AccessState: in.AccessState, OutputState: in.OutputState, OperationalOwnerID: item.OwnerParticipantID, ReportedByID: actor, ReportedAt: now}
	for i := range v.StreamRuns {
		if v.StreamRuns[i].StreamID == item.ID {
			run.OperationalOwnerID = v.StreamRuns[i].OperationalOwnerID
			run.RecoveryAttempts = v.StreamRuns[i].RecoveryAttempts
			v.StreamRuns[i] = run
			addEvent(&v, "stream.status_reported", actor, participantID, item.ID+":"+in.Status, now)
			err = s.write(v)
			v.Runtime = deriveRuntime(v, now)
			return v, err
		}
	}
	v.StreamRuns = append(v.StreamRuns, run)
	addEvent(&v, "stream.status_reported", actor, participantID, item.ID+":"+in.Status, now)
	err = s.write(v)
	v.Runtime = deriveRuntime(v, now)
	return v, err
}

func operationalOwner(v Team, item WorkStream, participantID, actor string) bool {
	wanted := item.OwnerParticipantID
	for _, run := range v.StreamRuns {
		if run.StreamID == item.ID && run.OperationalOwnerID != "" {
			wanted = run.OperationalOwnerID
		}
	}
	p, ok := participant(v, participantID)
	return ok && participantID == wanted && (p.Kind == "agent" || p.PrincipalID == actor)
}

func (s *Store) Control(repo, team, actor string, version int64, in ControlInput) (Team, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, team)
	if err != nil {
		return v, err
	}
	if version != v.Version {
		return v, ErrConflict
	}
	in.StreamID, in.Action, in.Instruction = strings.TrimSpace(in.StreamID), strings.TrimSpace(in.Action), strings.TrimSpace(in.Instruction)
	valid := map[string]bool{"guide": true, "pause": true, "resume": true, "cancel": true, "reassign": true, "narrow": true}
	if !valid[in.Action] || in.Instruction == "" || len(in.Instruction) > 4000 {
		return v, ErrInvalid
	}
	var item WorkStream
	if in.StreamID == "" {
		if actor != v.OrganizerID || in.Action == "reassign" || in.Action == "narrow" {
			return v, ErrForbidden
		}
	} else {
		var ok bool
		item, ok = stream(v, in.StreamID)
		if !ok {
			return v, ErrInvalid
		}
		if actor != v.OrganizerID {
			p, yes := participant(v, item.OwnerParticipantID)
			if !yes || p.Kind != "human" || p.PrincipalID != actor {
				return v, ErrForbidden
			}
		}
	}
	paths, ok := clean(in.NarrowedPaths, 500)
	if !ok {
		return v, ErrInvalid
	}
	if in.Action == "narrow" {
		if len(paths) == 0 || !pathsWithin(item, paths) {
			return v, ErrInvalid
		}
	} else if len(paths) > 0 {
		return v, ErrInvalid
	}
	if in.Action == "reassign" {
		target, yes := participant(v, in.TargetParticipantID)
		if !yes || !canOperate(item, target) {
			return v, ErrForbidden
		}
	} else if in.TargetParticipantID != "" {
		return v, ErrInvalid
	}
	now := s.now().UTC()
	c := Control{ID: id(), StreamID: item.ID, StreamRevision: item.Revision, Action: in.Action, Instruction: in.Instruction, TargetParticipantID: in.TargetParticipantID, NarrowedPaths: paths, PreservesAcceptedWork: true, ExpandsAuthority: false, ActorID: actor, CreatedAt: now}
	v.Controls = append(v.Controls, c)
	for i := range v.StreamRuns {
		if item.ID != "" && v.StreamRuns[i].StreamID != item.ID {
			continue
		}
		switch in.Action {
		case "pause":
			v.StreamRuns[i].Status = "paused"
		case "resume":
			v.StreamRuns[i].Status = "running"
		case "cancel":
			v.StreamRuns[i].Status = "canceled"
		case "reassign":
			v.StreamRuns[i].OperationalOwnerID = in.TargetParticipantID
		case "guide", "narrow":
		}
		if in.Action == "resume" && v.StreamRuns[i].RecoveryAttempts < 1 {
			v.StreamRuns[i].RecoveryAttempts++
		}
	}
	addEvent(&v, "control."+in.Action, actor, "", item.ID, now)
	err = s.write(v)
	v.Runtime = deriveRuntime(v, now)
	return v, err
}

func pathsWithin(item WorkStream, paths []string) bool {
	for _, p := range paths {
		p = strings.Trim(p, "/")
		found := false
		for _, scope := range item.RepositoryScope {
			for _, base := range scope.Paths {
				base = strings.Trim(base, "/")
				if p == base || strings.HasPrefix(p, base+"/") {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func canOperate(item WorkStream, p Participant) bool {
	for _, scope := range item.RepositoryScope {
		for _, need := range scope.RequiredActions {
			found := false
			for _, held := range p.Access.Actions {
				if need == held {
					found = true
				}
			}
			if !found {
				return false
			}
		}
	}
	return true
}

func deriveRuntime(v Team, now time.Time) RuntimeView {
	view := RuntimeView{Status: "not_started", Streams: []RuntimeStream{}, Blockers: []RuntimeBlocker{}, Questions: []string{}}
	if v.Plan == nil {
		return view
	}
	runBy := map[string]StreamRun{}
	for _, r := range v.StreamRuns {
		runBy[r.StreamID] = r
	}
	completed := map[string]bool{}
	for _, r := range v.StreamRuns {
		completed[r.StreamID] = r.Status == "completed"
		view.ResourceUse.Hours += r.ResourceUse.Hours
		view.ResourceUse.CostUnits += r.ResourceUse.CostUnits
		view.ResourceUse.AgentRuns += r.ResourceUse.AgentRuns
		if view.UpdatedAt == nil || r.ReportedAt.After(*view.UpdatedAt) {
			x := r.ReportedAt
			view.UpdatedAt = &x
		}
	}
	for _, item := range v.Plan.Current.Streams {
		rs := RuntimeStream{StreamID: item.ID, Title: item.Title, OwnerParticipantID: item.OwnerParticipantID, OperationalOwnerID: item.OwnerParticipantID, DependsOn: item.DependsOn, Status: "not_started", Blockers: []RuntimeBlocker{}, PredictedNextActions: []string{}}
		if r, ok := runBy[item.ID]; ok {
			x := r
			rs.Run = &x
			rs.Status = r.Status
			rs.OperationalOwnerID = r.OperationalOwnerID
			if r.Question != "" {
				view.Questions = append(view.Questions, item.Title+": "+r.Question)
			}
			if r.PredictedNextAction != "" {
				rs.PredictedNextActions = append(rs.PredictedNextActions, r.PredictedNextAction)
			}
			add := func(kind, detail, recovery string, esc bool) {
				b := RuntimeBlocker{Kind: kind, StreamID: item.ID, Detail: detail, Recovery: recovery, Escalates: esc}
				rs.Blockers = append(rs.Blockers, b)
				view.Blockers = append(view.Blockers, b)
			}
			acceptedRevision := false
			for _, scope := range item.RepositoryScope {
				acceptedRevision = acceptedRevision || r.ReportedRevision == scope.CommitID
			}
			if !acceptedRevision {
				add("stale_revision", "reported revision is outside the accepted stream revision", "pause and replan against explicit retained work", true)
			}
			if r.AccessState != "active" {
				add("access_"+r.AccessState, "the owner's independent access is "+r.AccessState, "pause and restore or explicitly reassign within existing authority", true)
			}
			if r.OutputState == "conflicting" {
				add("conflicting_output", "the stream reports incompatible output", "pause affected streams and reconcile cited artifacts", true)
			}
			if r.ResourceUse.Hours > item.Budget.Hours || r.ResourceUse.CostUnits > item.Budget.CostUnits || r.ResourceUse.AgentRuns > item.Budget.AgentRuns {
				add("budget_exhausted", "reported use exceeds the accepted stream ceiling", "pause and escalate for a charter or plan revision", true)
			}
			if r.Status == "failed" {
				if r.RecoveryAttempts < 1 {
					add("failed_run", "the execution context failed", "one bounded resume attempt remains", false)
				} else {
					add("recovery_exhausted", "the bounded recovery attempt failed", "escalate without transferring authority", true)
				}
			}
			if (r.Status == "running" || r.Status == "queued") && now.Sub(r.ReportedAt) > 15*time.Minute {
				add("participant_disconnected", "no stream heartbeat was reported for 15 minutes", "pause and contact the accepted escalation target", true)
			}
		}
		for _, dep := range item.DependsOn {
			if !completed[dep] {
				b := RuntimeBlocker{Kind: "dependency_pending", StreamID: item.ID, Detail: "waiting for " + dep, Recovery: "continue independent scoped work or wait for the accepted handoff"}
				rs.Blockers = append(rs.Blockers, b)
				view.Blockers = append(view.Blockers, b)
			}
		}
		for i := len(v.Controls) - 1; i >= 0; i-- {
			if v.Controls[i].StreamID == item.ID || v.Controls[i].StreamID == "" {
				x := v.Controls[i]
				rs.ActiveControl = &x
				break
			}
		}
		if len(rs.PredictedNextActions) == 0 {
			if len(rs.Blockers) > 0 {
				rs.PredictedNextActions = []string{rs.Blockers[0].Recovery}
			} else if rs.Status == "completed" {
				rs.PredictedNextActions = []string{"offer accepted artifacts to dependent streams"}
			} else {
				rs.PredictedNextActions = []string{"report the next bounded checkpoint"}
			}
		}
		view.Streams = append(view.Streams, rs)
	}
	if len(view.Blockers) > 0 {
		view.Status = "attention_required"
	} else {
		all := len(view.Streams) > 0
		active := false
		for _, s := range view.Streams {
			all = all && s.Status == "completed"
			active = active || s.Status != "not_started"
		}
		if all {
			view.Status = "completed"
		} else if active {
			view.Status = "running"
		}
	}
	sort.Strings(view.Questions)
	return view
}
