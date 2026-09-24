package billing

import (
	"strings"

	"github.com/hanzoai/commerce/billing/engine"
	gift "github.com/hanzoai/commerce/billing/grant"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/subscription"
)

// What renews a subscription, as the cycle report names it: the source it was
// bought from.
const (
	// sourceCard: the subscription's own saved card.
	sourceCard = "card"
	// sourcePrepaid: the subscriber's prepaid money, credit grants then the
	// balance (prepaid.go), for a plan bought with credits or the balance.
	sourcePrepaid = "prepaid"
	// sourceExternal: a processor outside Hanzo. Nothing here charges it; its
	// next period is recorded when that payment arrives (RecordSubscription).
	sourceExternal = "external"
	// sourceGift: nobody pays; it ends when the time it was granted for does.
	sourceGift = "gift"
	// sourceComped: never charged, and moved on period by period.
	sourceComped = "comped"
	// sourceNone: nothing on file pays it; it ends at its period end.
	sourceNone = "none"
)

// renewalOf is how sub's renewals are paid: from the source it was bought
// from, read off what the row and its first invoice record, in every org. The
// first match wins:
//
//  1. sub.ProviderType is gift.ProviderType (billing/grant): never charged, and it
//     ends when the time it was granted for does — its PeriodEnd, which
//     grant.Grant sets from the grant's duration.
//  2. sub.Type is subscription.External: the processor it was bought through
//     renews it outside Hanzo, and the engine never finds it due (IsDue).
//  3. The first invoice's PaymentMethod: "card" pays by the subscription's
//     card; "credit", "balance" or "prepaid" pays from the subscriber's prepaid
//     money. A "credit" first invoice on a paid ecosystem org's own account is
//     how a plan provisioned on enterprise terms was recorded, and it is comped.
//  4. No first invoice: sub.ProviderType "credit" or DefaultPaymentMethod
//     "credits" or "balance" is 3's prepaid; a card in DefaultPaymentMethod pays
//     by the card; a plan with no fee pays its invoice's metered usage, if any,
//     from prepaid money; an ecosystem org's own account is comped; anything
//     else records no payment and ends at its period end.
//
// A card is only ever the subscription's own DefaultPaymentMethod, never another
// card the subscriber saved. charge is the org's card charger and pre the org's
// prepaid money.
func renewalOf(org *organization.Organization, db *datastore.Datastore, sub *subscription.Subscription, charge engine.ProviderCharger, pre engine.Prepaid) (engine.Collection, string, error) {
	if sub.ProviderType == gift.ProviderType {
		return engine.Collection{End: engine.GiftEnded, Reason: "the gift ran the time it was granted for"}, sourceGift, nil
	}
	if sub.Type == subscription.External {
		return engine.Collection{Reason: "collected outside Hanzo; its next period is recorded when that payment arrives"}, sourceExternal, nil
	}

	eco := IsEcosystemAccount(org, sub.UserId)
	comped := engine.Collection{Comped: true, Reason: "provisioned on an ecosystem org's enterprise terms"}
	byCard := engine.CardPayer(charge)
	byPrepaid := engine.PrepaidPayer(pre)
	card := hasCard(db, sub)

	first, err := engine.FirstInvoice(db, sub)
	if err != nil {
		return engine.Collection{}, "", err
	}
	method := ""
	if first != nil {
		method = first.PaymentMethod
	}
	switch {
	case method != "":
	case sub.ProviderType == "credit" || sub.DefaultPaymentMethod == "credits" || sub.DefaultPaymentMethod == "balance":
		method = "credit"
	}
	switch method {
	case engine.PaidByCard:
		return byCard, sourceCard, nil
	case "credit":
		if eco {
			return comped, sourceComped, nil
		}
		return byPrepaid, sourcePrepaid, nil
	case "balance", engine.PaidByPrepaid:
		return byPrepaid, sourcePrepaid, nil
	}
	switch {
	case card:
		return byCard, sourceCard, nil
	case method == "" && sub.Plan.Price <= 0:
		return byPrepaid, sourcePrepaid, nil
	case method == "" && eco:
		return comped, sourceComped, nil
	}
	return engine.Collection{Reason: "nothing on file pays for it: no card and no recorded way it was bought"}, sourceNone, nil
}

// hasCard reports whether the subscription's DefaultPaymentMethod names a
// vaulted card.
func hasCard(db *datastore.Datastore, sub *subscription.Subscription) bool {
	id := strings.TrimSpace(sub.DefaultPaymentMethod)
	if id == "" || id == "credits" || id == "balance" {
		return false
	}
	pm := paymentmethod.New(db)
	if err := pm.GetById(id); err != nil {
		return false
	}
	return strings.TrimSpace(pm.ProviderRef) != ""
}
