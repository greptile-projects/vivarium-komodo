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

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

const uploadPackService = "git-upload-pack"
const receivePackService = "git-receive-pack"

func registerGitHTTP(mux *http.ServeMux, repositories storage.RepositoryStore) {
	mux.HandleFunc("GET /repositories/{repository}/info/refs", advertiseRepository(repositories))
	mux.HandleFunc("POST /repositories/{repository}/git-upload-pack", uploadPack(repositories))
	mux.HandleFunc("POST /repositories/{repository}/git-receive-pack", receivePack(repositories))
}

func advertiseRepository(repositories storage.RepositoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		service := r.URL.Query().Get("service")
		if service != uploadPackService && service != receivePackService {
			http.Error(w, "unsupported Git service", http.StatusBadRequest)
			return
		}
		repository, ok := openRepository(w, repositories, r.PathValue("repository"))
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

func uploadPack(repositories storage.RepositoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if contentType := r.Header.Get("Content-Type"); contentType != "application/x-git-upload-pack-request" {
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
			return
		}
		repository, ok := openRepository(w, repositories, r.PathValue("repository"))
		if !ok {
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

func receivePack(repositories storage.RepositoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if contentType := r.Header.Get("Content-Type"); contentType != "application/x-git-receive-pack-request" {
			http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
			return
		}
		repository, ok := openRepository(w, repositories, r.PathValue("repository"))
		if !ok {
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

func openRepository(w http.ResponseWriter, repositories storage.RepositoryStore, id string) (*storage.Repository, bool) {
	repository, err := repositories.Open(storage.ID(id))
	if err == nil {
		return repository, true
	}
	if errors.Is(err, storage.ErrInvalidID) || errors.Is(err, storage.ErrNotFound) {
		http.NotFound(w, nil)
		return nil, false
	}
	http.Error(w, "could not open repository", http.StatusInternalServerError)
	return nil, false
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
		commandArguments = append(commandArguments, "-c", "core.hooksPath="+hooksDirectory)
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
zero=0000000000000000000000000000000000000000
while read old new ref
do
	if [ "$ref" != "$primary" ]; then
		echo "only the primary branch may be updated" >&2
		exit 1
	fi
	if [ "$new" = "$zero" ]; then
		echo "the primary branch may not be deleted" >&2
		exit 1
	fi
	if [ "$old" != "$zero" ] && ! git merge-base --is-ancestor "$old" "$new"; then
		echo "non-fast-forward primary branch update" >&2
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
