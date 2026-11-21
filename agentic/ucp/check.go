// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package ucp

import (
	"errors"
	"fmt"
	"strings"
)

// CheckTotals validates a totals array: exactly one subtotal and one total,
// the sign convention of the well-known types (discount types negative,
// additive types not), a display_text on every type the specification does not
// define, and sub-lines that sum to their entry (shopping/checkout, "Total";
// schemas/common/types/totals.json).
func CheckTotals(ts []Total) error {
	subtotals, totals := 0, 0
	for i, t := range ts {
		switch t.Type {
		case "subtotal":
			subtotals++
			if t.Amount < 0 {
				return fmt.Errorf("totals[%d]: subtotal must not be negative", i)
			}
		case "total":
			totals++
		case "discount", "items_discount":
			if t.Amount >= 0 {
				return fmt.Errorf("totals[%d]: %s must be negative", i, t.Type)
			}
		case "fulfillment", "tax", "fee":
			if t.Amount < 0 {
				return fmt.Errorf("totals[%d]: %s must not be negative", i, t.Type)
			}
		case "":
			return fmt.Errorf("totals[%d]: type is required", i)
		default:
			if t.DisplayText == "" {
				return fmt.Errorf("totals[%d]: type %q needs display_text", i, t.Type)
			}
		}
		if len(t.Lines) > 0 {
			var sum int64
			for _, l := range t.Lines {
				sum += l.Amount
			}
			if sum != t.Amount {
				return fmt.Errorf("totals[%d]: lines sum to %d, not %d", i, sum, t.Amount)
			}
		}
	}
	if subtotals != 1 || totals != 1 {
		return fmt.Errorf("totals needs exactly one subtotal and one total, has %d and %d", subtotals, totals)
	}
	return nil
}

// Check validates a checkout response against the schema and the normative
// rules of the Checkout capability: the required members, a status of the enum,
// totals, messages, and the status invariants (a continue_url and a requires_*
// message when escalating, an order only once completed, no order while
// complete_in_progress).
func (c *Checkout) Check() error {
	var errs []string
	bad := func(format string, args ...any) { errs = append(errs, fmt.Sprintf(format, args...)) }
	if c.UCP == nil {
		bad("ucp is required")
	} else if c.UCP.Version == "" || c.UCP.PaymentHandlers == nil {
		bad("ucp.version and ucp.payment_handlers are required")
	}
	if c.ID == "" {
		bad("id is required")
	}
	if c.Currency == "" {
		bad("currency is required")
	}
	if c.LineItems == nil {
		bad("line_items is required")
	}
	for i, l := range c.LineItems {
		if l.ID == "" || l.Item.ID == "" || l.Item.Title == "" || l.Item.Price == nil || l.Quantity < 1 {
			bad("line_items[%d] needs id, item.id, item.title, item.price and a quantity of at least 1", i)
		}
		if l.Totals == nil {
			bad("line_items[%d].totals is required", i)
		}
	}
	if err := CheckTotals(c.Totals); err != nil {
		bad("%v", err)
	}
	if c.Links == nil {
		bad("links is required")
	}
	escalating := false
	for i, m := range c.Messages {
		if err := m.Check(); err != nil {
			bad("messages[%d]: %v", i, err)
		}
		escalating = escalating || (m.Type == "error" && (m.Severity == RequiresBuyerInput || m.Severity == RequiresBuyerReview))
	}
	switch c.Status {
	case Incomplete, ReadyForComplete:
	case RequiresEscalation:
		if c.ContinueURL == "" {
			bad("continue_url is required when status is requires_escalation")
		}
		if !escalating {
			bad("requires_escalation needs a message with severity requires_buyer_input or requires_buyer_review")
		}
	case CompleteInProgress:
		if c.Order != nil {
			bad("order must be absent while complete_in_progress")
		}
	case Completed:
		if c.Order == nil {
			bad("order is required when status is completed")
		}
	case Canceled:
	default:
		bad("status %q is not one of the enum", c.Status)
	}
	if c.ContinueURL != "" && !strings.HasPrefix(c.ContinueURL, "https://") {
		bad("continue_url must be an absolute https URL")
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
