package billing

import (
	"testing"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/test/ae"
)

// capEnforce reads the org's single stored cap back through the real list reader and
// reports its enforcement. It reads the PERSISTED row rather than the create response
// on purpose: what matters is what the gate will later load, not what the handler
// echoed back down the same request.
func capEnforce(t *testing.T, org *organization.Organization) bool {
	t.Helper()
	rows := listCaps(t, org)
	if len(rows) != 1 {
		t.Fatalf("org holds %d caps, want exactly 1", len(rows))
	}
	on, ok := rows[0]["enforce"].(bool)
	if !ok {
		t.Fatalf("enforce = %v (%T), want a bool", rows[0]["enforce"], rows[0]["enforce"])
	}
	return on
}

// A cap created with no opinion on enforcement is a HARD cap. The body a console or an
// API client sends when it simply wants a budget carries no "enforce" key at all, and a
// budget that watches spend run past its own ceiling is decoration — so ABSENT means
// enforcing. Opting out stays available and is still honored; it just has to be said.
func TestSpendAlert_CreateDefaultsToEnforcing(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	t.Run("absent enforce => hard cap", func(t *testing.T) {
		org := &organization.Organization{}
		org.Name = "enf-absent"
		createCap(t, org, `{"title":"budget","threshold":100}`)
		if !capEnforce(t, org) {
			t.Fatalf("cap created with no enforce key is soft, want hard (a new cap denies, it does not merely warn)")
		}
	})

	t.Run("explicit false => warn-only cap", func(t *testing.T) {
		org := &organization.Organization{}
		org.Name = "enf-false"
		createCap(t, org, `{"title":"budget","threshold":100,"enforce":false}`)
		if capEnforce(t, org) {
			t.Fatalf(`cap created with "enforce": false is hard, want soft (a stated opt-out must survive the default)`)
		}
	})

	t.Run("explicit true => hard cap", func(t *testing.T) {
		org := &organization.Organization{}
		org.Name = "enf-true"
		createCap(t, org, `{"title":"budget","threshold":100,"enforce":true}`)
		if !capEnforce(t, org) {
			t.Fatalf(`cap created with "enforce": true is soft, want hard`)
		}
	})
}

// The update path is untouched. It has always taken *bool so an ABSENT enforce
// PRESERVES whatever the row holds, and that must keep holding in BOTH directions now
// that create writes true by default: editing a threshold must not re-decide
// enforcement, and a soft cap must not harden itself the first time it is edited.
func TestSpendAlert_UpdateLeavesEnforceAlone(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	t.Run("absent enforce preserves a hard cap", func(t *testing.T) {
		org := &organization.Organization{}
		org.Name = "upd-hard"
		id := createCapAs(t, org, "alice", `{"title":"budget","threshold":100,"enforce":true}`)
		if code := patchCapAs(t, org, id, "alice", "", `{"threshold":200}`); code != 200 {
			t.Fatalf("PATCH = %d, want 200", code)
		}
		if !capEnforce(t, org) {
			t.Fatalf("a PATCH that never mentioned enforce softened a hard cap")
		}
	})

	t.Run("absent enforce preserves a soft cap", func(t *testing.T) {
		org := &organization.Organization{}
		org.Name = "upd-soft"
		id := createCapAs(t, org, "alice", `{"title":"budget","threshold":100,"enforce":false}`)
		if code := patchCapAs(t, org, id, "alice", "", `{"threshold":200}`); code != 200 {
			t.Fatalf("PATCH = %d, want 200", code)
		}
		if capEnforce(t, org) {
			t.Fatalf("a PATCH that never mentioned enforce hardened a soft cap — the create default must not reach the update path")
		}
	})

	t.Run("explicit enforce still flips both ways", func(t *testing.T) {
		org := &organization.Organization{}
		org.Name = "upd-flip"
		id := createCapAs(t, org, "alice", `{"title":"budget","threshold":100,"enforce":true}`)
		if code := patchCapAs(t, org, id, "alice", "", `{"enforce":false}`); code != 200 {
			t.Fatalf("PATCH off = %d, want 200", code)
		}
		if capEnforce(t, org) {
			t.Fatalf(`PATCH "enforce": false left the cap hard`)
		}
		if code := patchCapAs(t, org, id, "alice", "", `{"enforce":true}`); code != 200 {
			t.Fatalf("PATCH on = %d, want 200", code)
		}
		if !capEnforce(t, org) {
			t.Fatalf(`PATCH "enforce": true left the cap soft`)
		}
	})
}
