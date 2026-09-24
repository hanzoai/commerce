// Package secrets is commerce's read side of the HOST's secret plane.
//
// One value, set once at embed time, read by whichever handler needs a
// deployment secret. It exists so a plugin reads secrets the way it reads
// everything else — through the host it is embedded in — instead of through an
// env fan-out (KMS → a k8s Secret → a pod env var → os.Getenv), which is three
// places for one value to go stale and a restart to pick up a rotation.
//
// nil is a supported state, not a failure: a standalone commerce has no host to
// ask, and every caller is expected to fall back to what it did before.
package secrets

import (
	"context"
	"strings"
	"sync"
)

// Reader is the narrowest thing commerce needs: read one secret by reference.
// Commerce never writes, never lists, and never learns the transport — a host
// satisfies this in-process, over UDS, or from a map in a test.
type Reader interface {
	GetSecret(ctx context.Context, ref string) ([]byte, error)
}

var (
	mu sync.RWMutex
	r  Reader
)

// Set installs the host's reader. Called once, before routes register.
func Set(x Reader) {
	mu.Lock()
	defer mu.Unlock()
	r = x
}

// Get returns the reader, or nil when commerce runs standalone.
func Get() Reader {
	mu.RLock()
	defer mu.RUnlock()
	return r
}

// String reads a secret and trims it, answering "" for every failure mode —
// no reader, no such secret, unreadable. Callers treat absence and error the
// same way (fall back), so collapsing them here keeps that decision in one
// place instead of at each call site.
func String(ctx context.Context, ref string) string {
	rd := Get()
	if rd == nil {
		return ""
	}
	b, err := rd.GetSecret(ctx, ref)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
