// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync/atomic"

	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/payment/processor"
)

// THE CARD RAIL: the one processor every NEW payment Hanzo takes goes through.
//
// It is one value, "square" or "stripe", and it decides three things together:
// which processor a fresh card is vaulted at, which saved cards may pay for
// something new, and whether the other rails (PayPal, a Square wallet, a crypto
// deposit) still open new payments. With the rail at stripe, only Stripe does.
//
// What it does NOT touch is money already arranged: a subscription renews, a
// dunning retry collects and an automatic reload charges on the card it was set
// up with, whatever the rail says now, and a crypto deposit address already
// minted is still watched and credited. A Square subscriber keeps renewing on
// Square until they save a card at Stripe; nothing they bought stops.
//
// The host decides it (SetRail: on cloud it is a platform switch only a
// SuperAdmin sets). With no host reader it is COMMERCE_CARD_RAIL, and unset it is
// square — the rail this deployment has always used, so a commerce that says
// nothing keeps taking payments exactly as before.

// railReader is the host's reader of the rail; nil reads the environment.
var railReader atomic.Pointer[func(context.Context) string]

// SetRail installs the host's reader of the card rail. It is asked on every new
// payment, so it must answer from memory; an empty answer is the default.
func SetRail(read func(context.Context) string) {
	if read == nil {
		railReader.Store(nil)
		return
	}
	railReader.Store(&read)
}

// Rail is the processor new card payments go through: processor.Stripe or
// processor.Square. Any other value reads as Square.
func Rail(ctx context.Context) processor.ProcessorType {
	v := ""
	if r := railReader.Load(); r != nil {
		v = (*r)(ctx)
	} else {
		v = os.Getenv("COMMERCE_CARD_RAIL")
	}
	if processor.ProcessorType(strings.ToLower(strings.TrimSpace(v))) == processor.Stripe {
		return processor.Stripe
	}
	return processor.Square
}

// Refusal codes the rail answers with.
const (
	// codeRailMoved: the payment named a rail that takes no new payments.
	codeRailMoved = "rail_moved"
	// codeCardReadd: a saved card of a rail that takes no new payments was named
	// for a new one; it keeps paying what it already pays.
	codeCardReadd = "card_readd"
	// codeRequiresAction: the card's bank asked for the buyer (3-D Secure).
	codeRequiresAction = "requires_action"
)

// setterUp readies a card on file for charges made without the buyer (Stripe's
// SetupIntent). A processor that needs no such step has no SetUp.
type setterUp interface {
	SetUp(ctx context.Context, customerID, pm, key, resume string) error
}

// namedRail is the card processor a sale names, "" for none: its Provider when
// that is a card or wallet processor.
func namedRail(in SubscribeIn) processor.ProcessorType {
	switch p := processor.ProcessorType(strings.ToLower(strings.TrimSpace(in.Provider))); p {
	case processor.Square, processor.Stripe, processor.PayPal:
		return p
	}
	return ""
}

// errRailMoved is a new payment on a rail that no longer opens them.
var errRailMoved = errors.New("new payments now go through Stripe by card: reload the page and pay by card; nothing was charged")

// errCardReadd is a saved card of a closed rail named for a new payment.
var errCardReadd = errors.New("this card was saved before card payments moved to Stripe: add it again to pay with it; nothing was charged")

// opens reports whether a NEW payment may be taken on the named processor: the
// rail itself, or — while the rail is Square, as it always was — any of the
// rails Square shared the page with. An empty name is the rail's own.
func opens(ctx context.Context, named processor.ProcessorType) bool {
	rail := Rail(ctx)
	if named == "" || named == rail {
		return true
	}
	return rail == processor.Square
}

// spends reports whether a saved card may pay for something new: one vaulted
// at the rail. A saved card of another rail renews what it already pays for and
// buys nothing new.
func spends(ctx context.Context, pm *paymentmethod.PaymentMethod) bool {
	return kindOf(pm) == Rail(ctx)
}

// IsRailMoved reports whether a payment was refused because the rail it named
// takes no new payments, or because the saved card it named belongs to one.
func IsRailMoved(err error) bool {
	return errors.Is(err, errRailMoved) || errors.Is(err, errCardReadd)
}

// RailCode is the refusal code a rail refusal answers with, "" for any other
// error.
func RailCode(err error) string {
	switch {
	case errors.Is(err, errCardReadd):
		return codeCardReadd
	case errors.Is(err, errRailMoved):
		return codeRailMoved
	}
	return ""
}

// ActionBody is the 402 a page acts on when a card's bank wants the buyer: the
// code requires_action, the sentence, and the intent with its client secret —
// paymentIntent (pi_…) for a payment, setupIntent (seti_…) for a card set up
// without one. The page runs stripe.handleNextAction(clientSecret) and sends the
// same request again, with the same X-Idempotency-Key, naming the intent.
func ActionBody(act *processor.ActionRequired) map[string]any {
	body := map[string]any{
		"error":        act.Error(),
		"code":         codeRequiresAction,
		"clientSecret": act.ClientSecret,
	}
	if act.Setup() {
		body["setupIntent"] = act.Intent
	} else {
		body["paymentIntent"] = act.Intent
	}
	return body
}
