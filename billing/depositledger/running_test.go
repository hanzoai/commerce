package depositledger

import (
	"context"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/billing/depositwatch"
)

// oneAsset is a coherent single-asset watch table (base/usdc), which is exactly
// what the live deployment is configured with.
func oneAsset() []string {
	return []string{
		"CRYPTO_DEPOSIT_RPC_BASE=https://mainnet.base.org",
		"CRYPTO_DEPOSIT_TOKEN_BASE_USDC=0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
	}
}

// noChain builds a Service whose readers never touch a network, so the schedule
// can be started and stopped in a unit test.
func noChain(t *testing.T, environ []string) *Service {
	t.Helper()
	svc, err := New(environ,
		WithStore(stubStore{}),
		WithCursor(stubCursor{}),
		WithReader(func(depositwatch.Asset) (depositwatch.Reader, error) { return stubReader{}, nil }),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc
}

// TestRunningIsNotEnabled pins the distinction the mint gate depends on.
//
// Enabled means "an asset was configured". Running means "something is reading
// the chain for it". Only the second implies a deposit reaches a balance, and
// the gap between them is where a custody address gets handed out that nothing
// will ever look at. If these two ever collapse into one another, the gate stops
// meaning anything.
func TestRunningIsNotEnabled(t *testing.T) {
	svc := noChain(t, oneAsset())

	if !svc.Enabled() {
		t.Fatal("a configured asset must be Enabled")
	}
	if svc.Running() {
		t.Fatal("constructed but never started must NOT be Running — this is the state Bootstrap leaves behind, and a gate that called it running would hand out addresses nothing is watching")
	}

	svc.Start()
	if !svc.Running() {
		t.Fatal("started must be Running")
	}

	svc.Stop()
	if svc.Running() {
		t.Fatal("stopped must NOT be Running — a wedged or shut-down schedule must close the rail, not leave it open")
	}
	if !svc.Enabled() {
		t.Fatal("Stop must not un-configure the asset; Enabled describes the table, not the schedule")
	}
}

// TestRunningNilAndUnconfigured covers the two inert shapes. Both must answer
// false rather than panic: Default() returns nil before Bootstrap installs
// anything, and a deploy with no CRYPTO_DEPOSIT_* is the ordinary disabled case.
func TestRunningNilAndUnconfigured(t *testing.T) {
	var nilSvc *Service
	if nilSvc.Running() {
		t.Fatal("a nil Service must not report Running")
	}
	if nilSvc.Enabled() {
		t.Fatal("a nil Service must not report Enabled")
	}

	empty := noChain(t, nil)
	if empty.Running() {
		t.Fatal("an unconfigured watcher must not report Running")
	}
	empty.Start() // must be a no-op, not a goroutine over an empty table
	if empty.Running() {
		t.Fatal("Start on an unconfigured watcher must stay not-Running")
	}
}

// TestStatusReportsTheWatchTable is the replacement for a boot log that could
// not be read back off a running pod.
func TestStatusReportsTheWatchTable(t *testing.T) {
	svc := noChain(t, oneAsset())
	ctx := context.Background()

	st := svc.Status(ctx)
	if st.Running {
		t.Fatal("Status must report the real schedule state, not the configuration")
	}
	if len(st.Assets) != 1 {
		t.Fatalf("want 1 watched asset, got %d", len(st.Assets))
	}
	a := st.Assets[0]
	if a.Chain != "base" || a.Token != "usdc" {
		t.Fatalf("wrong asset: %+v", a)
	}
	if a.Contract != "0x833589fcd6edb6e08f4c7c32d4f71b54bda02913" {
		t.Fatalf("contract must be folded as its chain folds one: %q", a.Contract)
	}

	svc.Start()
	defer svc.Stop()
	if !svc.Status(ctx).Running {
		t.Fatal("Status must follow Start")
	}
}

// TestStatusAssetsNeverNil: a nil slice marshals to `null`, and an admin page
// handed `null` where it expects a list is a client-side crash rather than an
// empty table.
func TestStatusAssetsNeverNil(t *testing.T) {
	if got := noChain(t, nil).Status(context.Background()).Assets; got == nil {
		t.Fatal("Status().Assets must be [] and never nil")
	}
	var nilSvc *Service
	if got := nilSvc.Status(context.Background()).Assets; got == nil {
		t.Fatal("a nil Service must still render an empty asset list")
	}
}

// TestStatusNeverEchoesACredential is a security property, not a formatting
// preference. A managed node URL carries its API key in the path or the query,
// so echoing RPCURL verbatim onto an admin endpoint publishes a credential to
// every reader of that page.
func TestStatusNeverEchoesACredential(t *testing.T) {
	const key = "s3cr3t-api-key"
	svc := noChain(t, []string{
		"CRYPTO_DEPOSIT_RPC_BASE=https://base-mainnet.g.alchemy.com/v2/" + key + "?token=" + key,
		"CRYPTO_DEPOSIT_TOKEN_BASE_USDC=0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
	})

	st := svc.Status(context.Background())
	if len(st.Assets) != 1 {
		t.Fatalf("want 1 asset, got %d", len(st.Assets))
	}
	if got := st.Assets[0].Endpoint; got != "https://base-mainnet.g.alchemy.com" {
		t.Fatalf("endpoint must reduce to scheme://host, got %q", got)
	}
	// Belt and braces: the whole rendered status must not contain the secret
	// anywhere, including a field added later without thinking about this.
	if rendered := renderAll(st); strings.Contains(rendered, key) {
		t.Fatalf("status leaked the RPC credential: %s", rendered)
	}
}

func TestRPCHostDropsPathAndQuery(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"https://mainnet.base.org", "https://mainnet.base.org"},
		{"https://x.example/v2/KEY", "https://x.example"},
		{"https://x.example/v2/KEY?apikey=KEY", "https://x.example"},
		{"https://user:pass@x.example/v2/KEY", "https://x.example"},
		{"  https://x.example:8545/rpc  ", "https://x.example:8545"},
		{"", ""},
		{"not-a-url", ""},
	} {
		if got := rpcHost(tc.in); got != tc.want {
			t.Errorf("rpcHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func renderAll(st Status) string {
	var b strings.Builder
	for _, a := range st.Assets {
		b.WriteString(a.Chain + "|" + a.Token + "|" + a.Contract + "|" + a.Endpoint + "|" + a.Pooled + "\n")
	}
	return b.String()
}

// --- stubs: no chain, no database ---

type stubReader struct{}

func (stubReader) BlockNumber(context.Context) (uint64, error) { return 100, nil }
func (stubReader) TransfersTo(context.Context, []string, uint64, uint64) ([]depositwatch.Transfer, error) {
	return nil, nil
}
func (stubReader) Decimals(context.Context) (int, error) { return 6, nil }
func (stubReader) Symbol(context.Context) (string, error) { return "USDC", nil }

type stubStore struct{}

func (stubStore) Watched(context.Context, string, string) ([]depositwatch.Watched, error) {
	return nil, nil
}
func (stubStore) Sight(context.Context, depositwatch.Sighting) error   { return nil }
func (stubStore) Unsight(context.Context, depositwatch.Sighting) error { return nil }
func (stubStore) Credit(context.Context, depositwatch.Credit) (bool, error) {
	return false, nil
}
func (stubStore) RecordUnattributed(context.Context, depositwatch.Unattributed) error { return nil }

type stubCursor struct{}

func (stubCursor) Last(context.Context, string) (uint64, error) { return 42, nil }
func (stubCursor) Save(context.Context, string, uint64) error   { return nil }
