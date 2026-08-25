package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewrouting"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewwork"
)

func registerReviewWorkHTTP(mux *http.ServeMux, work *reviewwork.Store, plans *reviewplans.Store, routes *reviewrouting.Store, pulls pullRequestStore, repos pullRequestRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/pull-requests/{pull_request}/review-work"
	context := func(w http.ResponseWriter, r *http.Request, write bool) (reviewplans.Version, reviewrouting.Routing, string, bool) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, write)
		if !ok {
			return reviewplans.Version{}, reviewrouting.Routing{}, "", false
		}
		pull, ok := readPullRequest(w, pulls, string(repo.ID), r.PathValue("pull_request"))
		if !ok {
			return reviewplans.Version{}, reviewrouting.Routing{}, "", false
		}
		p, e := plans.Get(string(repo.ID), pull.ID)
		if e != nil || len(p.Versions) == 0 {
			writeJSON(w, 422, map[string]string{"error": "current_review_plan_required"})
			return reviewplans.Version{}, reviewrouting.Routing{}, "", false
		}
		v := p.Versions[len(p.Versions)-1]
		if v.Revision != pull.SourceCommitID || v.TargetRevision != pull.TargetCommitID {
			writeJSON(w, 409, map[string]string{"error": "review_plan_stale"})
			return v, reviewrouting.Routing{}, "", false
		}
		routing, e := routes.Get(string(repo.ID), pull.ID)
		if e != nil || routing.PlanVersion != v.Number || routing.Revision != v.Revision {
			writeJSON(w, 422, map[string]string{"error": "current_review_routing_required"})
			return v, routing, "", false
		}
		return v, routing, actor.UserID, true
	}
	open := func(w http.ResponseWriter, r *http.Request) (reviewplans.Version, reviewrouting.Routing, string, bool) {
		v, rt, actor, ok := context(w, r, r.Method != "GET")
		if !ok {
			return v, rt, actor, false
		}
		_, e := work.Open(r.PathValue("repository"), r.PathValue("pull_request"), v, rt)
		if errors.Is(e, reviewwork.ErrConflict) {
			writeJSON(w, 409, map[string]string{"error": "review_work_stale"})
			return v, rt, actor, false
		}
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return v, rt, actor, false
		}
		return v, rt, actor, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		v, rt, _, ok := context(w, r, false)
		if !ok {
			return
		}
		x, e := work.Open(r.PathValue("repository"), r.PathValue("pull_request"), v, rt)
		if errors.Is(e, reviewwork.ErrConflict) {
			x, e = work.Get(r.PathValue("repository"), r.PathValue("pull_request"))
		}
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/progress", func(w http.ResponseWriter, r *http.Request) {
		_, rt, actor, ok := open(w, r)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64    `json:"expected_version"`
			AssignmentID    string   `json:"assignment_id"`
			State           string   `json:"state"`
			QueueItemIDs    []string `json:"queue_item_ids"`
			Coverage        []string `json:"coverage"`
			Uncertainty     []string `json:"uncertainty"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		x, e := work.RecordProgress(r.PathValue("repository"), r.PathValue("pull_request"), actor, rt, in.AssignmentID, in.State, in.QueueItemIDs, in.Coverage, in.Uncertainty, in.ExpectedVersion)
		writeReviewWork(w, x, e)
	})
	mux.HandleFunc("POST "+base+"/findings", func(w http.ResponseWriter, r *http.Request) {
		_, rt, actor, ok := open(w, r)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64                 `json:"expected_version"`
			AssignmentID    string                `json:"assignment_id"`
			Summary         string                `json:"summary"`
			Severity        string                `json:"severity"`
			Conclusion      string                `json:"conclusion"`
			Citations       []reviewwork.Citation `json:"citations"`
			Uncertainty     []string              `json:"uncertainty"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		x, e := work.AddFinding(r.PathValue("repository"), r.PathValue("pull_request"), actor, rt, in.AssignmentID, in.Summary, in.Severity, in.Conclusion, in.Citations, in.Uncertainty, in.ExpectedVersion)
		writeReviewWork(w, x, e)
	})
	mux.HandleFunc("POST "+base+"/findings/{finding}/decisions", func(w http.ResponseWriter, r *http.Request) {
		_, _, actor, ok := open(w, r)
		if !ok {
			return
		}
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion    int64      `json:"expected_version"`
			Classification     string     `json:"classification"`
			Rationale          string     `json:"rationale"`
			Dissent            string     `json:"dissent"`
			RelatedFindingID   string     `json:"related_finding_id"`
			ExceptionScope     []string   `json:"exception_scope"`
			ExceptionExpiresAt *time.Time `json:"exception_expires_at"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		x, e := work.Decide(r.PathValue("repository"), r.PathValue("pull_request"), actor, r.PathValue("finding"), in.Classification, in.Rationale, in.Dissent, in.RelatedFindingID, repo.OwnerID == actor, in.ExceptionScope, in.ExceptionExpiresAt, in.ExpectedVersion)
		writeReviewWork(w, x, e)
	})
	mux.HandleFunc("POST "+base+"/findings/{finding}/work-links", func(w http.ResponseWriter, r *http.Request) {
		_, _, actor, ok := open(w, r)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			Kind            string `json:"kind"`
			Reference       string `json:"reference"`
			Revision        string `json:"revision"`
			Purpose         string `json:"purpose"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		x, e := work.LinkWork(r.PathValue("repository"), r.PathValue("pull_request"), actor, r.PathValue("finding"), in.Kind, in.Reference, in.Revision, in.Purpose, in.ExpectedVersion)
		writeReviewWork(w, x, e)
	})
	mux.HandleFunc("POST "+base+"/findings/{finding}/verifications", func(w http.ResponseWriter, r *http.Request) {
		_, _, actor, ok := open(w, r)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64                 `json:"expected_version"`
			Kind            string                `json:"kind"`
			Reference       string                `json:"reference"`
			BaseRevision    string                `json:"base_revision"`
			Revision        string                `json:"revision"`
			Outcome         string                `json:"outcome"`
			Summary         string                `json:"summary"`
			Citations       []reviewwork.Citation `json:"citations"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		x, e := work.Verify(r.PathValue("repository"), r.PathValue("pull_request"), actor, r.PathValue("finding"), in.Kind, in.Reference, in.BaseRevision, in.Revision, in.Outcome, in.Summary, in.Citations, in.ExpectedVersion)
		writeReviewWork(w, x, e)
	})
	mux.HandleFunc("POST "+base+"/revision-transitions", func(w http.ResponseWriter, r *http.Request) {
		plan, _, actor, ok := context(w, r, true)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64                             `json:"expected_version"`
			Findings        []reviewwork.FindingApplicability `json:"findings"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		x, e := work.Transition(r.PathValue("repository"), r.PathValue("pull_request"), actor, plan, in.Findings, in.ExpectedVersion)
		writeReviewWork(w, x, e)
	})
	mux.HandleFunc("POST "+base+"/messages", func(w http.ResponseWriter, r *http.Request) {
		_, rt, actor, ok := open(w, r)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64                 `json:"expected_version"`
			AssignmentID    string                `json:"assignment_id"`
			AreaID          string                `json:"area_id"`
			Kind            string                `json:"kind"`
			Body            string                `json:"body"`
			FindingIDs      []string              `json:"finding_ids"`
			Citations       []reviewwork.Citation `json:"citations"`
		}
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		x, e := work.AddMessage(r.PathValue("repository"), r.PathValue("pull_request"), actor, in.AreaID, in.Kind, in.Body, rt, in.AssignmentID, in.FindingIDs, in.Citations, in.ExpectedVersion)
		writeReviewWork(w, x, e)
	})
	mux.HandleFunc("POST "+base+"/handoffs", func(w http.ResponseWriter, r *http.Request) {
		_, rt, actor, ok := open(w, r)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion     int64    `json:"expected_version"`
			FromAssignmentID    string   `json:"from_assignment_id"`
			ToAssignmentID      string   `json:"to_assignment_id"`
			Reason              string   `json:"reason"`
			QueueItemIDs        []string `json:"queue_item_ids"`
			FindingIDs          []string `json:"finding_ids"`
			ResidualUncertainty []string `json:"residual_uncertainty"`
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		x, e := work.Handoff(r.PathValue("repository"), r.PathValue("pull_request"), actor, rt, in.FromAssignmentID, in.ToAssignmentID, in.Reason, in.QueueItemIDs, in.FindingIDs, in.ResidualUncertainty, in.ExpectedVersion)
		writeReviewWork(w, x, e)
	})
	mux.HandleFunc("POST "+base+"/handoffs/{handoff}/acceptance", func(w http.ResponseWriter, r *http.Request) {
		_, rt, actor, ok := open(w, r)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64 `json:"expected_version"`
		}
		if !readJSON(w, r, &in, 32<<10) {
			return
		}
		x, e := work.AcceptHandoff(r.PathValue("repository"), r.PathValue("pull_request"), actor, r.PathValue("handoff"), rt, in.ExpectedVersion)
		writeReviewWork(w, x, e)
	})
}

func writeReviewWork(w http.ResponseWriter, x reviewwork.Workspace, e error) {
	if errors.Is(e, reviewwork.ErrConflict) {
		writeJSON(w, 409, map[string]string{"error": "review_work_version_conflict"})
		return
	}
	if e != nil {
		writeJSON(w, 422, map[string]string{"error": "invalid_review_work"})
		return
	}
	writeJSON(w, 201, x)
}
