package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

const uploadPackService = "git-upload-pack"
const receivePackService = "git-receive-pack"

type credentialAuthenticator interface {
	Authenticate(string, auth.Scope) (auth.Grant, error)
}

type gitRepositoryStore interface {
	Inspect(storage.ID) (repositories.Repository, error)
	Open(storage.ID) (*storage.Repository, error)
}

type repositoryCollaboratorStore interface {
	IsCollaborator(storage.ID, string) (bool, error)
}
type gitHistoryPolicy interface{ PushPause(string) (bool, string) }

func registerGitHTTP(mux *http.ServeMux, repositoryStore gitRepositoryStore, credentials credentialAuthenticator, policies ...gitHistoryPolicy) {
	mux.HandleFunc("GET /repositories/{repository}/info/refs", advertiseRepository(repositoryStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/git-upload-pack", uploadPack(repositoryStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/git-receive-pack", receivePack(repositoryStore, credentials, policies...))
}

func advertiseRepository(repositoryStore gitRepositoryStore, credentials credentialAuthenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		service := r.URL.Query().Get("service")
		if service != uploadPackService && service != receivePackService {
			http.Error(w, "unsupported Git service", http.StatusBadRequest)
			return
		}
		scope := auth.GitRead
		if service == receivePackService {
			scope = auth.GitWrite
		}
		repository, _, grant, ok := authorizeGitRepository(w, r, repositoryStore, credentials, scope)
		if !ok {
			return
		}

		advertisement, err := runGitServiceWithVisibility(r, repository, service, grant.Branch, "--advertise-refs")
		if err != nil {
			http.Error(w, "could not advertise repository", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-"+service+"-advertisement")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = io.WriteString(w, packetLine("# service="+service+"\n"))
		_, _ = io.WriteString(w, "0000")
		_, _ = w.Write(advertisement)
	}
}

func uploadPack(repositoryStore gitRepositoryStore, credentials credentialAuthenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, _, grant, ok := authorizeGitRepository(w, r, repositoryStore, credentials, auth.GitRead)
		if !ok {
			return
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/x-git-upload-pack-request" {
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
			return
		}

		result, err := runGitServiceWithVisibility(r, repository, uploadPackService, grant.Branch)
		if err != nil {
			http.Error(w, "could not read repository state", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(result)
	}
}

func receivePack(repositoryStore gitRepositoryStore, credentials credentialAuthenticator, policies ...gitHistoryPolicy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, owner, grant, ok := authorizeGitRepository(w, r, repositoryStore, credentials, auth.GitWrite)
		if !ok {
			return
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/x-git-receive-pack-request" {
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
			return
		}
		for _, policy := range policies {
			if paused, guidance := policy.PushPause(r.PathValue("repository")); paused {
				http.Error(w, "history rewrite migration in progress: "+guidance, http.StatusConflict)
				return
			}
		}
		defaultBranch, err := repository.DefaultBranch()
		if err != nil {
			http.Error(w, "could not read repository state", http.StatusInternalServerError)
			return
		}
		result, err := runGitServiceWithPolicy(r, repository, receivePackService, owner, string(defaultBranch), grant.Branch)
		if err != nil {
			http.Error(w, "could not update repository state", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(result)
	}
}

func authorizeGitRepository(w http.ResponseWriter, r *http.Request, store gitRepositoryStore, credentials credentialAuthenticator, scope auth.Scope) (*storage.Repository, bool, auth.Grant, bool) {
	item, err := store.Inspect(storage.ID(r.PathValue("repository")))
	if errors.Is(err, repositories.ErrNotFound) || errors.Is(err, storage.ErrInvalidID) || errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, nil)
		return nil, false, auth.Grant{}, false
	}
	if err != nil {
		http.Error(w, "could not open repository", http.StatusInternalServerError)
		return nil, false, auth.Grant{}, false
	}
	actor, authenticated, valid := authenticateGitOptional(w, r, credentials, scope)
	if !valid {
		return nil, false, auth.Grant{}, false
	}
	if authenticated && actor.RepositoryID != "" && actor.RepositoryID != string(item.ID) {
		http.NotFound(w, nil)
		return nil, false, auth.Grant{}, false
	}
	read := scope == auth.GitRead
	if !authenticated && (!read || item.Visibility != repositories.Public) {
		writeGitUnauthenticated(w)
		return nil, false, auth.Grant{}, false
	}
	owner := authenticated && actor.UserID == item.OwnerID
	collaborator := false
	if authenticated && !owner {
		collaborators, supported := store.(repositoryCollaboratorStore)
		if supported {
			collaborator, err = collaborators.IsCollaborator(item.ID, actor.UserID)
		}
		if err != nil {
			http.Error(w, "could not authorize repository", http.StatusInternalServerError)
			return nil, false, auth.Grant{}, false
		}
	}
	// A repository-and-branch scoped credential is an explicit, narrow write
	// delegation. It does not make its holder a repository collaborator.
	scopedWrite := authenticated && !read && actor.RepositoryID == string(item.ID) && actor.Branch != ""
	if authenticated && !owner && !collaborator && !scopedWrite && (!read || item.Visibility != repositories.Public) {
		http.NotFound(w, nil)
		return nil, false, auth.Grant{}, false
	}
	repository, err := store.Open(item.ID)
	if err != nil {
		http.Error(w, "could not open repository", http.StatusInternalServerError)
		return nil, false, auth.Grant{}, false
	}
	return repository, owner, actor, true
}

func runGitService(r *http.Request, repository *storage.Repository, service string, arguments ...string) ([]byte, error) {
	return runGitServiceWithPolicy(r, repository, service, true, "", "", arguments...)
}

// Embargo refs are absent from ordinary repository discovery, including an
// owner's clone. Only a credential scoped to one exact repair branch receives
// the negative hideRefs exception needed to fetch or push that branch.
func runGitServiceWithVisibility(r *http.Request, repository *storage.Repository, service, allowedBranch string, arguments ...string) ([]byte, error) {
	visibility := []string{"-c", "transfer.hideRefs=refs/heads/embargo/"}
	if strings.HasPrefix(allowedBranch, "refs/heads/embargo/") {
		visibility = append(visibility, "-c", "transfer.hideRefs=!"+allowedBranch)
	}
	return runGitServiceWithConfig(r, repository, service, visibility, arguments)
}

func runGitServiceWithConfig(r *http.Request, repository *storage.Repository, service string, config, arguments []string) ([]byte, error) {
	commandName := strings.TrimPrefix(service, "git-")
	commandArguments := append([]string{}, config...)
	commandArguments = append(commandArguments, commandName, "--stateless-rpc")
	commandArguments = append(commandArguments, arguments...)
	commandArguments = append(commandArguments, repository.GitDir())
	command := exec.CommandContext(r.Context(), "git", commandArguments...)
	command.Stdin = r.Body
	command.Env = os.Environ()
	if protocol := r.Header.Get("Git-Protocol"); protocol == "version=1" || protocol == "version=2" {
		command.Env = append(command.Env, "GIT_PROTOCOL="+protocol)
	}
	var output, stderr bytes.Buffer
	command.Stdout = &output
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", commandName, err, strings.TrimSpace(stderr.String()))
	}
	return output.Bytes(), nil
}

func runGitServiceWithPolicy(r *http.Request, repository *storage.Repository, service string, owner bool, defaultBranch string, allowedBranch string, arguments ...string) ([]byte, error) {
	commandName := strings.TrimPrefix(service, "git-")
	commandArguments := []string{}
	if service == receivePackService && len(arguments) == 0 {
		hooksDirectory, err := receiveHooks(owner, defaultBranch, allowedBranch)
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(hooksDirectory)
		// A receive command has no force flag: stock clients send a non-fast-forward
		// update only after the user explicitly bypasses their local safety check.
		// Keep HEAD symbolic when the default branch is deleted so clones can select
		// the same unborn branch and a later push can recreate it.
		commandArguments = append(commandArguments,
			"-c", "core.hooksPath="+hooksDirectory,
			"-c", "receive.denyDeleteCurrent=ignore",
		)
	}
	arguments = append([]string{commandName, "--stateless-rpc"}, arguments...)
	arguments = append(arguments, repository.GitDir())
	commandArguments = append(commandArguments, arguments...)
	command := exec.CommandContext(r.Context(), "git", commandArguments...)
	command.Stdin = r.Body
	command.Env = os.Environ()
	if protocol := r.Header.Get("Git-Protocol"); protocol == "version=1" || protocol == "version=2" {
		command.Env = append(command.Env, "GIT_PROTOCOL="+protocol)
	}
	var output, stderr bytes.Buffer
	command.Stdout = &output
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", commandName, err, strings.TrimSpace(stderr.String()))
	}
	return output.Bytes(), nil
}

func receiveHooks(owner bool, defaultBranch, allowedBranch string) (string, error) {
	directory, err := os.MkdirTemp("", "git-receive-hooks-")
	if err != nil {
		return "", fmt.Errorf("create receive hooks: %w", err)
	}
	hook := `#!/bin/sh
while read old new ref
do
	case "$ref" in
		refs/heads/*) ;;
		*)
			echo "only branches may be updated" >&2
			exit 1
			;;
	esac
`
	if allowedBranch != "" {
		hook += `
	if [ "$ref" != "` + allowedBranch + `" ]; then
		echo "credential is limited to ` + allowedBranch + `" >&2
		exit 1
	fi
`
	}
	if !owner {
		hook += `
	if [ "$ref" = "` + defaultBranch + `" ]; then
		echo "contributors cannot update the default branch" >&2
		exit 1
	fi
`
	}
	hook += `
done
`
	if err := os.WriteFile(filepath.Join(directory, "pre-receive"), []byte(hook), 0o700); err != nil {
		os.RemoveAll(directory)
		return "", fmt.Errorf("create receive hook: %w", err)
	}
	return directory, nil
}

func packetLine(payload string) string {
	return fmt.Sprintf("%04x%s", len(payload)+4, payload)
}
