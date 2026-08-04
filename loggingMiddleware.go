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

		// Default to 200: if the handler never calls WriteHeader
		// explicitly (common for simple 200-OK responses), Go implicitly
		// sends 200 on the first Write() — our recorder should reflect
		// that same default rather than showing 0.
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		log.Printf("%s %s -> %d (%s)", r.Method, r.URL.Path, rec.status, duration)
	})
}
