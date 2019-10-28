package plaid

import (
	"context"

	"github.com/plaid/plaid-go/plaid"

	. "hanzo.io/thirdparty/paymentmethods"
)

type Environment plaid.Environment

const (
	SandboxEnvironment     Environment = Environment(plaid.Sandbox)
	DevelopmentEnvironment Environment = Environment(plaid.Development)
	ProductionEnvironment  Environment = Environment(plaid.Production)
)

type Client struct {
	*plaid.Client
	ctx context.Context
}

func (c Client) GetPayToken(p PaymentMethodParams) (*PaymentMethodOutput, error) {
	res, err := c.ExchangePublicToken(p.VerifierToken)

	if err != nil {
		return nil, err
	}

	// Stripe Round Trip
	// res2, err := client.CreateStripeToken(
	//   res.AccessToken,
	//   p.ExternalUserId,
	// )

	return &PaymentMethodOutput{
		PaymentMethodParams: p,
		//PayToken:            res2..StripeBankAccountToken,
		PayToken:       res.AccessToken,
		PayTokenId:     res.ItemID,
		ExternalUserId: p.ExternalUserId,
		Type:           PlaidType,
	}, nil
}
