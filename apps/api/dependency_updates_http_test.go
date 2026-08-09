package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyinventory"
	"github.com/greptile-projects/vivarium-komodo/apps/api/dependencyupdates"
	packagecatalog "github.com/greptile-projects/vivarium-komodo/apps/api/packages"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/releases"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func TestDependencyUpdatePolicyOpensEvidenceBackedConsumerProposal(t *testing.T) {
	gitStore, _ := storage.New(t.TempDir())
	catalog, _ := repositories.New(t.TempDir(), gitStore)
	publisher, _ := catalog.Create("publisher", repositories.Metadata{Name: "sdk", Visibility: repositories.Public})
	consumer, _ := catalog.Create("consumer", repositories.Metadata{Name: "app", Visibility: repositories.Private})
	packageStore, _ := packagecatalog.New(t.TempDir())
	releaseStore, _ := releases.New(t.TempDir())
	proposalsStore, _ := proposals.New(t.TempDir())
	inventoryStore, _ := dependencyinventory.New(t.TempDir())
	updateStore, _ := dependencyupdates.New(t.TempDir())
	credentials, _ := auth.New(t.TempDir())
	consumerToken := issueAccess(t, credentials, "consumer", auth.API, auth.RepositoryRead, auth.RepositoryWrite)
	publisherToken := issueAccess(t, credentials, "publisher", auth.API, auth.RepositoryRead, auth.RepositoryWrite)

	artifact := []byte("package")
	digest := sha256.Sum256(artifact)
	publish := func(version, releaseID, commit string) packagecatalog.Version {
		item, err := packageStore.Publish(packagecatalog.PublishParams{OwnerID: "publisher", Name: "sdk", Version: version, RepositoryID: string(publisher.ID), ReleaseID: releaseID, SourceCommitID: commit, ArtifactID: "artifact-" + version, ArtifactPath: "sdk.tgz", ArtifactMediaType: "application/gzip", ArtifactSize: int64(len(artifact)), ExpectedSHA256: hex.EncodeToString(digest[:]), Build: packagecatalog.BuildAttestation{RunID: "build-" + version, BuildName: "package", Command: "build", CompletedAt: time.Now()}, Platform: packagecatalog.Platform{OS: "linux", Arch: "amd64"}, PublisherID: "publisher", Visibility: "public"}, bytes.NewReader(artifact))
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	current := publish("1.2.0", "release-old", "source-old")
	candidate := publish("1.3.0", "release-new", "source-new")
	_, _ = releaseStore.Create(releases.CreateParams{RepositoryID: string(publisher.ID), Version: "1.3.0", Notes: "Adds the verified batch API.", CommitID: "source-new", CreatedByID: "publisher"})
	// The package points at release-new; create a release with that exact ID is not supported by the store,
	// so use the store-created release identity for the candidate evidence.
	releasesList, _ := releaseStore.List(string(publisher.ID))
	candidate.ReleaseID = releasesList[0].ID
	// Persist the adjusted test package by republishing a distinct eligible version.
	candidate = publish("1.4.0", candidate.ReleaseID, "source-new")

	manifest := []byte(`{"version":1,"direct_dependencies":["@publisher/sdk"]}`)
	lock, _ := json.Marshal(packageLock{Version: 1, Packages: []lockedPackage{{Identity: current.Identity, Version: current.Version, PackageVersionID: current.ID, Dependencies: []string{}}}})
	opened, _ := catalog.Open(consumer.ID)
	manifestBlob := writeObject(t, opened, storage.BlobObject, manifest)
	lockBlob := writeObject(t, opened, storage.BlobObject, lock)
	komodoTree := writeObject(t, opened, storage.TreeObject, append(treeEntry("100644", "packages.json", manifestBlob), treeEntry("100644", "packages.lock.json", lockBlob)...))
	root := writeObject(t, opened, storage.TreeObject, treeEntry("40000", ".komodo", komodoTree))
	commit := writeCommit(t, opened, root, nil, "consumer lock")
	_ = opened.CreateReference(storage.Reference{Name: "refs/heads/main", ObjectID: commit})
	inventory, err := inventoryStore.Create(dependencyinventory.CreateParams{RepositoryID: string(consumer.ID), CommitID: string(commit), ManifestSHA256: "manifest", LockSHA256: "lock", CreatedByID: "consumer", Resolutions: []dependencyinventory.Resolution{{Identity: current.Identity, PackageVersionID: current.ID, Version: current.Version, Direct: true, Dependencies: []string{}, Status: "resolved"}}})
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerDependencyUpdateHTTP(mux, updateStore, inventoryStore, packageStore, releaseStore, proposalsStore, catalog, credentials, nil)
	policyBody := bytes.NewBufferString(`{"target_branch":"main","allowed":"minor","enabled":true}`)
	policyRequest := httptest.NewRequest(http.MethodPut, "/repositories/"+string(consumer.ID)+"/dependency-update-policies/"+url.PathEscape(current.Identity), policyBody)
	policyRequest.Header.Set("Authorization", "Bearer "+consumerToken)
	policyResponse := httptest.NewRecorder()
	mux.ServeHTTP(policyResponse, policyRequest)
	if policyResponse.Code != 200 {
		t.Fatalf("policy = %d %s", policyResponse.Code, policyResponse.Body.String())
	}

	scanBody, _ := json.Marshal(map[string]string{"inventory_id": inventory.ID})
	scan := httptest.NewRequest(http.MethodPost, "/repositories/"+string(consumer.ID)+"/dependency-updates", bytes.NewReader(scanBody))
	scan.Header.Set("Authorization", "Bearer "+consumerToken)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, scan)
	if response.Code != 201 {
		t.Fatalf("scan = %d %s", response.Code, response.Body.String())
	}
	var result struct {
		Items []dependencyupdates.Update `json:"items"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &result)
	if len(result.Items) != 1 || result.Items[0].Evidence.CandidatePackageVersionID != candidate.ID || result.Items[0].CreatedByID != "consumer" {
		t.Fatalf("updates = %#v", result.Items)
	}
	proposal, err := proposalsStore.Get(string(consumer.ID), result.Items[0].ProposalID)
	if err != nil || !bytes.Contains([]byte(proposal.Body), []byte("Adds the verified batch API")) || !bytes.Contains([]byte(proposal.Body), []byte(candidate.SHA256)) {
		t.Fatalf("proposal = %#v, %v", proposal, err)
	}
	plan, _ := proposalsStore.GetPlan(string(consumer.ID), proposal.ID)
	if len(plan.Tasks) != 1 || plan.Tasks[0].Status != proposals.TaskPlanned {
		t.Fatalf("plan = %#v", plan)
	}
	repeated := httptest.NewRequest(http.MethodPost, "/repositories/"+string(consumer.ID)+"/dependency-updates", bytes.NewReader(scanBody))
	repeated.Header.Set("Authorization", "Bearer "+consumerToken)
	repeatedResponse := httptest.NewRecorder()
	mux.ServeHTTP(repeatedResponse, repeated)
	if repeatedResponse.Code != 201 || !bytes.Contains(repeatedResponse.Body.Bytes(), []byte(`"total_count":0`)) {
		t.Fatalf("repeated scan = %d %s", repeatedResponse.Code, repeatedResponse.Body.String())
	}

	denied := httptest.NewRequest(http.MethodPut, "/repositories/"+string(consumer.ID)+"/dependency-update-policies/"+url.PathEscape(current.Identity), bytes.NewBufferString(`{"target_branch":"main","allowed":"major","enabled":true}`))
	denied.Header.Set("Authorization", "Bearer "+publisherToken)
	deniedResponse := httptest.NewRecorder()
	mux.ServeHTTP(deniedResponse, denied)
	if deniedResponse.Code != 404 {
		t.Fatalf("publisher policy write = %d", deniedResponse.Code)
	}
}
