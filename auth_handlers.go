package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"financetracker/internal/domain"
)

// cookieSecure marks the session cookie Secure by default, which is what
// a real HTTPS deployment needs (browsers refuse to send Secure cookies
// over plain HTTP). Set ENV=development to test over http://localhost.
func cookieSecure() bool {
	return os.Getenv("ENV") != "development"
}

// cookieSameSite defaults to Strict - the strongest CSRF protection a
// cookie can get, appropriate since the UI and API are served from the
// same origin and never need the cookie sent on a cross-site request.
// Override with COOKIE_SAMESITE=lax only if you split the frontend onto
// a different subdomain that still needs top-level-navigation cookies
// (e.g. following a link in from elsewhere); "none" additionally
// requires Secure and real cross-site use, which this app doesn't do.
func cookieSameSite() http.SameSite {
	switch strings.ToLower(os.Getenv("COOKIE_SAMESITE")) {
	case "lax":
		return http.SameSiteLaxMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteStrictMode
	}
}

func sessionCookie(token string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   cookieSecure(),
		SameSite: cookieSameSite(),
		MaxAge:   maxAge,
	}
}

func (s *apiServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	user, err := s.authService.Register(r.Context(), req.Username, req.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"id":       user.ID,
		"username": user.Username,
	})
}

func (s *apiServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if _, ok := s.currentUserID(r); ok {
		http.Error(w, "already logged in - log out first", http.StatusConflict)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	token, err := s.authService.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}

	http.SetCookie(w, sessionCookie(token, sessionTTLSeconds))
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged in"})
}

func (s *apiServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_ = s.authService.Logout(r.Context(), cookie.Value)
	}
	http.SetCookie(w, sessionCookie("", -1))
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

func (s *apiServer) handleAccount(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		user, ok := s.authService.GetUser(userID)
		if !ok {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"id":       user.ID,
			"username": user.Username,
		})
	case http.MethodDelete:
		var token string
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			token = cookie.Value
		}
		if err := s.authService.DeleteAccount(r.Context(), userID, token); err != nil {
			writeAuthError(w, err)
			return
		}
		http.SetCookie(w, sessionCookie("", -1))
		writeJSON(w, http.StatusOK, map[string]string{"status": "account deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeAuthError(w http.ResponseWriter, err error) {
	var policyErr *domain.PasswordPolicyError
	switch {
	case errors.As(err, &policyErr):
		http.Error(w, policyErr.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrUsernameTaken):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, domain.ErrUsernameRequired):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrInvalidCredentials):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, domain.ErrUserNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
