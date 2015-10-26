package ipn

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"appengine/urlfetch"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/middleware"
	"crowdstart.com/util/router"
)

func Webhook(c *gin.Context) {
	// Send empty HTTP 200
	c.String(200, "")

	var confirm = "cmd=_notify_validate&%s"

	ipnBytes, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return
	}

	var ipnString = string(ipnBytes)

	var confirmResponse = fmt.Sprintf(confirm, ipnString)
	// Send command as received with cmd=_notify_validate, in its own request client.  Check to make sure Paypal responds with "VALIDATED".
	c.String(200, confirmResponse)

	respStr, err := getResponseBody(urlfetch.Client(middleware.GetAppEngine(c)).Post(
		config.Paypal.IpnUrl, "application/x-www-form-urlencoded", strings.NewReader(confirmResponse)))
	if err != nil {
		return
	}
	if respStr != "VERIFIED" {
		return
	}

	values, err := url.ParseQuery(ipnString)
	if err != nil {
		return
	}

	// Message is now trustable.  Parse into an object and take action.
	_ = &PayPalIpnMessage{
		Status:     values.Get("status"),
		PayerEmail: values.Get("sender_email"),
		PayeeEmail: values.Get("transaction[0].receiver"),
		PayKey:     values.Get("pay_key"),
	}

}

func Route(router router.Router, args ...gin.HandlerFunc) {
	api := router.Group("ipn")
	api.POST("/ipn", Webhook)
}

func getResponseBody(resp *http.Response, err error) (string, error) {
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.New(resp.Status)
	}
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
