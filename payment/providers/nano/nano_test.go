package nano

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/payment/providers/nano/nanoverify"
)

const (
	merchant = "nano_3t6k35gi95xu6tergt6p69ck76ogmitsa8mnijtpxm9fkcm736xtoncuohr3"
	payer    = "nano_1111111111111111111111111111111111111111111111111111hifc8npp"
	blockA   = "991CF190094C00F0B68E2E5F75F6BEE95A2E0BD93CEAA4A6734DB9F19B728948"
	blockB   = "A170D51B94E00371ACE76E35AC81DC9405D5D04D4CEBC399AEACE07AE05DD293"
)

// node answers block_info with a confirmed send of amounts[hash] to merchant.
func node(t *testing.T, amounts map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)
		switch req["action"] {
		case "block_info":
			amt, ok := amounts[req["hash"]]
			if !ok {
				json.NewEncoder(w).Encode(map[string]string{"error": "Block not found"})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"block_account": payer, "amount": amt, "confirmed": "true", "subtype": "send",
				"contents": map[string]string{"type": "state", "link_as_account": merchant},
			})
		case "receivable":
			blocks := map[string]interface{}{}
			for h, a := range amounts {
				blocks[h] = map[string]string{"amount": a, "source": payer}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"blocks": blocks})
		case "account_history":
			json.NewEncoder(w).Encode(map[string]interface{}{"history": []interface{}{}})
		default:
			json.NewEncoder(w).Encode(map[string]string{"error": "unexpected"})
		}
	}))
}

func TestOffUnlessConfigured(t *testing.T) {
	ctx := context.Background()
	for _, cfg := range []Config{
		{},
		{RPCURL: "http://node", Recipient: merchant},
		{RPCURL: "http://node", ClaimDir: t.TempDir()},
		{Recipient: merchant, ClaimDir: t.TempDir()},
		{RPCURL: "http://node", Recipient: merchant[:len(merchant)-1] + "4", ClaimDir: t.TempDir()},
	} {
		p := NewProvider(cfg)
		if p.IsAvailable(ctx) {
			t.Errorf("%+v: available without full configuration", cfg)
		}
		if _, err := p.Charge(ctx, processor.PaymentRequest{Amount: 1, Currency: currency.XNO}); err == nil {
			t.Errorf("%+v: charged without full configuration", cfg)
		}
	}
	p := NewProvider(Config{RPCURL: "http://node", Recipient: merchant, ClaimDir: t.TempDir()})
	if !p.IsAvailable(ctx) || p.Type() != processor.Nano {
		t.Fatal("fully configured provider is not available")
	}
}

func TestChargeIsExactAndBound(t *testing.T) {
	ctx := context.Background()
	p := NewProvider(Config{RPCURL: "http://node", Recipient: merchant, ClaimDir: t.TempDir()})
	if _, err := p.Charge(ctx, processor.PaymentRequest{Amount: 100, Currency: currency.USD}); err == nil {
		t.Fatal("charged USD")
	}
	a, err := p.Charge(ctx, processor.PaymentRequest{Amount: 1_500_000, Currency: currency.XNO})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := p.Charge(ctx, processor.PaymentRequest{Amount: 1_500_000, Currency: currency.XNO})
	if a.TransactionID == b.TransactionID {
		t.Fatal("two orders of the same price share one amount")
	}
	// 1.5 XNO = 15 followed by 29 digits; the tag sits below one micro-XNO.
	if len(a.TransactionID) != 31 || !strings.HasPrefix(a.TransactionID, "1500000") {
		t.Fatalf("amount_raw = %s", a.TransactionID)
	}
	if a.Metadata["nano_uri"] != "nano:"+merchant+"?amount="+a.TransactionID || a.Metadata["network"] != "nano:mainnet" || a.Metadata["scheme"] != "exact" {
		t.Fatalf("metadata = %v", a.Metadata)
	}
}

func TestWebhookAcceptsOnceAndRefusesReplay(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	p := NewProvider(Config{RPCURL: "http://unset", Recipient: merchant, ClaimDir: dir})
	order1, _ := p.Charge(ctx, processor.PaymentRequest{Amount: 2_000_000, Currency: currency.XNO})
	order2, _ := p.Charge(ctx, processor.PaymentRequest{Amount: 2_000_000, Currency: currency.XNO})
	srv := node(t, map[string]string{blockA: order1.TransactionID, blockB: "2000000000000000000000000000000"})
	defer srv.Close()
	p = NewProvider(Config{RPCURL: srv.URL, Recipient: merchant, ClaimDir: dir})

	body := func(block, tx string) []byte {
		b, _ := json.Marshal(map[string]string{"block": block, "transactionId": tx})
		return b
	}
	ev, err := p.ValidateWebhook(ctx, body(blockA, order1.TransactionID), "")
	if err != nil || ev.Type != "payment.confirmed" || ev.Data["payer"] != payer {
		t.Fatalf("good payment: %+v %v", ev, err)
	}
	// Same order again (a retried delivery): not booked twice.
	ev, err = p.ValidateWebhook(ctx, body(blockA, order1.TransactionID), "")
	if err != nil || ev.Type != "payment.duplicate" {
		t.Fatalf("duplicate: %+v %v", ev, err)
	}
	// Block A cannot settle order 2: its amount is order 1's.
	if _, err := p.ValidateWebhook(ctx, body(blockA, order2.TransactionID), ""); !errors.Is(err, nanoverify.ErrWrongValue) {
		t.Fatalf("cross-order: %v", err)
	}
	// A round 2 XNO send without the order's tag is not order 2's payment.
	if _, err := p.ValidateWebhook(ctx, body(blockB, order2.TransactionID), ""); !errors.Is(err, nanoverify.ErrWrongValue) {
		t.Fatalf("untagged amount: %v", err)
	}
	if _, err := p.ValidateWebhook(ctx, body(strings.Repeat("C", 64), order1.TransactionID), ""); !errors.Is(err, processor.ErrTransactionNotFound) {
		t.Fatalf("unknown block: %v", err)
	}
	if _, err := p.ValidateWebhook(ctx, []byte("not json"), ""); err == nil {
		t.Fatal("garbage accepted")
	}

	// After a restart the claim is still on disk: a forged second claim of
	// block A, as if it paid an order with the same amount, is refused.
	restarted := NewProvider(Config{RPCURL: srv.URL, Recipient: merchant, ClaimDir: dir})
	if _, err := (nanoverify.FileClaims{Dir: dir}).Claim(blockA, "another-order"); !errors.Is(err, nanoverify.ErrReplayed) {
		t.Fatalf("claim forgotten across restart: %v", err)
	}
	ev, err = restarted.ValidateWebhook(ctx, body(blockA, order1.TransactionID), "")
	if err != nil || ev.Type != "payment.duplicate" {
		t.Fatalf("after restart: %+v %v", ev, err)
	}
}

func TestGetTransaction(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	p := NewProvider(Config{RPCURL: "http://unset", Recipient: merchant, ClaimDir: dir})
	paid, _ := p.Charge(ctx, processor.PaymentRequest{Amount: 3, Currency: currency.XNO})
	open, _ := p.Charge(ctx, processor.PaymentRequest{Amount: 3, Currency: currency.XNO})
	srv := node(t, map[string]string{blockA: paid.TransactionID})
	defer srv.Close()
	p = NewProvider(Config{RPCURL: srv.URL, Recipient: merchant, ClaimDir: dir})

	tx, err := p.GetTransaction(ctx, paid.TransactionID)
	if err != nil || tx.Status != "confirmed" || tx.ProcessorRef != blockA || tx.Amount != 3 {
		t.Fatalf("paid: %+v %v", tx, err)
	}
	tx, err = p.GetTransaction(ctx, open.TransactionID)
	if err != nil || tx.Status != "pending" {
		t.Fatalf("open: %+v %v", tx, err)
	}
	if _, err := p.GetTransaction(ctx, "abc"); !errors.Is(err, processor.ErrTransactionNotFound) {
		t.Fatalf("bad id: %v", err)
	}
}

func TestNoRefundNoFee(t *testing.T) {
	ctx := context.Background()
	p := NewProvider(Config{RPCURL: "http://node", Recipient: merchant, ClaimDir: t.TempDir()})
	if _, err := p.Refund(ctx, processor.RefundRequest{}); err == nil {
		t.Fatal("refund reported as possible")
	}
	if fee, _ := p.EstimateFee(ctx, processor.PaymentRequest{}); fee != 0 {
		t.Fatalf("fee = %d", fee)
	}
	if w, err := p.GenerateAddress(ctx, "c", "nano"); err != nil || w.Address != merchant {
		t.Fatalf("address: %+v %v", w, err)
	}
	if _, err := p.GenerateAddress(ctx, "c", "solana"); err == nil {
		t.Fatal("other chain accepted")
	}
}
