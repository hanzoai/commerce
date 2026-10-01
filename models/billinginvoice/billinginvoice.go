package billinginvoice

import (
	"fmt"
	"strings"
	"time"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/val"
	"github.com/hanzoai/orm"

	. "github.com/hanzoai/commerce/types"
)

// Status represents the billing invoice lifecycle state.
type Status string

const (
	Draft         Status = "draft"
	Open          Status = "open"
	Paid          Status = "paid"
	Void          Status = "void"
	Uncollectible Status = "uncollectible"
)

// LineItemType categorizes what a line item charges for.
type LineItemType string

const (
	LineSubscription LineItemType = "subscription"
	LineUsage        LineItemType = "usage"
	LineOneOff       LineItemType = "one_off"
	LineProration    LineItemType = "proration"
)

// LineItem represents a single charge on an invoice.
type LineItem struct {
	Id          string       `json:"id"`
	Type        LineItemType `json:"type"`
	Description string       `json:"description"`

	// Usage items
	MeterId   string `json:"meterId,omitempty"`
	Quantity  int64  `json:"quantity,omitempty"`
	UnitPrice int64  `json:"unitPrice,omitempty"` // cents

	// Subscription items
	PlanId   string `json:"planId,omitempty"`
	PlanName string `json:"planName,omitempty"`

	Amount   int64         `json:"amount"` // cents
	Currency currency.Type `json:"currency"`

	PeriodStart time.Time `json:"periodStart,omitempty"`
	PeriodEnd   time.Time `json:"periodEnd,omitempty"`
}

func init() { orm.Register[BillingInvoice]("billing-invoice") }

// BillingInvoice is a proper billing invoice with line items, status lifecycle,
// and payment tracking. This is distinct from the legacy Invoice model (Kind="payment")
// which is actually a charge/payment record.
type BillingInvoice struct {
	mixin.Model[BillingInvoice]

	// Customer
	UserId        string `json:"userId"`
	CustomerEmail string `json:"customerEmail,omitempty"`

	// Subscription link (empty for one-off invoices)
	SubscriptionId string `json:"subscriptionId,omitempty"`

	// Billing period
	PeriodStart time.Time `json:"periodStart"`
	PeriodEnd   time.Time `json:"periodEnd"`

	// Financial summary (all in cents)
	Subtotal      int64   `json:"subtotal"`
	Tax           int64   `json:"tax"`
	TaxPercent    float64 `json:"taxPercent"`
	Discount      int64   `json:"discount"`
	DiscountName  string  `json:"discountName,omitempty"`
	CreditApplied int64   `json:"creditApplied"`
	AmountDue     int64   `json:"amountDue"`
	AmountPaid    int64   `json:"amountPaid"`

	Currency currency.Type `json:"currency" orm:"default:usd"`

	// Status lifecycle: draft -> open -> paid | void | uncollectible
	Status   Status    `json:"status" orm:"default:draft"`
	DueDate  time.Time `json:"dueDate,omitempty"`
	PaidAt   time.Time `json:"paidAt,omitempty"`
	VoidedAt time.Time `json:"voidedAt,omitempty"`

	// Payment reference
	PaymentMethod string `json:"paymentMethod,omitempty"` // "balance", "stripe", "credit"
	PaymentRef    string `json:"paymentRef,omitempty"`    // e.g. Stripe PaymentIntent ID

	// Refunds is each refund given back against the payment that paid this
	// invoice, in cents, keyed by the id it was given under — the processor's
	// refund id, or a refund recorded here — so a refund reported twice counts once.
	Refunds map[string]int64 `json:"refunds,omitempty"`
	// Dispute is the state of a dispute against the payment that paid this
	// invoice, as the processor last reported it; empty when there is none.
	Dispute string `json:"dispute,omitempty"`

	// Invoice number (auto-increment per org)
	Number    int    `json:"number"`
	NumberStr string `json:"numberStr"` // "INV-0042"

	// Dunning retry tracking
	AttemptCount  int       `json:"attemptCount"`
	LastAttemptAt time.Time `json:"lastAttemptAt,omitempty"`
	NextAttemptAt time.Time `json:"nextAttemptAt,omitempty"`

	// Line items (JSON-serialized)
	LineItems  []LineItem `json:"lineItems,omitempty" datastore:"-"`
	LineItems_ string     `json:"-" datastore:",noindex"`

	Metadata  Map    `json:"metadata,omitempty" datastore:"-"`
	Metadata_ string `json:"-" datastore:",noindex"`
}

func (inv *BillingInvoice) Load(ps []datastore.Property) (err error) {
	if err = datastore.LoadStruct(inv, ps); err != nil {
		return err
	}

	if len(inv.LineItems_) > 0 {
		if err = json.DecodeBytes([]byte(inv.LineItems_), &inv.LineItems); err != nil {
			return err
		}
	}

	if len(inv.Metadata_) > 0 {
		err = json.DecodeBytes([]byte(inv.Metadata_), &inv.Metadata)
	}

	return err
}

func (inv *BillingInvoice) Save() (ps []datastore.Property, err error) {
	inv.LineItems_ = string(json.EncodeBytes(&inv.LineItems))
	inv.Metadata_ = string(json.EncodeBytes(&inv.Metadata))
	return datastore.SaveStruct(inv)
}

func (inv *BillingInvoice) Validator() *val.Validator {
	return nil
}

// Finalize transitions an invoice from draft to open and computes AmountDue.
func (inv *BillingInvoice) Finalize() error {
	if inv.Status != Draft {
		return fmt.Errorf("can only finalize draft invoices, current status: %s", inv.Status)
	}
	inv.Status = Open
	inv.AmountDue = inv.Subtotal + inv.Tax - inv.Discount - inv.CreditApplied
	if inv.AmountDue < 0 {
		inv.AmountDue = 0
	}
	return nil
}

// MarkPaid marks the invoice as paid.
func (inv *BillingInvoice) MarkPaid(method, ref string) error {
	if inv.Status != Open {
		return fmt.Errorf("can only pay open invoices, current status: %s", inv.Status)
	}
	inv.Status = Paid
	inv.PaidAt = time.Now()
	inv.AmountPaid = inv.AmountDue
	inv.PaymentMethod = method
	inv.PaymentRef = ref
	return nil
}

// MarkVoid voids an open invoice.
func (inv *BillingInvoice) MarkVoid() error {
	if inv.Status != Open && inv.Status != Draft {
		return fmt.Errorf("can only void draft or open invoices, current status: %s", inv.Status)
	}
	inv.Status = Void
	inv.VoidedAt = time.Now()
	return nil
}

// MarkUncollectible marks an invoice as uncollectible after all retries.
func (inv *BillingInvoice) MarkUncollectible() error {
	if inv.Status != Open {
		return fmt.Errorf("can only mark open invoices as uncollectible, current status: %s", inv.Status)
	}
	inv.Status = Uncollectible
	return nil
}

// The dispute states in which the payment stays with the merchant: a dispute won,
// and an inquiry that closed without becoming a chargeback.
const (
	DisputeWon           = "WON"
	DisputeInquiryClosed = "INQUIRY_CLOSED"
)

// Refund records amount given back under ref, once: a ref already recorded changes
// nothing. It reports whether it recorded anything.
func (inv *BillingInvoice) Refund(ref string, amount int64) bool {
	if ref == "" || amount <= 0 {
		return false
	}
	if _, ok := inv.Refunds[ref]; ok {
		return false
	}
	if inv.Refunds == nil {
		inv.Refunds = map[string]int64{}
	}
	inv.Refunds[ref] = amount
	return true
}

// Refunded is what has been given back against the invoice's payment, never more
// than was paid.
func (inv *BillingInvoice) Refunded() int64 {
	var total int64
	for _, cents := range inv.Refunds {
		total += cents
	}
	return min(total, inv.AmountPaid)
}

// SetDispute records the state a dispute against the payment reached, in upper
// case. A dispute won is final: a later report of an earlier state leaves it won.
func (inv *BillingInvoice) SetDispute(state string) {
	if inv.Dispute == DisputeWon {
		return
	}
	inv.Dispute = strings.ToUpper(strings.TrimSpace(state))
}

// Disputed reports whether a dispute holds the payment's money: one is open, or was
// lost. A dispute won, or an inquiry closed without a chargeback, holds nothing.
func (inv *BillingInvoice) Disputed() bool {
	return inv.Dispute != "" && inv.Dispute != DisputeWon && inv.Dispute != DisputeInquiryClosed
}

// RecalculateSubtotal sums all line item amounts.
func (inv *BillingInvoice) RecalculateSubtotal() {
	var total int64
	for _, li := range inv.LineItems {
		total += li.Amount
	}
	inv.Subtotal = total
}

// SetNumber assigns the invoice number and formatted string.
func (inv *BillingInvoice) SetNumber(n int) {
	inv.Number = n
	inv.NumberStr = fmt.Sprintf("INV-%04d", n)
}

func New(db *datastore.Datastore) *BillingInvoice {
	inv := new(BillingInvoice)
	inv.Init(db)
	inv.Parent = db.NewKey("synckey", "", 1, nil)
	return inv
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query("billing-invoice")
}
