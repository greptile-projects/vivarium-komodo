package federation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type MergeReceipt struct {
	SchemaVersion         int        `json:"schema_version"`
	ID                    string     `json:"id"`
	IdempotencyKey        string     `json:"idempotency_key"`
	UpstreamInstance      string     `json:"upstream_instance"`
	ContributorInstance   string     `json:"contributor_instance"`
	UpstreamPullReference string     `json:"upstream_pull_reference"`
	SourcePullReference   string     `json:"source_pull_reference,omitempty"`
	TargetReference       string     `json:"target_reference"`
	SourceRepositoryID    string     `json:"source_repository_id"`
	SourceBranch          string     `json:"source_branch"`
	SourceCommitID        string     `json:"source_commit_id"`
	MergeCommitID         string     `json:"merge_commit_id"`
	AuthorSubject         string     `json:"author_subject"`
	MergedBySubject       string     `json:"merged_by_subject"`
	ReviewEvidenceDigest  string     `json:"review_evidence_digest"`
	CheckEvidenceDigest   string     `json:"check_evidence_digest"`
	MergedAt              time.Time  `json:"merged_at"`
	KeyID                 string     `json:"key_id"`
	Signature             string     `json:"signature"`
	Verification          string     `json:"verification,omitempty"`
	VerifiedAt            *time.Time `json:"verified_at,omitempty"`
	CurrentTrust          string     `json:"current_trust,omitempty"`
	DeliveryStatus        string     `json:"delivery_status,omitempty"`
	DeliveryError         string     `json:"delivery_error,omitempty"`
}

func MergeReceiptBytes(v MergeReceipt) []byte {
	v.ID, v.KeyID, v.Signature, v.Verification, v.CurrentTrust, v.DeliveryStatus, v.DeliveryError = "", "", "", "", "", "", ""
	v.VerifiedAt = nil
	b, _ := json.Marshal(v)
	return b
}

func (s *Store) PutMergeReceipt(v MergeReceipt) (MergeReceipt, error) {
	if v.SchemaVersion != 1 || v.IdempotencyKey == "" || v.UpstreamInstance == "" || v.ContributorInstance == "" || v.SourceCommitID == "" || v.MergeCommitID == "" || v.Signature == "" {
		return MergeReceipt{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.root, "merge-receipts.json")
	items := []MergeReceipt{}
	if b, e := os.ReadFile(path); e == nil && json.Unmarshal(b, &items) != nil {
		return MergeReceipt{}, ErrInvalid
	}
	d := sha256.Sum256(MergeReceiptBytes(v))
	v.ID = "fmr_" + hex.EncodeToString(d[:12])
	for _, old := range items {
		if old.UpstreamInstance == v.UpstreamInstance && old.IdempotencyKey == v.IdempotencyKey {
			if old.ID != v.ID {
				return MergeReceipt{}, ErrConflict
			}
			return old, nil
		}
	}
	items = append(items, v)
	b, _ := json.MarshalIndent(items, "", "  ")
	tmp, e := os.CreateTemp(s.root, "receipts-*.tmp")
	if e != nil {
		return MergeReceipt{}, e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(b); e == nil {
		e = tmp.Sync()
	}
	if ce := tmp.Close(); e == nil {
		e = ce
	}
	if e == nil {
		e = os.Rename(name, path)
	}
	return v, e
}

func (s *Store) MergeReceipts() ([]MergeReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, e := os.ReadFile(filepath.Join(s.root, "merge-receipts.json"))
	if errors.Is(e, os.ErrNotExist) {
		return []MergeReceipt{}, nil
	}
	if e != nil {
		return nil, e
	}
	var out []MergeReceipt
	if json.Unmarshal(b, &out) != nil {
		return nil, ErrInvalid
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MergedAt.Before(out[j].MergedAt) })
	return out, nil
}

func (s *Store) MergeReceipt(idempotencyKey string) (MergeReceipt, error) {
	items, err := s.MergeReceipts()
	if err != nil {
		return MergeReceipt{}, err
	}
	for _, item := range items {
		if item.IdempotencyKey == idempotencyKey {
			return item, nil
		}
	}
	return MergeReceipt{}, ErrNotFound
}
