package main

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/changesessions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/decisions"
	"github.com/greptile-projects/vivarium-komodo/apps/api/deliveryteams"
	"github.com/greptile-projects/vivarium-komodo/apps/api/investigations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/organizations"
	"github.com/greptile-projects/vivarium-komodo/apps/api/pullrequests"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/workspaces"
)

type deliveryTeamStore interface {
	Create(string, string, string, deliveryteams.Outcome, deliveryteams.CharterInput) (deliveryteams.Team, error)
	Get(string, string) (deliveryteams.Team, error)
	List(string) ([]deliveryteams.Team, error)
	Revise(string, string, string, int64, deliveryteams.CharterInput) (deliveryteams.Team, error)
	Invite(string, string, string, int64, deliveryteams.ParticipantInput) (deliveryteams.Team, error)
	Respond(string, string, string, string, string, int64) (deliveryteams.Team, error)
	Remove(string, string, string, string, string, int64) (deliveryteams.Team, error)
	Replace(string, string, string, string, int64, deliveryteams.ParticipantInput) (deliveryteams.Team, error)
	ProposePlan(string, string, string, string, int64, deliveryteams.PlanInput) (deliveryteams.Team, error)
	AcceptPlan(string, string, string, string, int64, int64) (deliveryteams.Team, error)
	AttachExecution(string, string, string, string, int64, deliveryteams.ExecutionContext, string) (deliveryteams.Team, error)
	PublishTimeline(string, string, string, string, int64, deliveryteams.TimelineInput) (deliveryteams.Team, error)
	RequestHandoff(string, string, string, string, int64, deliveryteams.HandoffInput) (deliveryteams.Team, error)
	AcceptHandoff(string, string, string, string, string, string, int64) (deliveryteams.Team, error)
	ReportStream(string, string, string, string, string, int64, deliveryteams.StreamStatusInput) (deliveryteams.Team, error)
	Control(string, string, string, int64, deliveryteams.ControlInput) (deliveryteams.Team, error)
	Reconcile(string, string, string, int64, deliveryteams.IntegrationInput) (deliveryteams.Team, error)
	LinkPullRequest(string, string, string, string, string, string, int64) (deliveryteams.Team, error)
}
type deliveryOrganizationStore interface {
	Get(string) (organizations.Organization, error)
}

type deliveryExecutionStores struct {
	changes interface {
		Get(string, string, string) (changesessions.Session, error)
	}
	investigations interface {
		Get(string, string) (investigations.Investigation, error)
	}
	decisions interface {
		Get(string, string) (decisions.Decision, error)
	}
	workspaces interface {
		Get(string, string) (workspaces.Workspace, error)
	}
}

func registerDeliveryTeamsHTTP(mux *http.ServeMux, teams deliveryTeamStore, repos pullRequestRepositoryStore, credentials authStore, orgs deliveryOrganizationStore, execution deliveryExecutionStores, extras ...any) {
	var pulls pullRequestStore
	var checks checkRunStarter
	for _, extra := range extras {
		switch value := extra.(type) {
		case pullRequestStore:
			pulls = value
		case checkRunStarter:
			checks = value
		}
	}
	base := "/repositories/{repository}/delivery-teams"
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := teams.List(string(repo.ID))
		writeDeliveryTeamResult(w, v, e, 200)
	})
	mux.HandleFunc("POST "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			Name    string                     `json:"name"`
			Source  deliveryteams.Outcome      `json:"source"`
			Charter deliveryteams.CharterInput `json:"charter"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := teams.Create(string(repo.ID), in.Name, a.UserID, in.Source, in.Charter)
		if e == nil {
			w.Header().Set("Location", base+"/"+v.ID)
		}
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("GET "+base+"/{team}", func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		v, e := teams.Get(string(repo.ID), r.PathValue("team"))
		writeDeliveryTeamResult(w, v, e, 200)
	})
	mux.HandleFunc("PATCH "+base+"/{team}/charter", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			deliveryteams.CharterInput
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		v, e := teams.Revise(string(repo.ID), r.PathValue("team"), a.UserID, in.ExpectedVersion, in.CharterInput)
		writeDeliveryTeamResult(w, v, e, 200)
	})
	mux.HandleFunc("POST "+base+"/{team}/participants", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			deliveryteams.ParticipantInput
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		preview, e := deliveryAccess(repo, a.UserID, in.Kind, in.PrincipalID, in.RequestedActions, repos, orgs)
		if e != nil {
			writeDeliveryTeamResult(w, deliveryteams.Team{}, e, 422)
			return
		}
		in.Access = preview
		v, e := teams.Invite(string(repo.ID), r.PathValue("team"), a.UserID, in.ExpectedVersion, in.ParticipantInput)
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{team}/participants/{participant}/response", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			Response        string `json:"response"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		v, e := teams.Get(string(repo.ID), r.PathValue("team"))
		if e == nil {
			var p *deliveryteams.Participant
			for i := range v.Participants {
				if v.Participants[i].ID == r.PathValue("participant") {
					p = &v.Participants[i]
				}
			}
			if p == nil {
				e = deliveryteams.ErrNotFound
			} else if p.Kind == "human" && p.PrincipalID != a.UserID {
				e = deliveryteams.ErrForbidden
			} else if p.Kind == "agent" && !agentOperated(repo, *p, a.UserID, orgs) {
				e = deliveryteams.ErrForbidden
			}
		}
		if e == nil {
			v, e = teams.Respond(string(repo.ID), r.PathValue("team"), r.PathValue("participant"), a.UserID, in.Response, in.ExpectedVersion)
		}
		writeDeliveryTeamResult(w, v, e, 200)
	})
	mux.HandleFunc("DELETE "+base+"/{team}/participants/{participant}", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			Reason          string `json:"reason"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		v, e := teams.Remove(string(repo.ID), r.PathValue("team"), r.PathValue("participant"), a.UserID, in.Reason, in.ExpectedVersion)
		writeDeliveryTeamResult(w, v, e, 200)
	})
	mux.HandleFunc("POST "+base+"/{team}/participants/{participant}/replacement", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			deliveryteams.ParticipantInput
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		preview, e := deliveryAccess(repo, a.UserID, in.Kind, in.PrincipalID, in.RequestedActions, repos, orgs)
		if e != nil {
			writeDeliveryTeamResult(w, deliveryteams.Team{}, e, 422)
			return
		}
		in.Access = preview
		v, e := teams.Replace(string(repo.ID), r.PathValue("team"), r.PathValue("participant"), a.UserID, in.ExpectedVersion, in.ParticipantInput)
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{team}/plan/versions", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			deliveryteams.PlanInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		current, e := teams.Get(string(repo.ID), r.PathValue("team"))
		acting := ""
		if e == nil && a.UserID != current.OrganizerID {
			for _, p := range current.Participants {
				if p.State == "accepted" && ((p.Kind == "human" && p.PrincipalID == a.UserID) || (p.Kind == "agent" && agentOperated(repo, p, a.UserID, orgs))) {
					acting = p.ID
					break
				}
			}
			if acting == "" {
				e = deliveryteams.ErrForbidden
			}
		}
		var v deliveryteams.Team
		if e == nil {
			v, e = teams.ProposePlan(string(repo.ID), r.PathValue("team"), a.UserID, acting, in.ExpectedVersion, in.PlanInput)
		}
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{team}/plan/versions/{planVersion}/acceptances", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			ParticipantID   string `json:"participant_id"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		pv, e := strconv.ParseInt(r.PathValue("planVersion"), 10, 64)
		if e != nil {
			writeDeliveryTeamResult(w, deliveryteams.Team{}, deliveryteams.ErrInvalid, 422)
			return
		}
		current, e := teams.Get(string(repo.ID), r.PathValue("team"))
		var invited *deliveryteams.Participant
		if e == nil {
			for i := range current.Participants {
				if current.Participants[i].ID == in.ParticipantID {
					invited = &current.Participants[i]
					break
				}
			}
		}
		if e == nil && (invited == nil || (invited.Kind == "human" && invited.PrincipalID != a.UserID) || (invited.Kind == "agent" && !agentOperated(repo, *invited, a.UserID, orgs))) {
			e = deliveryteams.ErrForbidden
		}
		var v deliveryteams.Team
		if e == nil {
			v, e = teams.AcceptPlan(string(repo.ID), r.PathValue("team"), in.ParticipantID, a.UserID, in.ExpectedVersion, pv)
		}
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{team}/streams/{stream}/contexts", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64                          `json:"expected_version"`
			ParticipantID   string                         `json:"participant_id"`
			Context         deliveryteams.ExecutionContext `json:"context"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		if !deliveryContextExists(in.Context, execution) {
			writeDeliveryTeamResult(w, deliveryteams.Team{}, deliveryteams.ErrInvalid, 422)
			return
		}
		current, e := teams.Get(string(repo.ID), r.PathValue("team"))
		if e == nil {
			e = authorizeDeliveryParticipant(repo, current, in.ParticipantID, a.UserID, orgs)
		}
		var v deliveryteams.Team
		if e == nil {
			v, e = teams.AttachExecution(string(repo.ID), r.PathValue("team"), a.UserID, in.ParticipantID, in.ExpectedVersion, in.Context, r.PathValue("stream"))
		}
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{team}/timeline", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			ParticipantID   string `json:"participant_id"`
			deliveryteams.TimelineInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		current, e := teams.Get(string(repo.ID), r.PathValue("team"))
		if e == nil {
			e = authorizeDeliveryParticipant(repo, current, in.ParticipantID, a.UserID, orgs)
		}
		var v deliveryteams.Team
		if e == nil {
			v, e = teams.PublishTimeline(string(repo.ID), r.PathValue("team"), a.UserID, in.ParticipantID, in.ExpectedVersion, in.TimelineInput)
		}
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{team}/handoffs", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			ParticipantID   string `json:"participant_id"`
			deliveryteams.HandoffInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		current, e := teams.Get(string(repo.ID), r.PathValue("team"))
		if e == nil {
			e = authorizeDeliveryParticipant(repo, current, in.ParticipantID, a.UserID, orgs)
		}
		var v deliveryteams.Team
		if e == nil {
			v, e = teams.RequestHandoff(string(repo.ID), r.PathValue("team"), a.UserID, in.ParticipantID, in.ExpectedVersion, in.HandoffInput)
		}
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{team}/handoffs/{handoff}/acceptance", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			ParticipantID   string `json:"participant_id"`
			Note            string `json:"note"`
		}
		if !readJSON(w, r, &in, 16<<10) {
			return
		}
		current, e := teams.Get(string(repo.ID), r.PathValue("team"))
		if e == nil {
			e = authorizeDeliveryParticipant(repo, current, in.ParticipantID, a.UserID, orgs)
		}
		var v deliveryteams.Team
		if e == nil {
			v, e = teams.AcceptHandoff(string(repo.ID), r.PathValue("team"), r.PathValue("handoff"), a.UserID, in.ParticipantID, in.Note, in.ExpectedVersion)
		}
		writeDeliveryTeamResult(w, v, e, 200)
	})
	mux.HandleFunc("POST "+base+"/{team}/streams/{stream}/status", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			ParticipantID   string `json:"participant_id"`
			deliveryteams.StreamStatusInput
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		current, e := teams.Get(string(repo.ID), r.PathValue("team"))
		if e == nil {
			e = authorizeDeliveryParticipant(repo, current, in.ParticipantID, a.UserID, orgs)
		}
		var v deliveryteams.Team
		if e == nil {
			v, e = teams.ReportStream(string(repo.ID), r.PathValue("team"), r.PathValue("stream"), a.UserID, in.ParticipantID, in.ExpectedVersion, in.StreamStatusInput)
		}
		writeDeliveryTeamResult(w, v, e, 200)
	})
	mux.HandleFunc("POST "+base+"/{team}/controls", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			deliveryteams.ControlInput
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		v, e := teams.Control(string(repo.ID), r.PathValue("team"), a.UserID, in.ExpectedVersion, in.ControlInput)
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{team}/integration/reconciliations", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
			deliveryteams.IntegrationInput
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		// Branch identities and conflicts are derived from Git, never trusted from
		// a participant's reconciliation narrative.
		opened, err := repos.Open(repo.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		for i := range in.Contributions {
			c := &in.Contributions[i]
			if c.SourceRepositoryID != "" && c.SourceRepositoryID != string(repo.ID) {
				writeDeliveryTeamResult(w, deliveryteams.Team{}, deliveryteams.ErrForbidden, 422)
				return
			}
			source, sourceName, sourceOK := branchTip(opened, c.SourceBranch)
			target, targetName, targetOK := branchTip(opened, c.TargetBranch)
			if !sourceOK || !targetOK || sourceName == targetName {
				writeDeliveryTeamResult(w, deliveryteams.Team{}, deliveryteams.ErrInvalid, 422)
				return
			}
			c.SourceRepositoryID = string(repo.ID)
			c.SourceBranch = sourceName
			c.TargetBranch = targetName
			c.SourceCommitID = string(source)
			c.TargetCommitID = string(target)
			c.Conflict, err = mergeHasConflicts(r.Context(), opened, target, source)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": "internal_error"})
				return
			}
		}
		v, e := teams.Reconcile(string(repo.ID), r.PathValue("team"), a.UserID, in.ExpectedVersion, in.IntegrationInput)
		writeDeliveryTeamResult(w, v, e, 201)
	})
	mux.HandleFunc("POST "+base+"/{team}/integration/reconciliations/{integration}/streams/{stream}/pull-request", func(w http.ResponseWriter, r *http.Request) {
		if pulls == nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		repo, a, ok := deliveryRepositoryWriteAccess(w, r, repos, credentials)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
		}
		if !readJSON(w, r, &in, 8<<10) {
			return
		}
		team, e := teams.Get(string(repo.ID), r.PathValue("team"))
		if e != nil {
			writeDeliveryTeamResult(w, team, e, 200)
			return
		}
		if in.ExpectedVersion != team.Version {
			writeDeliveryTeamResult(w, team, deliveryteams.ErrConflict, 200)
			return
		}
		var integration *deliveryteams.Integration
		for i := range team.Integrations {
			if team.Integrations[i].ID == r.PathValue("integration") {
				integration = &team.Integrations[i]
			}
		}
		if integration == nil || integration.Status != "ready" {
			writeDeliveryTeamResult(w, team, deliveryteams.ErrInvalid, 422)
			return
		}
		var contribution *deliveryteams.Contribution
		for i := range integration.Contributions {
			if integration.Contributions[i].StreamID == r.PathValue("stream") {
				contribution = &integration.Contributions[i]
			}
		}
		if contribution == nil || contribution.PullRequestID != "" {
			writeDeliveryTeamResult(w, team, deliveryteams.ErrInvalid, 422)
			return
		}
		for _, prior := range integration.Contributions {
			if prior.StreamID == contribution.StreamID {
				break
			}
			if prior.PullRequestID == "" {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "integration_order_pending", "stream_id": prior.StreamID})
				return
			}
		}
		opened, openErr := repos.Open(repo.ID)
		if openErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		source, _, sourceOK := branchTip(opened, contribution.SourceBranch)
		target, _, targetOK := branchTip(opened, contribution.TargetBranch)
		if !sourceOK || !targetOK || string(source) != contribution.SourceCommitID || string(target) != contribution.TargetCommitID {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "reconciliation_stale"})
			return
		}
		contributors := []string{}
		body := "Team outcome: " + team.Charter.Outcome + "\n\n" + contribution.Summary + "\n\nEvidence: " + strings.Join(contribution.EvidenceEntryIDs, ", ") + "\nHandoffs: " + strings.Join(contribution.HandoffIDs, ", ") + "\nResidual risks: " + strings.Join(contribution.ResidualRisks, "; ")
		for _, run := range team.StreamRuns {
			if run.StreamID == contribution.StreamID {
				body += "\nResource use: " + strconv.Itoa(run.ResourceUse.Hours) + " hours, " + strconv.Itoa(run.ResourceUse.CostUnits) + " cost units, " + strconv.Itoa(run.ResourceUse.AgentRuns) + " agent runs"
			}
		}
		for _, p := range team.Participants {
			for _, entry := range team.Timeline {
				if entry.StreamID == contribution.StreamID && entry.AuthorParticipantID == p.ID {
					contributors = append(contributors, p.PrincipalID)
					break
				}
			}
		}
		criteria := []pullrequests.CriterionStatus{}
		for _, x := range contribution.Criteria {
			criteria = append(criteria, pullrequests.CriterionStatus{Criterion: x.Criterion, Status: x.Status, Evidence: strings.Join(x.EvidenceEntryIDs, ", ")})
		}
		pr, err := pulls.Create(pullrequests.CreateParams{RepositoryID: string(repo.ID), SourceRepositoryID: string(repo.ID), AuthorID: a.UserID, Title: contribution.Title, Body: body, SourceBranch: contribution.SourceBranch, TargetBranch: contribution.TargetBranch, SourceCommitID: contribution.SourceCommitID, TargetCommitID: contribution.TargetCommitID, ContributorIDs: contributors, DeliveryEvidence: &pullrequests.DeliveryEvidence{Reasoning: "Delivery team " + team.ID + ", plan version " + strconv.FormatInt(integration.PlanVersion, 10) + ", stream " + contribution.StreamID, ResidualRisks: contribution.ResidualRisks, CompletionCriteria: criteria, RecordedByID: a.UserID, RecordedAt: time.Now().UTC()}})
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_pull_request"})
			return
		}
		updated, err := teams.LinkPullRequest(string(repo.ID), team.ID, integration.ID, contribution.StreamID, a.UserID, pr.ID, in.ExpectedVersion)
		if err != nil {
			writeDeliveryTeamResult(w, updated, err, 200)
			return
		}
		if checks != nil {
			_ = checks.Start(string(repo.ID), string(repo.ID), pr.ID, pr.SourceCommitID)
		}
		w.Header().Set("Location", "/repositories/"+string(repo.ID)+"/pull-requests/"+pr.ID)
		writeJSON(w, 201, map[string]any{"team": updated, "pull_request": pr})
	})
}

func deliveryContextExists(context deliveryteams.ExecutionContext, stores deliveryExecutionStores) bool {
	switch context.Kind {
	case "change_session":
		if stores.changes == nil || context.ParentID == "" {
			return false
		}
		item, err := stores.changes.Get(context.RepositoryID, context.ParentID, context.ID)
		return err == nil && item.SourceCommitID == context.Revision
	case "investigation":
		if stores.investigations == nil {
			return false
		}
		item, err := stores.investigations.Get(context.RepositoryID, context.ID)
		return err == nil && item.CommitID == context.Revision
	case "experiment":
		if stores.decisions == nil || context.ParentID == "" {
			return false
		}
		item, err := stores.decisions.Get(context.RepositoryID, context.ParentID)
		if err != nil {
			return false
		}
		for _, alternative := range item.Alternatives {
			for _, experiment := range alternative.Experiments {
				if experiment.ID == context.ID && experiment.Revision == context.Revision {
					return true
				}
			}
		}
		return false
	case "workspace":
		if stores.workspaces == nil {
			return false
		}
		item, err := stores.workspaces.Get(context.RepositoryID, context.ID)
		return err == nil && item.Revision == context.Revision
	default:
		return false
	}
}

func authorizeDeliveryParticipant(repo repositories.Repository, team deliveryteams.Team, participantID, actor string, orgs deliveryOrganizationStore) error {
	for _, p := range team.Participants {
		if p.ID != participantID || p.State != "accepted" {
			continue
		}
		if (p.Kind == "human" && p.PrincipalID == actor) || (p.Kind == "agent" && agentOperated(repo, p, actor, orgs)) {
			return nil
		}
	}
	return deliveryteams.ErrForbidden
}

func deliveryRepositoryWriteAccess(w http.ResponseWriter, r *http.Request, repos proposalRepositoryStore, credentials authStore) (repositories.Repository, auth.Grant, bool) {
	repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
	if !ok {
		return repo, actor, false
	}
	if actor.UserID == repo.OwnerID {
		return repo, actor, true
	}
	member, err := repos.IsCollaborator(repo.ID, actor.UserID)
	if err != nil || !member {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return repositories.Repository{}, auth.Grant{}, false
	}
	return repo, actor, true
}

func agentOperated(repo repositories.Repository, p deliveryteams.Participant, user string, orgs deliveryOrganizationStore) bool {
	if repo.OrganizationID == "" {
		return false
	}
	o, e := orgs.Get(repo.OrganizationID)
	if e != nil {
		return false
	}
	for _, a := range o.Agents {
		if a.ID == p.PrincipalID {
			for _, x := range a.OperatorIDs {
				if x == user {
					return true
				}
			}
		}
	}
	return false
}
func deliveryAccess(repo repositories.Repository, organizer, kind, principal string, requested []string, repos proposalRepositoryStore, orgs deliveryOrganizationStore) (deliveryteams.AccessPreview, error) {
	allowed := map[string]bool{"contents:read": true, "discussion:write": true, "candidate_branch:write": true}
	sources := []string{"repository participation"}
	if repo.OwnerID == organizer {
		allowed["metadata:write"] = true
		allowed["default_branch:write"] = true
		allowed["merge"] = true
		sources = []string{"repository ownership"}
	}
	participant := map[string]bool{}
	expiry := (*time.Time)(nil)
	if kind == "human" {
		member := principal == repo.OwnerID
		if !member {
			member, _ = repos.IsCollaborator(repo.ID, principal)
		}
		if !member {
			return deliveryteams.AccessPreview{}, deliveryteams.ErrForbidden
		}
		participant["contents:read"] = true
		participant["discussion:write"] = true
		participant["candidate_branch:write"] = true
		if principal == repo.OwnerID {
			participant["metadata:write"] = true
			participant["default_branch:write"] = true
			participant["merge"] = true
		}
		sources = []string{"existing repository role"}
	} else if kind == "agent" {
		if repo.OrganizationID == "" {
			return deliveryteams.AccessPreview{}, deliveryteams.ErrForbidden
		}
		o, e := orgs.Get(repo.OrganizationID)
		if e != nil {
			return deliveryteams.AccessPreview{}, deliveryteams.ErrForbidden
		}
		approved := false
		for _, a := range o.Agents {
			if a.ID == principal {
				approved = true
			}
		}
		if !approved {
			return deliveryteams.AccessPreview{}, deliveryteams.ErrForbidden
		}
		now := time.Now()
		for _, g := range o.RoleGrants {
			if g.PrincipalKind != "agent" || g.PrincipalID != principal || g.RevokedAt != nil || !now.Before(g.ExpiresAt) {
				continue
			}
			covers := false
			for _, x := range g.Resources {
				if x.RepositoryID == string(repo.ID) || (x.Kind == "repository" && x.ID == string(repo.ID)) {
					covers = true
				}
			}
			if !covers {
				continue
			}
			participant["contents:read"] = true
			if g.Role == "contributor" || g.Role == "maintainer" || g.Role == "operator" {
				participant["discussion:write"] = true
				participant["candidate_branch:write"] = true
			}
			if g.Role == "maintainer" || g.Role == "operator" {
				participant["metadata:write"] = true
			}
			sources = append(sources, "organization grant "+g.ID)
			if expiry == nil || g.ExpiresAt.Before(*expiry) {
				x := g.ExpiresAt
				expiry = &x
			}
		}
	} else {
		return deliveryteams.AccessPreview{}, deliveryteams.ErrInvalid
	}
	actions := []string{}
	missing := []string{}
	for _, x := range requested {
		if allowed[x] && participant[x] {
			actions = append(actions, x)
		} else {
			missing = append(missing, x)
		}
	}
	sort.Strings(actions)
	sort.Strings(missing)
	return deliveryteams.AccessPreview{Actions: actions, Sources: sources, ExpiresAt: expiry, Missing: missing, GrantsAuthority: false}, nil
}
func writeDeliveryTeamResult(w http.ResponseWriter, v any, e error, status int) {
	if e == nil {
		writeJSON(w, status, v)
		return
	}
	switch {
	case errors.Is(e, deliveryteams.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "delivery_team_not_found"})
	case errors.Is(e, deliveryteams.ErrForbidden):
		writeJSON(w, 403, map[string]string{"error": "authority_not_held"})
	case errors.Is(e, deliveryteams.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "delivery_team_changed"})
	case errors.Is(e, deliveryteams.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_delivery_team"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
}
