package plaid

import (
	"context"

	"github.com/plaid/plaid-go/plaid"
)

type Client struct {
	*plaid.Client
	ctx context.Context
}
