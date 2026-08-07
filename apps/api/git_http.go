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

func registerGitHTTP(mux *http.ServeMux, repositoryStore gitRepositoryStore, credentials credentialAuthenticator) {
	mux.HandleFunc("GET /repositories/{repository}/info/refs", advertiseRepository(repositoryStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/git-upload-pack", uploadPack(repositoryStore, credentials))
	mux.HandleFunc("POST /repositories/{repository}/git-receive-pack", receivePack(repositoryStore, credentials))
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
		repository, ok := authorizeGitRepository(w, r, repositoryStore, credentials, scope)
		if !ok {
			return
		}

		advertisement, err := runGitService(r, repository, service, "--advertise-refs")
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
		repository, ok := authorizeGitRepository(w, r, repositoryStore, credentials, auth.GitRead)
		if !ok {
			return
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/x-git-upload-pack-request" {
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
			return
		}

		result, err := runGitService(r, repository, uploadPackService)
		if err != nil {
			http.Error(w, "could not read repository state", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(result)
	}
}

func receivePack(repositoryStore gitRepositoryStore, credentials credentialAuthenticator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repository, ok := authorizeGitRepository(w, r, repositoryStore, credentials, auth.GitWrite)
		if !ok {
			return
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/x-git-receive-pack-request" {
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
			return
		}
		result, err := runGitService(r, repository, receivePackService)
		if err != nil {
			http.Error(w, "could not update repository state", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-git-receive-pack-result")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(result)
	}
}

func authorizeGitRepository(w http.ResponseWriter, r *http.Request, store gitRepositoryStore, credentials credentialAuthenticator, scope auth.Scope) (*storage.Repository, bool) {
	item, err := store.Inspect(storage.ID(r.PathValue("repository")))
	if errors.Is(err, repositories.ErrNotFound) || errors.Is(err, storage.ErrInvalidID) || errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, nil)
		return nil, false
	}
	if err != nil {
		http.Error(w, "could not open repository", http.StatusInternalServerError)
		return nil, false
	}
	actor, authenticated, valid := authenticateGitOptional(w, r, credentials, scope)
	if !valid {
		return nil, false
	}
	read := scope == auth.GitRead
	if !authenticated && (!read || item.Visibility != repositories.Public) {
		writeGitUnauthenticated(w)
		return nil, false
	}
	if authenticated && actor.UserID != item.OwnerID && (!read || item.Visibility != repositories.Public) {
		http.NotFound(w, nil)
		return nil, false
	}
	repository, err := store.Open(item.ID)
	if err != nil {
		http.Error(w, "could not open repository", http.StatusInternalServerError)
		return nil, false
	}
	return repository, true
}

func runGitService(r *http.Request, repository *storage.Repository, service string, arguments ...string) ([]byte, error) {
	commandName := strings.TrimPrefix(service, "git-")
	commandArguments := []string{}
	if service == receivePackService && len(arguments) == 0 {
		hooksDirectory, err := receiveHooks()
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(hooksDirectory)
		// A receive command has no force flag: stock clients send a non-fast-forward
		// update only after the user explicitly bypasses their local safety check.
		// Keep HEAD symbolic when its branch is deleted so clones can select the
		// same unborn primary branch and a later push can recreate it.
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

func receiveHooks() (string, error) {
	directory, err := os.MkdirTemp("", "git-receive-hooks-")
	if err != nil {
		return "", fmt.Errorf("create receive hooks: %w", err)
	}
	const hook = `#!/bin/sh
primary=$(git symbolic-ref HEAD) || exit 1
while read old new ref
do
	if [ "$ref" != "$primary" ]; then
		echo "only the primary branch may be updated" >&2
		exit 1
	fi
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
