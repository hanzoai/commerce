// Copyright © 2026 Hanzo AI. MIT License.

package risk

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hanzoai/commerce/models/dispute"
	"github.com/hanzoai/commerce/models/paymentintent"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestAssemble_CarriesTheJudgementThatAdmittedTheCharge — the part of a defence
// only we can produce.
func TestAssemble_CarriesTheJudgementThatAdmittedTheCharge(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("evidence", ctx, &oracle{answer: &Decision{ID: "d1", Action: Allow, Score: 0.11, Agency: "human"}})

	pi := paymentintent.New(s.DB)
	pi.CustomerId = "c1"
	pi.Amount = 4200
	pi.Currency = currency.USD
	pi.Status = paymentintent.Succeeded
	pi.ReceiptEmail = "buyer@example.com"
	pi.Description = "one annual seat"
	pi.ProviderType = "square"
	pi.ProviderRef = "sq_ch_1"
	pi.MustCreate()

	if _, err := s.Screen(context.Background(), Move{
		Stage: Payment, Subject: customer("c1"), Amount: 4200, Currency: currency.USD, Reference: pi.Id(),
	}); err != nil {
		t.Fatalf("screen: %v", err)
	}

	d := dispute.New(s.DB)
	d.PaymentIntentId = pi.Id()
	d.Amount = 4200
	d.Currency = currency.USD
	d.Reason = "product_not_received"
	d.MustCreate()

	e, err := Assemble(s.DB, d.Id())
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if e.Charge == nil || e.Charge.ID != pi.Id() {
		t.Fatalf("the charge was not found: %+v", e.Charge)
	}
	if len(e.Screens) != 1 || e.Screens[0].Decision != "d1" {
		t.Fatalf("the judgement was not cited: %+v", e.Screens)
	}
	if e.Fields.CustomerEmailAddress != "buyer@example.com" || e.Fields.ProductDescription != "one annual seat" {
		t.Fatalf("the submission fields were not filled from the record: %+v", e.Fields)
	}
	if !strings.Contains(e.Fields.UncategorizedText, "42.00") {
		t.Fatalf("the narrative does not state the exact amount: %q", e.Fields.UncategorizedText)
	}
	if !strings.Contains(e.Fields.UncategorizedText, "decision d1") {
		t.Fatalf("the narrative does not cite the decision: %q", e.Fields.UncategorizedText)
	}
}

// TestAssemble_NamesWhatItCannotFind — a blank evidence field looks like a fact
// that was checked and found empty; a stated gap tells the merchant what to add.
func TestAssemble_NamesWhatItCannotFind(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	s := tenant("gaps", ctx, &oracle{answer: &Decision{Action: Allow}})
	d := dispute.New(s.DB)
	d.Amount = 100
	d.Currency = currency.USD
	d.MustCreate()

	e, err := Assemble(s.DB, d.Id())
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	joined := strings.Join(e.Gaps, " | ")
	for _, want := range []string{"payment intent", "never screened", "customerName"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("gaps %q do not name %q", joined, want)
		}
	}
}

// TestAssemble_IsTenantScoped — a dispute id from another org simply is not
// found; the boundary is the store's, not a check this function could forget.
func TestAssemble_IsTenantScoped(t *testing.T) {
	ctx := ae.NewContext()
	defer ctx.Close()

	a := tenant("evida", ctx, &oracle{answer: &Decision{Action: Allow}})
	b := tenant("evidb", ctx, &oracle{answer: &Decision{Action: Allow}})

	d := dispute.New(a.DB)
	d.Amount = 100
	d.Currency = currency.USD
	d.MustCreate()

	if _, err := Assemble(b.DB, d.Id()); err == nil {
		t.Fatalf("org B assembled org A's dispute %s", d.Id())
	}
}

// TestSubmit_RefusesBecauseThereIsNoAdjudicator states the honest product gap:
// no provider implements DisputeProcessor and we belong to no dispute network,
// so a submission returns a stated refusal rather than a success that went
// nowhere. When a real adjudicator lands, this test changes with it.
func TestSubmit_RefusesBecauseThereIsNoAdjudicator(t *testing.T) {
	reg := processor.NewRegistry(processor.DefaultConfig())
	reg.Register(&gateway{})

	_, err := Submit(context.Background(), reg, "square", "dp_1", &Evidence{})
	if !errors.Is(err, ErrNoAdjudicator) {
		t.Fatalf("err=%v want ErrNoAdjudicator", err)
	}
	if _, err := Submit(context.Background(), nil, "square", "dp_1", &Evidence{}); !errors.Is(err, ErrNoAdjudicator) {
		t.Fatalf("no registry: err=%v want ErrNoAdjudicator", err)
	}
	if _, err := Submit(context.Background(), reg, "square", "", &Evidence{}); !errors.Is(err, ErrNoAdjudicator) {
		t.Fatalf("no dispute ref: err=%v want ErrNoAdjudicator", err)
	}
}

// adjudicator is a processor that CAN carry a submission — the seam a real
// integration fills. It exists to prove the seam is reachable, not to claim we
// have one.
type adjudicator struct {
	gateway
	got map[string]string
}

func (a *adjudicator) SubmitEvidence(_ context.Context, ref string, fields map[string]string) (*processor.DisputeReceipt, error) {
	a.got = fields
	return &processor.DisputeReceipt{Reference: ref, Status: "under_review", Accepted: true}, nil
}

func TestSubmit_ReachesAnAdjudicatorThatImplementsTheSeam(t *testing.T) {
	reg := processor.NewRegistry(processor.DefaultConfig())
	adj := &adjudicator{}
	reg.Register(adj)

	e := &Evidence{}
	e.Fields.CustomerName = "A Buyer"
	e.Fields.UncategorizedText = "the goods shipped"

	receipt, err := Submit(context.Background(), reg, "square", "dp_1", e)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !receipt.Accepted || receipt.Reference != "dp_1" {
		t.Fatalf("receipt=%+v", receipt)
	}
	if adj.got["customer_name"] != "A Buyer" || adj.got["uncategorized_text"] != "the goods shipped" {
		t.Fatalf("the packet was not rendered into the network's field names: %v", adj.got)
	}
}

// TestMinor_RendersExactlyWithoutAFloat.
func TestMinor_RendersExactlyWithoutAFloat(t *testing.T) {
	cases := map[int64]string{
		0: "0.00", 1: "0.01", 9: "0.09", 10: "0.10", 100: "1.00",
		4200: "42.00", 123456: "1234.56", -5: "-0.05",
		999999999999999: "9999999999999.99",
	}
	for cents, want := range cases {
		if got := minor(cents); got != want {
			t.Fatalf("minor(%d)=%q want %q", cents, got, want)
		}
	}
}
