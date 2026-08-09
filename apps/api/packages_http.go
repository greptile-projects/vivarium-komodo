package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

type packageStore interface {
	Publish(packagecatalog.PublishParams, io.Reader) (packagecatalog.Version, error)
	Get(string, string) (packagecatalog.Version, error)
	List(string) ([]packagecatalog.Version, error)
	OpenArtifact(string, string) (packagecatalog.Version, *os.File, error)
	GetByID(string) (packagecatalog.Version, error)
	Search(string) ([]packagecatalog.Version, error)
}

type packageCredentialIssuer interface {
	IssueRepositoryPackage(string, string, string, []string, time.Duration) (auth.IssuedGrant, error)
}

func registerPackagesHTTP(mux *http.ServeMux, packages packageStore, releaseStore releaseStore, builds releaseBuildStore, repositories pullRequestRepositoryStore, credentials authStore) {
	mux.HandleFunc("POST /repositories/{repository}/packages", publishPackage(packages, releaseStore, builds, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/packages", listPackages(packages, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/packages/{package}", getPackage(packages, repositories, credentials))
	mux.HandleFunc("GET /repositories/{repository}/packages/{package}/artifact", getPackageArtifact(packages, repositories, credentials))
	mux.HandleFunc("GET /packages", searchPackages(packages))
	mux.HandleFunc("GET /packages/{package}", inspectPublicPackage(packages))
	mux.HandleFunc("POST /repositories/{repository}/package-credentials", createPackageCredential(packages, repositories, credentials))
	mux.HandleFunc("GET /package-registry/{package}", packageRegistryMetadata(packages, repositories, credentials))
	mux.HandleFunc("GET /package-registry/artifacts/{package}", packageRegistryArtifact(packages, repositories, credentials))
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
			Name          string                  `json:"name"`
			Version       string                  `json:"version"`
			ReleaseID     string                  `json:"release_id"`
			BuildRunID    string                  `json:"build_run_id"`
			ArtifactID    string                  `json:"artifact_id"`
			Platform      packagecatalog.Platform `json:"platform"`
			Dependencies  map[string]string       `json:"dependencies"`
			Documentation string                  `json:"documentation"`
			Visibility    string                  `json:"visibility"`
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
		item, err := packages.Publish(packagecatalog.PublishParams{OwnerID: repository.OwnerID, Name: input.Name, Version: input.Version, RepositoryID: string(repository.ID), ReleaseID: release.ID, SourceCommitID: release.CommitID, ArtifactID: artifact.ID, ArtifactPath: artifact.Path, ArtifactMediaType: artifact.MediaType, ArtifactSize: artifact.Size, ExpectedSHA256: artifact.SHA256, Build: packagecatalog.BuildAttestation{RunID: selected.ID, BuildName: selected.Definition.Name, Command: selected.Definition.Command, CompletedAt: completed}, Platform: input.Platform, Dependencies: input.Dependencies, Documentation: input.Documentation, PublisherID: actor.UserID, Visibility: input.Visibility}, file)
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

func searchPackages(store packageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := store.Search(r.URL.Query().Get("q"))
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		visible := make([]packagecatalog.Version, 0, len(items))
		for _, item := range items {
			if item.Visibility == "public" {
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

func inspectPublicPackage(store packageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, err := store.GetByID(r.PathValue("package"))
		if err != nil || item.Visibility != "public" {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		writeJSON(w, 200, item)
	}
}

func createPackageCredential(store packageStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		consumer, actor, ok := proposalRepositoryAccess(w, r, repositories, credentials, auth.RepositoryRead, true)
		if !ok {
			return
		}
		var input struct {
			Name              string   `json:"name"`
			PackageVersionIDs []string `json:"package_version_ids"`
			ExpiresInHours    int      `json:"expires_in_hours"`
		}
		if !readJSON(w, r, &input, 32<<10) {
			return
		}
		if input.ExpiresInHours <= 0 || input.ExpiresInHours > 24 {
			writeJSON(w, 422, map[string]string{"error": "invalid_package_credential"})
			return
		}
		for _, id := range input.PackageVersionIDs {
			item, err := store.GetByID(id)
			if err != nil {
				writeJSON(w, 422, map[string]string{"error": "unauthorized_package"})
				return
			}
			if item.Visibility == "private" {
				publisher, err := repositories.Inspect(storage.ID(item.RepositoryID))
				if err != nil {
					writeJSON(w, 422, map[string]string{"error": "unauthorized_package"})
					return
				}
				allowed := publisher.OwnerID == actor.UserID
				if !allowed {
					allowed, _ = repositories.IsCollaborator(publisher.ID, actor.UserID)
				}
				if !allowed {
					writeJSON(w, 422, map[string]string{"error": "unauthorized_package"})
					return
				}
			}
		}
		issuer, ok := credentials.(packageCredentialIssuer)
		if !ok {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		name := strings.TrimSpace(input.Name)
		if name == "" {
			name = "Package install for " + consumer.Name
		}
		issued, err := issuer.IssueRepositoryPackage(actor.UserID, name, string(consumer.ID), input.PackageVersionIDs, time.Duration(input.ExpiresInHours)*time.Hour)
		if err != nil {
			writeJSON(w, 422, map[string]string{"error": "invalid_package_credential"})
			return
		}
		writeJSON(w, 201, grantResponse(issued.Grant, issued.Token))
	}
}

// packageGrant authenticates npm-style Bearer credentials and enforces the
// immutable version allowlist captured for the consuming repository.
func packageGrant(r *http.Request, credentials authStore, versionID string) (auth.Grant, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return auth.Grant{}, false
	}
	grant, err := credentials.Authenticate(strings.TrimPrefix(header, "Bearer "), auth.PackageRead)
	if err != nil {
		return auth.Grant{}, false
	}
	for _, id := range grant.PackageVersionIDs {
		if id == versionID {
			return grant, true
		}
	}
	return auth.Grant{}, false
}

func registryAccess(r *http.Request, store packageStore, repositories pullRequestRepositoryStore, credentials authStore, item packagecatalog.Version) bool {
	if item.Visibility == "public" {
		return true
	}
	grant, ok := packageGrant(r, credentials, item.ID)
	if !ok || grant.RepositoryID == "" {
		return false
	}
	consumer, err := repositories.Inspect(storage.ID(grant.RepositoryID))
	if err != nil {
		return false
	}
	if consumer.OwnerID != grant.UserID && !collaborator(repositories, consumer.ID, grant.UserID) {
		return false
	}
	publisher, err := repositories.Inspect(storage.ID(item.RepositoryID))
	if err != nil {
		return false
	}
	return publisher.OwnerID == grant.UserID || collaborator(repositories, publisher.ID, grant.UserID)
}
func collaborator(repositories pullRequestRepositoryStore, id storage.ID, user string) bool {
	ok, _ := repositories.IsCollaborator(id, user)
	return ok
}

func packageRegistryMetadata(store packageStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := url.PathUnescape(r.PathValue("package"))
		items, err := store.Search(identity)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		versions := map[string]any{}
		latest := ""
		for _, item := range items {
			if !strings.EqualFold(item.Identity, identity) || !registryAccess(r, store, repositories, credentials, item) {
				continue
			}
			digest, _ := hexToBytes(item.SHA256)
			versions[item.Version] = map[string]any{"name": item.Identity, "version": item.Version, "description": item.Documentation, "os": []string{item.Platform.OS}, "cpu": []string{item.Platform.Arch}, "dependencies": item.Dependencies, "dist": map[string]any{"tarball": registryArtifactURL(r, item.ID), "integrity": "sha256-" + base64.StdEncoding.EncodeToString(digest)}, "komodo": item}
			if latest == "" {
				latest = item.Version
			}
		}
		if len(versions) == 0 {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"name": identity, "dist-tags": map[string]string{"latest": latest}, "versions": versions})
	}
}

func hexToBytes(value string) ([]byte, error) {
	return hex.DecodeString(value)
}
func registryArtifactURL(r *http.Request, id string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded == "http" || forwarded == "https" {
		scheme = forwarded
	}
	host := r.Host
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		host = forwarded
	}
	prefix := strings.TrimSuffix(strings.TrimSpace(r.Header.Get("X-Forwarded-Prefix")), "/")
	return scheme + "://" + host + prefix + "/package-registry/artifacts/" + id
}
func packageRegistryArtifact(store packageStore, repositories pullRequestRepositoryStore, credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		item, err := store.GetByID(r.PathValue("package"))
		if err != nil || !registryAccess(r, store, repositories, credentials, item) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		_, file, err := store.OpenArtifact(item.RepositoryID, item.ID)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		}
		defer file.Close()
		w.Header().Set("Content-Type", item.ArtifactMediaType)
		w.Header().Set("X-Checksum-SHA256", item.SHA256)
		http.ServeContent(w, r, path.Base(item.ArtifactPath), item.PublishedAt, file)
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
