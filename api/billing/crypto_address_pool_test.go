package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hanzoai/commerce/billing/bucket"
	"github.com/hanzoai/commerce/billing/depositledger"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	txutil "github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/routing"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
	"github.com/hanzoai/money"
)

// fakePoolCustody implements processor.ManagedAccountProcessor and
// processor.DepositPoolProcessor with zero Kraken code, proving the core commerce
// layer abstracts deposit-address pooling, cross-org exclusivity, and post-tx
// rotation behind generic processor interfaces.
type fakePoolCustody struct {
	*processor.BaseProcessor
	mu       sync.Mutex
	pool     []string
	retired  map[string]bool
	seq      int
	deposits []processor.InboundDeposit
}

func newFakePoolCustody() *fakePoolCustody {
	f := &fakePoolCustody{
		BaseProcessor: processor.NewBaseProcessor(processor.OpenNode, []currency.Type{"usdc"}),
		retired:       make(map[string]bool),
	}
	for i := 1; i <= 5; i++ {
		f.seq = i
		f.pool = append(f.pool, fmt.Sprintf("0xfakepool%032x", i))
	}
	f.SetConfigured(true)
	return f
}

func (f *fakePoolCustody) SupportedChains() []string { return []string{"base"} }
func (f *fakePoolCustody) Charge(context.Context, processor.PaymentRequest) (*processor.PaymentResult, error) {
	return nil, errors.New("not supported")
}
func (f *fakePoolCustody) Authorize(context.Context, processor.PaymentRequest) (*processor.PaymentResult, error) {
	return nil, errors.New("not supported")
}
func (f *fakePoolCustody) Capture(context.Context, string, money.Amount) (*processor.PaymentResult, error) {
	return nil, errors.New("not supported")
}
func (f *fakePoolCustody) Refund(context.Context, processor.RefundRequest) (*processor.RefundResult, error) {
	return nil, errors.New("not supported")
}
func (f *fakePoolCustody) GetTransaction(context.Context, string) (*processor.Transaction, error) {
	return nil, errors.New("not supported")
}
func (f *fakePoolCustody) ValidateWebhook(context.Context, []byte, string) (*processor.WebhookEvent, error) {
	return nil, processor.ErrWebhookValidationFailed
}
func (f *fakePoolCustody) GetBalance(context.Context, string, string) (*processor.Balance, error) {
	return &processor.Balance{}, nil
}
func (f *fakePoolCustody) EstimateFee(context.Context, processor.PaymentRequest) (currency.Cents, error) {
	return 0, nil
}
func (f *fakePoolCustody) GenerateAddress(ctx context.Context, _ string, chain string) (processor.Wallet, error) {
	free := processor.AddressFreeFromContext(ctx)
	fresh := processor.AddressFreshFromContext(ctx)
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.pool {
		if f.retired[a] {
			continue
		}
		if fresh != nil && !fresh(a) {
			continue
		}
		if free != nil && !free(a) {
			continue
		}
		return processor.Wallet{Address: a, ID: "opennode:" + chain + ":" + a}, nil
	}
	return processor.Wallet{}, processor.NewPaymentError(f.Type(), "NO_FREE_ADDRESS", "no free address", nil)
}

func (f *fakePoolCustody) RequiredSecretRefs(secretRef string) []string {
	return []string{secretRef}
}
func (f *fakePoolCustody) ConfigureAccount(_ context.Context, _, secretRef string, read processor.SecretReader) bool {
	ok := read(context.Background(), secretRef) != ""
	f.SetConfigured(ok)
	return ok
}
func (f *fakePoolCustody) ClearAccount()                                     { f.SetConfigured(false) }
func (f *fakePoolCustody) SeedSecretEnvNames() []string                      { return nil }
func (f *fakePoolCustody) SeedAccountFromEnv(map[string]string, func(string) string) (processor.SeededAccount, bool, bool) {
	return processor.SeededAccount{}, false, false
}
func (f *fakePoolCustody) OwnsWallet(walletID string) bool {
	return strings.HasPrefix(walletID, "opennode:")
}
func (f *fakePoolCustody) PoolLeaseTerms() (time.Duration, time.Duration) {
	return 20 * time.Minute, 10 * time.Minute
}
func (f *fakePoolCustody) WatchedPairs() ([]processor.CryptoAssetPair, string) {
	return []processor.CryptoAssetPair{{Chain: "base", Token: "usdc"}}, "https://custody.example"
}
func (f *fakePoolCustody) WarmPools(context.Context, []processor.CryptoAssetPair) error { return nil }
func (f *fakePoolCustody) ReplenishPools(context.Context) error                         { return nil }
func (f *fakePoolCustody) RetireAddress(_ context.Context, _, _, address string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.retired[strings.ToLower(address)] = true
	f.seq++
	f.pool = append(f.pool, fmt.Sprintf("0xfakepool%032x", f.seq))
	return nil
}
func (f *fakePoolCustody) ListInboundDeposits(context.Context, string, string) ([]processor.InboundDeposit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]processor.InboundDeposit, len(f.deposits))
	copy(cp, f.deposits)
	return cp, nil
}
func (f *fakePoolCustody) CachedMinimumCents(string, string) int64 { return 150 }

func TestDepositPoolProcessorAbstraction_ProviderAgnosticEndToEnd(t *testing.T) {
	c := ae.NewContext()
	defer c.Close()

	prev, _ := processor.Global().Registered(processor.OpenNode)
	fake := newFakePoolCustody()
	processor.Register(fake)
	t.Cleanup(func() {
		if prev != nil {
			processor.Register(prev)
		}
	})

	useSecrets(t, mapSecrets{"/secrets/opennode": "tok"})
	acct := routing.Account{Processor: string(processor.OpenNode), ID: "pool", SecretRef: "/secrets/opennode", On: true}
	pol := routing.Default()
	pol.Processors = append(pol.Processors, routing.ProcessorRule{Processor: string(processor.OpenNode), On: true})
	pol.Accounts = []routing.Account{acct}
	pol.CryptoRails = []routing.CryptoRoute{{Asset: "usdc", Processor: string(processor.OpenNode)}}
	if _, flt := WriteRouting(c, "", pol, "test", ""); flt != nil {
		t.Fatalf("WriteRouting: %v", flt.Err)
	}

	svc, err := depositledger.New([]routing.Account{acct})
	if err != nil {
		t.Fatalf("depositledger.New: %v", err)
	}
	svc.Start()
	depositledger.SetDefault(svc)
	t.Cleanup(func() {
		svc.Stop()
		depositledger.SetDefault(nil)
	})

	org1 := organization.New(datastore.New(c))
	org1.Name = "abstract-org-1"
	org1.Live = true
	if err := org1.Create(); err != nil {
		t.Fatal(err)
	}
	org2 := organization.New(datastore.New(c))
	org2.Name = "abstract-org-2"
	org2.Live = true
	if err := org2.Create(); err != nil {
		t.Fatal(err)
	}

	dep1, err := CreateCryptoDeposit(context.Background(), org1, "abstract-org-1/u1", "base", "usdc", 1000)
	if err != nil {
		t.Fatalf("dep1: %v", err)
	}
	dep2, err := CreateCryptoDeposit(context.Background(), org2, "abstract-org-2/u2", "base", "usdc", 1000)
	if err != nil {
		t.Fatalf("dep2: %v", err)
	}
	if dep1.DepositAddress == dep2.DepositAddress {
		t.Fatalf("collision across orgs: %s", dep1.DepositAddress)
	}
	if dep1.MinimumCents != 150 {
		t.Fatalf("minimumCents = %d, want 150 from DepositPoolProcessor", dep1.MinimumCents)
	}

	fake.mu.Lock()
	fake.deposits = append(fake.deposits, processor.InboundDeposit{
		RefID:     "REF-ABS-1",
		TxHash:    "0xabstracttx1",
		Address:   dep1.DepositAddress,
		Amount:    "10.00",
		Timestamp: time.Now().Unix(),
		Confirmed: true,
	})
	fake.mu.Unlock()

	if n, err := svc.SyncOnce(context.Background()); err != nil || n != 1 {
		t.Fatalf("SyncOnce = (%d, %v), want (1, nil)", n, err)
	}

	fake.mu.Lock()
	wasRetired := fake.retired[strings.ToLower(dep1.DepositAddress)]
	poolLen := len(fake.pool)
	fake.mu.Unlock()
	if !wasRetired || poolLen != 6 {
		t.Fatalf("expected RetireAddress rotation on generic DepositPoolProcessor (retired=%v poolLen=%d)", wasRetired, poolLen)
	}

	org1NS := datastore.New(nscontext.WithNamespace(context.Background(), org1.Name))
	raw, err := txutil.GetRawByCurrency(org1NS.Context, "abstract-org-1/u1", "iam-user", "usd", false)
	if err != nil {
		t.Fatal(err)
	}
	if got := int64(bucket.Compute(raw, "abstract-org-1/u1", time.Now()).Balance); got != 1000 {
		t.Fatalf("balance = %d, want 1000", got)
	}
}
