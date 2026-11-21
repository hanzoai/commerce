// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import "errors"

// TokenizedCard declares the card handler of rfc.payment_handlers.md: handler name
// dev.acp.tokenized.card, version 2026-01-22, paid with a shared payment token the agent
// obtained from the PSP's delegate_payment endpoint (requires_delegate_payment true, no PCI
// data reaches the seller). The spec URLs are those of the handler specification, as the
// released examples carry them.
//
// id is the handler id agents name in payment_data.handler_id, merchant the seller's
// account at its PSP (the allowance the agent requests is scoped to it) and psp the PSP's
// identifier (for example "stripe"). sandbox declares the sandbox environment instead of
// production (rfc.payment_handlers.md §3.1 names the two). brands and funding, when given,
// list the card brands and funding types accepted.
func TokenizedCard(id, merchant, psp string, sandbox bool, brands, funding []string) (PaymentHandler, error) {
	if id == "" || merchant == "" || psp == "" {
		return PaymentHandler{}, errors.New("acp: a handler needs an id, a merchant account and a psp")
	}
	env := "production"
	if sandbox {
		env = "sandbox"
	}
	cfg := map[string]any{"merchant_id": merchant, "psp": psp, "supports_3ds": false, "environment": env}
	if len(brands) > 0 {
		cfg["accepted_brands"] = brands
	}
	if len(funding) > 0 {
		cfg["accepted_funding_types"] = funding
	}
	return PaymentHandler{
		ID:                      id,
		Name:                    "dev.acp.tokenized.card",
		Version:                 "2026-01-22",
		Spec:                    "https://acp.dev/handlers/tokenized.card",
		RequiresDelegatePayment: true,
		RequiresPCICompliance:   false,
		PSP:                     psp,
		ConfigSchema:            "https://acp.dev/schemas/handlers/tokenized.card/config.json",
		InstrumentSchemas:       []string{"https://acp.dev/schemas/handlers/tokenized.card/instrument.json"},
		Config:                  cfg,
	}, nil
}
