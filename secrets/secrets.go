// Package secrets is commerce's read side of the HOST's secret plane.
//
// One value, set once at embed time, read by whichever handler needs a
// deployment secret. It exists so a plugin reads secrets the way it reads
// everything else — through the host it is embedded in — instead of through an
// env fan-out (KMS → a k8s Secret → a pod env var → os.Getenv), which is three
// places for one value to go stale and a restart to pick up a rotation.
//
// An org's own material lives at orgs/<org>/<name> in the default environment —
// the address cloud's KMS gives every org (apps/kms OrgPath) and the one every
// other in-process reader uses. [Org] is the one way commerce reads it.
//
// nil is a supported state: a commerce with no reader installed reads nothing,
// and every caller treats that as "not configured".
package secrets

import (
	"context"
	"strings"
	"sync"

	"github.com/hanzoai/namespace"
	"github.com/zap-proto/zip"
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

// Org reads one of org's own secrets, orgs/<org>/<name>, and asks AS org.
//
// The host's store serves a ref naming a tenant only to a call acting for that
// same tenant (cloud apps/kms admit), and the org a call acts for rides its
// caller, never the ref. A request's caller is whoever sent it — the PAYER, when a
// customer pays a brand — so the read re-points that caller at org with
// zip.ActingAs, which names who is acting on the call. A call with no caller reads
// nothing: cloud's KMS answers an unstated call as its own brand, so "no caller"
// must not reach it. org is always one commerce resolved itself (the charging
// org, or the brand that owns the host), never a value a request supplied as a
// ref.
//
// "" for every failure: no reader, an org that is not already a clean namespace
// slug (namespace.Sanitize, the fold cloud keys every org's store by), a name
// that cannot form a ref, no such secret, an unreadable one.
func Org(ctx context.Context, org, name string) string {
	if org == "" || namespace.Sanitize(org) != org || name == "" || name != strings.Trim(name, "/ ") || strings.Contains(name, "@") {
		return ""
	}
	actx, err := as(ctx, org)
	if err != nil {
		return ""
	}
	return String(actx, "orgs/"+org+"/"+name)
}

// as is ctx acting for org: unchanged when its caller already acts for org, and
// re-pointed with zip.ActingAs when it acts for another. ActingAs refuses a
// context with no authenticated caller, which is the refusal wanted here.
func as(ctx context.Context, org string) (context.Context, error) {
	if zip.CallerOf(ctx).Org == org {
		return ctx, nil
	}
	return zip.ActingAs(ctx, org)
}
