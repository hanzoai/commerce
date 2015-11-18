package ipn

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"appengine"

	"appengine/urlfetch"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/log"
	"crowdstart.com/util/router"
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

func respond(ctx appengine.Context, message url.Values) (string, error) {
	req, err := http.NewRequest("POST", config.Paypal.PaypalIpnUrl, bytes.NewBufferString(message.Encode()))
	if err != nil {
		log.Panic("Could create request: %s", err, ctx)
	}

	dump, _ := httputil.DumpRequestOut(req, true)
	log.Debug("IPN response: %s", string(dump), ctx)

	client := urlfetch.Client(ctx)
	client.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(20) * time.Second, // Update deadline to 10 seconds
	}

	res, err := client.Do(req)
	if err != nil {
		log.Panic("Unable to make request: %v", err, ctx)
	}

	return readBody(res)
}

func Webhook(c *gin.Context) {
	org := c.Params.ByName("organization")
	if org == "" {
		log.Panic("Organization not specified", c)
	}

	// Get namespaced db
	db := datastore.New(c)
	db.SetNamespace(org)

	ctx := db.Context

	// Parse form
	if err := c.Request.ParseForm(); err != nil {
		log.Panic("Failed to parse request from PayPal", c)
	}

	form := c.Request.Form
	log.Debug("IPN message: %v", form, ctx)

	// Append cmd=_notify-validate
	form.Add("cmd", "_notify-validate")

	// Send command as received with cmd=_notify-validate, in its own request client.  Check to make sure Paypal responds with "VALIDATED".
	c.String(200, "")

	// Send response
	status, err := respond(ctx, form)
	if err != nil {
		log.Panic("Failed to respond to PayPal: %s", err, ctx)
	}

	if status != "VERIFIED" {
		log.Panic("Response was not verified", ctx)
	}

	// Parse form into ipnMessage for ease of use.
	ipnMessage := &PayPalIpnMessage{
		Status:     form.Get("status"),
		PayerEmail: form.Get("sender_email"),
		PayeeEmail: form.Get("transaction[0].receiver"),
		PayKey:     form.Get("pay_key"),
		Amount:     currency.CentsFromString(form.Get("payment_gross")),
	}

	// Update payment
	p := payment.New(db)
	_, err = p.Query().Filter("Account.PayKey=", ipnMessage.PayKey).First()
	if err != nil {
		log.Panic("Could not find paykey: %s", err, ctx)
		return
	}
	if ipnMessage.Status != "Completed" {
		if ipnMessage.Status == "Processed" || ipnMessage.Status == "Pending" {
			return
		}
		// Denied, Failed, Refunded, Reversed, Voided
		p.Status = payment.Failed
		// No need to call Refund API.
		err = p.Put()
		if err != nil {
			log.Panic("Could not put payment: %s", err, ctx)
			return
		}
		return
	}
	if p.Amount != ipnMessage.Amount {
		// Probably fraud.
		p.Status = payment.Fraudulent
		p.Put()
		// call refund API
		return
	}

	// Looking good.
	p.Status = payment.Paid
	p.Put()
	return

}

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("paypal")
	api.POST("/ipn/:organization", Webhook)
}
