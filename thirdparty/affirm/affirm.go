package affirm

import (
	"encoding/json"

	"crowdstart.com/thirdparty/affirm/requests"
	"crowdstart.com/thirdparty/affirm/responses"

	"appengine"
)

type Client struct {
	ctx                appengine.Context
	publicAccessToken  string
	privateAccessToken string
}

func New(ctx appengine.Context, publicKey string, privateKey string) *Client {

	return &Client{ctx: ctx, publicAccessToken: publicKey, privateAccessToken: privateKey}
}

// Example Request:
// curl https://sandbox.affirm.com/api/v2/charges \
//      -X POST \
//      -u "<public_api_key>:<private_api_key>" \
//      -H "Content-Type:application/json" \
//      -d '{"checkout_token": "<checkout_token>"}'
func (c Client) Authorize(checkoutToken string) (string, error) {
	// <public_api_key> = c.publicAccessToken
	// <private_api_key> = c.privateAccessToken
	// <checkout_token> = checkoutToken

	// Submit via POST

	// You will get back JSON conforming to responses/authorize.goo
	var authResponse responses.AuthorizeResponse
	err := json.Unmarshal(nil, &authResponse)
	if err != nil {
		return "", err
	}

	// Return AuthorizeResponse.Id.  You will need it for the Capture.
	return authResponse.Id, nil
}

// Example request:
// curl https://sandbox.affirm.com/api/v2/charges/<auth_id>/capture \
//		-X POST \
//      -u "<public_api_key>:<private_api_key>" \
//		-H "Content-Type:application/json" \
//      -d '<request_json>'
func (c Client) Capture(authId string, orderId string, shippingCarrier string, shippingConfirmation string) (string, error) {
	// Note the POST target requires the auth ID from the Authorize call.
	// Affirm insists on calling this the Charge ID in their documentation when you go poking around in it, for some reason.

	var capRequest = requests.CaptureRequest{OrderId: orderId, ShippingCarrier: shippingCarrier, ShippingConfirmation: shippingConfirmation}

	_, err := json.Marshal(capRequest)
	if err != nil {
		return "", err
	}

	// Submit via POST, you will get back json conforming to responses/capture.go
	var capResponse responses.CaptureResponse
	err = json.Unmarshal(nil, &capResponse)
	if err != nil {
		return "", err
	}

	return capResponse.Id, nil
}
