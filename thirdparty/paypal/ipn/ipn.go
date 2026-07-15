package ipn

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/payment"
)

// Read body from response
func readBody(res *http.Response) (string, error) {
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", errors.New(fmt.Sprintf("Invalid status code: %v", res.Status))
	}

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func respond(ctx context.Context, message url.Values) (string, error) {
	req, err := http.NewRequest("POST", config.Paypal.PaypalIpnUrl, bytes.NewBufferString(message.Encode()))
	if err != nil {
		log.Panic("Could create request: %s", err, ctx)
	}

	dump, _ := httputil.DumpRequestOut(req, true)
	log.Debug("IPN response: %s", string(dump), ctx)

	// Set timeout
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	// Create client
	client := &http.Client{Timeout: 55 * time.Second}

	// Make Post request
	res, err := client.Do(req)
	if err != nil {
		log.Panic("Unable to make request: %v", err, ctx)
	}

	return readBody(res)
}

func Webhook(c *zip.Ctx) error {
	orgName := c.Param("organization")
	if orgName == "" {
		log.Panic("Organization not specified", c)
	}

	// Get org
	db := datastore.New(c.Context())
	org := organization.New(db)
	err := org.GetById(orgName)

	// Get namespaced db
	db.SetNamespace(org.Name)

	ctx := db.Context

	// Parse form — PayPal IPN posts application/x-www-form-urlencoded, so the
	// raw request body IS the form. c.Body() is zero-copy; ParseQuery copies
	// what it keeps, so nothing is retained past the request.
	form, err := url.ParseQuery(string(c.Body()))
	if err != nil {
		log.Panic("Failed to parse request from PayPal", c)
	}
	log.Debug("IPN message: %v", form, ctx)

	// Append cmd=_notify-validate
	form.Add("cmd", "_notify-validate")

	// PayPal only needs an empty 200 ack; we return that at every exit below
	// (single-render) and treat the validation result as a side effect so an
	// internal lookup failure never makes PayPal retry. Send command as received
	// with cmd=_notify-validate, in its own request client. Check to make sure
	// Paypal responds with "VALIDATED".
	status, err := respond(ctx, form)
	if err != nil {
		log.Error("Failed to respond to PayPal: %s", err, ctx)
		return c.String(200, "")
	}

	if status != "VERIFIED" {
		log.Error("Response was not verified", ctx)
		return c.String(200, "")
	}

	// Parse form into ipnMessage for ease of use.
	ipnMessage := NewIpnMessage(form)

	// Update payment
	pay := payment.New(db)
	_, err = pay.Query().Filter("Account.PayKey=", ipnMessage.PayKey).Get()
	if err != nil {
		log.Error("Could not find PayKey: %s", err, ctx)
		return c.String(200, "")
	}

	ord := order.New(db)
	err = ord.GetById(pay.OrderId)
	if err != nil {
		log.Error("Could not find Order: %s", err, ctx)
		return c.String(200, "")
	}

	if ipnMessage.Status != "Completed" {
		switch ipnMessage.Status {
		case "Processing", "Pending", "Created":
			return c.String(200, "")
		case "Refunded", "Partially_Refunded", "Reversed":
			pay.Status = payment.Refunded
			ord.Status = order.Cancelled
		// Denied, Failed, Voided
		default:
			pay.Status = payment.Failed
			ord.Status = order.Cancelled
		}

		ord.PaymentStatus = pay.Status

		// No need to call Refund API.
		pay.MustUpdate()
		ord.MustUpdate()
		return c.String(200, "")
	}

	if pay.Amount != ipnMessage.Amount || pay.Currency != ipnMessage.Currency {
		// Probably fraud.
		pay.Status = payment.Fraudulent
		pay.MustUpdate()

		ord.Status = order.Cancelled
		ord.PaymentStatus = pay.Status
		ord.MustUpdate()

		// call refund API
		return c.String(200, "")
	}

	// Looking good.
	pay.Status = payment.Paid
	pay.MustUpdate()

	// TODO: Make this part of the payment model API
	// checkoutApi.CompleteCapture(c, org, ord, []*aeds.Key{pay.Key().(*aeds.Key)}, []*payment.Payment{pay})
	return c.String(200, "")
}

func Route(router zip.Router, args ...zip.Handler) {
	api := router.Group("paypal")
	api.Post("/ipn/:organization", Webhook)
}
