package subscription

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/api/subscription/stripe"
	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/plan"
	"crowdstart.com/models/subscription"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json"
	"crowdstart.com/util/log"
)

func subscriptionRequest(c *gin.Context, org *organization.Organization) (*SubscriptionReq, error) {
	// Create AuthReq properly by calling order.New
	sr := new(SubscriptionReq)
	sr.Db = datastore.New(org.Namespace(c))

	// Try decode request body
	if err := json.Decode(c.Request.Body, &sr); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Request.Body, err, c)
		return nil, FailedToDecodeRequestBody
	}

	return sr, nil
}

func subscribe(c *gin.Context, org *organization.Organization) (*subscription.Subscription, *user.User, error) {
	ctx := org.Db.Context
	nsCtx := org.Namespace(ctx)
	db := datastore.New(nsCtx)

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

	if sub.Quantity < 1 {
		sub.Quantity = 1
	}

	pln := plan.New(db)
	err = pln.GetById(sub.PlanId)
	if err != nil {
		return nil, nil, PlanDoesNotExist
	}
	log.Debug("Plan: %#v", pln, c)

	usr, err := sr.User()
	if err != nil {
		return nil, nil, err
	}
	log.Debug("User: %#v", usr, c)

	sub.Buyer = usr.Buyer()
	log.Debug("Buyer: %#v", sub.Buyer, c)

	if org.IsTestEmail(sub.Buyer.Email) {
		sub.Test = true
	}

	sub.Parent = usr.Key()
	sub.UserId = usr.Id()
	sub.PlanId = pln.Id()
	sub.Plan = *pln

	err = stripe.Subscribe(org, usr, sub)
	if err != nil {
		return nil, nil, err
	}

	usr.MustPut()
	sub.MustPut()

	return sub, usr, nil
}

func updateSubscribe(c *gin.Context, org *organization.Organization, sub *subscription.Subscription) (*subscription.Subscription, error) {
	ctx := org.Db.Context
	nsCtx := org.Namespace(ctx)
	db := datastore.New(nsCtx)

	userId := sub.UserId

	// Try decode request body
	if err := json.Decode(c.Request.Body, &sub); err != nil {
		log.Error("Failed to decode request body: %v\n%v", c.Request.Body, err, c)
		return nil, FailedToDecodeRequestBody
	}

	if userId != sub.UserId {
		return nil, CannotChangeUser
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

	sub.Plan = *pln

	err = stripe.UpdateSubscription(org, sub)
	if err != nil {
		return nil, err
	}

	sub.MustPut()

	return sub, nil
}

func unsubscribe(c *gin.Context, org *organization.Organization, sub *subscription.Subscription) (*subscription.Subscription, error) {
	err := stripe.Unsubscribe(org, sub)
	if err != nil {
		return nil, err
	}

	sub.MustPut()

	return sub, nil
}
