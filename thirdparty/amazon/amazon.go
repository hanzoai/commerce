package amazon

import (
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/amazon/requests"
	"crowdstart.com/thirdparty/amazon/responses"
	"crowdstart.com/thirdparty/amazon/types"

	"appengine"
	"appengine/urlfetch"
)

type Client struct {
	ctx         appengine.Context
	accessToken string
}

func New(ctx appengine.Context, accessToken string) *Client {
	return &Client{ctx: ctx, accessToken: accessToken}
}

func (c Client) Authorize(pay *payment.Payment, ord *order.Order, sellerAuthNote string, sellerId string) (string, error) {
	authRequest := requests.AuthorizeRequest{
		AmazonOrderReferenceId:   ord.ExternalId(), //NOT SURE ABOUT THIS DATA POINT
		AuthorizationReferenceId: ord.DisplayId(),
		AuthorizationAmount:      types.Price{Amount: ord.DisplayTotal(), CurrencyCode: ord.Currency.Code()},
		TransactionTimeout:       1440,
		CaptureNow:               false,
		SoftDescriptor:           "",
	}

	_, err := xml.Marshal(authRequest)
	if err != nil {
		return "", err
	}

	// send off xml request, get xml response back
	client := urlfetch.Client(c.ctx)

	//log.Debug("Req: %v", req, c.ctx)

	// Marshal request
	data := url.Values{}
	data.Set("AWSAccessKeyId", "amzn1.application-oa2-client.aa882edbf0ab4e2aa936e018928f1e80") // should be login with amazon app id
	data.Set("Action", "Authorize")
	data.Set("AmazonOrderReferenceId", authRequest.AmazonOrderReferenceId)
	data.Set("AuthorizationAmount.Amount", authRequest.AuthorizationAmount.Amount)
	data.Set("AuthorizationAmount.CurrencyCode", authRequest.AuthorizationAmount.CurrencyCode)
	data.Set("AuthorizationReferenceId", authRequest.AuthorizationReferenceId)
	data.Set("MWSAuthToken", "AKIA_REDACTED")    // static for crowdstart, came from amazon payments advanced
	data.Set("SellerAuthorizationNote", sellerAuthNote) // should come from dashboard
	data.Set("SellerId", sellerId)                      // should come from dashboard
	data.Set("SignatureMethod", "")
	data.Set("SignatureVersion", "")
	data.Set("Timestamp", "")
	data.Set("TransactionTimeout", "60")
	data.Set("Version", "")
	data.Set("Signature", "")

	tokenReq, err := http.NewRequest("POST", "", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	res, err := client.Do(tokenReq)
	defer res.Body.Close()
	if err != nil {
		return "", err
	}

	xmlBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	authResponse := responses.AuthorizeResponse{}

	err = xml.Unmarshal(xmlBody, &authResponse)
	if err != nil {
		return "", err
	}
	return authResponse.AuthorizationDetails.AmazonAuthorizationId, nil
}

func (c Client) Capture(pay *payment.Payment, ord *order.Order, authId string, sellerAuthNote string, sellerId string) (string, error) {
	capRequest := requests.CaptureRequest{
		AmazonAuthorizationId: authId,
		CaptureReferenceId:    pay.Key().String(),
		CaptureAmount:         types.Price{Amount: ord.DisplayTotal(), CurrencyCode: ord.Currency.Code()},
		SellerCaptureNote:     "",
		SoftDescriptor:        "",
	}

	_, err := xml.Marshal(capRequest)
	if err != nil {
		return "", err
	}

	// send off xml request, get xml response back
	client := urlfetch.Client(c.ctx)

	data := url.Values{}
	data.Set("AWSAccessKeyId", "amzn1.application-oa2-client.aa882edbf0ab4e2aa936e018928f1e80") // should be login with amazon app id
	data.Set("Action", "Capture")
	data.Set("AmazonAuthorizationId", capRequest.AmazonAuthorizationId)
	data.Set("CaptureAmount.Amount", capRequest.CaptureAmount.Amount)
	data.Set("AuthorizationAmount.CurrencyCode", capRequest.CaptureAmount.CurrencyCode)
	data.Set("CaptureReferenceId", capRequest.CaptureReferenceId)
	data.Set("MWSAuthToken", "AKIA_REDACTED")    // static for crowdstart, came from amazon payments advanced
	data.Set("SellerAuthorizationNote", sellerAuthNote) // should come from dashboard
	data.Set("SellerId", sellerId)                      // should come from dashboard
	data.Set("SignatureMethod", "")
	data.Set("SignatureVersion", "")
	data.Set("Timestamp", "")
	data.Set("TransactionTimeout", "")
	data.Set("Version", "")
	data.Set("Signature", "")

	tokenReq, err := http.NewRequest("POST", "", strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}

	res, err := client.Do(tokenReq)
	defer res.Body.Close()
	if err != nil {
		return "", err
	}

	xmlBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	capResponse := responses.CaptureResponse{}

	err = xml.Unmarshal(xmlBody, &capResponse)
	if err != nil {
		return "", err
	}
	return capResponse.CaptureDetails.CaptureReferenceId, nil
}
