package account

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/middleware"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
)

func get(c *gin.Context) {
	usr := middleware.GetUser(c)

	if err := usr.LoadReferrals(); err != nil {
		http.Fail(c, 500, "User referral data could get be queried", err)
		return
	}

	if err := usr.LoadOrders(); err != nil {
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
	usr := middleware.GetUser(c)

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
		http.Render(c, 200, usr)
	}
}
