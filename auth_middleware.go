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
	// "/api/register": true,
	"/api/status": true,
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

		token, userID, ok := s.currentSession(r)
		if !ok {
			respondUnauthenticated(w, r)
			return
		}

		// The Redis-side session TTL already slides forward on every
		// lookup (see SessionStore.UserID), but the *cookie* the browser
		// holds does not - its MaxAge was fixed once, at login. That
		// mismatch is the "session/access token never refreshes" bug:
		// the browser silently drops the cookie sessionTTLSeconds after
		// login even though the server-side session is still alive and
		// the user has been active the whole time. Re-issuing the same
		// token with a fresh MaxAge on every authenticated request keeps
		// the client-side expiry in lockstep with the server-side one.
		http.SetCookie(w, sessionCookie(token, sessionTTLSeconds))

		ctx := context.WithValue(r.Context(), userIDCtxKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// currentUserID resolves the session cookie on r (if any) to a user ID.
func (s *apiServer) currentUserID(r *http.Request) (string, bool) {
	_, userID, ok := s.currentSession(r)
	return userID, ok
}

// currentSession resolves the session cookie on r (if any) to its raw
// token plus the user ID it belongs to. Callers that only need the user
// ID can use currentUserID instead; sessionMiddleware needs the token
// too, so it can re-issue the cookie with a refreshed MaxAge.
func (s *apiServer) currentSession(r *http.Request) (token string, userID string, ok bool) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		token = cookie.Value
	}
	userID, ok = s.authService.CurrentUser(r.Context(), token)
	return token, userID, ok
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