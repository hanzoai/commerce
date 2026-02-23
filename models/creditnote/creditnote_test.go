package creditnote

import (
	"testing"
)

// --- MarkVoid ---

func TestMarkVoid_FromIssued(t *testing.T) {
	cn := &CreditNote{Status: Issued}
	if err := cn.MarkVoid(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cn.Status != Void {
		t.Errorf("expected %s, got %s", Void, cn.Status)
	}
}

func TestMarkVoid_InvalidStatus_Void(t *testing.T) {
	cn := &CreditNote{Status: Void}
	err := cn.MarkVoid()
	if err == nil {
		t.Fatal("expected error voiding already-void credit note")
	}
}

func TestMarkVoid_InvalidStatus_Empty(t *testing.T) {
	cn := &CreditNote{Status: ""}
	err := cn.MarkVoid()
	if err == nil {
		t.Fatal("expected error voiding credit note with empty status")
	}
}

// --- SetNumber ---

func TestSetNumber_SingleDigit(t *testing.T) {
	cn := &CreditNote{}
	cn.SetNumber(1)
	if cn.Number != "CN-0001" {
		t.Errorf("expected CN-0001, got %s", cn.Number)
	}
}

func TestSetNumber_DoubleDigit(t *testing.T) {
	cn := &CreditNote{}
	cn.SetNumber(42)
	if cn.Number != "CN-0042" {
		t.Errorf("expected CN-0042, got %s", cn.Number)
	}
}

func TestSetNumber_FourDigit(t *testing.T) {
	cn := &CreditNote{}
	cn.SetNumber(9999)
	if cn.Number != "CN-9999" {
		t.Errorf("expected CN-9999, got %s", cn.Number)
	}
}

func TestSetNumber_FiveDigit(t *testing.T) {
	cn := &CreditNote{}
	cn.SetNumber(12345)
	if cn.Number != "CN-12345" {
		t.Errorf("expected CN-12345, got %s", cn.Number)
	}
}

func TestSetNumber_Zero(t *testing.T) {
	cn := &CreditNote{}
	cn.SetNumber(0)
	if cn.Number != "CN-0000" {
		t.Errorf("expected CN-0000, got %s", cn.Number)
	}
}

func TestSetNumber_Overwrites(t *testing.T) {
	cn := &CreditNote{Number: "CN-0001"}
	cn.SetNumber(2)
	if cn.Number != "CN-0002" {
		t.Errorf("expected CN-0002, got %s", cn.Number)
	}
}

// --- Status constants ---

func TestStatusConstants(t *testing.T) {
	cases := []struct {
		status Status
		want   string
	}{
		{Issued, "issued"},
		{Void, "void"},
	}
	for _, tc := range cases {
		if string(tc.status) != tc.want {
			t.Errorf("status %q != %q", tc.status, tc.want)
		}
	}
}

// --- Struct fields ---

func TestCreditNoteZeroValue(t *testing.T) {
	cn := &CreditNote{}
	if cn.Amount != 0 {
		t.Errorf("expected zero amount, got %d", cn.Amount)
	}
	if cn.Status != "" {
		t.Errorf("expected empty status, got %s", cn.Status)
	}
	if cn.LineItems != nil {
		t.Error("expected nil line items")
	}
	if cn.Metadata != nil {
		t.Error("expected nil metadata")
	}
}

func TestCreditNoteFieldAssignment(t *testing.T) {
	cn := &CreditNote{
		InvoiceId:  "inv_123",
		CustomerId: "cus_456",
		Amount:     5000,
		Currency:   "usd",
		Status:     Issued,
		Reason:     "duplicate",
		Memo:       "Refund for duplicate charge",
	}
	if cn.InvoiceId != "inv_123" {
		t.Errorf("expected inv_123, got %s", cn.InvoiceId)
	}
	if cn.CustomerId != "cus_456" {
		t.Errorf("expected cus_456, got %s", cn.CustomerId)
	}
	if cn.Amount != 5000 {
		t.Errorf("expected 5000, got %d", cn.Amount)
	}
	if cn.Reason != "duplicate" {
		t.Errorf("expected 'duplicate', got %q", cn.Reason)
	}
}

// --- LineItems ---

func TestCreditNoteLineItems(t *testing.T) {
	cn := &CreditNote{
		Status: Issued,
		LineItems: []CreditNoteLineItem{
			{Description: "Widget", Amount: 2500, Currency: "usd", Quantity: 1, UnitPrice: 2500},
			{Description: "Gadget", Amount: 5000, Currency: "usd", Quantity: 2, UnitPrice: 2500},
		},
	}
	if len(cn.LineItems) != 2 {
		t.Fatalf("expected 2 line items, got %d", len(cn.LineItems))
	}
	if cn.LineItems[0].Description != "Widget" {
		t.Errorf("expected Widget, got %s", cn.LineItems[0].Description)
	}
	if cn.LineItems[1].Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", cn.LineItems[1].Quantity)
	}
}

// --- Full lifecycle ---

func TestFullLifecycle_IssueAndVoid(t *testing.T) {
	cn := &CreditNote{
		Status:     Issued,
		Amount:     3000,
		InvoiceId:  "inv_789",
		CustomerId: "cus_abc",
	}
	cn.SetNumber(5)
	if cn.Number != "CN-0005" {
		t.Errorf("expected CN-0005, got %s", cn.Number)
	}
	if err := cn.MarkVoid(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cn.Status != Void {
		t.Errorf("expected Void, got %s", cn.Status)
	}
}

func TestFullLifecycle_VoidedCannotVoidAgain(t *testing.T) {
	cn := &CreditNote{Status: Issued}
	if err := cn.MarkVoid(); err != nil {
		t.Fatalf("first void: %v", err)
	}
	err := cn.MarkVoid()
	if err == nil {
		t.Fatal("expected error voiding already-void credit note")
	}
}

// --- Kind ---

func TestKind(t *testing.T) {
	cn := &CreditNote{}
	if cn.Kind() != "credit-note" {
		t.Errorf("expected 'credit-note', got %q", cn.Kind())
	}
}

// --- Validator ---

func TestValidator_ReturnsNil(t *testing.T) {
	cn := &CreditNote{}
	if cn.Validator() != nil {
		t.Error("expected nil validator")
	}
}

// --- MarkVoid from bogus/unknown statuses ---

func TestMarkVoid_InvalidStatus_UnknownString(t *testing.T) {
	cn := &CreditNote{Status: Status("refunded")}
	err := cn.MarkVoid()
	if err == nil {
		t.Fatal("expected error voiding credit note with unknown status")
	}
}

func TestMarkVoid_InvalidStatus_AnotherUnknown(t *testing.T) {
	cn := &CreditNote{Status: Status("pending")}
	err := cn.MarkVoid()
	if err == nil {
		t.Fatal("expected error voiding credit note with 'pending' status")
	}
}

func TestMarkVoid_PreservesOtherFields(t *testing.T) {
	cn := &CreditNote{
		InvoiceId:  "inv_999",
		CustomerId: "cus_888",
		Amount:     7500,
		Currency:   "eur",
		Status:     Issued,
		Reason:     "order_change",
		Memo:       "Adjusted pricing",
	}
	if err := cn.MarkVoid(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cn.InvoiceId != "inv_999" {
		t.Errorf("expected inv_999, got %s", cn.InvoiceId)
	}
	if cn.CustomerId != "cus_888" {
		t.Errorf("expected cus_888, got %s", cn.CustomerId)
	}
	if cn.Amount != 7500 {
		t.Errorf("expected 7500, got %d", cn.Amount)
	}
	if string(cn.Currency) != "eur" {
		t.Errorf("expected eur, got %s", cn.Currency)
	}
	if cn.Reason != "order_change" {
		t.Errorf("expected order_change, got %s", cn.Reason)
	}
	if cn.Memo != "Adjusted pricing" {
		t.Errorf("expected 'Adjusted pricing', got %s", cn.Memo)
	}
}

// --- SetNumber edge cases ---

func TestSetNumber_Negative(t *testing.T) {
	cn := &CreditNote{}
	cn.SetNumber(-1)
	// fmt.Sprintf("%04d", -1) yields "-001"
	if cn.Number != "CN--001" {
		t.Errorf("expected CN--001, got %s", cn.Number)
	}
}

func TestSetNumber_Large(t *testing.T) {
	cn := &CreditNote{}
	cn.SetNumber(1000000)
	if cn.Number != "CN-1000000" {
		t.Errorf("expected CN-1000000, got %s", cn.Number)
	}
}

// --- OutOfBandAmount ---

func TestCreditNoteOutOfBandAmount(t *testing.T) {
	cn := &CreditNote{
		Status:          Issued,
		Amount:          10000,
		OutOfBandAmount: 2500,
	}
	if cn.OutOfBandAmount != 2500 {
		t.Errorf("expected 2500, got %d", cn.OutOfBandAmount)
	}
}

// --- CreditBalanceTransaction and RefundId ---

func TestCreditNoteBalanceTransaction(t *testing.T) {
	cn := &CreditNote{
		Status:                  Issued,
		CreditBalanceTransaction: "txn_abc",
	}
	if cn.CreditBalanceTransaction != "txn_abc" {
		t.Errorf("expected txn_abc, got %s", cn.CreditBalanceTransaction)
	}
}

func TestCreditNoteRefundId(t *testing.T) {
	cn := &CreditNote{
		Status:   Issued,
		RefundId: "re_xyz",
	}
	if cn.RefundId != "re_xyz" {
		t.Errorf("expected re_xyz, got %s", cn.RefundId)
	}
}

// --- LineItem zero value ---

func TestCreditNoteLineItemZeroValue(t *testing.T) {
	li := CreditNoteLineItem{}
	if li.Amount != 0 {
		t.Errorf("expected 0, got %d", li.Amount)
	}
	if li.Description != "" {
		t.Errorf("expected empty, got %q", li.Description)
	}
	if li.Quantity != 0 {
		t.Errorf("expected 0, got %d", li.Quantity)
	}
}

// --- Multiple line items sum ---

func TestCreditNoteLineItemsSum(t *testing.T) {
	cn := &CreditNote{
		Status: Issued,
		LineItems: []CreditNoteLineItem{
			{Amount: 1000},
			{Amount: 2000},
			{Amount: 3000},
		},
	}
	var total int64
	for _, li := range cn.LineItems {
		total += li.Amount
	}
	if total != 6000 {
		t.Errorf("expected sum 6000, got %d", total)
	}
}
