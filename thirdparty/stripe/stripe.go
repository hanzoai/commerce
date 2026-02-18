package stripe

import (
	"context"
	"net/http"
	"time"

	stripe "github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/client"
)

func New(ctx context.Context, accessToken string) *Client {
	// Set deadline
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	// Set HTTP Client
	httpClient := &http.Client{Timeout: 55 * time.Second}

	stripe.SetBackend(stripe.APIBackend, nil)
	stripe.SetHTTPClient(httpClient)

	sc := &client.API{}
	sc.Init(accessToken, nil)
	return &Client{sc, ctx}
}
