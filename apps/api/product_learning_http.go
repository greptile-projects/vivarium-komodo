package main

import (
	"errors"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productfeedback"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productlearning"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productopportunities"
	"github.com/greptile-projects/vivarium-komodo/apps/api/productroadmaps"
	"github.com/greptile-projects/vivarium-komodo/apps/api/roadmapdelivery"
)

func registerProductLearningHTTP(mux *http.ServeMux, s *productlearning.Store, deliveries *roadmapdelivery.Store, feedback *productfeedback.Store, opportunities *productopportunities.Store, roadmaps *productroadmaps.Store, repos proposalRepositoryStore, credentials authStore) {
	base := "/repositories/{repository}/roadmap-deliveries/{delivery}/learning"
	access := func(w http.ResponseWriter, r *http.Request, write bool) (string, string, bool) {
		p := auth.RepositoryRead
		if write {
			p = auth.RepositoryWrite
		}
		repo, a, ok := proposalRepositoryAccess(w, r, repos, credentials, p, write)
		if !ok {
			return "", "", false
		}
		return string(repo.ID), a.UserID, true
	}
	ensure := func(repo, id string) (productlearning.Record, error) {
		d, e := deliveries.Get(repo, id)
		if e != nil {
			return productlearning.Record{}, productlearning.ErrNotFound
		}
		return s.Ensure(repo, d.ID, d.RoadmapID, d.OutcomeID, d.OpportunityID, d.OpportunityVersion)
	}
	project := func(v productlearning.Record, actor string, writer bool) productlearning.Record {
		if writer {
			return v
		}
		out := []productlearning.Update{}
		allowed := map[string]bool{}
		for _, u := range v.Updates {
			target := learningContains(u.StakeholderIDs, actor)
			for _, fid := range u.FeedbackIDs {
				if f, e := feedback.Get(v.RepositoryID, fid); e == nil && f.ReporterID == actor && f.Consent.ProductUpdates && f.Consent.WithdrawnAt == nil {
					target = true
					allowed[fid] = true
				}
			}
			left := false
			for _, d := range v.Departures {
				if d.ActorID == actor && (d.FeedbackID == "" || learningContains(u.FeedbackIDs, d.FeedbackID)) {
					left = true
				}
			}
			if target && !left {
				if u.Audience != "public" {
					for i := range u.Links {
						if !u.Links[i].Public {
							u.Links[i].ResourceID = ""
						}
					}
				}
				out = append(out, u)
			}
		}
		v.Updates = out
		rs := []productlearning.Response{}
		for _, x := range v.Responses {
			if x.AuthorID == actor && allowed[x.FeedbackID] {
				rs = append(rs, x)
			}
		}
		v.Responses = rs
		v.Departures = nil
		return v
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, false)
		if !ok {
			return
		}
		v, e := ensure(repo, r.PathValue("delivery"))
		if learningError(w, e) {
			return
		}
		_, _, writer := accessSilent(r, repos, credentials, auth.RepositoryWrite)
		writeJSON(w, 200, project(v, a, writer))
	})
	mux.HandleFunc("POST "+base+"/updates", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		if _, e := ensure(repo, r.PathValue("delivery")); learningError(w, e) {
			return
		}
		var in productlearning.UpdateInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		for _, fid := range in.FeedbackIDs {
			f, e := feedback.Get(repo, fid)
			if e != nil || !f.Consent.ProductUpdates || f.Consent.WithdrawnAt != nil {
				writeJSON(w, 422, map[string]string{"error": "feedback_updates_not_permitted"})
				return
			}
		}
		v, e := s.Publish(repo, r.PathValue("delivery"), a, in)
		if learningError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
	mux.HandleFunc("POST "+base+"/updates/{update}/responses", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			FeedbackID string `json:"feedback_id"`
			productlearning.ResponseInput
		}
		if !readJSON(w, r, &in, 128<<10) {
			return
		}
		f, e := feedback.Get(repo, in.FeedbackID)
		if e != nil || f.ReporterID != a || !f.Consent.ProductUpdates || f.Consent.WithdrawnAt != nil {
			writeJSON(w, 403, map[string]string{"error": "feedback_reporter_with_updates_required"})
			return
		}
		v, e := s.Respond(repo, r.PathValue("delivery"), r.PathValue("update"), in.FeedbackID, a, in.ResponseInput)
		if learningError(w, e) {
			return
		}
		writeJSON(w, 201, project(v, a, false))
	})
	mux.HandleFunc("POST "+base+"/leave", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, false)
		if !ok {
			return
		}
		var in struct {
			FeedbackID string `json:"feedback_id"`
			Reason     string `json:"reason"`
		}
		if !readJSON(w, r, &in, 4096) {
			return
		}
		if in.FeedbackID != "" {
			f, e := feedback.Get(repo, in.FeedbackID)
			if e != nil || f.ReporterID != a {
				writeJSON(w, 403, map[string]string{"error": "feedback_reporter_required"})
				return
			}
		}
		v, e := s.Leave(repo, r.PathValue("delivery"), a, in.FeedbackID, in.Reason)
		if learningError(w, e) {
			return
		}
		writeJSON(w, 200, project(v, a, false))
	})
	mux.HandleFunc("POST "+base+"/lessons", func(w http.ResponseWriter, r *http.Request) {
		repo, a, ok := access(w, r, true)
		if !ok {
			return
		}
		v, e := ensure(repo, r.PathValue("delivery"))
		if learningError(w, e) {
			return
		}
		var in productlearning.LessonInput
		if !readJSON(w, r, &in, 256<<10) {
			return
		}
		if in.RoadmapID != v.RoadmapID {
			writeJSON(w, 422, map[string]string{"error": "invalid_learning_roadmap"})
			return
		}
		rm, e := roadmaps.Get(repo, in.RoadmapID)
		if e != nil || rm.CurrentVersion != in.RoadmapVersion {
			writeJSON(w, 422, map[string]string{"error": "roadmap_revision_not_current"})
			return
		}
		opp, e := opportunities.Get(repo, v.OpportunityID)
		if e != nil || opp.CurrentVersion < v.OpportunityVersion {
			writeJSON(w, 422, map[string]string{"error": "invalid_learning_opportunity"})
			return
		}
		v, e = s.RecordLesson(repo, v.DeliveryID, a, in)
		if learningError(w, e) {
			return
		}
		writeJSON(w, 201, v)
	})
}

func accessSilent(r *http.Request, repos proposalRepositoryStore, c authStore, p auth.Scope) (string, string, bool) {
	rw := &discardWriter{h: http.Header{}}
	repo, a, ok := proposalRepositoryAccess(rw, r, repos, c, p, true)
	return string(repo.ID), a.UserID, ok
}

type discardWriter struct{ h http.Header }

func (d *discardWriter) Header() http.Header       { return d.h }
func (d *discardWriter) Write([]byte) (int, error) { return 0, nil }
func (d *discardWriter) WriteHeader(int)           {}
func learningContains(v []string, x string) bool {
	for _, y := range v {
		if x == y {
			return true
		}
	}
	return false
}
func learningError(w http.ResponseWriter, e error) bool {
	if e == nil {
		return false
	}
	switch {
	case errors.Is(e, productlearning.ErrNotFound):
		writeJSON(w, 404, map[string]string{"error": "product_learning_not_found"})
	case errors.Is(e, productlearning.ErrConflict):
		writeJSON(w, 409, map[string]string{"error": "product_learning_conflict"})
	case errors.Is(e, productlearning.ErrInvalid):
		writeJSON(w, 422, map[string]string{"error": "invalid_product_learning"})
	default:
		writeJSON(w, 500, map[string]string{"error": "internal_error"})
	}
	return true
}
