package plaid

import (
	"context"

	"github.com/plaid/plaid-go/plaid"

	. "hanzo.io/thirdparty/paymentmethods"
)

type Client struct {
	*plaid.Client
	ctx context.Context
}

func (c Client) GetPayToken(p PaymentMethodParams) (*PaymentMethodOutput, error) {
	res, err := c.ExchangePublicToken(p.PublicToken)

	if err != nil {
		return nil, err
	}

	return &PaymentMethodOutput{
		PayToken:   res.AccessToken,
		PayTokenId: res.ItemID,
		Type:       PlaidType,
	}, nil
}
