package apiconsumers

import "time"

// OperationalObservation is bounded operational evidence, never a request
// payload or credential. Audience controls whether it crosses the ownership
// boundary or remains visible to only one side.
type OperationalObservation struct {
	ID                     string    `json:"id"`
	Kind                   string    `json:"kind"`
	Audience               string    `json:"audience"`
	ContractID             string    `json:"contract_id"`
	ContractVersion        int64     `json:"contract_version"`
	ContractSourceRevision string    `json:"contract_source_revision"`
	ReleaseID              string    `json:"release_id"`
	Environment            string    `json:"environment"`
	OperationID            string    `json:"operation_id,omitempty"`
	WindowStart            time.Time `json:"window_start"`
	WindowEnd              time.Time `json:"window_end"`
	AvailabilityPercent    *float64  `json:"availability_percent,omitempty"`
	LatencyMilliseconds    *float64  `json:"latency_milliseconds,omitempty"`
	QuotaUsed              *int      `json:"quota_used,omitempty"`
	QuotaLimit             *int      `json:"quota_limit,omitempty"`
	ErrorCode              string    `json:"error_code,omitempty"`
	SchemaConformant       *bool     `json:"schema_conformant,omitempty"`
	UsageCount             *int64    `json:"usage_count,omitempty"`
	Summary                string    `json:"summary"`
	InaccessibleEvidence   []string  `json:"inaccessible_evidence,omitempty"`
	ObservedBy             string    `json:"observed_by"`
	CreatedAt              time.Time `json:"created_at"`
}

type ObservationInput struct {
	Kind                 string    `json:"kind"`
	Audience             string    `json:"audience"`
	ReleaseID            string    `json:"release_id"`
	Environment          string    `json:"environment"`
	OperationID          string    `json:"operation_id"`
	WindowStart          time.Time `json:"window_start"`
	WindowEnd            time.Time `json:"window_end"`
	AvailabilityPercent  *float64  `json:"availability_percent"`
	LatencyMilliseconds  *float64  `json:"latency_milliseconds"`
	QuotaUsed            *int      `json:"quota_used"`
	QuotaLimit           *int      `json:"quota_limit"`
	ErrorCode            string    `json:"error_code"`
	SchemaConformant     *bool     `json:"schema_conformant"`
	UsageCount           *int64    `json:"usage_count"`
	Summary              string    `json:"summary"`
	InaccessibleEvidence []string  `json:"inaccessible_evidence"`
}

type InvestigationEntry struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	Body        string    `json:"body"`
	EvidenceIDs []string  `json:"evidence_ids,omitempty"`
	AuthorID    string    `json:"author_id"`
	AuthorKind  string    `json:"author_kind"`
	CreatedAt   time.Time `json:"created_at"`
}
type AgentInvitation struct {
	AgentID   string    `json:"agent_id"`
	InvitedBy string    `json:"invited_by"`
	Scope     string    `json:"scope"`
	CreatedAt time.Time `json:"created_at"`
}
type SandboxReproduction struct {
	ID           string     `json:"id"`
	OperationID  string     `json:"operation_id"`
	Failure      string     `json:"failure,omitempty"`
	Inspection   Inspection `json:"inspection"`
	ReproducedBy string     `json:"reproduced_by"`
	CreatedAt    time.Time  `json:"created_at"`
}
type ChangeRoute struct {
	ID           string    `json:"id"`
	DefectOwner  string    `json:"defect_owner"`
	ResourceKind string    `json:"resource_kind"`
	ResourceID   string    `json:"resource_id"`
	RepositoryID string    `json:"repository_id"`
	Revision     string    `json:"revision"`
	RoutedBy     string    `json:"routed_by"`
	CreatedAt    time.Time `json:"created_at"`
}
type SupportInvestigation struct {
	ID              string                `json:"id"`
	Title           string                `json:"title"`
	ObservationIDs  []string              `json:"observation_ids"`
	Status          string                `json:"status"`
	Classification  string                `json:"classification"`
	ContractID      string                `json:"contract_id"`
	ContractVersion int64                 `json:"contract_version"`
	ReleaseID       string                `json:"release_id"`
	Environment     string                `json:"environment"`
	OpenedBy        string                `json:"opened_by"`
	Invitations     []AgentInvitation     `json:"invitations,omitempty"`
	Entries         []InvestigationEntry  `json:"entries,omitempty"`
	Reproductions   []SandboxReproduction `json:"reproductions,omitempty"`
	ChangeRoutes    []ChangeRoute         `json:"change_routes,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
}
type InvestigationInput struct {
	Title          string   `json:"title"`
	ObservationIDs []string `json:"observation_ids"`
}
type EntryInput struct {
	Kind           string   `json:"kind"`
	Body           string   `json:"body"`
	EvidenceIDs    []string `json:"evidence_ids"`
	Classification string   `json:"classification"`
}
type RouteInput struct {
	DefectOwner  string `json:"defect_owner"`
	ResourceKind string `json:"resource_kind"`
	ResourceID   string `json:"resource_id"`
	RepositoryID string `json:"repository_id"`
	Revision     string `json:"revision"`
}
type InvestigationView struct {
	Investigation SupportInvestigation     `json:"investigation"`
	Evidence      []OperationalObservation `json:"evidence"`
}

func canSeeObservation(x OperationalObservation, producer bool) bool {
	return x.Audience == "shared" || producer && x.Audience == "producer" || !producer && x.Audience == "consumer"
}
func participant(a *Application, actor string, producer bool, i *SupportInvestigation) bool {
	if producer || a.OwnerID == actor {
		return true
	}
	for _, x := range i.Invitations {
		if x.AgentID == actor {
			return true
		}
	}
	return false
}

func (s *Store) RecordObservation(repo, application, actor string, producer bool, in ObservationInput) (OperationalObservation, error) {
	validKind := map[string]bool{"availability": true, "latency": true, "quota": true, "error": true, "schema_conformance": true, "usage": true}
	if !validKind[in.Kind] || !map[string]bool{"shared": true, "producer": true, "consumer": true}[in.Audience] || in.ReleaseID == "" || in.Environment == "" || in.Summary == "" || !in.WindowEnd.After(in.WindowStart) || !safeStrings(in.InaccessibleEvidence) || sensitive(in.Summary) {
		return OperationalObservation{}, ErrInvalid
	}
	if producer && in.Audience == "consumer" || !producer && in.Audience == "producer" {
		return OperationalObservation{}, ErrForbidden
	}
	metricValid := false
	switch in.Kind {
	case "availability":
		metricValid = in.AvailabilityPercent != nil && *in.AvailabilityPercent >= 0 && *in.AvailabilityPercent <= 100
	case "latency":
		metricValid = in.LatencyMilliseconds != nil && *in.LatencyMilliseconds >= 0
	case "quota":
		metricValid = in.QuotaUsed != nil && in.QuotaLimit != nil && *in.QuotaUsed >= 0 && *in.QuotaLimit > 0 && *in.QuotaUsed <= *in.QuotaLimit
	case "error":
		metricValid = in.ErrorCode != "" && !sensitive(in.ErrorCode)
	case "schema_conformance":
		metricValid = in.SchemaConformant != nil
	case "usage":
		metricValid = in.UsageCount != nil && *in.UsageCount >= 0
	}
	if !metricValid {
		return OperationalObservation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return OperationalObservation{}, e
	}
	for n := range d.Applications {
		a := &d.Applications[n]
		if a.RepositoryID != repo || a.ID != application {
			continue
		}
		if !producer && a.OwnerID != actor {
			return OperationalObservation{}, ErrForbidden
		}
		environmentApproved := false
		for _, environment := range a.ApprovedEnvironments {
			if environment == in.Environment {
				environmentApproved = true
			}
		}
		if !environmentApproved {
			return OperationalObservation{}, ErrInvalid
		}
		source, _, ok := contractSnapshot(s, *a)
		if !ok {
			return OperationalObservation{}, ErrInvalid
		}
		now := s.now().UTC()
		x := OperationalObservation{ID: ident("apiobs"), Kind: in.Kind, Audience: in.Audience, ContractID: a.Registration.ContractID, ContractVersion: a.Registration.ContractVersion, ContractSourceRevision: source, ReleaseID: in.ReleaseID, Environment: in.Environment, OperationID: in.OperationID, WindowStart: in.WindowStart, WindowEnd: in.WindowEnd, AvailabilityPercent: in.AvailabilityPercent, LatencyMilliseconds: in.LatencyMilliseconds, QuotaUsed: in.QuotaUsed, QuotaLimit: in.QuotaLimit, ErrorCode: in.ErrorCode, SchemaConformant: in.SchemaConformant, UsageCount: in.UsageCount, Summary: in.Summary, InaccessibleEvidence: in.InaccessibleEvidence, ObservedBy: actor, CreatedAt: now}
		a.Observations = append(a.Observations, x)
		event(a, actor, "operational_evidence_recorded", x.ID, now)
		return x, s.save(d)
	}
	return OperationalObservation{}, ErrNotFound
}

func (s *Store) OpenInvestigation(repo, application, actor string, producer bool, in InvestigationInput) (SupportInvestigation, error) {
	if in.Title == "" || !unique(in.ObservationIDs) || sensitive(in.Title) {
		return SupportInvestigation{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return SupportInvestigation{}, e
	}
	for n := range d.Applications {
		a := &d.Applications[n]
		if a.RepositoryID != repo || a.ID != application {
			continue
		}
		if !producer && a.OwnerID != actor {
			return SupportInvestigation{}, ErrForbidden
		}
		var release, env string
		for _, id := range in.ObservationIDs {
			found := false
			for _, o := range a.Observations {
				if o.ID == id && o.Audience == "shared" && canSeeObservation(o, producer) {
					found = true
					if release == "" {
						release, env = o.ReleaseID, o.Environment
					}
					if release != o.ReleaseID || env != o.Environment {
						return SupportInvestigation{}, ErrInvalid
					}
				}
			}
			if !found {
				return SupportInvestigation{}, ErrForbidden
			}
		}
		now := s.now().UTC()
		x := SupportInvestigation{ID: ident("apiinvestigation"), Title: in.Title, ObservationIDs: in.ObservationIDs, Status: "open", Classification: "unconfirmed", ContractID: a.Registration.ContractID, ContractVersion: a.Registration.ContractVersion, ReleaseID: release, Environment: env, OpenedBy: actor, CreatedAt: now, UpdatedAt: now}
		a.Investigations = append(a.Investigations, x)
		event(a, actor, "support_investigation_opened", x.ID, now)
		return x, s.save(d)
	}
	return SupportInvestigation{}, ErrNotFound
}

func (s *Store) GetInvestigation(repo, application, id, actor string, producer bool) (InvestigationView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return InvestigationView{}, e
	}
	for n := range d.Applications {
		a := &d.Applications[n]
		if a.RepositoryID != repo || a.ID != application {
			continue
		}
		for j := range a.Investigations {
			i := &a.Investigations[j]
			if i.ID != id {
				continue
			}
			if !participant(a, actor, producer, i) {
				return InvestigationView{}, ErrForbidden
			}
			view := InvestigationView{Investigation: *i}
			for _, evidenceID := range i.ObservationIDs {
				for _, o := range a.Observations {
					if o.ID == evidenceID && o.Audience == "shared" {
						view.Evidence = append(view.Evidence, o)
					}
				}
			}
			return view, nil
		}
	}
	return InvestigationView{}, ErrNotFound
}

func (s *Store) mutateInvestigation(repo, application, id, actor string, producer bool, fn func(*Application, *SupportInvestigation, time.Time) error) (SupportInvestigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, e := s.load()
	if e != nil {
		return SupportInvestigation{}, e
	}
	for n := range d.Applications {
		a := &d.Applications[n]
		if a.RepositoryID != repo || a.ID != application {
			continue
		}
		for j := range a.Investigations {
			i := &a.Investigations[j]
			if i.ID != id {
				continue
			}
			if !participant(a, actor, producer, i) {
				return SupportInvestigation{}, ErrForbidden
			}
			now := s.now().UTC()
			if e = fn(a, i, now); e != nil {
				return SupportInvestigation{}, e
			}
			i.UpdatedAt = now
			return *i, s.save(d)
		}
	}
	return SupportInvestigation{}, ErrNotFound
}
func (s *Store) InviteAgent(repo, app, id, actor string, producer bool, agent string) (SupportInvestigation, error) {
	if agent == "" || sensitive(agent) {
		return SupportInvestigation{}, ErrInvalid
	}
	return s.mutateInvestigation(repo, app, id, actor, producer, func(a *Application, i *SupportInvestigation, now time.Time) error {
		if !producer && a.OwnerID != actor {
			return ErrForbidden
		}
		for _, x := range i.Invitations {
			if x.AgentID == agent {
				return ErrConflict
			}
		}
		i.Invitations = append(i.Invitations, AgentInvitation{AgentID: agent, InvitedBy: actor, Scope: "read_only_sanitized_evidence_and_thread", CreatedAt: now})
		return nil
	})
}
func (s *Store) AddEntry(repo, app, id, actor string, producer bool, in EntryInput) (SupportInvestigation, error) {
	if !map[string]bool{"finding": true, "question": true, "comment": true, "decision": true}[in.Kind] || in.Body == "" || sensitive(in.Body) || !safeStrings(in.EvidenceIDs) {
		return SupportInvestigation{}, ErrInvalid
	}
	return s.mutateInvestigation(repo, app, id, actor, producer, func(a *Application, i *SupportInvestigation, now time.Time) error {
		for _, evidenceID := range in.EvidenceIDs {
			found := false
			for _, allowedID := range i.ObservationIDs {
				if evidenceID == allowedID {
					found = true
				}
			}
			if !found {
				return ErrForbidden
			}
		}
		if in.Classification != "" {
			if !producer && a.OwnerID != actor {
				return ErrForbidden
			}
			if in.Kind != "decision" || !map[string]bool{"service": true, "contract": true, "client": true, "environment": true, "unconfirmed": true}[in.Classification] {
				return ErrInvalid
			}
			i.Classification = in.Classification
			if in.Classification != "unconfirmed" {
				i.Status = "confirmed"
			}
		}
		kind := "human"
		for _, x := range i.Invitations {
			if x.AgentID == actor {
				kind = "agent"
			}
		}
		i.Entries = append(i.Entries, InvestigationEntry{ID: ident("apientry"), Kind: in.Kind, Body: in.Body, EvidenceIDs: in.EvidenceIDs, AuthorID: actor, AuthorKind: kind, CreatedAt: now})
		return nil
	})
}
func (s *Store) Reproduce(repo, app, id, actor string, producer bool, in SandboxInput) (SupportInvestigation, error) {
	if in.OperationID == "" || !safeValue(in.Body) {
		return SupportInvestigation{}, ErrInvalid
	}
	return s.mutateInvestigation(repo, app, id, actor, producer, func(a *Application, i *SupportInvestigation, now time.Time) error {
		var opID, method, path string
		for _, o := range a.ContractOperations {
			if o.ID == in.OperationID {
				opID, method, path = o.ID, o.Method, o.Path
			}
		}
		if opID == "" {
			return ErrInvalid
		}
		status := 200
		resp := a.SyntheticData[opID]
		if resp == nil {
			resp = map[string]any{"synthetic": true, "operation": opID}
		}
		failure := ""
		if in.Failure != "" {
			for _, r := range a.FailureRules {
				if r.ID == in.Failure && r.OperationID == opID {
					status, failure, resp = r.Status, r.ID, r.Response
					if resp == nil {
						resp = map[string]any{"error": r.ErrorCode}
					}
				}
			}
			if failure == "" {
				return ErrInvalid
			}
		}
		inspection := Inspection{Sequence: int64(len(a.Inspections) + 1), OperationID: opID, Method: method, Path: path, RequestHeaders: map[string]string{"authorization": "[REDACTED]", "content-type": "application/json"}, RequestBody: in.Body, ResponseStatus: status, ResponseHeaders: map[string]string{"content-type": "application/json", "x-sandbox": "synthetic"}, ResponseBody: resp, FailureRule: failure, CreatedAt: now}
		a.Inspections = append(a.Inspections, inspection)
		i.Reproductions = append(i.Reproductions, SandboxReproduction{ID: ident("apirepro"), OperationID: opID, Failure: failure, Inspection: inspection, ReproducedBy: actor, CreatedAt: now})
		return nil
	})
}
func (s *Store) RouteChange(repo, app, id, actor string, producer bool, in RouteInput) (SupportInvestigation, error) {
	if !map[string]bool{"provider": true, "consumer": true}[in.DefectOwner] || !map[string]bool{"issue": true, "proposal": true, "task": true, "workspace": true}[in.ResourceKind] || in.ResourceID == "" || in.RepositoryID == "" || in.Revision == "" {
		return SupportInvestigation{}, ErrInvalid
	}
	return s.mutateInvestigation(repo, app, id, actor, producer, func(a *Application, i *SupportInvestigation, now time.Time) error {
		if !producer && a.OwnerID != actor {
			return ErrForbidden
		}
		if i.Status != "confirmed" || i.Classification == "unconfirmed" {
			return ErrConflict
		}
		if (in.DefectOwner == "provider" && i.Classification != "service" && i.Classification != "contract") || (in.DefectOwner == "consumer" && i.Classification != "client" && i.Classification != "environment") {
			return ErrInvalid
		}
		i.ChangeRoutes = append(i.ChangeRoutes, ChangeRoute{ID: ident("apiroute"), DefectOwner: in.DefectOwner, ResourceKind: in.ResourceKind, ResourceID: in.ResourceID, RepositoryID: in.RepositoryID, Revision: in.Revision, RoutedBy: actor, CreatedAt: now})
		i.Status = "routed"
		return nil
	})
}
