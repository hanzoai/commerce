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

func (c Client) GetPayKey(pay *payment.Payment, user *user.User, org *organization.Organization) (string, error) {

	data := url.Values{}
	data.Set("actionType", "PAY")
	// Standard sandbox APP ID, for testing
	data.Set("clientDetails.applicationId", config.Paypal.PaypalApplicationId)
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

	var csFee = math.Ceil(float64(pay.Amount) * fee)
	//TODO: Fee is not always going to be set on the organization.  That is for overrides.  We need to refactor our defaults into Config.
	var clientPayout = float64(pay.Amount) - csFee

	data.Set("receiverList.receiver(0).amount", strconv.FormatFloat(clientPayout, 'E', -1, 64)) // Our client
	data.Set("receiverList.receiver(0).email", org.Paypal.Email)
	data.Set("receiverList.receiver(1).amount", strconv.FormatFloat(csFee, 'E', -1, 64)) // Us
	data.Set("receiverList.receiver(1).email", "dev@hanzo.ai")
	data.Set("requestEnvelope.errorLanguage", "en-US")
	data.Set("returnUrl", org.Paypal.ConfirmUrl)
	data.Set("cancelUrl", org.Paypal.CancelUrl)

	req, err := http.NewRequest("POST", config.Paypal.ParallelPaymentsUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("X-PAYPAL-SECURITY-USERID", config.Paypal.PaypalSecurityUserId)
	req.Header.Set("X-PAYPAL-SECURITY-PASSWORD", config.Paypal.PaypalSecurityPassword)
	req.Header.Set("X-PAYPAL-SECURITY-SIGNATURE", config.Paypal.PaypalSecuritySignature)
	req.Header.Set("X-PAYPAL-REQUEST-DATA-FORMAT", "NV")
	req.Header.Set("X-PAYPAL-RESPONSE-DATA-FORMAT", "JSON")
	req.Header.Set("X-PAYPAL-APPLICATION-ID", config.Paypal.PaypalApplicationId)

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
		return "", errors.New(paymentResponse.Error[0].Message + errStr)
	}

	return paymentResponse.PayKey, nil
}
