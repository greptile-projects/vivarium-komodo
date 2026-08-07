package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/greptile-projects/vivarium-komodo/apps/api/auth"
	"github.com/greptile-projects/vivarium-komodo/apps/api/users"
)

const sessionCookie = "komodo_session"

type authStore interface {
	SetPassword(string, string) error
	VerifyPassword(string, string) error
	Issue(string, string, auth.Kind, []auth.Scope, time.Duration) (auth.IssuedGrant, error)
	Authenticate(string, auth.Scope) (auth.Grant, error)
	List(string) ([]auth.Grant, error)
	Revoke(string, string) (auth.Grant, error)
}

type userAuthStore interface {
	FindByHandle(string) (users.User, error)
	Get(users.ID) (users.User, error)
}

func registerAuthHTTP(mux *http.ServeMux, credentials authStore, userStore userAuthStore) {
	mux.HandleFunc("POST /sessions", createSession(credentials, userStore))
	mux.HandleFunc("GET /session", inspectSession(credentials, userStore))
	mux.HandleFunc("DELETE /session", deleteSession(credentials))
	mux.HandleFunc("POST /access-grants", createAccessGrant(credentials))
	mux.HandleFunc("GET /access-grants", listAccessGrants(credentials))
	mux.HandleFunc("DELETE /access-grants/{grant}", revokeAccessGrant(credentials))
}

func createSession(credentials authStore, userStore userAuthStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Handle   string `json:"handle"`
			Password string `json:"password"`
		}
		if !readJSON(w, r, &input, 4096) {
			return
		}
		user, err := userStore.FindByHandle(input.Handle)
		if err != nil || credentials.VerifyPassword(string(user.ID), input.Password) != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_credentials"})
			return
		}
		issued, err := credentials.Issue(string(user.ID), "Web session", auth.Web, []auth.Scope{auth.ProfileRead, auth.ProfileWrite, auth.AccessManage, auth.RepositoryRead, auth.RepositoryWrite}, 12*time.Hour)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: issued.Token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: int((12 * time.Hour).Seconds())})
		writeJSON(w, http.StatusCreated, grantResponse(issued.Grant, ""))
	}
}

func inspectSession(credentials authStore, userStore userAuthStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		grant, ok := authenticateRequest(w, r, credentials, auth.ProfileRead)
		if !ok {
			return
		}
		user, err := userStore.Get(users.ID(grant.UserID))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": user, "access": grantResponse(grant, "")})
	}
}

func deleteSession(credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		grant, ok := authenticateRequest(w, r, credentials, auth.AccessManage)
		if !ok {
			return
		}
		if _, err := credentials.Revoke(grant.UserID, grant.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
		w.WriteHeader(http.StatusNoContent)
	}
}

func createAccessGrant(credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.AccessManage)
		if !ok {
			return
		}
		var input struct {
			Name           string       `json:"name"`
			Kind           auth.Kind    `json:"kind"`
			Scopes         []auth.Scope `json:"scopes"`
			ExpiresInHours int          `json:"expires_in_hours"`
		}
		if !readJSON(w, r, &input, 4096) {
			return
		}
		if input.Kind == auth.Web {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid_grant"})
			return
		}
		maximumHours := map[auth.Kind]int{auth.API: 90 * 24, auth.Git: 30 * 24}[input.Kind]
		if input.ExpiresInHours <= 0 || input.ExpiresInHours > maximumHours {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid_grant"})
			return
		}
		issued, err := credentials.Issue(actor.UserID, input.Name, input.Kind, input.Scopes, time.Duration(input.ExpiresInHours)*time.Hour)
		if errors.Is(err, auth.ErrInvalidGrant) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid_grant"})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
		writeJSON(w, http.StatusCreated, grantResponse(issued.Grant, issued.Token))
	}
}

func listAccessGrants(credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.AccessManage)
		if !ok {
			return
		}
		grants, err := credentials.List(actor.UserID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		page, perPage, ok := readPagination(w, r)
		if !ok {
			return
		}
		total := len(grants)
		grants = paginate(grants, page, perPage)
		output := make([]map[string]any, len(grants))
		for i, grant := range grants {
			output[i] = grantResponse(grant, "")
		}
		writeJSON(w, 200, map[string]any{"items": output, "page": page, "per_page": perPage, "total_count": total})
	}
}
func revokeAccessGrant(credentials authStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := authenticateRequest(w, r, credentials, auth.AccessManage)
		if !ok {
			return
		}
		if _, err := credentials.Revoke(actor.UserID, r.PathValue("grant")); errors.Is(err, auth.ErrNotFound) {
			writeJSON(w, 404, map[string]string{"error": "not_found"})
			return
		} else if err != nil {
			writeJSON(w, 500, map[string]string{"error": "internal_error"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func authenticateRequest(w http.ResponseWriter, r *http.Request, credentials authStore, scope auth.Scope) (auth.Grant, bool) {
	grant, authenticated, ok := authenticateOptionalRequest(w, r, credentials, scope)
	if ok && !authenticated {
		writeUnauthenticated(w, "Bearer", "komodo")
		return auth.Grant{}, false
	}
	return grant, ok
}

func authenticateOptionalRequest(w http.ResponseWriter, r *http.Request, credentials authStore, scope auth.Scope) (auth.Grant, bool, bool) {
	token := ""
	if authorization := r.Header.Get("Authorization"); strings.HasPrefix(authorization, "Bearer ") {
		token = strings.TrimPrefix(authorization, "Bearer ")
	}
	if token == "" {
		if cookie, err := r.Cookie(sessionCookie); err == nil {
			token = cookie.Value
		}
	}
	if token == "" {
		return auth.Grant{}, false, true
	}
	grant, err := credentials.Authenticate(token, scope)
	if err != nil {
		writeUnauthenticated(w, "Bearer", "komodo")
		return auth.Grant{}, false, false
	}
	return grant, true, true
}

func writeUnauthenticated(w http.ResponseWriter, scheme, realm string) {
	w.Header().Set("WWW-Authenticate", scheme+` realm="`+realm+`"`)
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
}

func authenticateGitOptional(w http.ResponseWriter, r *http.Request, credentials credentialAuthenticator, scope auth.Scope) (auth.Grant, bool, bool) {
	_, token, ok := r.BasicAuth()
	if !ok {
		return auth.Grant{}, false, true
	}
	grant, err := credentials.Authenticate(token, scope)
	if err != nil {
		writeGitUnauthenticated(w)
		return auth.Grant{}, false, false
	}
	return grant, true, true
}

func writeGitUnauthenticated(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="komodo git"`)
	http.Error(w, "authentication required", http.StatusUnauthorized)
}

func grantResponse(grant auth.Grant, token string) map[string]any {
	value := map[string]any{"id": grant.ID, "user_id": grant.UserID, "name": grant.Name, "kind": grant.Kind, "scopes": grant.Scopes, "created_at": grant.CreatedAt, "expires_at": grant.ExpiresAt, "last_used_at": grant.LastUsedAt, "revoked_at": grant.RevokedAt, "repository_id": grant.RepositoryID, "branch": grant.Branch}
	if token != "" {
		value["token"] = token
	}
	return value
}

func readJSON(w http.ResponseWriter, r *http.Request, value any, limit int64) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return false
	}
	return true
}
