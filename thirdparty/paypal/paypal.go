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

func setupHeaders(req *http.Request, ord *order.Order, org *organization.Organization) {
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

func (c Client) Pay(pay *payment.Payment, usr *user.User, ord *order.Order, org *organization.Organization) (string, error) {
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
	if usr.PaypalEmail != "" {
		data.Set("senderEmail", usr.PaypalEmail)
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

	// memo := ord.LineItemsAsString()
	// if memo != "" {
	// 	data.Set("memo", memo)
	// }
	data.Set("receiverList.receiver(1).amount", strconv.FormatFloat(csFee, 'E', -1, 64)) // Us
	data.Set("receiverList.receiver(1).email", config.Paypal.Email)
	data.Set("receiverList.receiver(1).primary", "false")
	data.Set("requestEnvelope.errorLanguage", "en-US")
	data.Set("returnUrl", org.Paypal.ConfirmUrl+"#checkoutsuccess")
	data.Set("cancelUrl", org.Paypal.CancelUrl+"#checkoutfailure")
	data.Set("ipnNotificationUrl", config.Paypal.IpnUrl+org.Name)

	req, err := http.NewRequest("POST", config.Paypal.Api+"/AdaptivePayments/Pay", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	setupHeaders(req, ord, org)

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

func (c Client) SetPaymentOptions(payKey string, user *user.User, ord *order.Order, org *organization.Organization) error {

	// Set invoice information
	data := url.Values{}
	data.Set("requestEnvelope.errorLanguage", "en-US")
	if config.IsProduction {
		data.Set("receiverOptions[0].receiver.email", org.Paypal.Live.Email)
	} else {
		data.Set("receiverOptions[0].receiver.email", org.Paypal.Test.Email)
	}
	for i, lineItem := range ord.Items {
		data.Set("receiverOptions[0].invoiceData.item["+strconv.Itoa(i)+"].name", lineItem.String())
		data.Set("receiverOptions[0].invoiceData.item["+strconv.Itoa(i)+"].price", ord.Currency.ToStringNoSymbol(lineItem.TotalPrice()))
		data.Set("receiverOptions[0].invoiceData.item["+strconv.Itoa(i)+"].itemCount", strconv.Itoa(lineItem.Quantity))
		data.Set("receiverOptions[0].invoiceData.item["+strconv.Itoa(i)+"].itemPrice", ord.Currency.ToStringNoSymbol(lineItem.Price))
	}
	// log.Warn("Tax %v, Shipping %v", ord.Currency.ToStringNoSymbol(ord.Tax), ord.Currency.ToStringNoSymbol(ord.Shipping))
	data.Set("receiverOptions[0].invoiceData.totalTax", ord.Currency.ToStringNoSymbol(ord.Tax))
	data.Set("receiverOptions[0].invoiceData.totalShipping", ord.Currency.ToStringNoSymbol(ord.Shipping))
	data.Set("payKey", payKey)

	req, err := http.NewRequest("POST", config.Paypal.Api+"/AdaptivePayments/SetPaymentOptions", strings.NewReader(data.Encode()))
	if err != nil {
		log.Error("Request Came Back With Error %v", err, c.ctx)
		return err
	}

	setupHeaders(req, ord, org)

	req.PostForm = data

	dump, _ := httputil.DumpRequestOut(req, true)
	log.Info("REQ %s", string(dump), c.ctx)

	client := urlfetch.Client(c.ctx)
	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		log.Error("Request Came Back With Error %v", err, c.ctx)
		return err
	}

	responseBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Error("Could Not Decode Response %v", err, c.ctx)
		return err
	}

	log.Info("Response Bytes: %v", string(responseBytes), c.ctx)

	setPaymentOptionsResponse := responses.SetPaymentOptionsResponse{}
	err = json.Unmarshal(responseBytes, &setPaymentOptionsResponse)

	if err != nil {
		log.Error("Could Not Unmarshal Response: %v", err, c.ctx)
		return err
	}

	if setPaymentOptionsResponse.ResponseEnvelope.Ack != "Success" {
		log.Error("Problem encountered while setting payment options.  Returned code: %v", setPaymentOptionsResponse.ResponseEnvelope.Error)
	}

	return nil
}

func (c Client) GetPayKey(pay *payment.Payment, usr *user.User, ord *order.Order, org *organization.Organization) (string, error) {
	payKey, err := c.Pay(pay, usr, ord, org)
	c.SetPaymentOptions(payKey, usr, ord, org)
	return payKey, err
}

func (c Client) GetPaymentDetails(payKey string, ord *order.Order, org *organization.Organization) (*responses.PaymentDetailsResponse, error) {
	req, err := http.NewRequest("POST", config.Paypal.Api+"/AdaptivePayments/PaymentDetails", nil)
	if err != nil {
		return nil, err
	}

	setupHeaders(req, ord, org)

	return nil, nil
}
