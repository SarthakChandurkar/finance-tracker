package main

import (
	"context"
	"net/http"
	"strings"
)

const sessionCookieName = "session_token"

// publicPaths lists the exact routes that must stay reachable without a
// session, so a signed-out visitor can actually reach and submit the
// login/register screens.
var publicPaths = map[string]bool{
	"/login.html":    true,
	"/register.html": true,
	"/api/login":     true,
	"/api/register":  true,
}

// publicAssetSuffixes lets static files the login/register pages depend
// on (css/js/images/fonts) load before the user is authenticated. Add to
// this list if your ui/ folder needs other shared static assets reachable
// pre-login.
var publicAssetSuffixes = []string{".css", ".js", ".ico", ".png", ".svg", ".woff", ".woff2"}

func isPublicPath(p string) bool {
	if publicPaths[p] {
		return true
	}
	for _, suf := range publicAssetSuffixes {
		if strings.HasSuffix(p, suf) {
			return true
		}
	}
	return false
}

type ctxKey string

const usernameCtxKey ctxKey = "username"

// sessionMiddleware protects every route other than login/register (and
// their static assets) behind a valid session cookie. The idle window is
// SESSION_TTL_MINUTES (see main.go); each authenticated request slides
// that window forward, so a session only dies if the user goes quiet for
// the whole window - exactly the "no refresh -> log in again" behavior.
func (s *apiServer) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		var token string
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			token = cookie.Value
		}

		username, ok := s.authService.CurrentUser(r.Context(), token)
		if !ok {
			respondUnauthenticated(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), usernameCtxKey, username)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func respondUnauthenticated(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login.html", http.StatusFound)
}
