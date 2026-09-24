package organization

import "testing"

// TestTwoKinds is the rule itself. An org may hold as many environments as it
// likes and each one chooses its kind, so a team can run an experimental build
// against real money while a stable one keeps serving customers.
func TestTwoKinds(t *testing.T) {
	org := Organization{Name: "hanzo"}

	if !org.In(Live).Live {
		t.Fatal("a live environment must transact live")
	}
	test := org.In(Test)
	if test.Live || !test.TestMode() {
		t.Fatal("a test environment must transact test")
	}

	// Several of each, all from the one org, none disturbing the others.
	for _, k := range []Kind{Live, Test, Live, Test} {
		if got := org.In(k).Live; got != (k == Live) {
			t.Fatalf("%s came out Live=%v", k, got)
		}
	}
}

// TestUnknownKindIsTest holds the fail-closed direction: only the exact kind
// live charges a card, so an environment that arrives with no kind, or with a
// kind nobody recognises, reaches the sandbox.
func TestUnknownKindIsTest(t *testing.T) {
	for _, k := range []Kind{"", "LIVE", "Live", "production", "prod", "livee"} {
		if (Organization{}).In(k).Live {
			t.Fatalf("%q was read as live", k)
		}
	}
}

// TestScopingLeavesTheRecordAlone pins the value semantics. Scoping an org to a
// test environment for one request must not make the stored org a test org for
// everyone.
func TestScopingLeavesTheRecordAlone(t *testing.T) {
	live := Organization{Name: "hanzo", Live: true}
	if sandbox := live.In(Test); sandbox.Live || !live.Live {
		t.Fatalf("scoping mutated the org: live=%v test=%v", live.Live, sandbox.Live)
	}
	if !live.In(Test).In(Live).Live {
		t.Fatal("an org could not be scoped back to live")
	}
}

// TestCredentialsFollowTheKind is the point of the whole exercise: one org holds
// both credential pairs at once, and the environment picks between them.
func TestCredentialsFollowTheKind(t *testing.T) {
	org := Organization{Name: "hanzo"}
	org.Square.Production.ApplicationId = "live-app"
	org.Square.Sandbox.ApplicationId = "sandbox-app"

	live := org.In(Live)
	if got := live.SquareConfig(live.TestMode()).ApplicationId; got != "live-app" {
		t.Fatalf("a live environment reached %q", got)
	}
	test := org.In(Test)
	if got := test.SquareConfig(test.TestMode()).ApplicationId; got != "sandbox-app" {
		t.Fatalf("a test environment reached %q", got)
	}
}

// TestEveryProcessorSpellsBothKinds keeps the translation honest. The two kinds
// are ours; each processor's word for them is its own, and a table that answers
// the same word for both kinds would send a sandbox key to a live endpoint.
func TestEveryProcessorSpellsBothKinds(t *testing.T) {
	for processor := range word {
		live, test := Live.Word(processor), Test.Word(processor)
		if live == "" || test == "" {
			t.Fatalf("%s has an empty word", processor)
		}
		if live == test {
			t.Fatalf("%s spells both kinds %q", processor, live)
		}
	}
	if got := Live.Word("square"); got != "production" {
		t.Fatalf("Square live is %q", got)
	}
	if got := Test.Word("square"); got != "sandbox" {
		t.Fatalf("Square test is %q", got)
	}
	if got := Test.Word("stripe"); got != "test" {
		t.Fatalf("Stripe test is %q", got)
	}
	// A processor nobody has taught us about gets our own word.
	if got := Test.Word("worldpay"); got != "test" {
		t.Fatalf("an unknown processor answered %q", got)
	}
}

// TestSquareEnvironmentReadsTheTable proves the existing accessor and the table
// are one source, so Square cannot be spelled in two places and drift.
func TestSquareEnvironmentReadsTheTable(t *testing.T) {
	if got := (Organization{Live: true}).SquareEnvironment(); got != Live.Word("square") {
		t.Fatalf("live org says %q, table says %q", got, Live.Word("square"))
	}
	if got := (Organization{}).SquareEnvironment(); got != Test.Word("square") {
		t.Fatalf("test org says %q, table says %q", got, Test.Word("square"))
	}
}
