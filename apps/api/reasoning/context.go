// Package reasoning defines immutable links from shared understanding to delivery work.
package reasoning

type Evidence struct {
	RepositoryID string `json:"repository_id"`
	CommitID     string `json:"commit_id"`
	Kind         string `json:"kind"`
	Path         string `json:"path,omitempty"`
	Line         int    `json:"line,omitempty"`
	ResourceID   string `json:"resource_id,omitempty"`
	Label        string `json:"label,omitempty"`
}

type Acknowledgement struct {
	OwnerID     string `json:"owner_id"`
	State       string `json:"state"`
	Note        string `json:"note,omitempty"`
	DecidedByID string `json:"decided_by_id,omitempty"`
}

// Context is copied into work records. It never follows a branch or gets
// rewritten when the originating analysis is rerun.
type Context struct {
	Kind             string            `json:"kind"`
	IssueID          string            `json:"issue_id,omitempty"`
	ReproductionID   string            `json:"reproduction_id,omitempty"`
	DecisionID       string            `json:"decision_id,omitempty"`
	DecisionVersion  int               `json:"decision_version,omitempty"`
	OrganizationID   string            `json:"organization_id,omitempty"`
	OpportunityID    string            `json:"opportunity_id,omitempty"`
	MandateID        string            `json:"mandate_id,omitempty"`
	MandateVersion   int64             `json:"mandate_version,omitempty"`
	InvestigationID  string            `json:"investigation_id,omitempty"`
	ConversationID   string            `json:"conversation_id,omitempty"`
	ConclusionID     string            `json:"conclusion_id,omitempty"`
	AssessmentID     string            `json:"assessment_id,omitempty"`
	ImpactID         string            `json:"impact_id,omitempty"`
	RepositoryID     string            `json:"repository_id"`
	CommitID         string            `json:"commit_id"`
	Claim            string            `json:"claim"`
	Risk             string            `json:"risk,omitempty"`
	State            string            `json:"state,omitempty"`
	Rationale        string            `json:"rationale,omitempty"`
	Verification     []string          `json:"verification,omitempty"`
	Evidence         []Evidence        `json:"evidence,omitempty"`
	Acknowledgements []Acknowledgement `json:"acknowledgements,omitempty"`
}
