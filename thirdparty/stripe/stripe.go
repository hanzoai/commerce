package stripe

import (
	"context"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/client"
)

func New(ctx context.Context, accessToken string) *Client {
	// Set deadline
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)

	httpClient.Transport = &urlfetch.Transport{
		Context: ctx,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}
	stripe.SetBackend(stripe.APIBackend, nil)
	stripe.SetHTTPClient(httpClient)

	sc := &client.API{}
	sc.Init(accessToken, nil)
	return &Client{sc, ctx}
}

// Enable debug logging in development
func init() {
	if appengine.IsDevAppServer() {
		stripe.LogLevel = 2
	}
}

// Subscribe to a plan
func (c Client) NewSubscription(token string, source interface{}, sub *subscription.Subscription) (*Sub, error) {
	log.Debug("sub.Plan %v", sub.Plan)
	params := &stripe.SubParams{
		Plan: sub.Plan.StripeId,
	}

	switch v := source.(type) {
	case *Customer:
		params.Customer = v.ID
	case *user.User:
		params.Customer = v.Accounts.Stripe.CustomerId
		params.AddMeta("user", v.Id())
	}

	params.AddMeta("plan", sub.Plan.Id())

	s, err := c.Subs.New(params)
	if err != nil {
		return nil, errors.New(err)
	}

	sub.Account.SubscriptionId = s.ID
	sub.Account.CustomerId = s.Customer.ID
	sub.FeePercent = s.FeePercent
	sub.EndCancel = s.EndCancel
	sub.PeriodStart = time.Unix(s.PeriodStart, 0)
	sub.PeriodEnd = time.Unix(s.PeriodEnd, 0)
	// sub.Start = time.Unix(s.Start, 0)
	sub.Ended = time.Unix(s.Ended, 0)
	sub.TrialStart = time.Unix(s.TrialStart, 0)
	sub.TrialEnd = time.Unix(s.TrialEnd, 0)

	sub.Quantity = int(s.Quantity)
	sub.Status = string(s.Status)

	return (*Sub)(s), nil
}

// Update subscribe to a plan
func (c Client) UpdateSubscription(sub *subscription.Subscription) (*Sub, error) {
	params := &stripe.SubParams{
		Customer: sub.Account.CustomerId,
		Plan:     sub.Plan.StripeId,
		Quantity: uint64(sub.Quantity),
	}

	params.AddMeta("plan", sub.Plan.Id())

	s, err := c.Subs.Update(sub.Account.SubscriptionId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	sub.Account.SubscriptionId = s.ID
	sub.Account.CustomerId = s.Customer.ID
	sub.FeePercent = s.FeePercent
	sub.EndCancel = s.EndCancel
	sub.PeriodStart = time.Unix(s.PeriodStart, 0)
	sub.PeriodEnd = time.Unix(s.PeriodEnd, 0)
	// sub.Start = time.Unix(s.Start, 0)
	sub.Ended = sub.PeriodEnd
	sub.TrialStart = time.Unix(s.TrialStart, 0)
	sub.TrialEnd = time.Unix(s.TrialEnd, 0)

	sub.Quantity = int(s.Quantity)
	sub.Status = string(s.Status)

	return (*Sub)(s), nil
}

// Subscribe to a plan
func (c Client) CancelSubscription(sub *subscription.Subscription) (*Sub, error) {
	params := &stripe.SubParams{
		Customer:  sub.Account.CustomerId,
		EndCancel: true,
	}
	s, err := c.Subs.Get(sub.Account.SubscriptionId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	_, err = c.Subs.Cancel(sub.Account.SubscriptionId, params)
	if err != nil {
		return nil, errors.New(err)
	}

	sub.Account.SubscriptionId = s.ID
	sub.Account.CustomerId = s.Customer.ID
	sub.FeePercent = s.FeePercent
	sub.EndCancel = s.EndCancel
	sub.PeriodStart = time.Unix(s.PeriodStart, 0)
	sub.PeriodEnd = time.Unix(s.PeriodEnd, 0)
	// sub.Start = time.Unix(s.Start, 0)
	sub.Ended = sub.PeriodEnd
	sub.TrialStart = time.Unix(s.TrialStart, 0)
	sub.TrialEnd = time.Unix(s.TrialEnd, 0)
	sub.CanceledAt = time.Now()
	sub.EndCancel = true

	sub.Quantity = int(s.Quantity)
	sub.Status = string(s.Status)

	return (*Sub)(s), nil
}
