package main

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
)

type packageStore interface {
	Publish(packagecatalog.PublishParams, io.Reader) (packagecatalog.Version, error)
	Get(string, string) (packagecatalog.Version, error)
	List(string) ([]packagecatalog.Version, error)
	OpenArtifact(string, string) (packagecatalog.Version, *os.File, error)
}

func registerPackagesHTTP(mux *http.ServeMux, packages packageStore, releaseStore releaseStore, builds releaseBuildStore, repositories pullRequestRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories/{repository}/packages", publishPackage(packages, releaseStore, builds, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/packages", listPackages(packages, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/packages/{package}", getPackage(packages, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/packages/{package}/artifact", getPackageArtifact(packages, repositories, credentials))
}

func publishPackage(packages packageStore, releaseStore releaseStore, builds releaseBuildStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryWrite, true)
		if !ok {
			return
		}
		if actor.UserID != repository.OwnerID {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		var input struct {
			Name         string                  `json:"name"`
			Version      string                  `json:"version"`
			ReleaseID    string                  `json:"release_id"`
			BuildRunID   string                  `json:"build_run_id"`
			ArtifactID   string                  `json:"artifact_id"`
			Platform     packagecatalog.Platform `json:"platform"`
			Dependencies map[string]string       `json:"dependencies"`
			Visibility   string                  `json:"visibility"`
		}
		if !readJSON(w, r, &input, 64<<10) {
			return
		}
		if input.Visibility == "public" && repository.Visibility != "public" {
			writeJSON(w, 422, map[string]string{"error": "public_package_requires_public_repository"})
			return
		}
		release, err := releaseStore.Get(string(repository.ID), input.ReleaseID)
		if errors.Is(err, releases.ErrNotFound) {
			writeJSON(w, 422, map[string]string{"error": "invalid_release"})
			return
		}
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		attempts, err := builds.List(string(repository.ID), "release:"+release.ID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		latest := map[string]checkruns.Run{}
		for _, run := range attempts {
			if _, exists := latest[run.Definition.Name]; !exists {
				latest[run.Definition.Name] = run
			}
		}
		verified := len(latest) > 0
		for _, run := range latest {
			if run.State != checkruns.Succeeded || run.CommitID != release.CommitID {
				verified = false
			}
		}
		selected, selectedOK := latestRun(attempts, input.BuildRunID)
		if !verified || !selectedOK || selected.State != checkruns.Succeeded || latest[selected.Definition.Name].ID != selected.ID {
			writeJSON(w, 409, map[string]string{"error": "release_not_verified"})
			return
		}
		artifact, file, err := builds.OpenArtifact(string(repository.ID), "release:"+release.ID, selected.ID, input.ArtifactID)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_artifact"})
			return
		}
		defer file.Close()
		completed := time.Time{}
		if selected.CompletedAt != nil {
			completed = *selected.CompletedAt
		}
		item, err := packages.Publish(packagecatalog.PublishParams{OwnerID: repository.OwnerID, Name: input.Name, Version: input.Version, RepositoryID: string(repository.ID), ReleaseID: release.ID, SourceCommitID: release.CommitID, ArtifactID: artifact.ID, ArtifactPath: artifact.Path, ArtifactMediaType: artifact.MediaType, ArtifactSize: artifact.Size, ExpectedSHA256: artifact.SHA256, Build: packagecatalog.BuildAttestation{RunID: selected.ID, BuildName: selected.Definition.Name, Command: selected.Definition.Command, CompletedAt: completed}, Platform: input.Platform, Dependencies: input.Dependencies, PublisherID: actor.UserID, Visibility: input.Visibility}, file)
		switch {
		case errors.Is(err, packagecatalog.ErrInvalid):
			writeJSON(w, 422, map[string]string{"error": "invalid_package"})
		case errors.Is(err, packagecatalog.ErrVersionConflict):
			writeJSON(w, 409, map[string]string{"error": "package_version_taken"})
		case err != nil:
			writeJSON(w, 500, map[string]string{"error": "publication_failed"})
		default:
			w.Header().Set("Location", "/repositories/"+string(repository.ID)+"/packages/"+item.ID)
			writeJSON(w, 201, item)
		}
	}
}

func latestRun(items []checkruns.Run, id string) (checkruns.Run, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return checkruns.Run{}, false
}

func listPackages(store packageStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
		if !ok {
			return
		}
		items, err := store.List(string(repository.ID))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		visible := []packagecatalog.Version{}
		participant := actor.UserID == repository.OwnerID
		if actor.UserID != "" && !participant {
			participant, _ = repositories.IsCollaborator(repository.ID, actor.UserID)
		}
		for _, item := range items {
			if item.Visibility == "public" || participant {
				visible = append(visible, item)
			}
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		writeJSON(w, 200, map[string]any{"items": paginate(visible, page, perPage), "page": page, "per_page": perPage, "total_count": len(visible)})
	}
}

func packageAccess(w http.ResponseWriter, r *http.Request, store packageStore, repositories pullRequestRepositoryStore, credentials authStore) (packagecatalog.Version, bool) {
	repository, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, false)
	if !ok {
		return packagecatalog.Version{}, false
	}
	item, err := store.Get(string(repository.ID), r.PathValue("package"))
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not_found"})
		return packagecatalog.Version{}, false
	}
	if item.Visibility == "private" {
		participant := actor.UserID == repository.OwnerID
		if actor.UserID != "" && !participant {
			participant, _ = repositories.IsCollaborator(repository.ID, actor.UserID)
		}
		if !participant {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return packagecatalog.Version{}, false
		}
	}
	return item, true
}
func getPackage(store packageStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, ok := packageAccess(w, r, store, repositories, credentials)
		if ok {
			writeJSON(w, 200, item)
		}
	}
}
func getPackageArtifact(store packageStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, ok := packageAccess(w, r, store, repositories, credentials)
		if !ok {
			return
		}
		_, file, err := store.OpenArtifact(item.RepositoryID, item.ID)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", item.ArtifactMediaType)
		w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(path.Base(item.ArtifactPath)))
		w.Header().Set("X-Checksum-SHA256", item.SHA256)
		http.ServeContent(w, r, path.Base(item.ArtifactPath), item.PublishedAt, file)
	}
}
