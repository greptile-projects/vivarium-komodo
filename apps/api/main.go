package main

import (
	"log"
	"net/http"
	"os"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/proposals"
	"github.com/greptile-projects/vivarium-komodo/apps/api/repositories"
	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

func main() {
	repositoryRoot := os.Getenv("REPOSITORY_ROOT")
	if repositoryRoot == "" {
		repositoryRoot = "repositories"
	}
	repositoryStorage, err := storage.New(repositoryRoot)
	if err != nil {
		log.Fatal(err)
	}
	repositoryCatalogRoot := os.Getenv("REPOSITORY_CATALOG_ROOT")
	if repositoryCatalogRoot == "" {
		repositoryCatalogRoot = "data/repositories"
	}
	repositoryCatalog, err := repositories.New(repositoryCatalogRoot, repositoryStorage)
	if err != nil {
		log.Fatal(err)
	}
	userRoot := os.Getenv("USER_ROOT")
	if userRoot == "" {
		userRoot = "data/users"
	}
	userStore, err := users.New(userRoot)
	if err != nil {
		log.Fatal(err)
	}
	authRoot := os.Getenv("AUTH_ROOT")
	if authRoot == "" {
		authRoot = "data/auth"
	}
	credentials, err := auth.New(authRoot)
	if err != nil {
		log.Fatal(err)
	}
	proposalRoot := os.Getenv("PROPOSAL_ROOT")
	if proposalRoot == "" {
		proposalRoot = "data/proposals"
	}
	proposalStore, err := proposals.New(proposalRoot)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	registerGitHTTP(mux, repositoryCatalog, credentials)
	registerRepositoriesHTTP(mux, repositoryCatalog, credentials)
	registerCollaboratorsHTTP(mux, repositoryCatalog, userStore, credentials)
	registerProposalsHTTP(mux, proposalStore, repositoryCatalog, credentials)
	registerUsersHTTP(mux, userStore, credentials)
	registerAuthHTTP(mux, credentials, userStore)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
