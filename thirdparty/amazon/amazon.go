package amazon

import (
	"encoding/xml"
	"time"

	"github.com/stripe/stripe-go/client"

	"crowdstart.com/models/order"
	"crowdstart.com/models/payment"
	"crowdstart.com/thirdparty/amazon/requests"
	"crowdstart.com/thirdparty/amazon/responses"
	"crowdstart.com/thirdparty/amazon/types"

	"appengine"
	"appengine/urlfetch"
)

type Client struct {
	*client.API
	ctx appengine.Context
}

func New(ctx appengine.Context, accessToken string) *Client {
	httpClient := urlfetch.Client(ctx)
	httpClient.Transport = &urlfetch.Transport{
		Context:  ctx,
		Deadline: time.Duration(20) * time.Second, // Deadline to 10 seconds
	}

	return &Client{}
}

func (c Client) Authorize(pay *payment.Payment, ord *order.Order) (string, error) {
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

	authResponse := responses.AuthorizeResponse{}

	err = xml.Unmarshal(nil, &authResponse)
	return authResponse.AuthorizationDetails.AmazonAuthorizationId, nil
}
