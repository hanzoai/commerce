// Copyright (c) 2014-present Hanzo AI, Inc.
// Licensed under MIT OR Apache-2.0. See LICENSE-MIT and LICENSE-APACHE.

package billing

import (
	"context"
	"fmt"
	netmail "net/mail"
	"os"
	"strings"
	"time"

	"github.com/hanzoai/commerce/billing/engine"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/mail"
	"github.com/hanzoai/commerce/models/plan"
)

// saleNotifyEnv names the addresses told about every paid plan sale, comma
// separated. Unset tells nobody. The value is deployment configuration, set by
// the operator; no address is written in source.
const saleNotifyEnv = "BILLING_SALE_NOTIFY_TO"

// saleNoticeTimeout bounds one notice's delivery. The sale has already answered
// by then; this only stops a stuck mail rail from holding a goroutine forever.
const saleNoticeTimeout = 30 * time.Second

// saleNote is one paid sale, as the operator is told about it.
type saleNote struct {
	Org       string
	Email     string
	Plan      *plan.Plan // the plan as bought: its name, slug and period
	Seats     int64
	Sale      *Sale
	ListCents int64 // the first period at catalog price, before promo and coupon
	PromoName string
	Coupon    engine.Coupon
	Test      bool
}

// noticeSale tells the operator about a sale that is already sealed. It never
// holds the sale up and never fails it: delivery runs on its own goroutine,
// detached from the request, and a failure is logged and dropped. It is called
// once per fresh sale and never for a replay, which returns before a sale is made.
func noticeSale(ctx context.Context, n saleNote) {
	to := saleRecipients()
	if len(to) == 0 || n.Sale == nil {
		return
	}
	subject, body := n.render(time.Now().UTC())
	go func() {
		// A panic in the mail rail is the notice's failure, never the process's:
		// this goroutine runs inside whatever binary embeds commerce.
		defer func() {
			if r := recover(); r != nil {
				log.Error("sale notice for subscription %s (org %s) panicked: %v", n.Sale.SubscriptionID, n.Org, r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), saleNoticeTimeout)
		defer cancel()
		if err := mail.Send(ctx, to, subject, body); err != nil {
			log.Warn("sale notice for subscription %s (org %s) was not sent: %v", n.Sale.SubscriptionID, n.Org, err)
		}
	}()
}

// saleRecipients reads saleNotifyEnv: every well-formed address in it.
func saleRecipients() []string {
	var to []string
	for _, a := range strings.Split(os.Getenv(saleNotifyEnv), ",") {
		if addr, err := netmail.ParseAddress(strings.TrimSpace(a)); err == nil {
			to = append(to, addr.Address)
		}
	}
	return to
}

// render is the notice's subject and plain-text body.
func (n saleNote) render(at time.Time) (subject, body string) {
	cur := strings.ToUpper(n.Sale.Currency)
	name := n.Plan.Name
	if name == "" {
		name = n.Sale.PlanID
	}
	coupon := "none"
	if label := n.Coupon.Label(); label != "" {
		coupon = label
	}
	email := strings.TrimSpace(n.Email)
	if email == "" {
		email = "(not given)"
	}
	mode := "live"
	if n.Test {
		mode = "test"
	}

	subject = fmt.Sprintf("Sale: %s (%s) for %s, %s", name, n.Sale.Interval, n.Org, cents(n.Sale.AmountCents, cur))
	if n.Coupon.Code != "" {
		subject += ", coupon " + n.Coupon.Code
	}

	var b strings.Builder
	line := func(k, v string) { fmt.Fprintf(&b, "%-16s%s\n", k+":", v) }
	b.WriteString("A paid plan subscription was sold.\n\n")
	line("Org", n.Org)
	line("Customer email", email)
	line("Plan", fmt.Sprintf("%s (%s)", name, n.Sale.PlanID))
	line("Interval", string(n.Sale.Interval))
	if n.Seats > 1 {
		line("Seats", fmt.Sprintf("%d", n.Seats))
	}
	line("Charged", cents(n.Sale.AmountCents, cur))
	line("List price", cents(n.ListCents, cur))
	if n.PromoName != "" {
		line("Promo", n.PromoName)
	}
	line("Coupon", coupon)
	line("Time", at.Format(time.RFC3339))
	line("Subscription", n.Sale.SubscriptionID)
	line("Invoice", n.Sale.InvoiceID)
	line("Mode", mode)
	return subject, b.String()
}

// cents renders minor units as an amount: 1050, "USD" is "10.50 USD".
func cents(v int64, cur string) string {
	return fmt.Sprintf("%d.%02d %s", v/100, v%100, cur)
}
