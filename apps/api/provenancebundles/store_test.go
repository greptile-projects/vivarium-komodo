package provenancebundles

import "testing"

func TestSignedClaimSurvivesTrustChangesAndRestart(t *testing.T) {
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	in := PublishInput{RepositoryID: "repo", ReleaseID: "release", ReleaseVersion: "1.0.0", Revision: "commit", Audience: "public", GraphID: "graph", AssessmentID: "assessment", PolicyVersion: 2, PublishedByID: "owner",
		Artifacts:  []Artifact{{ID: "artifact", Name: "app.tar", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Size: 10, MediaType: "application/x-tar", BuildRunID: "build"}},
		Components: []Component{{Kind: "package", Name: "parser", Version: "2.0.0", License: "Apache-2.0", Origin: "https://upstream.test/parser", Dependencies: []string{}, Notices: []string{}, AttestationIDs: []string{"source"}}},
		Licenses:   []string{"Apache-2.0"}, SourceAttestations: []Attestation{{ID: "source", Kind: "source", SubjectSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Issuer: "builder", Reference: "attestation:source"}},
		BuildAttestations: []Attestation{{ID: "build", Kind: "build", SubjectSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Issuer: "runner", Reference: "attestation:build"}}, Omissions: []Omission{{Subject: "optional test fixtures", Reason: "not shipped", Impact: "not part of runtime SBOM"}}}
	v, err := s.Publish(in)
	if err != nil || !s.Verify(v) {
		t.Fatalf("publish/verify: %v %#v", err, v)
	}
	signature, digest := v.Signature, v.Verification.PayloadSHA256
	v, err = s.Observe("repo", v.ID, "owner", Notice{Kind: "attestation_revoked", Subject: "attestation:build", Detail: "builder key was revoked after publication", EvidenceReference: "trust-log:9", Action: "replace affected artifact", CampaignID: "campaign:repair"})
	if err != nil || v.TrustStatus != "attention_required" || v.Signature != signature || v.Verification.PayloadSHA256 != digest || !s.Verify(v) {
		t.Fatalf("notice rewrote original claim: %v %#v", err, v)
	}
	restarted, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := restarted.Get("repo", v.ID)
	if err != nil || !restarted.Verify(got) || len(got.TrustNotices) != 1 {
		t.Fatalf("restart verification: %v %#v", err, got)
	}
}

func TestAudienceAndReleaseClaimAreImmutable(t *testing.T) {
	s, _ := New(t.TempDir())
	base := PublishInput{RepositoryID: "r", ReleaseID: "x", ReleaseVersion: "1.0.0", Revision: "c", Audience: "repository", GraphID: "g", AssessmentID: "a", PolicyVersion: 1, PublishedByID: "o", Artifacts: []Artifact{{ID: "a", Name: "a", SHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", BuildRunID: "b"}}, Components: []Component{{Kind: "source", Name: "app", License: "MIT", Origin: "repository:r", Dependencies: []string{}, Notices: []string{}, AttestationIDs: []string{"source"}}}, SourceAttestations: []Attestation{{ID: "source", Kind: "source", SubjectSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Issuer: "owner", Reference: "source:1"}}, BuildAttestations: []Attestation{{ID: "build", Kind: "build", SubjectSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Issuer: "runner", Reference: "build:1"}}}
	if _, e := s.Publish(base); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Publish(base); e != ErrConflict {
		t.Fatalf("expected conflict, got %v", e)
	}
}
