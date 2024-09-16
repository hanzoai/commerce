package account

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/log"
	"hanzo.io/middleware"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
)

func get(c *gin.Context) {
	org := middleware.GetOrganization(c)
	usr := middleware.GetUser(c)

	if err := usr.LoadReferrals(); err != nil {
		http.Fail(c, 500, "User referral data could get be queried", err)
		return
	}

	if err := usr.LoadPaymentMethods(); err != nil {
		http.Fail(c, 500, "User paymentmethods data could get be queried", err)
		return
	}

	if err := usr.LoadOrders(); err != nil {
		http.Fail(c, 500, "User order data could get be queried", err)
		return
	}

	if err := usr.LoadAffiliateAndPendingFees(); err != nil {
		http.Fail(c, 500, "User affiliate '"+usr.AffiliateId+"' could get be queried", err)
		return
	}

	if err := usr.LoadTokenTransactions(); err != nil {
		http.Fail(c, 500, "User token transaction data could get be queried", err)
		return
	}

	if err := usr.CalculateBalances(!org.Live); err != nil {
		http.Fail(c, 500, "User balance data could get be queried", err)
		return
	}

	http.Render(c, 200, usr)
}

func update(c *gin.Context) {
	// org := middleware.GetOrganization(c)
	// db := datastore.New(org.Namespaced(c))
	// usr := middleware.GetUser(c)

	// id := usr.Id()
	// newUsr := user.New(db)
	// if err := json.Decode(c.Request.Body, newUsr); err != nil {
	// 	newUsr.SetKey(id)
	// }

	// if err := newUsr.Put(); err != nil {
	// 	http.Fail(c, 400, "Failed to update user", err)
	// } else {
	// 	http.Render(c, 200, usr)
	// }
}

func patch(c *gin.Context) {
	org := middleware.GetOrganization(c)
	usr := middleware.GetUser(c)
	ctx := org.Db.Context

	usr.Email = strings.ToLower(strings.TrimSpace(usr.Email))

	req := &confirmPasswordReq{User: usr}

	usr2 := user.New(usr.Db)
	// Email can't already exist or if it does, can't have a password
	if err := usr2.GetByEmail(req.Email); err == nil {
		if usr2.Id() != usr.Id() {
			http.Fail(c, 400, "Email is already taken", err)
			return
		}
	}

	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if req.Password != "" {
		if !password.HashAndCompare(usr.PasswordHash, req.CurrentPassword) {
			http.Fail(c, 401, "Password is incorrect", errors.New("password is incorrect"))
			return
		}
		if err := resetPassword(usr, req); err != nil {
			switch err {
			case ErrPasswordMismatch, ErrPasswordMinLength:
				http.Fail(c, 400, err.Error(), err)
			default:
				http.Fail(c, 500, err.Error(), err)
			}
			return
		}
	}

	if err := usr.Put(); err != nil {
		http.Fail(c, 400, "Failed to update user", err)
	} else {
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

		http.Render(c, 200, usr)
	}
}
