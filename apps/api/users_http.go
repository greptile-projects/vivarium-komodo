package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

type userStore interface {
	Create(users.Profile) (users.User, error)
	Get(users.ID) (users.User, error)
	Update(users.ID, users.Profile) (users.User, error)
}

func registerUsersHTTP(mux *http.ServeMux, store userStore) {
	mux.HandleFunc("POST /users", createUser(store))
	mux.HandleFunc("GET /users/{user}", getUser(store))
	mux.HandleFunc("PUT /users/{user}", updateUser(store))
}

func createUser(store userStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profile, ok := readProfile(w, r)
		if !ok {
			return
		}
		user, err := store.Create(profile)
		if err != nil {
			writeUserError(w, err)
			return
		}
		w.Header().Set("Location", "/users/"+string(user.ID))
		writeJSON(w, http.StatusCreated, user)
	}
}

func getUser(store userStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := store.Get(users.ID(r.PathValue("user")))
		if err != nil {
			writeUserError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, user)
	}
}

func updateUser(store userStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		profile, ok := readProfile(w, r)
		if !ok {
			return
		}
		user, err := store.Update(users.ID(r.PathValue("user")), profile)
		if err != nil {
			writeUserError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, user)
	}
}

func readProfile(w http.ResponseWriter, r *http.Request) (users.Profile, bool) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	var profile users.Profile
	if err := decoder.Decode(&profile); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return users.Profile{}, false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return users.Profile{}, false
	}
	return profile, true
}

func writeUserError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, users.ErrInvalidProfile):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid_profile"})
	case errors.Is(err, users.ErrHandleTaken):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "handle_taken"})
	case errors.Is(err, users.ErrInvalidID), errors.Is(err, users.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
