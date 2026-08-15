package main

import (
	"log"
	"net/http"
	"time"
)

// statusRecorder wraps http.ResponseWriter so we can observe the status
// code a handler actually wrote. The standard http.ResponseWriter
// interface has no getter for this — wrapping it and intercepting
// WriteHeader is the idiomatic way to capture it without touching every
// handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware logs method, path, response status, and how long the
// request took, for every request that passes through it. Wrap this
// around your mux (or compose it with your other middleware) in routes().
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Pass the request to the next handler first so we can log the total duration
		next.ServeHTTP(w, r)

		// Extract both the direct network address and the forwarded header
		remoteAddr := r.RemoteAddr
		forwardedFor := r.Header.Get("X-Forwarded-For")

		// Extract Protocol
		protocol := r.Proto

		// Log the result with both IP values
		log.Printf("[%s] %s | Proto: %s | RemoteAddr: %s | X-Forwarded-For: %s | Duration: %v",
			r.Method, r.URL.Path, protocol, remoteAddr, forwardedFor, time.Since(start))
	})
}
