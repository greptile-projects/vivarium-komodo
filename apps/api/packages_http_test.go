package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestPackagePublicationRetainsVerifiedProvenance(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "source", Visibility: repositories.Public})
	_, _ = catalog.AddCollaborator("owner", repository.ID, "collaborator")
	releaseStore, _ := releases.New(t.TempDir())
	release, _ := releaseStore.Create(releases.CreateParams{RepositoryID: string(repository.ID), Version: "v1", CommitID: "reviewed", CreatedByID: "owner"})
	builds, _ := checkruns.New(t.TempDir())
	run, _ := builds.Create(string(repository.ID), "release:"+release.ID, release.CommitID, checkruns.Definition{Name: "package", Command: "make package"})
	run, _ = builds.Start(run.ID)
	artifact, _ := builds.AddArtifact(run.ID, "dist/sdk.tgz", "application/gzip", []byte("package bytes"))
	run, _ = builds.Complete(run.ID, 0, false, "")
	packages, _ := packagecatalog.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	collaborator := issueAccess(t, credentials, "collaborator", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPackagesHTTP(mux, packages, releaseStore, builds, catalog, credentials)
	body := map[string]any{"name": "sdk", "version": "1.2.3", "release_id": release.ID, "build_run_id": run.ID, "artifact_id": artifact.ID, "platform": map[string]string{"os": "linux", "arch": "amd64", "runtime": "go1.24"}, "dependencies": map[string]string{"@owner/core": "^1.0.0"}, "documentation": "# SDK\n\nVerified client library.", "visibility": "public"}
	request := func(token string, payload map[string]any) *httptest.ResponseRecorder {
		data, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/repositories/"+string(repository.ID)+"/packages", bytes.NewReader(data))
		req.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, req)
		return response
	}
	if response := request(collaborator, body); response.Code != 404 {
		t.Fatalf("collaborator publish = %d %s", response.Code, response.Body.String())
	}
	response := request(owner, body)
	if response.Code != 201 {
		t.Fatalf("publish = %d %s", response.Code, response.Body.String())
	}
	var item packagecatalog.Version
	_ = json.Unmarshal(response.Body.Bytes(), &item)
	if item.Identity != "@owner/sdk" || item.SourceCommitID != release.CommitID || item.Build.RunID != run.ID || item.SHA256 != artifact.SHA256 || item.PublisherID != "owner" || item.Visibility != "public" || item.DocumentationSHA256 == "" {
		t.Fatalf("package = %#v", item)
	}
	if response = request(owner, body); response.Code != 409 {
		t.Fatalf("conflict = %d %s", response.Code, response.Body.String())
	}
	retry, _ := builds.CreateAttempt(string(repository.ID), "release:"+release.ID, release.CommitID, run.Definition, "owner", run.ID)
	retry, _ = builds.Start(retry.ID)
	retry, _ = builds.Complete(retry.ID, 1, false, "newest attempt failed")
	body["version"] = "1.2.4"
	if response = request(owner, body); response.Code != 409 || !bytes.Contains(response.Body.Bytes(), []byte("release_not_verified")) {
		t.Fatalf("stale success = %d %s", response.Code, response.Body.String())
	}
	download := httptest.NewRequest(http.MethodGet, "/repositories/"+string(repository.ID)+"/packages/"+item.ID+"/artifact", nil)
	downloadResponse := httptest.NewRecorder()
	mux.ServeHTTP(downloadResponse, download)
	if downloadResponse.Code != 200 || downloadResponse.Body.String() != "package bytes" {
		t.Fatalf("download = %d %q", downloadResponse.Code, downloadResponse.Body.String())
	}
}

func TestPackageDiscoveryAndRepositoryScopedRegistryCredential(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	publisher, _ := catalog.Create("owner", repositories.Metadata{Name: "sdk", Visibility: repositories.Private})
	consumer, _ := catalog.Create("owner", repositories.Metadata{Name: "app", Visibility: repositories.Private})
	packages, _ := packagecatalog.New(t.TempDir())
	publish := func(name, visibility string) packagecatalog.Version {
		content := []byte("package " + name)
		digest := sha256.Sum256(content)
		item, err := packages.Publish(packagecatalog.PublishParams{OwnerID: "owner", Name: name, Version: "1.0.0", RepositoryID: string(publisher.ID), ReleaseID: "release", SourceCommitID: "commit", ArtifactID: "artifact-" + name, ArtifactPath: name + ".tgz", ArtifactMediaType: "application/gzip", ArtifactSize: int64(len(content)), ExpectedSHA256: hex.EncodeToString(digest[:]), Build: packagecatalog.BuildAttestation{RunID: "run", BuildName: "package", Command: "build", CompletedAt: time.Now()}, Platform: packagecatalog.Platform{OS: "linux", Arch: "amd64"}, Documentation: "# " + name + "\nInstall documentation.", PublisherID: "owner", Visibility: visibility}, bytes.NewReader(content))
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	private := publish("private-sdk", "private")
	public := publish("public-sdk", "public")
	credentials, _ := auth.New(t.TempDir())
	owner := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead)
	mux := http.NewServeMux()
	registerPackagesHTTP(mux, packages, nil, nil, catalog, credentials)

	search := httptest.NewRecorder()
	mux.ServeHTTP(search, httptest.NewRequest(http.MethodGet, "/packages?q=sdk", nil))
	if search.Code != 200 || bytes.Contains(search.Body.Bytes(), []byte(private.ID)) || !bytes.Contains(search.Body.Bytes(), []byte(public.ID)) {
		t.Fatalf("public search = %d %s", search.Code, search.Body.String())
	}

	data, _ := json.Marshal(map[string]any{"name": "isolated build", "package_version_ids": []string{private.ID}, "expires_in_hours": 1})
	req := httptest.NewRequest(http.MethodPost, "/repositories/"+string(consumer.ID)+"/package-credentials", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+owner)
	created := httptest.NewRecorder()
	mux.ServeHTTP(created, req)
	if created.Code != 201 {
		t.Fatalf("credential = %d %s", created.Code, created.Body.String())
	}
	var grant struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(created.Body.Bytes(), &grant)

	metadataReq := httptest.NewRequest(http.MethodGet, "/package-registry/%40owner%2Fprivate-sdk", nil)
	metadataReq.Header.Set("Authorization", "Bearer "+grant.Token)
	metadata := httptest.NewRecorder()
	mux.ServeHTTP(metadata, metadataReq)
	if metadata.Code != 200 || !bytes.Contains(metadata.Body.Bytes(), []byte(private.SourceCommitID)) {
		t.Fatalf("metadata = %d %s", metadata.Code, metadata.Body.String())
	}
	deniedReq := httptest.NewRequest(http.MethodGet, "/package-registry/artifacts/"+public.ID, nil)
	deniedReq.Header.Set("Authorization", "Bearer "+grant.Token)
	// Public bytes remain public, while private unlisted versions are never exposed.
	private2 := publish("other-private", "private")
	deniedReq = httptest.NewRequest(http.MethodGet, "/package-registry/artifacts/"+private2.ID, nil)
	deniedReq.Header.Set("Authorization", "Bearer "+grant.Token)
	denied := httptest.NewRecorder()
	mux.ServeHTTP(denied, deniedReq)
	if denied.Code != 404 {
		t.Fatalf("unrelated private package = %d %s", denied.Code, denied.Body.String())
	}
}

func TestPackagePublicationRejectsUnverifiedRelease(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	repository, _ := catalog.Create("owner", repositories.Metadata{Name: "source", Visibility: repositories.Public})
	releaseStore, _ := releases.New(t.TempDir())
	release, _ := releaseStore.Create(releases.CreateParams{RepositoryID: string(repository.ID), Version: "v1", CommitID: "reviewed", CreatedByID: "owner"})
	builds, _ := checkruns.New(t.TempDir())
	run, _ := builds.Create(string(repository.ID), "release:"+release.ID, release.CommitID, checkruns.Definition{Name: "package", Command: "false"})
	run, _ = builds.Start(run.ID)
	artifact, _ := builds.AddArtifact(run.ID, "dist/sdk", "application/octet-stream", []byte("bad"))
	run, _ = builds.Complete(run.ID, 1, false, "failed")
	packages, _ := packagecatalog.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	token := issueAccess(t, credentials, "owner", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	mux := http.NewServeMux()
	registerPackagesHTTP(mux, packages, releaseStore, builds, catalog, credentials)
	data, _ := json.Marshal(map[string]any{"name": "sdk", "version": "1.0.0", "release_id": release.ID, "build_run_id": run.ID, "artifact_id": artifact.ID, "platform": map[string]string{"os": "linux", "arch": "amd64"}, "visibility": "public"})
	req := httptest.NewRequest(http.MethodPost, "/repositories/"+string(repository.ID)+"/packages", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, req)
	if response.Code != 409 {
		t.Fatalf("unverified = %d %s", response.Code, response.Body.String())
	}
	items, _ := packages.List(string(repository.ID))
	if len(items) != 0 {
		t.Fatalf("partial package = %#v", items)
	}
}
