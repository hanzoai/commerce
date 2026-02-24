package paymentintent

import "github.com/hanzoai/commerce/datastore"

var kind = "payment-intent"

func (pi PaymentIntent) Kind() string {
	return kind
}

func (pi *PaymentIntent) Init(db *datastore.Datastore) {
	pi.BaseModel.Init(db, pi)
}

func (pi *PaymentIntent) Defaults() {
	pi.Parent = pi.Db.NewKey("synckey", "", 1, nil)
	if pi.Status == "" {
		pi.Status = RequiresPaymentMethod
	}
	if pi.Currency == "" {
		pi.Currency = "usd"
	}
	if pi.CaptureMethod == "" {
		pi.CaptureMethod = "automatic"
	}
	if pi.ConfirmationMethod == "" {
		pi.ConfirmationMethod = "automatic"
	}
}

func New(db *datastore.Datastore) *PaymentIntent {
	pi := new(PaymentIntent)
	pi.Init(db)
	pi.Defaults()
	return pi
}

func Query(db *datastore.Datastore) datastore.Query {
	return db.Query(kind)
}
