package user

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth/password"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/demo/tokentransaction"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/affiliate"
	"github.com/hanzoai/commerce/models/deprecated/subscription"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/referral"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/transfer"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/util/json/http"
	"github.com/hanzoai/commerce/util/rand"
)

type Password struct {
	Password string `json:"password"`
}

func resetPassword(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	u := user.New(db)
	if err := u.GetById(id); err != nil {
		return http.Fail(c, 400, "Could not query user", err)
	}

	newPassword := rand.ShortPassword()
	if hash, err := password.Hash(newPassword); err != nil { // pragma: allowlist secret
		return http.Fail(c, 400, "Password generation failed", err)
	} else {
		u.PasswordHash = hash
	}

	u.MustPut()
	return http.Render(c, 200, Password{Password: newPassword})
}

func getReferrals(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	referrals := make([]referral.Referral, 0)
	if _, err := referral.Query(db).Filter("Referrer.UserId=", id).GetAll(&referrals); err != nil {
		return http.Fail(c, 400, "Could not query referral", err)
	}

	return http.Render(c, 200, referrals)
}

func getReferrers(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	referrers := make([]referrer.Referrer, 0)
	if _, err := referrer.Query(db).Filter("UserId=", id).GetAll(&referrers); err != nil {
		return http.Fail(c, 400, "Could not query referrer", err)
	}

	return http.Render(c, 200, referrers)
}

func getPaymentMethods(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	paymentMethods := make([]paymentmethod.PaymentMethod, 0)
	if _, err := paymentmethod.Query(db).Filter("UserId=", id).GetAll(&paymentMethods); err != nil {
		return http.Fail(c, 400, "Could not query paymentMethod", err)
	}

	return http.Render(c, 200, paymentMethods)
}

func getSubscriptions(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	subscriptions := make([]subscription.Subscription, 0)
	if _, err := subscription.Query(db).Filter("UserId=", id).GetAll(&subscriptions); err != nil {
		return http.Fail(c, 400, "Could not query subscription", err)
	}

	return http.Render(c, 200, subscriptions)
}

func getOrders(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	orders := make([]order.Order, 0)
	if _, err := order.Query(db).Filter("UserId=", id).GetAll(&orders); err != nil {
		return http.Fail(c, 400, "Could not query order", err)
	}

	return http.Render(c, 200, orders)
}

func getTransactions(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c.Context())
	id := c.Param("userid")

	res, err := util.GetTransactions(ctx, id, "user", !org.Live)
	if err != nil {
		log.Error("transaction/%v/%v error: '%v'", id, "user", err, c)
		return http.Fail(c, 400, err.Error(), err)
	}

	return http.Render(c, 200, res)
}

func getTokenTransactions(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	tt := make([]*tokentransaction.Transaction, 0)
	if _, err := tokentransaction.Query(db).Filter("SendingUserId=", id).GetAll(&tt); err != nil {
		log.Error("tokentransaction/%v/%v error: '%v'", id, "user", err, c)
		return http.Fail(c, 400, err.Error(), err)
	}

	tt2 := make([]*tokentransaction.Transaction, 0)
	if _, err := tokentransaction.Query(db).Filter("ReceivingUserId=", id).GetAll(&tt2); err != nil {
		log.Error("tokentransaction/%v/%v error: '%v'", id, "user", err, c)
		return http.Fail(c, 400, err.Error(), err)
	}

	tt3 := append(tt, tt2...)

	return http.Render(c, 200, tt3)
}

func getTransfers(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	usr := user.New(db)
	if err := usr.GetById(id); err != nil {
		return http.Fail(c, 400, "Could not query user", err)
	}

	trans := make([]transfer.Transfer, 0)
	if _, err := transfer.Query(db).Filter("PayeeId=", usr.AffiliateId).GetAll(&trans); err != nil {
		return http.Fail(c, 400, "Could not query transfer", err)
	}

	return http.Render(c, 200, trans)
}

func getAffiliate(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c.Context()))
	id := c.Param("userid")

	usr := user.New(db)
	if err := usr.GetById(id); err != nil {
		return http.Fail(c, 400, "Could not query user", err)
	}

	aff := affiliate.New(db)
	if err := aff.GetById(usr.AffiliateId); err != nil {
		return http.Fail(c, 400, "Could not query affiliate", err)
	}

	return http.Render(c, 200, aff)
}
