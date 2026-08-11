package issues

import "time"

type Repair struct {
	ID                 string    `json:"id"`
	ReproductionID     string    `json:"reproduction_id"`
	InvestigationID    string    `json:"investigation_id"`
	ConclusionEntryID  string    `json:"conclusion_entry_id"`
	Revision           string    `json:"revision"`
	AcceptanceCriteria []string  `json:"acceptance_criteria"`
	ProposalID         string    `json:"proposal_id"`
	TaskID             string    `json:"task_id"`
	OwnerKind          string    `json:"owner_kind"`
	OwnerID            string    `json:"owner_id"`
	PullRequestID      string    `json:"pull_request_id,omitempty"`
	CreatedByID        string    `json:"created_by_id"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
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
