package main

import (
	"log"
	"net/http"
	"os"

	"github.com/greptile-projects/vivarium-komodo/apps/api/storage"
)

func main() {
	repositoryRoot := os.Getenv("REPOSITORY_ROOT")
	if repositoryRoot == "" {
		repositoryRoot = "repositories"
	}
	repositories, err := storage.New(repositoryRoot)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	registerGitHTTP(mux, repositories)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
