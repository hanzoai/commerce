package paypal

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"crowdstart.com/config"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/paypal/responses"

	"appengine"
	"appengine/urlfetch"
)

type Client struct {
	ctx appengine.Context
}

func New(ctx appengine.Context) *Client {
	return &Client{ctx: ctx}
}

func (c Client) GetPayKey(pay *payment.Payment, ord *order.Order, user *user.User, org *organization.Organization) (string, error) {

	data := url.Values{}
	data.Set("actionType", "PAY")
	data.Set("clientDetails.applicationId", config.Paypal.PaypalApplicationId) // Standard sandbox APP ID, for testing
	data.Set("clientDetails.ipAddress", pay.Client.Ip)                         // IP address from which request is sent.
	data.Set("senderEmail", user.PaypalEmail)
	data.Set("currencyCode", pay.Currency.Code())

	var csFee = float64(pay.Amount) * org.Fee
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

	client := urlfetch.Client(c.ctx)
	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		return "", err
	}

	responseBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	paymentResponse := responses.ParallelPaymentResponse{}
	err = json.Unmarshal(responseBytes, &paymentResponse)

	if err != nil {
		return "", err
	}
	return paymentResponse.PayKey, nil
}
