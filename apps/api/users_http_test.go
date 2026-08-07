package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

func TestUserHTTPAccountLifecycle(t *testing.T) {
	store, err := users.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := auth.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerUsersHTTP(mux, store, credentials)
	registerAuthHTTP(mux, credentials, store)

	create := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(`{"handle":"octocat","display_name":"The Octocat","password":"correct horse battery staple"}`))
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
	byHandleResponse := httptest.NewRecorder()
	mux.ServeHTTP(byHandleResponse, httptest.NewRequest(http.MethodGet, "/users/by-handle/OCTOCAT", nil))
	if byHandleResponse.Code != http.StatusOK || byHandleResponse.Header().Get("Content-Location") != "/users/"+string(created.ID) {
		t.Fatalf("handle lookup = %d, %q", byHandleResponse.Code, byHandleResponse.Header().Get("Content-Location"))
	}

	loginResponse := httptest.NewRecorder()
	mux.ServeHTTP(loginResponse, httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"handle":"octocat","password":"correct horse battery staple"}`)))
	if loginResponse.Code != http.StatusCreated {
		t.Fatalf("login status = %d: %s", loginResponse.Code, loginResponse.Body.String())
	}
	updateRequest := httptest.NewRequest(http.MethodPut, "/users/"+string(created.ID), strings.NewReader(`{"handle":"mona","display_name":"Mona Lisa Octocat"}`))
	updateRequest.AddCookie(loginResponse.Result().Cookies()[0])
	updateResponse := httptest.NewRecorder()
	mux.ServeHTTP(updateResponse, updateRequest)
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
	credentials, err := auth.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerUsersHTTP(mux, store, credentials)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
		return response
	}
	if response := request(http.MethodPost, "/users", `{"handle":"bad handle","display_name":"Name","password":"long enough password"}`); response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid status = %d", response.Code)
	}
	first := request(http.MethodPost, "/users", `{"handle":"taken","display_name":"First","password":"long enough password"}`)
	if first.Code != http.StatusCreated {
		t.Fatal(first.Body.String())
	}
	if response := request(http.MethodPost, "/users", `{"handle":"TAKEN","display_name":"Second","password":"long enough password"}`); response.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d", response.Code)
	}
	if response := request(http.MethodGet, "/users/not-an-id", ""); response.Code != http.StatusNotFound {
		t.Fatalf("not found status = %d", response.Code)
	}
	if response := request(http.MethodPost, "/users", `{"handle":"valid","display_name":"Name","password":"long enough password","extra":true}`); response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d", response.Code)
	}
}
