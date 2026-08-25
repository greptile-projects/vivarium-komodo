package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewrouting"
)

func registerReviewRoutingHTTP(mux *http.ServeMux, routes *reviewrouting.Store, plans *reviewplans.Store, pulls pullRequestStore, repos pullRequestRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/pull-requests/{pull_request}/review-routing"
	context := func(w http.ResponseWriter, r *http.Request) (reviewplans.Plan, reviewplans.Version, bool) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return reviewplans.Plan{}, reviewplans.Version{}, false
		}
		pull, ok := readPullRequest(w, pulls, string(repo.ID), r.PathValue("pull_request"))
		if !ok {
			return reviewplans.Plan{}, reviewplans.Version{}, false
		}
		p, e := plans.Get(string(repo.ID), pull.ID)
		if e != nil || len(p.Versions) == 0 {
			writeJSON(w, 422, map[string]string{"error": "current_review_plan_required"})
			return p, reviewplans.Version{}, false
		}
		v := p.Versions[len(p.Versions)-1]
		if v.Revision != pull.SourceCommitID || v.TargetRevision != pull.TargetCommitID {
			writeJSON(w, 409, map[string]string{"error": "review_plan_stale"})
			return p, v, false
		}
		return p, v, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, _, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		pull, ok := readPullRequest(w, pulls, string(repo.ID), r.PathValue("pull_request"))
		if !ok {
			return
		}
		x, e := routes.Get(string(repo.ID), pull.ID)
		if errors.Is(e, reviewrouting.ErrNotFound) {
			writeJSON(w, 200, map[string]any{"suggestions": []any{}, "assignments": []any{}, "events": []any{}, "reassignment_areas": []any{}})
			return
		}
		if e != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/suggestions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "maintainer_required"})
			return
		}
		_, v, ok := context(w, r)
		if !ok {
			return
		}
		var body struct {
			Candidates []reviewrouting.Candidate `json:"candidates"`
		}
		if !readJSON(w, r, &body, 256<<10) {
			return
		}
		areas := make([]reviewrouting.Area, 0, len(v.Areas))
		for _, a := range v.Areas {
			areas = append(areas, reviewrouting.Area{ID: a.ID, Expertise: a.Expertise, Paths: a.Paths, Questions: a.Questions})
		}
		x, e := routes.Suggest(string(repo.ID), r.PathValue("pull_request"), v.Revision, v.Number, areas, body.Candidates)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_reviewer_candidates"})
			return
		}
		writeJSON(w, 200, x)
	})
	mux.HandleFunc("POST "+base+"/areas/{area}/invitations", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		if actor.UserID != repo.OwnerID {
			writeJSON(w, 403, map[string]string{"error": "maintainer_required"})
			return
		}
		_, v, ok := context(w, r)
		if !ok {
			return
		}
		var body struct {
			Candidate  reviewrouting.Candidate `json:"candidate"`
			Deadline   *time.Time              `json:"deadline,omitempty"`
			Escalation string                  `json:"escalation"`
			Reason     string                  `json:"reason"`
			Replaces   string                  `json:"replaces,omitempty"`
		}
		if !readJSON(w, r, &body, 64<<10) {
			return
		}
		var area reviewrouting.Area
		for _, a := range v.Areas {
			if a.ID == r.PathValue("area") {
				area = reviewrouting.Area{ID: a.ID, Expertise: a.Expertise, Paths: a.Paths, Questions: a.Questions}
			}
		}
		x, e := routes.Invite(string(repo.ID), r.PathValue("pull_request"), actor.UserID, v.Number, v.Revision, area, body.Candidate, body.Deadline, body.Escalation, body.Reason, body.Replaces)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "reviewer_not_eligible_or_routing_changed"})
			return
		}
		writeJSON(w, 201, x)
	})
	mux.HandleFunc("POST "+base+"/assignments/{assignment}/transitions", func(w http.ResponseWriter, r *http.Request) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var body struct {
			State  string `json:"state"`
			Reason string `json:"reason"`
		}
		if !readJSON(w, r, &body, 32<<10) {
			return
		}
		x, e := routes.Transition(string(repo.ID), r.PathValue("pull_request"), r.PathValue("assignment"), actor.UserID, body.State, body.Reason, actor.UserID == repo.OwnerID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_assignment_transition"})
			return
		}
		writeJSON(w, 200, x)
	})
}
