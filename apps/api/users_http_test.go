package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

func TestUserHTTPAccountLifecycle(t *testing.T) {
	store, err := users.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerUsersHTTP(mux, store)

	create := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"handle":"octocat","display_name":"The Octocat"}`))
	createdResponse := httptest.NewRecorder()
	mux.ServeHTTP(createdResponse, create)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", createdResponse.Code, createdResponse.Body.String())
	}
	var created users.User
	if err := json.NewDecoder(createdResponse.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if createdResponse.Header().Get("Location") != "/users/"+string(created.ID) {
		t.Fatalf("unexpected Location: %q", createdResponse.Header().Get("Location"))
	}

	getResponse := httptest.NewRecorder()
	mux.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "/users/"+string(created.ID), nil))
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status = %d", getResponse.Code)
	}
	var got users.User
	if err := json.NewDecoder(getResponse.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got != created {
		t.Fatalf("got %#v, want %#v", got, created)
	}

	updateResponse := httptest.NewRecorder()
	mux.ServeHTTP(updateResponse, httptest.NewRequest(http.MethodPut, "/users/"+string(created.ID), strings.NewReader(`{"handle":"mona","display_name":"Mona Lisa Octocat"}`)))
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", updateResponse.Code, updateResponse.Body.String())
	}
	var updated users.User
	if err := json.NewDecoder(updateResponse.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.Handle != "mona" || updated.DisplayName != "Mona Lisa Octocat" {
		t.Fatalf("unexpected update: %#v", updated)
	}
}

func TestUserHTTPErrorsAreStable(t *testing.T) {
	store, err := users.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerUsersHTTP(mux, store)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
		return response
	}
	if response := request(http.MethodPost, "/users", `{"handle":"bad handle","display_name":"Name"}`); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid status = %d", response.Code)
	}
	first := request(http.MethodPost, "/users", `{"handle":"taken","display_name":"First"}`)
	if first.Code != http.StatusCreated {
		t.Fatal(first.Body.String())
	}
	if response := request(http.MethodPost, "/users", `{"handle":"TAKEN","display_name":"Second"}`); response.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d", response.Code)
	}
	if response := request(http.MethodGet, "/users/not-an-id", ""); response.Code != http.StatusNotFound {
		t.Fatalf("not found status = %d", response.Code)
	}
	if response := request(http.MethodPost, "/users", `{"handle":"valid","display_name":"Name","extra":true}`); response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d", response.Code)
	}
}
