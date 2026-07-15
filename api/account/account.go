package account

import (
	"errors"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/auth/password"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/thirdparty/mailchimp"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

func get(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	usr := middleware.GetUser(c)

	if err := usr.LoadReferrals(); err != nil {
		return http.Fail(c, 500, "User referral data could get be queried", err)
	}

	if err := usr.LoadPaymentMethods(); err != nil {
		return http.Fail(c, 500, "User paymentmethods data could get be queried", err)
	}

	if err := usr.LoadOrders(); err != nil {
		return http.Fail(c, 500, "User order data could get be queried", err)
	}

	if err := usr.LoadAffiliateAndPendingFees(); err != nil {
		return http.Fail(c, 500, "User affiliate '"+usr.AffiliateId+"' could get be queried", err)
	}

	if err := usr.LoadTokenTransactions(); err != nil {
		return http.Fail(c, 500, "User token transaction data could get be queried", err)
	}

	if err := usr.CalculateBalances(!org.Live); err != nil {
		return http.Fail(c, 500, "User balance data could get be queried", err)
	}

	return http.Render(c, 200, usr)
}

func update(c *zip.Ctx) error {
	// org := middleware.GetOrganization(c)
	// db := datastore.New(org.Namespaced(c.Context()))
	// usr := middleware.GetUser(c)

	// id := usr.Id()
	// newUsr := user.New(db)
	// if err := json.DecodeBytes(c.Body(), newUsr); err != nil {
	// 	newUsr.SetKey(id)
	// }

	// if err := newUsr.Put(); err != nil {
	// 	http.Fail(c, 400, "Failed to update user", err)
	// } else {
	// 	http.Render(c, 200, usr)
	// }
	return nil
}

func patch(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	usr := middleware.GetUser(c)
	ctx := org.Datastore().Context

	usr.Email = strings.ToLower(strings.TrimSpace(usr.Email))

	req := &confirmPasswordReq{User: usr}

	usr2 := user.New(usr.Datastore())
	// Email can't already exist or if it does, can't have a password
	if err := usr2.GetByEmail(req.Email); err == nil {
		if usr2.Id() != usr.Id() {
			return http.Fail(c, 400, "Email is already taken", err)
		}
	}

	if err := json.DecodeBytes(c.Body(), req); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	if req.Password != "" {
		if !password.HashAndCompare(usr.PasswordHash, req.CurrentPassword) {
			return http.Fail(c, 401, "Password is incorrect", errors.New("password is incorrect"))
		}
		if err := resetPassword(usr, req); err != nil {
			switch err {
			case ErrPasswordMismatch, ErrPasswordMinLength:
				return http.Fail(c, 400, err.Error(), err)
			default:
				return http.Fail(c, 500, err.Error(), err)
			}
		}
	}

	if err := usr.Put(); err != nil {
		return http.Fail(c, 400, "Failed to update user", err)
	}

	// Create new mailchimp client
	client := mailchimp.New(ctx, org.Mailchimp)

	// Determine store to use
	storeId := usr.StoreId
	if storeId == "" {
		storeId = org.DefaultStore
	}

	// Update customer in mailchimp for this user
	if err := client.UpdateCustomer(storeId, usr); err != nil {
		log.Warn("Failed to update Mailchimp customer: %v", err, ctx)
	}

	return http.Render(c, 200, usr)
}
