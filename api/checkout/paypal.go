package checkout

import (
	"github.com/gin-gonic/gin"

	aeds "appengine/datastore"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/paypal"
	"crowdstart.com/util/json/http"
)

type PayKeyResponse struct {
	order.Order

	PayKey string `json:"payKey"`
}

func PayPalPayKey(c *gin.Context) {
	org, ord := getOrganizationAndOrder(c)
	if ord == nil {
		http.Fail(c, 404, "Failed to retrieve order", OrderDoesNotExist)
		return
	}

	ord.Type = "paypal"

	pay, usr, err := authorize(c, org, ord)
	if err != nil {
		http.Fail(c, 500, "Error during authorize", err)
		return
	}

	ctx := org.Db.Context
	client := paypal.New(ctx)

	payKey, err := client.GetPayKey(pay, usr, ord, org)
	if err != nil {
		ord.Status = order.Cancelled
		pay.Status = payment.Cancelled
		pay.Account.Error = err.Error()
		ord.MustPut()
		pay.MustPut()

		http.Fail(c, 500, "Paypal Error", err)
		return
	}

	pay.Account.PayKey = payKey
	ord.MustPut()
	pay.MustPut()

	payKeyResponse := PayKeyResponse{*ord, payKey}

	http.Render(c, 200, payKeyResponse)
}

func PayPalConfirm(c *gin.Context) {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)
	db := datastore.New(ctx)

	var err error
	var ord *order.Order
	var payments []*payment.Payment

	ord = order.New(db)
	payments = make([]*payment.Payment, 0)

	if payKey := c.Params.ByName("payKey"); payKey != "" {
		_, err = payment.Query(db).Filter("Account.PayKey=", payKey).GetAll(&payments)
		if err != nil {
			http.Fail(c, 500, "Failed to retrieve payment", err)
			return
		}
	}

	if len(payments) == 0 {
		http.Fail(c, 404, "Failed to retrieve payment", PaymentDoesNotExist)
		return
	}

	err = ord.GetById(payments[0].OrderId)
	if err != nil {
		http.Fail(c, 404, "Failed to retrieve order", OrderDoesNotExist)
		return
	}

	for _, pay := range payments {
		pay.Status = payment.Paid
	}

	ord.PaymentStatus = payment.Paid
	ord.Payments = payments

	ord, err = capture(c, org, ord)
	if err != nil {
		http.Fail(c, 500, "Error during capture", err)
		return
	}

	http.Render(c, 200, ord)
}

func PayPalCancel(c *gin.Context) {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)
	db := datastore.New(ctx)

	var err error
	var keys []*aeds.Key
	var ord *order.Order
	var payments []*payment.Payment

	ord = order.New(db)
	payments = make([]*payment.Payment, 0)

	if payKey := c.Params.ByName("payKey"); payKey != "" {
		keys, err = payment.Query(db).Filter("Account.PayKey=", payKey).GetAll(&payments)
		if err != nil {
			http.Fail(c, 500, "Failed to retrieve payment", err)
			return
		}
	}

	if len(payments) == 0 {
		http.Fail(c, 404, "Failed to retrieve payment", PaymentDoesNotExist)
		return
	}

	err = ord.GetById(payments[0].OrderId)
	if err != nil {
		http.Fail(c, 404, "Failed to retrieve order", OrderDoesNotExist)
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

	http.Render(c, 200, ord)
}
