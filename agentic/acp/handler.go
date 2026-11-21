// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package acp

import (
	"errors"
	"maps"
)

// TokenizedCard declares the handler dev.acp.tokenized.card (rfc.payment_handlers.md §7.1,
// §8.1): a card paid with a shared payment token from the PSP's delegate_payment endpoint. The
// URLs are those of the released examples. id is the handler id agents name in
// payment_data.handler_id; config is the handler config, which must carry the seller's
// merchant_id (§10); psp is added to it.
func TokenizedCard(id, psp string, config map[string]any) (PaymentHandler, error) {
	if m, _ := config["merchant_id"].(string); id == "" || psp == "" || m == "" {
		return PaymentHandler{}, errors.New("acp: a handler needs an id, a psp and a config with a merchant_id")
	}
	cfg := maps.Clone(config)
	cfg["psp"] = psp
	return PaymentHandler{
		ID:                      id,
		Name:                    "dev.acp.tokenized.card",
		Version:                 "2026-01-22",
		Spec:                    "https://acp.dev/handlers/tokenized.card",
		RequiresDelegatePayment: true,
		PSP:                     psp,
		ConfigSchema:            "https://acp.dev/schemas/handlers/tokenized.card/config.json",
		InstrumentSchemas:       []string{"https://acp.dev/schemas/handlers/tokenized.card/instrument.json"},
		Config:                  cfg,
	}, nil
}
