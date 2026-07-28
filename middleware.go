package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"financetracker/internal/domain"
	"financetracker/internal/storage"
)

// requestTimeKey is the context key nowMiddleware stores the request's
// "now" under. It's an unexported type (not just a plain string) specifically
// so no other package's context.WithValue call could ever collide with it
// by accident — this is the standard Go idiom for context keys.
type contextKey string

const requestTimeKey contextKey = "requestTime"

// nowMiddleware computes time.Now() exactly ONCE per incoming request and
// stores it on the request's context. Every handler downstream reads the
// SAME value back out (via nowFromContext) instead of calling time.Now()
// itself — so if one request happens to touch several quotas' month/year
// filtering (breakdowns, sweeps, funding checks, etc.), they're all
// evaluated against one single consistent instant, not several
// microseconds-apart calls to time.Now() that could theoretically disagree
// right at a month/year boundary.
//
// This does NOT get passed into the domain package as a context.Context —
// domain functions already take an explicit `now time.Time` parameter
// (see MonthlyBreakdown, RunMonthEndSweep, etc.), which is what keeps them
// easy to unit test without any HTTP machinery involved. This middleware's
// only job is to compute that value once and make it available to the
// handler, which then passes it down as that same plain argument.
func nowMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), requestTimeKey, time.Now())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// nowFromContext retrieves the current request's "now", as set by
// nowMiddleware. Handlers should call this instead of time.Now() directly.
//
// The fallback to a fresh time.Now() (rather than panicking) is deliberate:
// it means handler code, or a test calling a handler directly without going
// through the full middleware chain, still works correctly — it just loses
// the "single consistent instant per request" guarantee in that edge case.
func nowFromContext(ctx context.Context) time.Time {
	if now, ok := ctx.Value(requestTimeKey).(time.Time); ok {
		return now
	}
	return time.Now()
}

// catchUpIfMonthChanged is the "lazy cron" check for the month-rollover
// feature: it compares the stored LastMonthYear against the current
// month, and if the server was closed across one or more month
// boundaries, runs the missing RunMonthEndSweep(s) before letting the
// request through — then updates LastMonthYear to match.
//
// now is passed in explicitly (rather than calling time.Now() internally)
// so this stays consistent with the rest of the domain package's testable
// style, and so a caller can reuse the same request-scoped "now" that
// nowMiddleware already computed, instead of a second, slightly different
// timestamp.
func catchUpIfMonthChanged(store *storage.Store, schedule *domain.ScheduleService, now time.Time) {
	const monthYearLayout = "2006-01"

	lastMonthYear := ""
	store.View(func(d storage.Data) {
		lastMonthYear = d.LastMonthYear
	})

	currentMonthYear := now.Format(monthYearLayout)

	if lastMonthYear == "" {
		// First run ever (or upgrading from a version that predates this
		// field) — there's no prior known month to catch up FROM, so
		// there's nothing to sweep. Just record the current month.
		if err := store.Update(func(d *storage.Data) error {
			d.LastMonthYear = currentMonthYear
			return nil
		}); err != nil {
			log.Printf("catchUpIfMonthChanged: failed to record initial month: %v", err)
		}
		return
	}

	if lastMonthYear == currentMonthYear {
		// Already caught up — the common case, on nearly every request.
		return
	}

	cursor, err := time.Parse(monthYearLayout, lastMonthYear)
	if err != nil {
		// Stored value is corrupt/unexpected — don't get stuck retrying
		// forever on every request; log it, reset to the current month,
		// and move on.
		log.Printf("catchUpIfMonthChanged: invalid stored LastMonthYear %q: %v", lastMonthYear, err)
		if updateErr := store.Update(func(d *storage.Data) error {
			d.LastMonthYear = currentMonthYear
			return nil
		}); updateErr != nil {
			log.Printf("catchUpIfMonthChanged: failed to reset month: %v", updateErr)
		}
		return
	}

	// Sweep ONLY the last genuinely active month's leftover — that's the
	// real money that needs accounting for. We deliberately do NOT loop
	// through and sweep every calendar month in between (e.g. August,
	// September, October if the server was off that whole time): those
	// months never had any real activity, since the server wasn't
	// running. Sweeping each one individually would grant every skipped
	// month its own fresh MonthlyAllocation and immediately sweep it —
	// fabricating money nobody actually earned or was present to use,
	// which is exactly what happened the first time I tested this (see
	// the comment history above this function).

	if err := schedule.RunMonthEndSweep(cursor); err != nil {
		log.Printf("catchUpIfMonthChanged: sweep for %s failed: %v", lastMonthYear, err)
	}

	if err := store.Update(func(d *storage.Data) error {
		d.LastMonthYear = currentMonthYear
		return nil
	}); err != nil {
		log.Printf("catchUpIfMonthChanged: failed to save caught-up month: %v", err)
	}
}
