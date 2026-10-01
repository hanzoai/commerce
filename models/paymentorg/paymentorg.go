// Package paymentorg records, for every settled payment, the org whose books its
// receipt lives in — by the processor's payment id, in the system namespace.
//
// A processor's callback names a payment and no org: Square sends no X-Org-Id, and a
// sessionless request resolves to the default org. So a refund or a dispute of a
// customer's top-up, or of the payment for a plan's period, is traced to its org
// here, never read off the request.
package paymentorg

import (
	"context"
	"strings"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/mixin"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/orm"
)

func init() {
	orm.Register[PaymentOrg]("payment-org", orm.WithStringKey[PaymentOrg]())
}

// PaymentOrg is where one payment's receipt lives: the org, the wallet it credited,
// and whether the charge was a sandbox one. A payment that paid an invoice names it
// in Invoice and credited no wallet.
type PaymentOrg struct {
	mixin.Model[PaymentOrg]

	Org     string `json:"org"`
	Subject string `json:"subject"`
	Invoice string `json:"invoice,omitempty"`
	Test    bool   `json:"test"`
}

func (p *PaymentOrg) Load(ps []datastore.Property) error  { return datastore.LoadStruct(p, ps) }
func (p *PaymentOrg) Save() ([]datastore.Property, error) { return datastore.SaveStruct(p) }

func db() *datastore.Datastore {
	return datastore.New(nscontext.WithNamespace(context.Background(), "system"))
}

// Put records that payment's receipt lives in org, for subject, and the invoice it
// paid ("" for a payment that credited subject's wallet). Idempotent: the payment id
// is the key.
func Put(payment, org, subject, invoice string, test bool) error {
	payment = strings.TrimSpace(payment)
	if payment == "" || org == "" {
		return nil
	}
	d := db()
	p := new(PaymentOrg)
	p.Init(d)
	p.SetId(payment)
	p.Org, p.Subject, p.Invoice, p.Test = org, subject, invoice, test
	return p.Put()
}

// Get answers where payment's receipt lives, and false when nothing recorded it.
func Get(payment string) (*PaymentOrg, bool, error) {
	d := db()
	p := new(PaymentOrg)
	p.Init(d)
	err := p.Get(d.NewKey(p.Kind(), strings.TrimSpace(payment), 0, nil))
	switch {
	case err == nil:
		return p, true, nil
	case err == datastore.ErrNoSuchEntity:
		return nil, false, nil
	default:
		return nil, false, err
	}
}
