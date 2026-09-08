package billing

import (
	"context"
	"errors"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
)

// The card-subscribe path reaches its provider through exactly one package var,
// processorsForOrg (subscribe_card.go). A test in THIS package can assign it
// directly; a test in another module cannot, because the var is unexported —
// and unexporting it is right, since production must never be able to swap the
// processor at runtime.
//
// So the seam is opened by a function instead of by the variable. Callers get a
// registry of their own for the length of a test and a cleanup that puts the
// real resolver back, and nothing about the production path changes: the var
// still holds payment.ProcessorsForOrg everywhere else.

// SetProcessorsForOrg makes f the resolver the card-subscribe path uses, and
// returns the func that restores the previous one. Not safe under parallel
// tests — it is one process-wide var, which is the same reason production
// cannot reach it.
func SetProcessorsForOrg(f ProcessorsFunc) func() {
	prev := processorsForOrg
	processorsForOrg = f
	return func() { processorsForOrg = prev }
}

// ProcessorsFunc resolves an org to the processors that may charge it.
type ProcessorsFunc = func(*organization.Organization) *processor.Registry

// SquareStub answers the Square face the card-on-file path asks for
// (square_cardonfile.go: CreateCustomer, AddPaymentMethod, RemovePaymentMethod)
// plus Charge, with values fixed at construction. It exists so a caller outside
// this module can exercise a real subscribe against a provider that is not
// there — every id it returns is the one it was given, so an assertion names a
// value rather than a shape.
type SquareStub struct {
	processor.BaseProcessor

	customer string
	card     string
	ref      string
}

// NewMockSquareProcessor returns a Square stub that reports the given customer
// id, card-on-file id and charge reference.
func NewMockSquareProcessor(customerID, cardID, paymentRef string) *SquareStub {
	s := &SquareStub{
		BaseProcessor: *processor.NewBaseProcessor(processor.Square, []currency.Type{currency.USD}),
		customer:      customerID,
		card:          cardID,
		ref:           paymentRef,
	}
	s.SetConfigured(true)
	return s
}

func (s *SquareStub) Type() processor.ProcessorType { return processor.Square }

func (s *SquareStub) Charge(ctx context.Context, req processor.PaymentRequest) (*processor.PaymentResult, error) {
	return &processor.PaymentResult{
		Success:       true,
		TransactionID: s.ref,
		ProcessorRef:  s.ref,
		Status:        "COMPLETED",
	}, nil
}

func (s *SquareStub) CreateCustomer(ctx context.Context, email, name string, metadata map[string]interface{}) (string, error) {
	return s.customer, nil
}

func (s *SquareStub) AddPaymentMethod(ctx context.Context, customerID, token string) (string, error) {
	return s.card, nil
}

func (s *SquareStub) RemovePaymentMethod(ctx context.Context, customerID, paymentMethodID string) error {
	return nil
}

// The rest of processor.PaymentProcessor. A stub that silently succeeded here
// would let a test pass on a path it never meant to take, so each says it does
// not serve this.
func (s *SquareStub) Refund(ctx context.Context, req processor.RefundRequest) (*processor.RefundResult, error) {
	return nil, errors.New("square stub: refund is not served")
}

func (s *SquareStub) GetTransaction(ctx context.Context, txID string) (*processor.Transaction, error) {
	return nil, errors.New("square stub: transaction lookup is not served")
}

func (s *SquareStub) ValidateWebhook(ctx context.Context, payload []byte, signature string) (*processor.WebhookEvent, error) {
	return nil, errors.New("square stub: webhook validation is not served")
}
