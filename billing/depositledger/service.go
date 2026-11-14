package depositledger

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hanzoai/commerce/billing/depositwatch"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/payment/providers/kraken"
	"github.com/hanzoai/commerce/payment/routing"
)

// defaultInterval is how often the watcher looks at the chains.
//
// It is a constant, not config. The scan window is bounded by the cursor rather
// than by the clock, so the interval only trades RPC calls against how quickly a
// customer sees "confirming" — and 30s is comfortably inside every supported
// chain's confirmation time (Ethereum's 12 blocks alone are ~2.5 minutes). There
// is nothing here an operator needs to tune, and a knob on a money path is a
// knob someone eventually sets to zero.
const defaultInterval = 30 * time.Second

// Service is the scheduled crypto deposit watcher, wired to production stores.
//
// It is DISABLED, silently and safely, when no asset is configured — a deploy
// with no CRYPTO_DEPOSIT_* environment watches nothing and behaves exactly as
// before this package existed.
type Service struct {
	// st is what is watched now. It is replaced whole when the policy's accounts change
	// (Apply), so a pass in flight finishes on the set it began with.
	st       atomic.Pointer[state]
	store    depositwatch.Store
	cursor   depositwatch.Cursor
	interval time.Duration
	reader   func(depositwatch.Asset) (depositwatch.Reader, error)

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// state is one reading of the policy's deposit accounts: the assets, the chain watcher
// over them, and the exchange processor when an exchange account is on.
type state struct {
	assets  []depositwatch.Asset
	watcher *depositwatch.Watcher
	kraken  *kraken.Processor
}

// Option configures a Service (seams for tests: a fake chain, a fake store).
type Option func(*builder)

type builder struct {
	store    depositwatch.Store
	cursor   depositwatch.Cursor
	reader   func(depositwatch.Asset) (depositwatch.Reader, error)
	interval time.Duration
}

// WithStore overrides the intent/ledger store.
func WithStore(s depositwatch.Store) Option { return func(b *builder) { b.store = s } }

// WithCursor overrides the persisted scan position.
func WithCursor(c depositwatch.Cursor) Option { return func(b *builder) { b.cursor = c } }

// WithReader overrides how an asset's chain reader is built (tests inject a fake
// chain so the schedule is proven without an RPC endpoint).
func WithReader(fn func(depositwatch.Asset) (depositwatch.Reader, error)) Option {
	return func(b *builder) { b.reader = fn }
}

// WithInterval overrides the schedule period.
func WithInterval(d time.Duration) Option { return func(b *builder) { b.interval = d } }

// New builds the watcher from the policy's accounts.
//
// It returns an error ONLY on an incoherent configuration — a token with no
// endpoint, an unpriceable token, a malformed address. That is deliberately
// fatal to boot (see commerce.Bootstrap): an operator who configured a deposit
// rail must not get a process that silently watches less than they asked for,
// because the failure mode is money arriving at an address nobody reads.
func New(accounts []routing.Account, opts ...Option) (*Service, error) {
	b := &builder{
		store:    intentStore{},
		cursor:   cursorStore{},
		interval: defaultInterval,
		reader:   newReader,
	}
	for _, o := range opts {
		o(b)
	}

	s := &Service{store: b.store, cursor: b.cursor, interval: b.interval, reader: b.reader}
	st, err := s.build(context.Background(), accounts)
	if err != nil {
		return nil, err
	}
	s.st.Store(st)
	return s, nil
}

// build reads the accounts into a watch set. It fails closed on an incoherent table and
// changes nothing.
func (s *Service) build(ctx context.Context, accounts []routing.Account) (*state, error) {
	assets, err := depositwatch.AssetsFromAccounts(accounts)
	if err != nil {
		return nil, err
	}
	kp, krakenAssets := krakenFromAccounts(ctx, accounts)
	st := &state{assets: assets, kraken: kp}
	if len(assets) > 0 {
		bound := make([]depositwatch.Bound, 0, len(assets))
		for _, a := range assets {
			r, err := s.reader(a)
			if err != nil {
				return nil, err
			}
			bound = append(bound, depositwatch.Bound{Asset: a, Reader: r})
		}
		st.watcher = depositwatch.New(bound, s.store, s.cursor)
	}
	if len(krakenAssets) > 0 {
		existing := make(map[string]bool, len(st.assets))
		for _, a := range st.assets {
			existing[strings.ToLower(a.Chain)+":"+strings.ToLower(a.Token)] = true
		}
		for _, ka := range krakenAssets {
			if !existing[ka.Chain+":"+ka.Token] {
				st.assets = append(st.assets, ka)
			}
		}
	}
	return st, nil
}

// Apply makes accounts what is watched, now. A table that does not hold is refused and
// the previous one stays. A network whose account was switched off or removed stops
// being watched, one added starts, and the schedule keeps turning (or stays stopped) as
// it was: this changes WHAT is read, never whether the process is reading.
func (s *Service) Apply(ctx context.Context, accounts []routing.Account) error {
	if s == nil {
		return nil
	}
	st, err := s.build(ctx, accounts)
	if err != nil {
		return err
	}
	s.st.Store(st)
	return nil
}

func (s *Service) state() *state {
	if s == nil {
		return &state{}
	}
	if st := s.st.Load(); st != nil {
		return st
	}
	return &state{}
}

// Enabled reports whether any asset is being watched.
func (s *Service) Enabled() bool {
	st := s.state()
	return s != nil && (st.watcher != nil || st.kraken != nil)
}

// Assets reports the watched assets (boot log / diagnostics).
func (s *Service) Assets() []depositwatch.Asset {
	if s == nil {
		return nil
	}
	return s.state().assets
}

// Running reports whether the schedule is actually turning.
//
// It is a DIFFERENT question from Enabled, and the gap between them is the one
// that loses money: Enabled means an asset was configured, Running means
// something is reading the chain for it. A process that constructed the watcher
// and never started it is configured and blind — it would hand out custody
// addresses under a gate that only ever asked "is this configured?" and nothing
// would follow the money to a balance. So this, not Enabled, is what the mint
// gate consults (api/billing.watchedAssets).
func (s *Service) Running() bool {
	if st := s.state(); s == nil || (st.watcher == nil && st.kraken == nil) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancel != nil
}

// AssetStatus is one watched asset as an OPERATOR needs to see it: what is
// being watched, and how far the scan has got.
//
// LastBlock is the whole point. "Is the watcher on?" is answered by Running;
// the question immediately after it is "is it still moving?", and a cursor that
// is parked while the chain advances is the signature of every wedged pass —
// an RPC that errors every 30s, an ambiguous identity that fails the asset
// closed, a token whose symbol stopped matching. None of those are visible from
// "enabled: true".
type AssetStatus struct {
	Chain     string `json:"chain"`
	Token     string `json:"token"`
	Contract  string `json:"contract"`
	Endpoint  string `json:"endpoint"`                // scheme://host — see rpcHost
	Pooled    string `json:"pooledAddress,omitempty"` // the shared custody account, pooled chains only
	LastBlock uint64 `json:"lastBlock"`               // deepest block fully committed; 0 = never committed
}

// Status is the watcher's runtime state, as a value.
//
// It replaces a pair of fmt.Fprintf(os.Stderr, …) lines at boot. Those were
// unobservable in the only place it mattered — a running pod, where the line has
// long scrolled away and is not in the log at all — so the one fact an operator
// needed ("is this rail armed, and for what?") could not be established from the
// process that holds the answer. A boot log states a fact once; this states it
// whenever asked.
type Status struct {
	Running  bool          `json:"running"`
	Interval string        `json:"interval"`
	Assets   []AssetStatus `json:"assets"`
}

// Status reports what is watched and how far each asset has been scanned.
//
// Assets is never nil, so an unconfigured watcher renders `[]` rather than
// `null` — the same reason api/billing.sortedKeys never returns nil.
//
// A cursor read that fails is reported as block 0 rather than as an error: this
// is a diagnostic surface, and one unreadable cursor must not blank the answer
// to "is the watcher running", which is the question being asked.
func (s *Service) Status(ctx context.Context) Status {
	st := Status{Running: s.Running(), Interval: defaultInterval.String(), Assets: []AssetStatus{}}
	if s == nil {
		return st
	}
	for _, a := range s.state().assets {
		as := AssetStatus{
			Chain: a.Chain, Token: a.Token, Contract: a.Contract,
			Endpoint: rpcHost(a.RPCURL), Pooled: a.PooledAddress,
		}
		if s.cursor != nil {
			if last, err := s.cursor.Last(ctx, a.Key()); err == nil {
				as.LastBlock = last
			}
		}
		st.Assets = append(st.Assets, as)
	}
	return st
}

// rpcHost reduces a configured endpoint to scheme://host.
//
// The path and query are DROPPED, and that is a security requirement rather
// than tidiness: a managed node URL carries its API key in the path
// (…/v2/<key>) or the query, so echoing RPCURL verbatim onto an admin endpoint
// would publish a credential to every reader of this status. The host alone
// answers the operational question — which provider is this asset pointed at —
// and carries no secret.
func rpcHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "" // unparseable: say nothing rather than risk echoing a key
	}
	return u.Scheme + "://" + u.Host
}

// SyncOnce runs one pass. Idempotent, and safe to run concurrently with the
// schedule or with another replica.
func (s *Service) SyncOnce(ctx context.Context) (int, error) {
	if !s.Enabled() {
		return 0, nil
	}
	var total int
	st := s.state()
	if st.watcher != nil {
		n, err := st.watcher.Sync(ctx)
		total += n
		if err != nil {
			return total, err
		}
	}
	if st.kraken != nil {
		n, err := s.syncKraken(ctx, st)
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// Start begins the schedule.
//
// It runs IN PROCESS rather than as an external CronJob hitting an admin route,
// and that is the point: every other periodic job in this repo is an endpoint
// waiting for a caller that lives in another repository, which is precisely why
// the HUSD indexer has never once run. A rail that credits money must not depend
// on a manifest somebody remembers to write.
//
// Running on every replica needs no leader election because a pass is idempotent
// on the on-chain event (see creditKey): two replicas racing produce the same
// ledger row, not two. The only cost of a second replica is a second read of the
// same blocks.
func (s *Service) Start() {
	if s == nil {
		return
	}
	// The schedule turns whether or not anything is watched yet: an account the admin
	// adds later is picked up by the next pass, and a pass over nothing is a no-op.
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return // already running
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	s.cancel = cancel
	s.done = done
	// The channel is passed BY VALUE, never re-read from the field: Stop clears
	// the field as it hands off, so a loop that closed `s.done` would close nil.
	go s.loop(ctx, done)
}

// Stop ends the schedule and waits for the pass in flight to unwind.
func (s *Service) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.cancel, s.done = nil, nil
	s.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
}

func (s *Service) loop(ctx context.Context, done chan struct{}) {
	defer close(done)
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		// Sweep immediately on start: a restart must credit whatever confirmed
		// while the process was down, not wait out an interval first.
		s.tick(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Service) tick(ctx context.Context) {
	n, err := s.SyncOnce(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return // shutting down; not a failure
		}
		log.Error("depositledger: crypto deposit sync: %v", err)
	}
	if n > 0 {
		log.Info("depositledger: credited %d crypto deposit(s)", n)
	}
}

// --- process-wide default (set at boot) ---

var defaultService *Service

// SetDefault installs the process-wide watcher. Called once at Bootstrap.
func SetDefault(s *Service) { defaultService = s }

// Default returns the process-wide watcher (may be a disabled Service).
func Default() *Service { return defaultService }
