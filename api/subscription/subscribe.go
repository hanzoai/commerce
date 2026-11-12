package subscription

import (
	"github.com/zap-proto/zip"

	billing "github.com/hanzoai/commerce/api/billing"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/plan"
	"github.com/hanzoai/commerce/models/subscription"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/json"
)

func subscriptionRequest(c *zip.Ctx, org *organization.Organization) (*SubscriptionReq, error) {
	// Create AuthReq properly by calling order.New
	sr := new(SubscriptionReq)
	sr.Db = datastore.New(org.Namespaced(c.Context()))

	// Try decode request body
	if err := json.DecodeBytes(c.Body(), &sr); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Body(), err, c)
		return nil, FailedToDecodeRequestBody
	}

	return sr, nil
}

func subscribe(c *zip.Ctx, org *organization.Organization) (*subscription.Subscription, *user.User, error) {
	ctx := org.Datastore().Context
	nsCtx := org.Namespaced(ctx)
	db := datastore.New(nsCtx)

	// Parse request
	sr, err := subscriptionRequest(c, org)
	if err != nil {
		return nil, nil, err
	}

	log.Debug("AuthorizationReq.User_: %#v", sr.User_, c)
	log.Debug("AuthorizationReq.Subscription_: %#v", sr.Subscription_, c)

	sub, err := sr.Subscription()
	if err != nil {
		return nil, nil, err
	}
	log.Debug("Subscription: %#v", sub, c)

	// The providers commerce's own billing writes are never the body's to name.
	if billing.Reserved(sub.ProviderType) {
		return nil, nil, ReservedProvider
	}

	// Subscription w/o quantity defaults to 1
	if sub.Quantity < 1 {
		sub.Quantity = 1
	}

	// Try to find plan
	pln := plan.New(db)
	err = pln.GetById(sub.PlanId)
	if err != nil {
		return nil, nil, PlanDoesNotExist
	}
	log.Debug("Plan: %#v", pln, c)

	// Get user
	usr, err := sr.User()
	if err != nil {
		return nil, nil, err
	}
	log.Debug("User: %#v", usr, c)

	// Payment information
	sub.Buyer = usr.Buyer()
	log.Debug("Buyer: %#v", sub.Buyer, c)

	// The mode is the server's: the org's, or a test buyer's. Never the body's.
	sub.Test = org.TestMode() || org.IsTestEmail(sub.Buyer.Email)

	// Parent subscription to user
	sub.Parent = usr.Key()
	sub.UserId = usr.Id()

	// Set plan on subscription
	sub.PlanId = pln.Id()
	sub.Plan = *pln

	// Save user and subscription
	usr.MustPut()
	sub.MustPut()

	return sub, usr, nil
}

func updateSubscribe(c *zip.Ctx, org *organization.Organization, sub *subscription.Subscription) (*subscription.Subscription, error) {
	ctx := org.Datastore().Context
	nsCtx := org.Namespaced(ctx)
	db := datastore.New(nsCtx)

	// A Hanzo subscription row shares the org's namespace with the merchant's
	// customers' rows; its billing is commerce's, never this API's to edit.
	if billing.Owns(ctx, sub) {
		return nil, HanzoSubscription
	}

	userId := sub.UserId
	test := sub.Test

	// Try decode request body
	if err := json.DecodeBytes(c.Body(), &sub); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Body(), err, c)
		return nil, FailedToDecodeRequestBody
	}
	sub.Test = test

	if userId != sub.UserId {
		return nil, CannotChangeUser
	}
	if billing.Reserved(sub.ProviderType) {
		return nil, ReservedProvider
	}

	log.Warn("Quantity %v", sub.Quantity)

	// Delete Case
	if sub.Quantity < 1 {
		return unsubscribe(c, org, sub)
	}

	pln := plan.New(db)
	err := pln.GetById(sub.PlanId)
	if err != nil {
		return nil, PlanDoesNotExist
	}
	log.Debug("Plan: %#v", pln, c)

	// The row names its plan by the plan's id, as a subscribe writes it, never by
	// the body's string.
	sub.PlanId = pln.Id()
	sub.Plan = *pln

	sub.MustPut()

	return sub, nil
}

func unsubscribe(c *zip.Ctx, org *organization.Organization, sub *subscription.Subscription) (*subscription.Subscription, error) {
	_ = c
	_ = org

	sub.MustPut()

	return sub, nil
}
