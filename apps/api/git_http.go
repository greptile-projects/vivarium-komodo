package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

const uploadPackService = "git-upload-pack"

func registerGitHTTP(mux *http.ServeMux, repositories storage.RepositoryStore) {
	mux.HandleFunc("GET /repositories/{repository}/info/refs", advertiseRepository(repositories))
	mux.HandleFunc("POST /repositories/{repository}/git-upload-pack", uploadPack(repositories))
}

func advertiseRepository(repositories storage.RepositoryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("service") != uploadPackService {
			http.Error(w, "unsupported Git service", http.StatusBadRequest)
			return
		}
		repository, ok := openRepository(w, repositories, r.PathValue("repository"))
		if !ok {
			return
		}

		advertisement, err := runUploadPack(r, repository, "--advertise-refs")
		if err != nil {
			http.Error(w, "could not advertise repository", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/x-git-upload-pack-advertisement")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = io.WriteString(w, packetLine("# service="+uploadPackService+"\n"))
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

		result, err := runUploadPack(r, repository)
		if err != nil {
			http.Error(w, "could not read repository state", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
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

func runUploadPack(r *http.Request, repository *storage.Repository, arguments ...string) ([]byte, error) {
	arguments = append([]string{"upload-pack", "--stateless-rpc"}, arguments...)
	arguments = append(arguments, repository.GitDir())
	command := exec.CommandContext(r.Context(), "git", arguments...)
	command.Stdin = r.Body
	command.Env = os.Environ()
	if protocol := r.Header.Get("Git-Protocol"); protocol == "version=1" || protocol == "version=2" {
		command.Env = append(command.Env, "GIT_PROTOCOL="+protocol)
	}
	var output, stderr bytes.Buffer
	command.Stdout = &output
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("git upload-pack: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return output.Bytes(), nil
}

func packetLine(payload string) string {
	return fmt.Sprintf("%04x%s", len(payload)+4, payload)
}
