package stripe

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/hanzoai/commerce/payment/processor"
)

// CreateLocalIntent opens the intent. Nothing is credited by it: the money is
// credited only when Stripe reports the intent SUCCEEDED, which for a delayed
// method (Pix, boleto, OXXO, SEPA debit) is by webhook, minutes to days after the
// buyer left. A voucher that is never paid expires and credits nothing.
func (p *Provider) CreateLocalIntent(ctx context.Context, in processor.LocalIntent) (*processor.LocalIntentOut, error) {
	if in.Amount <= 0 || in.Currency == "" {
		return nil, fmt.Errorf("stripe local intent: amount and currency are required")
	}
	params := url.Values{}
	params.Set("amount", strconv.FormatInt(in.Amount, 10))
	params.Set("currency", strings.ToLower(in.Currency))
	params.Set("automatic_payment_methods[enabled]", "true")
	// A voucher lives three days and a Pix code one, so an unpaid one ends by itself.
	params.Set("payment_method_options[boleto][expires_after_days]", "3")
	params.Set("payment_method_options[oxxo][expires_after_days]", "3")
	params.Set("payment_method_options[pix][expires_after_seconds]", "86400")
	if in.Desc != "" {
		params.Set("description", in.Desc)
	}
	if in.Customer != "" {
		params.Set("customer", in.Customer)
	}
	if e := strings.TrimSpace(in.ReceiptEmail); e != "" {
		params.Set("receipt_email", e)
	}
	params.Set("statement_descriptor_suffix", processor.StatementSuffix)
	for k, v := range in.Metadata {
		params.Set("metadata["+k+"]", fmt.Sprintf("%v", v))
	}
	var pi paymentIntent
	if err := p.postKey(ctx, "/payment_intents", params, in.Key, &pi); err != nil {
		return nil, err
	}
	return &processor.LocalIntentOut{ID: pi.ID, ClientSecret: pi.ClientSecret}, nil
}
