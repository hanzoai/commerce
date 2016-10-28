package account

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/mailchimp"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
)

type userResp struct {
	*user.User
	Referrals []referral.Referral `json:"referrals,omitempty" datastore:"-"`
	Referrers []referrer.Referrer `json:"referrers,omitempty" datastore:"-"`
	Orders    []order.Order       `json:"orders,omitempty" datastore:"-"`
}

func loadReferrals(u *userResp) error {
	if _, err := referrer.Query(u.Db).Filter("UserId=", u.Id()).GetAll(&u.Referrers); err != nil {
		return err
	}

	if _, err := referral.Query(u.Db).Filter("ReferrerUserId=", u.Id()).GetAll(&u.Referrals); err != nil {
		return err
	}

	log.Warn("Referrals %v", u.Referrals)

	return nil
}

func loadOrders(u *userResp) error {
	if _, err := order.Query(u.Db).Filter("UserId=", u.Id()).GetAll(&u.Orders); err != nil {
		return err
	}

	return nil
}


func get(c *gin.Context) {
	usrm := middleware.GetUser(c)
	usr := &userResp{}
	usr.User = usrm

	if err := loadReferrals(usr); err != nil {
		http.Fail(c, 500, "User referral data could get be queried", err)
		return
	}

	if err := loadOrders(usr); err != nil {
		http.Fail(c, 500, "User order data could get be queried", err)
		return
	}

	if err := usr.LoadAffiliateAndPendingFees(); err != nil {
		http.Fail(c, 500, "User order data could get be queried", err)
		return
	}

	if err := usr.CalculateBalances(); err != nil {
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

	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if req.Password != "" {
		if !password.HashAndCompare(usr.PasswordHash, req.CurrentPassword) {
			http.Fail(c, 401, "Password is incorrect", errors.New("Password is incorrect"))
			return
		}
		if err := resetPassword(usr, req); err != nil {
			switch err {
			case PasswordMismatchError, PasswordMinLengthError:
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
		client := mailchimp.New(ctx, org.Mailchimp.APIKey)

		// Update customer in mailchimp for this user
		if err := client.UpdateCustomer(org.DefaultStore, usr); err != nil {
			log.Warn("Failed to update Mailchimp customer: %v", err, ctx)
		}

		http.Render(c, 200, usr)
	}
}
