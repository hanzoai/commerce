package paypal

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"crowdstart.com/config"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/paypal/responses"
	"crowdstart.com/util/log"

	"appengine"
	"appengine/urlfetch"
)

type Client struct {
	ctx appengine.Context
}

func New(ctx appengine.Context) *Client {
	return &Client{ctx: ctx}
}

func setupHeaders(req *http.Request, org *organization.Organization) {
	if config.IsProduction {
		req.Header.Set("X-PAYPAL-SECURITY-USERID", org.Paypal.Live.SecurityUserId)
		req.Header.Set("X-PAYPAL-SECURITY-PASSWORD", org.Paypal.Live.SecurityPassword)
		req.Header.Set("X-PAYPAL-SECURITY-SIGNATURE", org.Paypal.Live.SecuritySignature)
		req.Header.Set("X-PAYPAL-APPLICATION-ID", org.Paypal.Live.ApplicationId)
	} else {
		req.Header.Set("X-PAYPAL-SECURITY-USERID", org.Paypal.Test.SecurityUserId)
		req.Header.Set("X-PAYPAL-SECURITY-PASSWORD", org.Paypal.Test.SecurityPassword)
		req.Header.Set("X-PAYPAL-SECURITY-SIGNATURE", org.Paypal.Test.SecuritySignature)
		req.Header.Set("X-PAYPAL-APPLICATION-ID", org.Paypal.Test.ApplicationId)
	}

	req.Header.Set("X-PAYPAL-REQUEST-DATA-FORMAT", "NV")
	req.Header.Set("X-PAYPAL-RESPONSE-DATA-FORMAT", "JSON")
}

func (c Client) GetPayKey(pay *payment.Payment, user *user.User, org *organization.Organization, ord *order.Order) (string, error) {
	data := url.Values{}
	data.Set("actionType", "PAY")
	// Standard sandbox APP ID, for testing
	if config.IsProduction {
		data.Set("clientDetails.applicationId", org.Paypal.Live.ApplicationId)
	} else {
		data.Set("clientDetails.applicationId", org.Paypal.Test.ApplicationId)
	}
	// IP address from which request is sent.
	data.Set("clientDetails.ipAddress", pay.Client.Ip)
	if user.PaypalEmail != "" {
		data.Set("senderEmail", user.PaypalEmail)
	}
	data.Set("currencyCode", pay.Currency.Code())

	fee := config.Fee
	if org.Fee > 0 {
		fee = org.Fee
	}

	var amount = float64(pay.Amount)
	var csFee = math.Ceil(amount * fee)
	//TODO: Fee is not always going to be set on the organization.  That is for overrides.  We need to refactor our defaults into Config.

	if !pay.Currency.IsZeroDecimal() {
		csFee /= 100
		amount /= 100
	}

	data.Set("receiverList.receiver(0).amount", strconv.FormatFloat(amount, 'E', -1, 64)) // Our client
	if config.IsProduction {
		data.Set("receiverList.receiver(0).email", org.Paypal.Live.Email)
	} else {
		data.Set("receiverList.receiver(0).email", org.Paypal.Test.Email)
	}
	data.Set("receiverList.receiver(0).primary", "true")
	data.Set("memo", ord.LineItemsAsString())
	data.Set("receiverList.receiver(1).amount", strconv.FormatFloat(csFee, 'E', -1, 64)) // Us
	data.Set("receiverList.receiver(1).email", config.Paypal.Email)
	data.Set("receiverList.receiver(1).primary", "false")
	data.Set("requestEnvelope.errorLanguage", "en-US")
	data.Set("returnUrl", org.Paypal.ConfirmUrl+"#checkoutsuccess")
	data.Set("cancelUrl", org.Paypal.CancelUrl+"#checkoutfailure")

	req, err := http.NewRequest("POST", config.Paypal.Api+"/AdaptivePayments/Pay", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	setupHeaders(req, org)

	req.PostForm = data

	dump, _ := httputil.DumpRequestOut(req, true)
	log.Info("REQ %s", string(dump), c.ctx)

	client := urlfetch.Client(c.ctx)
	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		log.Error("Request Came Back With Error %v", err, c.ctx)
		return "", err
	}

	responseBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Error("Could Not Decode Response %v", err, c.ctx)
		return "", err
	}

	log.Info("Response Bytes: %v", string(responseBytes), c.ctx)

	paymentResponse := responses.ParallelPaymentResponse{}
	err = json.Unmarshal(responseBytes, &paymentResponse)

	if err != nil {
		log.Error("Could Not Unmarshal Response: %v", err, c.ctx)
		return "", err
	}

	errs := len(paymentResponse.Error)
	if errs > 0 {
		errStr := ""
		if errs > 1 {
			errStr = " and " + strconv.Itoa(errs) + " others"
		}
		log.Error("PayPal Error: %v", paymentResponse.Error[0].Message+errStr, c.ctx)
		return "", errors.New("PayPal Error: " + paymentResponse.Error[0].Message + errStr)
	}

	return paymentResponse.PayKey, nil
}

func (c Client) GetPaymentDetails(payKey string, org *organization.Organization) (*responses.PaymentDetailsResponse, error) {
	req, err := http.NewRequest("POST", config.Paypal.Api+"/AdaptivePayments/PaymentDetails", nil)
	if err != nil {
		return nil, err
	}

	setupHeaders(req, org)

	return nil, nil
}
