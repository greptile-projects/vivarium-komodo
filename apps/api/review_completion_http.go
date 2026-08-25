package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewcompletion"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewplans"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewrouting"
	"github.com/greptile-projects/vivarium-komodo/apps/api/reviewwork"
)

type reviewCompletionSources struct {
	completion *reviewcompletion.Store
	plans      *reviewplans.Store
	routing    *reviewrouting.Store
	work       *reviewwork.Store
}

func currentReviewCompletion(s reviewCompletionSources, repo, pull, revision, target string) (reviewcompletion.View, error) {
	p, e := s.plans.Get(repo, pull)
	if e != nil || len(p.Versions) == 0 {
		return reviewcompletion.View{}, e
	}
	v := p.Versions[len(p.Versions)-1]
	if v.Revision != revision || v.TargetRevision != target {
		return reviewcompletion.View{}, reviewplans.ErrConflict
	}
	r, e := s.routing.Get(repo, pull)
	if e != nil {
		return reviewcompletion.View{}, e
	}
	w, e := s.work.Get(repo, pull)
	if e != nil {
		return reviewcompletion.View{}, e
	}
	return s.completion.View(repo, pull, v, r, w), nil
}

func registerReviewCompletionHTTP(mux *http.ServeMux, s reviewCompletionSources, pulls pullRequestStore, repos pullRequestRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/pull-requests/{pull_request}/review-completion"
	context := func(w http.ResponseWriter, r *http.Request) (reviewplans.Version, reviewrouting.Routing, string, string, bool) {
		repo, actor, ok := proposalRepositoryAccess(w, r, repos, credentials, auth.RepositoryRead, r.Method != "GET")
		if !ok {
			return reviewplans.Version{}, reviewrouting.Routing{}, "", "", false
		}
		pull, ok := readPullRequest(w, pulls, string(repo.ID), r.PathValue("pull_request"))
		if !ok {
			return reviewplans.Version{}, reviewrouting.Routing{}, "", "", false
		}
		p, e := s.plans.Get(string(repo.ID), pull.ID)
		if e != nil || len(p.Versions) == 0 {
			writeJSON(w, 422, map[string]string{"error": "current_review_plan_required"})
			return reviewplans.Version{}, reviewrouting.Routing{}, "", "", false
		}
		v := p.Versions[len(p.Versions)-1]
		if v.Revision != pull.SourceCommitID || v.TargetRevision != pull.TargetCommitID {
			writeJSON(w, 409, map[string]string{"error": "review_plan_stale"})
			return v, reviewrouting.Routing{}, "", "", false
		}
		rt, e := s.routing.Get(string(repo.ID), pull.ID)
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "current_review_routing_required"})
			return v, rt, "", "", false
		}
		return v, rt, actor.UserID, repo.OwnerID, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		v, rt, _, _, ok := context(w, r)
		if !ok {
			return
		}
		work, e := s.work.Get(r.PathValue("repository"), r.PathValue("pull_request"))
		if e != nil {
			writeJSON(w, 422, map[string]string{"error": "current_review_work_required"})
			return
		}
		writeJSON(w, 200, s.completion.View(r.PathValue("repository"), r.PathValue("pull_request"), v, rt, work))
	})
	mux.HandleFunc("PUT "+base+"/requirements", func(w http.ResponseWriter, r *http.Request) {
		v, rt, actor, owner, ok := context(w, r)
		if !ok {
			return
		}
		if actor != owner {
			writeJSON(w, 403, map[string]string{"error": "maintainer_required"})
			return
		}
		var in struct {
			ExpectedVersion int64    `json:"expected_version"`
			AreaIDs         []string `json:"area_ids"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.completion.SetRequired(r.PathValue("repository"), r.PathValue("pull_request"), actor, in.AreaIDs, in.ExpectedVersion)
		writeCompletion(w, s, x, e, v, rt)
	})
	mux.HandleFunc("POST "+base+"/areas/{area}/acknowledgements", func(w http.ResponseWriter, r *http.Request) {
		v, rt, actor, _, ok := context(w, r)
		if !ok {
			return
		}
		var in struct {
			ExpectedVersion int64  `json:"expected_version"`
			AssignmentID    string `json:"assignment_id"`
			Decision        string `json:"decision"`
			Rationale       string `json:"rationale"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.completion.Acknowledge(r.PathValue("repository"), r.PathValue("pull_request"), actor, r.PathValue("area"), in.AssignmentID, in.Decision, in.Rationale, v, rt, in.ExpectedVersion)
		writeCompletion(w, s, x, e, v, rt)
	})
	mux.HandleFunc("POST "+base+"/overrides", func(w http.ResponseWriter, r *http.Request) {
		v, rt, actor, owner, ok := context(w, r)
		if !ok {
			return
		}
		if actor != owner {
			writeJSON(w, 403, map[string]string{"error": "maintainer_required"})
			return
		}
		var in struct {
			ExpectedVersion int64     `json:"expected_version"`
			AreaIDs         []string  `json:"area_ids"`
			Reason          string    `json:"reason"`
			FollowUp        string    `json:"follow_up"`
			ExpiresAt       time.Time `json:"expires_at"`
		}
		if !readJSON(w, r, &in, 64<<10) {
			return
		}
		x, e := s.completion.Override(r.PathValue("repository"), r.PathValue("pull_request"), actor, in.Reason, in.FollowUp, in.AreaIDs, in.ExpiresAt, in.ExpectedVersion)
		writeCompletion(w, s, x, e, v, rt)
	})
}
func writeCompletion(w http.ResponseWriter, s reviewCompletionSources, x reviewcompletion.Record, e error, v reviewplans.Version, r reviewrouting.Routing) {
	if errors.Is(e, reviewcompletion.ErrConflict) {
		writeJSON(w, 409, map[string]string{"error": "review_completion_version_conflict"})
		return
	}
	if e != nil {
		writeJSON(w, 422, map[string]string{"error": "invalid_review_completion"})
		return
	}
	work, e := s.work.Get(x.RepositoryID, x.PullRequestID)
	if e != nil {
		writeJSON(w, 422, map[string]string{"error": "current_review_work_required"})
		return
	}
	writeJSON(w, 200, s.completion.View(x.RepositoryID, x.PullRequestID, v, r, work))
}
