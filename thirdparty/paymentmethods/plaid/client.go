package plaid

import (
	"context"

	"github.com/plaid/plaid-go/v15/plaid"

	. "github.com/hanzoai/commerce/thirdparty/paymentmethods"
)

type Environment plaid.Environment

const (
	SandboxEnvironment     Environment = Environment(plaid.Sandbox)
	DevelopmentEnvironment Environment = Environment(plaid.Development)
	ProductionEnvironment  Environment = Environment(plaid.Production)
)

type Client struct {
	*plaid.APIClient
	ctx context.Context
}

func (c Client) GetPayToken(p PaymentMethodParams) (*PaymentMethodOutput, error) {
	// Exchange the publicToken for an accessToken
	exchangePublicTokenResp, _, err := c.PlaidApi.ItemPublicTokenExchange(c.ctx).ItemPublicTokenExchangeRequest(
		*plaid.NewItemPublicTokenExchangeRequest(p.VerifierToken),
	).Execute()
	accessToken := exchangePublicTokenResp.GetAccessToken()

	// Get Accounts
	accountsGetResp, _, err := c.PlaidApi.AccountsGet(c.ctx).AccountsGetRequest(
		*plaid.NewAccountsGetRequest(accessToken),
	).Execute()
	accountID := accountsGetResp.GetAccounts()[0].GetAccountId()

	if err != nil {
		return nil, err
	}

	// Stripe Round Trip
	// res2, err := client.CreateStripeToken(
	//   res.AccessToken,
	//   p.ExternalUserId,
	// )

	return &PaymentMethodOutput{
		Inputs: p,
		//PayToken:            res2..StripeBankAccountToken,
		PayToken:       accessToken,
		PayTokenId:     accountID,
		ExternalUserId: p.ExternalUserId,
		Type:           PlaidType,
	}, nil
}
