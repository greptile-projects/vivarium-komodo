package issues

import (
	"strings"
	"time"
)

type RepairDecision struct {
	Kind           string    `json:"kind"`
	ActorID        string    `json:"actor_id"`
	Reason         string    `json:"reason,omitempty"`
	Revision       string    `json:"revision"`
	EvidenceDigest string    `json:"evidence_digest"`
	CreatedAt      time.Time `json:"created_at"`
}

type RepairVerification struct {
	ID                        string           `json:"id"`
	Revision                  string           `json:"revision"`
	PullRequestID             string           `json:"pull_request_id"`
	ReproductionAttemptID     string           `json:"reproduction_attempt_id,omitempty"`
	OriginalDefinitionDigest  string           `json:"original_definition_digest"`
	CandidateDefinitionDigest string           `json:"candidate_definition_digest,omitempty"`
	InputDigest               string           `json:"input_digest"`
	RequiredChecks            []string         `json:"required_checks"`
	CheckRunIDs               []string         `json:"check_run_ids"`
	AcceptanceCriteria        []string         `json:"acceptance_criteria"`
	InvalidReason             string           `json:"invalid_reason,omitempty"`
	PreviewArtifactPath       string           `json:"preview_artifact_path,omitempty"`
	Decisions                 []RepairDecision `json:"decisions"`
	CreatedByID               string           `json:"created_by_id"`
	CreatedAt                 time.Time        `json:"created_at"`
	UpdatedAt                 time.Time        `json:"updated_at"`
}

type Repair struct {
	ID                 string               `json:"id"`
	ReproductionID     string               `json:"reproduction_id"`
	InvestigationID    string               `json:"investigation_id"`
	ConclusionEntryID  string               `json:"conclusion_entry_id"`
	Revision           string               `json:"revision"`
	AcceptanceCriteria []string             `json:"acceptance_criteria"`
	ProposalID         string               `json:"proposal_id"`
	TaskID             string               `json:"task_id"`
	OwnerKind          string               `json:"owner_kind"`
	OwnerID            string               `json:"owner_id"`
	PullRequestID      string               `json:"pull_request_id,omitempty"`
	Verifications      []RepairVerification `json:"verifications"`
	CreatedByID        string               `json:"created_by_id"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
}

func (s *Store) AddRepairVerification(repo, issueID, repairID, actor string, verification RepairVerification) (Issue, RepairVerification, error) {
	if actor == "" || verification.Revision == "" || verification.PullRequestID == "" || verification.InputDigest == "" {
		return Issue{}, RepairVerification{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, issueID)
	if err != nil {
		return v, RepairVerification{}, err
	}
	for i := range v.Repairs {
		if v.Repairs[i].ID == repairID {
			id, _ := newID()
			now := s.now().UTC()
			verification.ID, verification.CreatedByID, verification.CreatedAt, verification.UpdatedAt = id, actor, now, now
			verification.Decisions = []RepairDecision{}
			v.Repairs[i].Verifications = append(v.Repairs[i].Verifications, verification)
			v.Repairs[i].UpdatedAt = now
			v.Version++
			v.UpdatedAt = now
			v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "repair.verification_started", ActorID: actor, Detail: id, CreatedAt: now})
			return v, verification, s.write(v)
		}
	}
	return Issue{}, RepairVerification{}, ErrNotFound
}

func (s *Store) DecideRepairVerification(repo, issueID, repairID, verificationID, actor, kind, reason, revision, digest string) (Issue, RepairVerification, error) {
	if actor == "" || revision == "" || digest == "" || (kind != "confirmed" && kind != "rejected" && kind != "override") || (kind == "override" && strings.TrimSpace(reason) == "") {
		return Issue{}, RepairVerification{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, issueID)
	if err != nil {
		return v, RepairVerification{}, err
	}
	for i := range v.Repairs {
		if v.Repairs[i].ID == repairID {
			for j := range v.Repairs[i].Verifications {
				if v.Repairs[i].Verifications[j].ID == verificationID {
					now := s.now().UTC()
					d := RepairDecision{Kind: kind, ActorID: actor, Reason: strings.TrimSpace(reason), Revision: revision, EvidenceDigest: digest, CreatedAt: now}
					v.Repairs[i].Verifications[j].Decisions = append(v.Repairs[i].Verifications[j].Decisions, d)
					v.Repairs[i].Verifications[j].UpdatedAt = now
					v.Repairs[i].UpdatedAt = now
					v.Version++
					v.UpdatedAt = now
					v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "repair.verification_" + kind, ActorID: actor, Detail: verificationID, CreatedAt: now})
					return v, v.Repairs[i].Verifications[j], s.write(v)
				}
			}
		}
	}
	return Issue{}, RepairVerification{}, ErrNotFound
}

func (s *Store) CreateRepair(repo, issueID, actor string, repair Repair) (Issue, Repair, error) {
	if actor == "" || repair.ReproductionID == "" || repair.InvestigationID == "" || repair.ConclusionEntryID == "" || repair.Revision == "" || repair.ProposalID == "" || repair.TaskID == "" || len(repair.AcceptanceCriteria) == 0 {
		return Issue{}, Repair{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, issueID)
	if err != nil {
		return v, Repair{}, err
	}
	id, _ := newID()
	now := s.now().UTC()
	repair.ID, repair.CreatedByID, repair.CreatedAt, repair.UpdatedAt = id, actor, now, now
	v.Repairs = append(v.Repairs, repair)
	v.Version++
	v.UpdatedAt = now
	v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "repair.created", ActorID: actor, Detail: id, CreatedAt: now})
	return v, repair, s.write(v)
}

func (s *Store) LinkRepairPullRequest(repo, issueID, repairID, pullID, actor string) (Issue, Repair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.read(repo, issueID)
	if err != nil {
		return v, Repair{}, err
	}
	for i := range v.Repairs {
		if v.Repairs[i].ID != repairID {
			continue
		}
		if v.Repairs[i].PullRequestID != "" && v.Repairs[i].PullRequestID != pullID {
			return Issue{}, Repair{}, ErrConflict
		}
		now := s.now().UTC()
		v.Repairs[i].PullRequestID, v.Repairs[i].UpdatedAt = pullID, now
		v.Relationships = append(v.Relationships, Relationship{ID: repairID, Kind: "pull_request", ResourceID: pullID, RepositoryID: repo, Revision: v.Repairs[i].Revision, Note: "governed issue repair", AddedByID: actor, CreatedAt: now})
		v.Version++
		v.UpdatedAt = now
		v.History = append(v.History, Event{Sequence: int64(len(v.History) + 1), Type: "repair.pull_request_linked", ActorID: actor, Detail: pullID, CreatedAt: now})
		return v, v.Repairs[i], s.write(v)
	}
	return Issue{}, Repair{}, ErrNotFound
}
