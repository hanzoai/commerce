package paypal

import (
	"github.com/gin-gonic/gin"

	aeds "appengine/datastore"

	"hanzo.io/datastore"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/payment"
	"hanzo.io/util/json/http"
)

type PayKeyResponse struct {
	order.Order

	PayKey string `json:"payKey"`
}

func Confirm(c *gin.Context, org *organization.Organization, ord *order.Order) (err error) {
	db := datastore.New(c)

	payments := make([]*payment.Payment, 0)

	if payKey := c.Params.ByName("payKey"); payKey != "" {
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

func Cancel(c *gin.Context, org *organization.Organization, ord *order.Order) (err error) {
	db := datastore.New(c)

	var keys []*aeds.Key
	var payments []*payment.Payment

	payments = make([]*payment.Payment, 0)

	if payKey := c.Params.ByName("payKey"); payKey != "" {
		keys, err = payment.Query(db).Filter("Account.PayKey=", payKey).GetAll(&payments)
		if err != nil {
			return PaymentDoesNotExist
		}
	}

	if len(payments) == 0 {
		http.Fail(c, 404, "Failed to retrieve payment", PaymentDoesNotExist)
		return
	}

	for i, pay := range payments {
		pay.Model.Db = db
		pay.Model.Entity = pay

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
