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

func TestHTTPAccessCanBeEstablishedInspectedAndRevoked(t *testing.T) {
	userStore, err := users.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := auth.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerUsersHTTP(mux, userStore, credentials)
	registerAuthHTTP(mux, credentials, userStore)
	request := func(method, path, body string, cookie *http.Cookie, bearer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}

	created := request("POST", "/users", `{"handle":"ada","display_name":"Ada Lovelace","password":"correct horse battery staple"}`, nil, "")
	if created.Code != 201 {
		t.Fatalf("create = %d: %s", created.Code, created.Body.String())
	}
	if invalid := request("POST", "/sessions", `{"handle":"ada","password":"wrong password"}`, nil, ""); invalid.Code != 401 {
		t.Fatalf("invalid login = %d", invalid.Code)
	}
	login := request("POST", "/sessions", `{"handle":"ada","password":"correct horse battery staple"}`, nil, "")
	if login.Code != 201 {
		t.Fatalf("login = %d: %s", login.Code, login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie = %#v", cookies)
	}
	if strings.Contains(login.Body.String(), auth.TokenPrefix) {
		t.Fatal("web session secret exposed in response body")
	}
	if inspect := request("GET", "/session", "", cookies[0], ""); inspect.Code != 200 {
		t.Fatalf("inspect = %d: %s", inspect.Code, inspect.Body.String())
	}

	issued := request("POST", "/access-grants", `{"name":"profile script","kind":"api","scopes":["profile:read"],"expires_in_hours":24}`, cookies[0], "")
	if issued.Code != 201 {
		t.Fatalf("issue = %d: %s", issued.Code, issued.Body.String())
	}
	var grant struct{ ID, Token string }
	if err := json.NewDecoder(issued.Body).Decode(&grant); err != nil {
		t.Fatal(err)
	}
	if grant.ID == "" || !strings.HasPrefix(grant.Token, auth.TokenPrefix) {
		t.Fatalf("issued grant = %#v", grant)
	}
	listed := request("GET", "/access-grants", "", cookies[0], "")
	if listed.Code != 200 {
		t.Fatalf("list = %d", listed.Code)
	}
	if strings.Contains(listed.Body.String(), grant.Token) {
		t.Fatal("list repeated bearer secret")
	}
	if inspect := request("GET", "/session", "", nil, grant.Token); inspect.Code != 200 {
		t.Fatalf("bearer inspect = %d", inspect.Code)
	}
	if update := request("PUT", "/users/00000000-0000-0000-0000-000000000000", `{"handle":"x","display_name":"X"}`, nil, grant.Token); update.Code != 401 {
		t.Fatalf("out-of-scope update = %d", update.Code)
	}
	if revoked := request("DELETE", "/access-grants/"+grant.ID, "", cookies[0], ""); revoked.Code != 204 {
		t.Fatalf("revoke = %d: %s", revoked.Code, revoked.Body.String())
	}
	if inspect := request("GET", "/session", "", nil, grant.Token); inspect.Code != 401 {
		t.Fatalf("revoked inspect = %d", inspect.Code)
	}
	if logout := request("DELETE", "/session", "", cookies[0], ""); logout.Code != 204 {
		t.Fatalf("logout = %d", logout.Code)
	}
	if inspect := request("GET", "/session", "", cookies[0], ""); inspect.Code != 401 {
		t.Fatalf("logged-out inspect = %d", inspect.Code)
	}
}
