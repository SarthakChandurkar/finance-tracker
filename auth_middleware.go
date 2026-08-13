package main

import (
	"context"
	"net/http"
	"strings"
)

const sessionCookieName = "session_token"

// authPages are the login/register screens themselves. They're only
// meant for a signed-out visitor - if an already-authenticated request
// hits one, we send them to "/" instead of rendering the form, so
// switching accounts requires an explicit logout first. Uncomment
// register.html here alongside the route/link toggles elsewhere once
// you're ready to test registration end-to-end.
var authPages = map[string]bool{
	"/login.html": true,
	// "/register.html": true,
}

// publicPaths lists additional routes that must stay reachable without a
// session (the login/register API endpoints themselves), regardless of
// whether the caller happens to be authenticated.
var publicPaths = map[string]bool{
	"/api/login": true,
	// "/api/register":  true,
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

const userIDCtxKey ctxKey = "userID"

// sessionMiddleware protects every route other than login/register (and
// their static assets) behind a valid session cookie. The idle window is
// SESSION_TTL_MINUTES (see main.go); each authenticated request slides
// that window forward, so a session only dies if the user goes quiet for
// the whole window - exactly the "no refresh -> log in again" behavior.
//
// The login/register pages are the mirror image of that protection: a
// visitor who is already signed in gets bounced away from them to "/"
// rather than being shown the form, so logging in as someone else always
// requires an explicit logout first.
func (s *apiServer) sessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authPages[r.URL.Path] {
			if _, ok := s.currentUserID(r); ok {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		userID, ok := s.currentUserID(r)
		if !ok {
			respondUnauthenticated(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), userIDCtxKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// currentUserID resolves the session cookie on r (if any) to a user ID.
func (s *apiServer) currentUserID(r *http.Request) (string, bool) {
	var token string
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		token = cookie.Value
	}
	return s.authService.CurrentUser(r.Context(), token)
}

// userIDFromContext reads the authenticated user's ID that
// sessionMiddleware attached to the request. Handlers use this to scope
// every read/write to that user's own data.
func userIDFromContext(r *http.Request) (string, bool) {
	userID, ok := r.Context().Value(userIDCtxKey).(string)
	return userID, ok
}

func respondUnauthenticated(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	http.Redirect(w, r, "/login.html", http.StatusFound)
}
