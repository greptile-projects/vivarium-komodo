package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/checkruns"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestStandardPackageClientInstallsAttestedVersion(t *testing.T) {
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm is required for the package client compatibility test")
	}
	var artifact bytes.Buffer
	gz := gzip.NewWriter(&artifact)
	tw := tar.NewWriter(gz)
	files := map[string]string{
		"package/package.json": `{"name":"@publisher/sdk","version":"1.2.3","main":"index.js"}`,
		"package/index.js":     `module.exports = {provenance: "attested"};`,
	}
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	publisher, _ := catalog.Create("publisher", repositories.Metadata{Name: "sdk", Visibility: repositories.Public})
	store, _ := packagecatalog.New(t.TempDir())
	digest := sha256.Sum256(artifact.Bytes())
	version, err := store.Publish(packagecatalog.PublishParams{OwnerID: "publisher", Name: "sdk", Version: "1.2.3", RepositoryID: string(publisher.ID), ReleaseID: "release", SourceCommitID: "reviewed-commit", ArtifactID: "artifact", ArtifactPath: "sdk.tgz", ArtifactMediaType: "application/gzip", ArtifactSize: int64(artifact.Len()), ExpectedSHA256: hex.EncodeToString(digest[:]), Build: packagecatalog.BuildAttestation{RunID: "build", BuildName: "package", Command: "npm pack", CompletedAt: time.Now()}, Platform: packagecatalog.Platform{OS: "linux", Arch: "amd64"}, PublisherID: "publisher", Visibility: "public"}, bytes.NewReader(artifact.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	credentials, _ := auth.New(t.TempDir())
	mux := http.NewServeMux()
	registerPackagesHTTP(mux, store, nil, nil, catalog, credentials)
	server := httptest.NewServer(mux)
	defer server.Close()

	consumer := t.TempDir()
	if err := os.WriteFile(filepath.Join(consumer, "package.json"), []byte(`{"private":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("npm", "install", "--ignore-scripts", "--no-audit", "--no-fund", "--registry", server.URL+"/package-registry", "@publisher/sdk@1.2.3")
	command.Dir = consumer
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("npm install: %v\n%s", err, output)
	}
	installed, err := os.ReadFile(filepath.Join(consumer, "node_modules", "@publisher", "sdk", "index.js"))
	if err != nil || !bytes.Contains(installed, []byte("attested")) {
		t.Fatalf("installed package = %q, %v", installed, err)
	}
	metadata := httptest.NewRecorder()
	mux.ServeHTTP(metadata, httptest.NewRequest(http.MethodGet, "/packages/"+version.ID, nil))
	if metadata.Code != http.StatusOK || !bytes.Contains(metadata.Body.Bytes(), []byte("reviewed-commit")) || !bytes.Contains(metadata.Body.Bytes(), []byte("npm pack")) {
		t.Fatalf("installed provenance = %d %s", metadata.Code, metadata.Body.String())
	}
}

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
	body := map[string]any{"name": "sdk", "version": "1.2.3", "release_id": release.ID, "build_run_id": run.ID, "artifact_id": artifact.ID, "platform": map[string]string{"os": "linux", "arch": "amd64", "runtime": "go1.24"}, "dependencies": map[string]string{"@owner/core": "^1.0.0"}, "documentation": "# SDK\n\nVerified client library.", "license": "Apache-2.0", "support_url": "https://example.test/support", "visibility": "public"}
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
	if item.Identity != "@owner/sdk" || item.SourceCommitID != release.CommitID || item.Build.RunID != run.ID || item.SHA256 != artifact.SHA256 || item.PublisherID != "owner" || item.Visibility != "public" || item.DocumentationSHA256 == "" || item.License != "Apache-2.0" || item.SupportURL != "https://example.test/support" {
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
