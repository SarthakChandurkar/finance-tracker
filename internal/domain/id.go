package domain

import (
	"crypto/rand"
	"encoding/hex"
)

// newID generates a random 16-character hex string, e.g. "a3f9c1b2e6d84071".
// This is a stand-in for what other languages might call a UUID. Go's
// standard library doesn't include a UUID generator, but for a local,
// single-user app, a random hex string is more than good enough — the
// odds of two colliding are astronomically small.
func newID() string {
	b := make([]byte, 8)
	// crypto/rand reads random bytes from the OS. We ignore the error
	// here (`_`) because reading from the OS's randomness source failing
	// is not something a small local app needs to handle gracefully —
	// if it ever happens, something is badly wrong with the machine.
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}