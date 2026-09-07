package organization

import "testing"

// TestOneProductionManySandboxes is the rule itself: an org transacts for real
// in exactly one environment, and may invent as many sandboxes as it likes.
func TestOneProductionManySandboxes(t *testing.T) {
	org := Organization{Name: "hanzo"}

	if !org.In(Production).Live {
		t.Fatal("production must be live")
	}
	for _, env := range []string{"sandbox", "staging", "demo", "alice", "load-test", ""} {
		s := org.In(env)
		if s.Live {
			t.Fatalf("%q charged live credentials; only production may", env)
		}
		if !s.TestMode() || s.SquareEnvironment() != "sandbox" {
			t.Fatalf("%q did not reach the sandbox books", env)
		}
	}
}

// TestSandboxIsWhereTyposLand holds the fail-closed direction. A name that is
// nearly production is a sandbox, because the cost of reading a typo as live is
// a real card charged against a merchant nobody meant to use.
func TestSandboxIsWhereTyposLand(t *testing.T) {
	for _, near := range []string{"Production", "PRODUCTION", "production ", "prod", "production2"} {
		if (Organization{}).In(near).Live {
			t.Fatalf("%q was read as production", near)
		}
	}
}

// TestScopingLeavesTheRecordAlone pins the value semantics. Scoping an org to a
// sandbox for one request must not make the stored org a sandbox for everyone.
func TestScopingLeavesTheRecordAlone(t *testing.T) {
	live := Organization{Name: "hanzo", Live: true}
	if sandbox := live.In("demo"); sandbox.Live || !live.Live {
		t.Fatalf("scoping mutated the org: live=%v sandbox=%v", live.Live, sandbox.Live)
	}
	if !live.In("demo").In(Production).Live {
		t.Fatal("an org could not be scoped back to production")
	}
}

// TestCredentialsFollowTheEnvironment is the point of the whole exercise: one
// org holds both credential pairs at once, and the environment picks.
func TestCredentialsFollowTheEnvironment(t *testing.T) {
	org := Organization{Name: "hanzo"}
	org.Square.Production.ApplicationId = "live-app"
	org.Square.Sandbox.ApplicationId = "sandbox-app"

	prod := org.In(Production)
	if got := prod.SquareConfig(prod.TestMode()).ApplicationId; got != "live-app" {
		t.Fatalf("production reached %q", got)
	}
	demo := org.In("demo")
	if got := demo.SquareConfig(demo.TestMode()).ApplicationId; got != "sandbox-app" {
		t.Fatalf("sandbox reached %q", got)
	}
}
