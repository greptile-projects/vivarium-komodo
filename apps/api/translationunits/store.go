// Package translationunits retains revision-exact localization extraction and proposals.
package translationunits

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotFound = errors.New("translation extraction not found")
	ErrInvalid  = errors.New("invalid translation extraction")
)

type Location struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	BlobID string `json:"blob_id"`
}
type Unit struct {
	ID             string                      `json:"id"`
	ResourceID     string                      `json:"resource_id"`
	Key            string                      `json:"key"`
	Message        string                      `json:"message"`
	PriorMessage   string                      `json:"prior_message,omitempty"`
	Context        string                      `json:"context,omitempty"`
	ScreenshotURLs []string                    `json:"screenshot_urls,omitempty"`
	Variables      []string                    `json:"variables,omitempty"`
	PluralRule     string                      `json:"plural_rule,omitempty"`
	Location       Location                    `json:"source_location"`
	Change         string                      `json:"change"`
	Translations   map[string]TranslationState `json:"translations"`
}
type TranslationState struct {
	Text          string `json:"text,omitempty"`
	Status        string `json:"status"`
	SourceMessage string `json:"source_message,omitempty"`
	ActorID       string `json:"actor_id,omitempty"`
	ProposalID    string `json:"proposal_id,omitempty"`
}
type Proposal struct {
	ID            string    `json:"id"`
	UnitID        string    `json:"unit_id"`
	LocaleID      string    `json:"locale_id"`
	Text          string    `json:"text"`
	SourceMessage string    `json:"source_message"`
	Revision      string    `json:"revision"`
	ActorID       string    `json:"actor_id"`
	CreatedAt     time.Time `json:"created_at"`
	Superseded    bool      `json:"superseded"`
}
type Extraction struct {
	ID             string     `json:"id"`
	RepositoryID   string     `json:"repository_id"`
	PullRequestID  string     `json:"pull_request_id"`
	Revision       string     `json:"revision"`
	TargetRevision string     `json:"target_revision"`
	SourceLocale   string     `json:"source_locale"`
	Locales        []string   `json:"locales"`
	ConfigPath     string     `json:"config_path"`
	ConfigBlobID   string     `json:"config_blob_id"`
	Units          []Unit     `json:"units"`
	Proposals      []Proposal `json:"proposals"`
	CreatedByID    string     `json:"created_by_id"`
	CreatedAt      time.Time  `json:"created_at"`
}
type Input struct {
	Revision, TargetRevision, SourceLocale, ConfigPath, ConfigBlobID string
	Locales                                                          []string
	Units                                                            []Unit
}
type Store struct {
	root string
	mu   sync.Mutex
	now  func() time.Time
}

func New(root string) (*Store, error) {
	if root == "" {
		return nil, ErrInvalid
	}
	a, e := filepath.Abs(root)
	if e == nil {
		e = os.MkdirAll(a, 0750)
	}
	return &Store{root: a, now: time.Now}, e
}
func newID() string                            { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (s *Store) path(repo, pull string) string { return filepath.Join(s.root, repo, pull+".json") }
func (s *Store) load(repo, pull string) (Extraction, error) {
	var x Extraction
	b, e := os.ReadFile(s.path(repo, pull))
	if os.IsNotExist(e) {
		return x, ErrNotFound
	}
	if e == nil {
		e = json.Unmarshal(b, &x)
	}
	return x, e
}
func (s *Store) save(x Extraction) error {
	if e := os.MkdirAll(filepath.Dir(s.path(x.RepositoryID, x.PullRequestID)), 0750); e != nil {
		return e
	}
	b, e := json.MarshalIndent(x, "", "  ")
	if e == nil {
		e = os.WriteFile(s.path(x.RepositoryID, x.PullRequestID), b, 0640)
	}
	return e
}
func (s *Store) Create(repo, pull, actor string, in Input) (Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if repo == "" || pull == "" || actor == "" || in.Revision == "" || in.TargetRevision == "" || in.SourceLocale == "" || in.ConfigBlobID == "" || len(in.Locales) == 0 {
		return Extraction{}, ErrInvalid
	}
	old, _ := s.load(repo, pull)
	proposals := old.Proposals
	currentMessages := map[string]string{}
	for _, unit := range in.Units {
		currentMessages[unit.ID] = unit.Message
	}
	for i := range proposals {
		if currentMessages[proposals[i].UnitID] != proposals[i].SourceMessage {
			proposals[i].Superseded = true
		}
	}
	x := Extraction{ID: newID(), RepositoryID: repo, PullRequestID: pull, Revision: in.Revision, TargetRevision: in.TargetRevision, SourceLocale: in.SourceLocale, Locales: in.Locales, ConfigPath: in.ConfigPath, ConfigBlobID: in.ConfigBlobID, Units: in.Units, Proposals: proposals, CreatedByID: actor, CreatedAt: s.now().UTC()}
	applyProposals(&x)
	return x, s.save(x)
}
func applyProposals(x *Extraction) {
	latest := map[string]Proposal{}
	for _, p := range x.Proposals {
		if !p.Superseded {
			latest[p.UnitID+"\x00"+p.LocaleID] = p
		}
	}
	for i := range x.Units {
		if x.Units[i].Translations == nil {
			x.Units[i].Translations = map[string]TranslationState{}
		}
		for _, l := range x.Locales {
			if p, ok := latest[x.Units[i].ID+"\x00"+l]; ok {
				x.Units[i].Translations[l] = TranslationState{Text: p.Text, Status: "proposed", SourceMessage: p.SourceMessage, ActorID: p.ActorID, ProposalID: p.ID}
			}
		}
	}
}
func (s *Store) Get(repo, pull string) (Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(repo, pull)
}
func (s *Store) List(repo string) ([]Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, e := filepath.Glob(filepath.Join(s.root, repo, "*.json"))
	if e != nil {
		return nil, e
	}
	out := []Extraction{}
	for _, f := range files {
		var x Extraction
		b, er := os.ReadFile(f)
		if er == nil {
			er = json.Unmarshal(b, &x)
		}
		if er != nil {
			return nil, er
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}
func (s *Store) Propose(repo, pull, actor, unit, locale, text string) (Extraction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	x, e := s.load(repo, pull)
	if e != nil {
		return x, e
	}
	text = strings.TrimSpace(text)
	found := false
	var message string
	for _, u := range x.Units {
		if u.ID == unit && u.Change != "removed" {
			found = true
			message = u.Message
		}
	}
	validLocale := false
	for _, l := range x.Locales {
		if l == locale {
			validLocale = true
		}
	}
	if actor == "" || !found || !validLocale || text == "" || len(text) > 10000 {
		return x, ErrInvalid
	}
	for i := range x.Proposals {
		if x.Proposals[i].UnitID == unit && x.Proposals[i].LocaleID == locale && !x.Proposals[i].Superseded {
			x.Proposals[i].Superseded = true
		}
	}
	x.Proposals = append(x.Proposals, Proposal{ID: newID(), UnitID: unit, LocaleID: locale, Text: text, SourceMessage: message, Revision: x.Revision, ActorID: actor, CreatedAt: s.now().UTC()})
	applyProposals(&x)
	return x, s.save(x)
}
