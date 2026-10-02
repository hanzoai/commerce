package billing

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentorg"
	"github.com/hanzoai/commerce/util/test/ae"
)

// A dispute moves money only when Square withholds the funds — a chargeback, or
// one lost — and every report of it is ordered by the dispute's own version, so a
// retried earlier report changes nothing.

// disputeAt is a dispute delivery carrying the dispute's version and updated_at,
// as Square sends them; version 0 and an empty updated_at leave each out.
func disputeAt(eventID, typ, disputeID, paymentID, state string, cents int64, version int, updatedAt string) []byte {
	order := ""
	if version > 0 {
		order += fmt.Sprintf(`"version":%d,`, version)
	}
	if updatedAt != "" {
		order += fmt.Sprintf(`"updated_at":%q,`, updatedAt)
	}
	return []byte(fmt.Sprintf(`{"merchant_id":"M1","type":%q,"event_id":%q,"created_at":%q,`+
		`"data":{"type":"dispute","id":%q,"object":{"dispute":{"id":%q,%s"state":%q,"disputed_payment":{"payment_id":%q},`+
		`"amount_money":{"amount":%d,"currency":"USD"}}}}}`, typ, eventID, time.Now().UTC().Format(time.RFC3339), disputeID, disputeID, order, state, paymentID, cents))
}

// invoiceOf is the paid invoice the subject's subscription holds.
func invoiceOf(t *testing.T, ctx ae.Context, org *organization.Organization, subject string) *billinginvoice.BillingInvoice {
	t.Helper()
	db := datastore.New(org.Namespaced(ctx))
	sub := subsOf(t, db, subject)[0]
	inv := billinginvoice.New(db)
	if err := inv.GetById(sub.CurrentInvoiceId); err != nil {
		t.Fatalf("load invoice: %v", err)
	}
	return inv
}

// Square's current name for a dispute's change of state is dispute.state.updated;
// a chargeback won under it restores the period, as under the older name.
func TestWebhook_DisputeStateUpdatedReachesThePeriod(t *testing.T) {
	const secret = "whsec_dp_updated"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "dp-updated", true)
	withFakeSquare(t, squareMock("cust_du", "ccof_du", "sqpay_du"))
	subscribed(t, ctx, org, "dp-updated", "dev", 1, nil)
	price := lookupPlan("dev").Price

	deliver(t, ctx, secret, disputeAt("evt_du_0", "dispute.created", "dp_du", "sqpay_du", "EVIDENCE_REQUIRED", price, 1, ""))
	paidAs(t, viewOf(t, ctx, org, "dp-updated"), "", 0, 0)
	deliver(t, ctx, secret, disputeAt("evt_du_1", "dispute.state.updated", "dp_du", "sqpay_du", "WON", price, 2, ""))
	paidAs(t, viewOf(t, ctx, org, "dp-updated"), "card", 1, price)
}

// An inquiry holds no funds, so it leaves the period paid; the invoice records it.
// A chargeback it escalates to unpays the period.
func TestWebhook_AnInquiryLeavesItsPeriodPaid(t *testing.T) {
	const secret = "whsec_dp_inquiry"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "dp-inquiry", true)
	withFakeSquare(t, squareMock("cust_di", "ccof_di", "sqpay_di"))
	subscribed(t, ctx, org, "dp-inquiry", "dev", 1, nil)
	price := lookupPlan("dev").Price

	deliver(t, ctx, secret, disputeAt("evt_di_0", "dispute.created", "dp_di", "sqpay_di", "INQUIRY_EVIDENCE_REQUIRED", price, 1, ""))
	paidAs(t, viewOf(t, ctx, org, "dp-inquiry"), "card", 1, price)
	if inv := invoiceOf(t, ctx, org, "dp-inquiry"); inv.Dispute != "INQUIRY_EVIDENCE_REQUIRED" {
		t.Fatalf("invoice dispute = %q; want the inquiry recorded", inv.Dispute)
	}
	deliver(t, ctx, secret, disputeAt("evt_di_1", "dispute.state.updated", "dp_di", "sqpay_di", "EVIDENCE_REQUIRED", price, 2, ""))
	paidAs(t, viewOf(t, ctx, org, "dp-inquiry"), "", 0, 0)
}

// A report older than the one the invoice holds changes nothing: an inquiry that
// escalated to a chargeback stays a chargeback when the retried INQUIRY_CLOSED lands,
// ordered by version, or by updated_at where no version is sent.
func TestWebhook_ALateDisputeReportChangesNothing(t *testing.T) {
	const secret = "whsec_dp_late"
	registerSquare(t, secret)
	ctx := ae.NewContext()
	defer ctx.Close()
	org := webhookOrg(t, ctx, "dp-late", true)
	price := lookupPlan("dev").Price

	withFakeSquare(t, squareMock("cust_dv", "ccof_dv", "sqpay_dv"))
	subscribed(t, ctx, org, "dp-late-version", "dev", 1, nil)
	deliver(t, ctx, secret,
		disputeAt("evt_dv_0", "dispute.created", "dp_dv", "sqpay_dv", "INQUIRY_EVIDENCE_REQUIRED", price, 1, ""),
		disputeAt("evt_dv_2", "dispute.state.updated", "dp_dv", "sqpay_dv", "EVIDENCE_REQUIRED", price, 3, ""),
		disputeAt("evt_dv_1", "dispute.state.updated", "dp_dv", "sqpay_dv", "INQUIRY_CLOSED", price, 2, ""))
	paidAs(t, viewOf(t, ctx, org, "dp-late-version"), "", 0, 0)

	withFakeSquare(t, squareMock("cust_dt", "ccof_dt", "sqpay_dt"))
	subscribed(t, ctx, org, "dp-late-time", "dev", 1, nil)
	deliver(t, ctx, secret,
		disputeAt("evt_dt_0", "dispute.created", "dp_dt", "sqpay_dt", "INQUIRY_EVIDENCE_REQUIRED", price, 0, "2026-09-01T10:00:00Z"),
		disputeAt("evt_dt_2", "dispute.state.updated", "dp_dt", "sqpay_dt", "EVIDENCE_REQUIRED", price, 0, "2026-09-03T10:00:00.250Z"),
		disputeAt("evt_dt_1", "dispute.state.updated", "dp_dt", "sqpay_dt", "INQUIRY_CLOSED", price, 0, "2026-09-02T10:00:00Z"))
	paidAs(t, viewOf(t, ctx, org, "dp-late-time"), "", 0, 0)
	if inv := invoiceOf(t, ctx, org, "dp-late-time"); inv.Dispute != "EVIDENCE_REQUIRED" || inv.DisputeAt.IsZero() {
		t.Fatalf("invoice dispute = %q at %s; want the chargeback and when it was reported", inv.Dispute, inv.DisputeAt)
	}
}

// walletPayment settles a top-up whose dispute the tests then report.
func walletPayment(t *testing.T, ctx ae.Context, secret, orgName, sub, payment string) {
	t.Helper()
	org := webhookOrg(t, ctx, orgName, true)
	seedProviderSubscription(t, org, ctx, sub, orgName+"/alice")
	if r := deliverWebhook(ctx, orgName, secret, renewalEvent("evt_"+payment, payment, sub, 5000), ""); r.StatusCode != http.StatusOK {
		t.Fatalf("settle: %d", r.StatusCode)
	}
}

// A wallet top-up's inquiry takes nothing back, opening or closing: Square holds no
// funds for one. A chargeback takes the money back once, and a chargeback won gives
// it back.
func TestWebhook_AWalletInquiryTakesNothing(t *testing.T) {
	const secret = "whsec_dp_wallet"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	walletPayment(t, ctx, secret, "dp-wallet", "sub_dw", "pay_dw")

	deliver(t, ctx, secret,
		disputeAt("evt_dw_0", "dispute.created", "dp_dw", "pay_dw", "INQUIRY_EVIDENCE_REQUIRED", 5000, 1, ""),
		disputeAt("evt_dw_1", "dispute.state.updated", "dp_dw", "pay_dw", "INQUIRY_CLOSED", 5000, 2, ""))
	if len(fake.clawbacks) != 0 || len(fake.restores) != 0 {
		t.Fatalf("an inquiry took %d and gave back %d; want neither", len(fake.clawbacks), len(fake.restores))
	}

	walletPayment(t, ctx, secret, "dp-wallet", "sub_dc", "pay_dc")
	deliver(t, ctx, secret,
		disputeAt("evt_dc_0", "dispute.created", "dp_dc", "pay_dc", "INQUIRY_EVIDENCE_REQUIRED", 5000, 1, ""),
		disputeAt("evt_dc_1", "dispute.state.updated", "dp_dc", "pay_dc", "EVIDENCE_REQUIRED", 5000, 2, ""),
		disputeAt("evt_dc_2", "dispute.state.updated", "dp_dc", "pay_dc", "PROCESSING", 5000, 3, ""),
		disputeAt("evt_dc_3", "dispute.state.updated", "dp_dc", "pay_dc", "WON", 5000, 4, ""))
	if len(fake.clawbacks) != 1 || fake.clawbacks[0].Ref != "dispute:dp_dc" || fake.clawbacks[0].AmountCents != 5000 {
		t.Fatalf("clawbacks = %+v; want the chargeback taken once", fake.clawbacks)
	}
	if len(fake.restores) != 1 || fake.restores[0].Ref != "dispute:dp_dc" {
		t.Fatalf("restores = %+v; want the chargeback won given back once", fake.restores)
	}
}

// A wallet dispute's late report changes nothing: a chargeback won stays won when
// the retried chargeback report lands, so nothing is taken back after Square
// returned the funds.
func TestWebhook_ALateWalletDisputeReportChangesNothing(t *testing.T) {
	const secret = "whsec_dp_wlate"
	registerSquare(t, secret)
	fake := newFakeLedger()
	injectLedger(t, fake)
	ctx := ae.NewContext()
	defer ctx.Close()
	walletPayment(t, ctx, secret, "dp-wlate", "sub_dl", "pay_dl")

	deliver(t, ctx, secret,
		disputeAt("evt_dl_0", "dispute.created", "dp_dl", "pay_dl", "INQUIRY_EVIDENCE_REQUIRED", 5000, 1, ""),
		disputeAt("evt_dl_2", "dispute.state.updated", "dp_dl", "pay_dl", "WON", 5000, 3, ""),
		disputeAt("evt_dl_1", "dispute.state.updated", "dp_dl", "pay_dl", "EVIDENCE_REQUIRED", 5000, 2, ""))
	if len(fake.clawbacks) != 0 {
		t.Fatalf("clawbacks = %+v; a report older than the dispute won took money back", fake.clawbacks)
	}
	if rec, found, err := paymentorg.Get("pay_dl"); err != nil || !found || rec.Dispute != "WON" || rec.DisputeVersion != 3 {
		t.Fatalf("payment record = %+v (found %v, %v); want the dispute won at version 3", rec, found, err)
	}
}
