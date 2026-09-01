package main

import (
	"net/http"
	"os"
	"strings"
)

// allowedOrigins is the set of origins (scheme+host, e.g.
// "https://app.example.com") that are permitted to make cross-origin,
// credentialed (cookie-carrying) requests to the API. It is populated
// once in main() from the ALLOWED_ORIGINS env var (comma-separated).
//
// Left empty (the default), no Access-Control-Allow-Origin header is
// ever sent, which means browsers block all cross-origin access - the
// safe default for a site whose frontend and backend are served from
// the same origin, as this one currently is. Only add an origin here if
// a *different* origin genuinely needs to call this API with the
// session cookie attached.
var allowedOrigins map[string]bool

// loadAllowedOrigins parses ALLOWED_ORIGINS ("https://a.com,https://b.com")
// into allowedOrigins. Called once from main().
func loadAllowedOrigins() {
	allowedOrigins = map[string]bool{}
	raw := os.Getenv("ALLOWED_ORIGINS")
	if raw == "" {
		return
	}
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(strings.TrimSuffix(o, "/"))
		if o != "" {
			allowedOrigins[o] = true
		}
	}
}

// corsMiddleware implements a default-deny CORS policy: only origins
// explicitly listed in ALLOWED_ORIGINS get Access-Control-Allow-Origin,
// and it is always echoed back as that single origin (never "*") because
// this API relies on credentialed (cookie) requests, and the CORS spec
// forbids combining a wildcard origin with Access-Control-Allow-Credentials.
//
// This runs before sessionMiddleware in the chain so that CORS preflight
// (OPTIONS) requests - which never carry the session cookie - are
// answered without ever hitting the auth check.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		// Always vary on Origin so shared caches/CDNs don't serve one
		// origin's CORS headers to a different origin.
		w.Header().Add("Vary", "Origin")

		if origin != "" && allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
		}

		if r.Method == http.MethodOptions {
			// Preflight: never reaches the mux/session middleware, no
			// body needed either way.
			if origin != "" && allowedOrigins[origin] {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusForbidden)
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}

// securityHeadersMiddleware sets response headers that are cheap,
// broadly-supported hardening defaults for a browser-facing site. None
// of these depend on knowing the frontend's internals except the
// Content-Security-Policy, which is intentionally conservative (no
// 'unsafe-inline'/'unsafe-eval' for scripts) - if the existing UI relies
// on inline <script> tags or a third-party CDN, tighten/loosen script-src
// via CSP_POLICY below rather than editing Go code.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// Stop the browser from guessing content types (blocks some
		// MIME-sniffing based XSS/drive-by-download tricks).
		h.Set("X-Content-Type-Options", "nosniff")

		// Nobody should be able to iframe this site (clickjacking).
		h.Set("X-Frame-Options", "DENY")

		// Don't leak the full referring URL (which may contain
		// tokens/paths) to other origins.
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Opt out of browser features this app has no legitimate use for.
		h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=(), payment=(), usb=()")

		// Isolate this browsing context from cross-origin windows/popups
		// and stop other origins from <img>/<script>-loading our
		// responses (Spectre-class protections).
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")

		// HSTS only makes sense once the site is actually served over
		// HTTPS - forcing it in local http://localhost dev would break
		// the browser's ability to load the site at all. Mirrors the
		// same ENV=development escape hatch used for the cookie's
		// Secure flag.
		if cookieSecure() {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		}

		h.Set("Content-Security-Policy", contentSecurityPolicy())

		next.ServeHTTP(w, r)
	})
}

// contentSecurityPolicy returns the CSP_POLICY env var verbatim if set,
// so the policy can be tuned to match the real frontend (inline scripts,
// CDNs, etc.) without a code change/redeploy. Otherwise it falls back to
// a same-origin-only default.
func contentSecurityPolicy() string {
	if custom := os.Getenv("CSP_POLICY"); custom != "" {
		return custom
	}
	return strings.Join([]string{
		"default-src 'self'",
		"base-uri 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"form-action 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data:",
		"font-src 'self' data:",
		"connect-src 'self'",
	}, "; ")
}
