package affiliate

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/affiliate"
	"github.com/hanzoai/commerce/models/order"
	mdlreferral "github.com/hanzoai/commerce/models/referral"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rest"
)

// affiliateConnect initiates payment processor OAuth for an affiliate.
// Affiliate payout rail pending — the transfer records what is owed.
func affiliateConnect(c *zip.Ctx) error {
	return http.Fail(c, 503, "affiliate payment connect not available", errors.New("payment processor connect not configured"))
}

// affiliateGetReferrals returns all referrals for an affiliate.
//
//	GET /v1/affiliate/:affiliateid/referrals
func affiliateGetReferrals(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("affiliateid")

	referrals := make([]mdlreferral.Referral, 0)
	if _, err := mdlreferral.Query(db).Filter("Referrer.AffiliateId=", id).GetAll(&referrals); err != nil {
		return http.Fail(c, 400, "Could not query referral", err)
	}

	return http.Render(c, 200, referrals)
}

// affiliateGetReferrers returns all referrers for an affiliate.
//
//	GET /v1/affiliate/:affiliateid/referrers
func affiliateGetReferrers(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("affiliateid")

	referrers := make([]referrer.Referrer, 0)
	if _, err := referrer.Query(db).Filter("AffiliateId=", id).GetAll(&referrers); err != nil {
		return http.Fail(c, 400, "Could not query referrer", err)
	}

	return http.Render(c, 200, referrers)
}

// affiliateGetOrders returns all orders attributed to an affiliate.
//
//	GET /v1/affiliate/:affiliateid/orders
func affiliateGetOrders(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("affiliateid")

	orders := make([]order.Order, 0)
	if _, err := order.Query(db).Filter("AffiliateId=", id).GetAll(&orders); err != nil {
		return http.Fail(c, 400, "Could not query order", err)
	}

	return http.Render(c, 200, orders)
}

// affiliateGetTransactions returns all transactions for an affiliate.
//
//	GET /v1/affiliate/:affiliateid/transactions
func affiliateGetTransactions(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("affiliateid")

	trans := make([]transaction.Transaction, 0)
	if _, err := transaction.Query(db).Filter("Test=", false).Filter("AffiliateId=", id).GetAll(&trans); err != nil {
		return http.Fail(c, 400, "Could not query transaction", err)
	}

	return http.Render(c, 200, trans)
}

// affiliateCreate returns the custom Create handler for affiliate CRUD.
func affiliateCreate(r *rest.Rest) func(*zip.Ctx) error {
	return func(c *zip.Ctx) error {
		if !r.CheckPermissions(c, "create") {
			return nil
		}

		db := datastore.New(c.Context())
		aff := affiliate.New(db)

		// Decode request
		if err := json.DecodeBytes(c.Body(), aff); err != nil {
			return r.Fail(c, 400, "Failed decode request body", err)
		}

		// Affiliates can only be created for pre-existing users
		if aff.UserId == "" {
			return r.Fail(c, 500, "UserId required", errors.New("UserId required"))
		}

		// Find user which will be turned into affiliate
		usr := user.New(db)
		if err := usr.GetById(aff.UserId); err != nil {
			return r.Fail(c, 500, "User does not exist: "+aff.UserId, err)
		}

		// Don't create multiple affiliates per user
		if usr.AffiliateId != "" {
			return r.Fail(c, 500, "User already is affiliate: "+usr.AffiliateId, errors.New("User already is affiliate: "+usr.AffiliateId))
		}

		// Create affiliate
		if err := aff.Create(); err != nil {
			return r.Fail(c, 500, "Failed to create "+r.Kind, err)
		}

		// Create special referrer for affiliate
		ref := referrer.New(db)
		ref.AffiliateId = aff.Id()
		ref.UserId = usr.Id()
		if err := ref.Create(); err != nil {
			return r.Fail(c, 500, "Failed to create "+r.Kind, err)
		}

		// Update user with affiliate information
		usr.AffiliateId = aff.Id()
		if err := usr.Update(); err != nil {
			return r.Fail(c, 500, "Failed to update user: "+usr.Id(), err)
		}

		c.SetHeader("Location", c.Path()+"/"+aff.Id())
		return r.Render(c, 201, aff)
	}
}
