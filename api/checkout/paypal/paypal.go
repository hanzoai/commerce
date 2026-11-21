package paypal

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/datastore/iface"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
)

type PayKeyResponse struct {
	order.Order

	PayKey string `json:"payKey"`
}

func Confirm(c *zip.Ctx, org *organization.Organization, ord *order.Order) (err error) {
	// Per-org store: payments live in the order's org store (Red MED-1). Raw
	// datastore.New(c) drops the namespace (zip.Ctx → Background) AND binds
	// systemDB, so it would query the wrong store.
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	payments := make([]*payment.Payment, 0)

	if payKey := c.Param("payKey"); payKey != "" {
		_, err = payment.Query(db).Filter("Account.PayKey=", payKey).GetAll(&payments)
		if err != nil {
			return PaymentDoesNotExist
		}
	}

	if len(payments) == 0 {
		return PaymentDoesNotExist
	}

	for _, pay := range payments {
		pay.Status = payment.Paid
	}

	ord.PaymentStatus = payment.Paid
	ord.Payments = payments
	ord.MustPut()

	return nil
}

func Cancel(c *zip.Ctx, org *organization.Organization, ord *order.Order) (err error) {
	// Per-org store (Red MED-1) — see Confirm above.
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))

	var keys []iface.Key
	var payments []*payment.Payment

	payments = make([]*payment.Payment, 0)

	if payKey := c.Param("payKey"); payKey != "" {
		keys, err = payment.Query(db).Filter("Account.PayKey=", payKey).GetAll(&payments)
		if err != nil {
			return PaymentDoesNotExist
		}
	}

	if len(payments) == 0 {
		// Single-render: return the error and let the calling handler render it
		// (mirrors Confirm above). The prior http.Fail(...)+return double-rendered
		// — under fiber's last-wins the caller's later render flipped 404→200.
		return PaymentDoesNotExist
	}

	for i, pay := range payments {
		pay.Init(db)
		pay.SetKey(keys[i])
		pay.Status = payment.Cancelled
		pay.Account.Error = PaymentCancelled.Error()
		pay.MustPut()
	}

	ord.Status = order.Cancelled
	ord.PaymentStatus = payment.Cancelled
	ord.MustPut()

	return nil
}
