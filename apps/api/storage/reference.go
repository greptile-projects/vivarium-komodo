package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrReferenceNotFound = errors.New("reference not found")
	ErrReferenceExists   = errors.New("reference already exists")
	ErrInvalidReference  = errors.New("invalid reference")
	ErrInvalidRefName    = errors.New("invalid reference name")
)

// ReferenceName is a full Git reference name, or the special symbolic HEAD.
type ReferenceName string

// Reference is either direct (ObjectID is set) or symbolic (Target is set).
type Reference struct {
	Name     ReferenceName
	ObjectID ObjectID
	Target   ReferenceName
}

// ReferenceStore manages the named pointers that make repository objects reachable.
type ReferenceStore interface {
	CreateReference(Reference) error
	ReadReference(ReferenceName) (Reference, error)
	UpdateReference(Reference) error
	ListReferences() ([]Reference, error)
	DeleteReference(ReferenceName) error
	DefaultBranch() (ReferenceName, error)
	SetDefaultBranch(ReferenceName) error
}

// CreateReference creates a direct or symbolic reference without replacing one.
func (r *Repository) CreateReference(reference Reference) error {
	return r.writeReference(reference, false)
}

// UpdateReference replaces an existing direct or symbolic reference.
func (r *Repository) UpdateReference(reference Reference) error {
	return r.writeReference(reference, true)
}

func (r *Repository) writeReference(reference Reference, replace bool) error {
	if err := r.validate(); err != nil {
		return err
	}
	contents, err := r.referenceContents(reference)
	if err != nil {
		return err
	}
	path := r.referencePath(reference.Name)
	_, statErr := os.Lstat(path)
	exists := statErr == nil
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("inspect reference: %w", statErr)
	}
	if !exists && reference.Name != "HEAD" {
		packed, err := r.readPackedReferences()
		if err != nil {
			return err
		}
		_, exists = packed[reference.Name]
	}
	if replace && !exists {
		return ErrReferenceNotFound
	}
	if !replace && exists {
		return ErrReferenceExists
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create reference directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".writing-ref-")
	if err != nil {
		return fmt.Errorf("stage reference: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.WriteString(contents); err == nil {
		err = temporary.Chmod(0o640)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write reference: %w", err)
	}
	if !replace {
		if err := os.Link(temporaryPath, path); err != nil {
			if _, statErr := os.Lstat(path); statErr == nil {
				return ErrReferenceExists
			}
			return fmt.Errorf("publish reference: %w", err)
		}
		return nil
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish reference: %w", err)
	}
	return nil
}

func (r *Repository) referenceContents(reference Reference) (string, error) {
	if !validReferenceName(reference.Name, true) {
		return "", ErrInvalidRefName
	}
	direct := reference.ObjectID != ""
	symbolic := reference.Target != ""
	if direct == symbolic {
		return "", ErrInvalidReference
	}
	if direct {
		if !validObjectID(reference.ObjectID) {
			return "", ErrInvalidObjectID
		}
		if _, err := r.ReadObject(reference.ObjectID); err != nil {
			return "", err
		}
		return string(reference.ObjectID) + "\n", nil
	}
	if !validReferenceName(reference.Target, false) {
		return "", ErrInvalidRefName
	}
	return "ref: " + string(reference.Target) + "\n", nil
}

// ReadReference returns a reference without resolving symbolic targets.
func (r *Repository) ReadReference(name ReferenceName) (Reference, error) {
	if !validReferenceName(name, true) {
		return Reference{}, ErrInvalidRefName
	}
	if err := r.validate(); err != nil {
		return Reference{}, err
	}
	contents, err := os.ReadFile(r.referencePath(name))
	if errors.Is(err, fs.ErrNotExist) {
		packed, packedErr := r.readPackedReferences()
		if packedErr != nil {
			return Reference{}, packedErr
		}
		id, found := packed[name]
		if !found {
			return Reference{}, ErrReferenceNotFound
		}
		return Reference{Name: name, ObjectID: id}, nil
	}
	if err != nil {
		return Reference{}, fmt.Errorf("read reference: %w", err)
	}
	value := strings.TrimSpace(string(contents))
	if target, found := strings.CutPrefix(value, "ref: "); found {
		if !validReferenceName(ReferenceName(target), false) {
			return Reference{}, ErrInvalidReference
		}
		return Reference{Name: name, Target: ReferenceName(target)}, nil
	}
	id := ObjectID(value)
	if !validObjectID(id) {
		return Reference{}, ErrInvalidReference
	}
	return Reference{Name: name, ObjectID: id}, nil
}

// ListReferences returns HEAD and every loose or packed refs/* reference by name.
func (r *Repository) ListReferences() ([]Reference, error) {
	if err := r.validate(); err != nil {
		return nil, err
	}
	packed, err := r.readPackedReferences()
	if err != nil {
		return nil, err
	}
	nameSet := map[ReferenceName]struct{}{"HEAD": {}}
	for name := range packed {
		nameSet[name] = struct{}{}
	}
	err = filepath.WalkDir(filepath.Join(r.gitDir, "refs"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(r.gitDir, path)
		if err != nil {
			return err
		}
		name := ReferenceName(filepath.ToSlash(relative))
		if validReferenceName(name, false) {
			nameSet[name] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list references: %w", err)
	}
	names := make([]ReferenceName, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	references := make([]Reference, 0, len(names))
	for _, name := range names {
		reference, err := r.ReadReference(name)
		if err != nil {
			return nil, fmt.Errorf("read listed reference %s: %w", name, err)
		}
		references = append(references, reference)
	}
	return references, nil
}

// DeleteReference removes an existing loose or packed reference. HEAD cannot be deleted.
func (r *Repository) DeleteReference(name ReferenceName) error {
	if !validReferenceName(name, false) {
		return ErrInvalidRefName
	}
	if err := r.validate(); err != nil {
		return err
	}
	removedLoose := false
	if err := os.Remove(r.referencePath(name)); err == nil {
		removedLoose = true
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete reference: %w", err)
	}
	packed, err := r.readPackedReferences()
	if err != nil {
		return err
	}
	_, packedExists := packed[name]
	if !removedLoose && !packedExists {
		return ErrReferenceNotFound
	}
	if packedExists {
		delete(packed, name)
		if err := r.writePackedReferences(packed); err != nil {
			return err
		}
	}
	return nil
}

// DefaultBranch reports the branch named by symbolic HEAD.
func (r *Repository) DefaultBranch() (ReferenceName, error) {
	head, err := r.ReadReference("HEAD")
	if err != nil {
		return "", err
	}
	if head.Target == "" || !strings.HasPrefix(string(head.Target), "refs/heads/") {
		return "", ErrInvalidReference
	}
	return head.Target, nil
}

// SetDefaultBranch points HEAD at a branch, including an unborn branch.
func (r *Repository) SetDefaultBranch(branch ReferenceName) error {
	if !validReferenceName(branch, false) || !strings.HasPrefix(string(branch), "refs/heads/") {
		return ErrInvalidRefName
	}
	return r.UpdateReference(Reference{Name: "HEAD", Target: branch})
}

func (r *Repository) referencePath(name ReferenceName) string {
	return filepath.Join(r.gitDir, filepath.FromSlash(string(name)))
}

func (r *Repository) readPackedReferences() (map[ReferenceName]ObjectID, error) {
	contents, err := os.ReadFile(filepath.Join(r.gitDir, "packed-refs"))
	if errors.Is(err, fs.ErrNotExist) {
		return map[ReferenceName]ObjectID{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read packed references: %w", err)
	}
	packed := make(map[ReferenceName]ObjectID)
	for _, rawLine := range strings.Split(string(contents), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "^") {
			continue
		}
		idText, nameText, found := strings.Cut(line, " ")
		name, id := ReferenceName(nameText), ObjectID(idText)
		if !found || !validReferenceName(name, false) || !validObjectID(id) {
			return nil, ErrInvalidReference
		}
		packed[name] = id
	}
	return packed, nil
}

func (r *Repository) writePackedReferences(packed map[ReferenceName]ObjectID) error {
	path := filepath.Join(r.gitDir, "packed-refs")
	temporary, err := os.CreateTemp(r.gitDir, ".writing-packed-refs-")
	if err != nil {
		return fmt.Errorf("stage packed references: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	names := make([]ReferenceName, 0, len(packed))
	for name := range packed {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })
	var contents strings.Builder
	contents.WriteString("# pack-refs with: sorted\n")
	for _, name := range names {
		fmt.Fprintf(&contents, "%s %s\n", packed[name], name)
	}
	if _, err = temporary.WriteString(contents.String()); err == nil {
		err = temporary.Chmod(0o640)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write packed references: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish packed references: %w", err)
	}
	return nil
}

func validReferenceName(name ReferenceName, allowHEAD bool) bool {
	value := string(name)
	if allowHEAD && value == "HEAD" {
		return true
	}
	if !strings.HasPrefix(value, "refs/") || strings.HasSuffix(value, "/") || strings.HasSuffix(value, ".") || strings.Contains(value, "//") || strings.Contains(value, "..") || strings.Contains(value, "@{") {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f || strings.ContainsRune(" ~^:?*[\\", character) {
			return false
		}
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}
