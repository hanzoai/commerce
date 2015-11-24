package subscription

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/plan"
	"crowdstart.com/models/subscription"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/stripe"
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
	db := datastore.New(org.Namespace(c))

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

	pln := plan.New(db)
	err = pln.GetById(sub.PlanId)
	if err != nil {
		return nil, nil, err
	}

	usr, err := sr.User()
	if err != nil {
		return nil, nil, err
	}
	log.Debug("User: %#v", usr, c)

	sub.Buyer = usr.Buyer()
	log.Debug("Buyer: %#v", sub.Buyer, c)

	// Override total to $0.50 is test email is used
	if org.IsTestEmail(sub.Buyer.Email) {
		sub.Test = true
	}

	sub.Parent = usr.Key()
	sub.UserId = usr.Id()

	client := stripe.New(ctx, org.StripeToken())
	client.NewSubscription(usr, pln, sub)

	usr.MustPut()
	sub.MustPut()

	return sub, usr, nil
}
