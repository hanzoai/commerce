package account

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hanzo.io/auth/password"
	"hanzo.io/datastore"
	"hanzo.io/middleware"
	"hanzo.io/models/referral"
	"hanzo.io/models/referrer"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/mailchimp"
	"hanzo.io/thirdparty/recaptcha"
	"hanzo.io/util/counter"
	"hanzo.io/util/emails"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"
)

var emailRegex = regexp.MustCompile("(\\w[-._\\w]*@\\w[-._\\w]*\\w\\.\\w{2,4})")

type createReq struct {
	*user.User
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
	Captcha         string `json:"g-recaptcha-response"`
	StoreId         string `json:"storeId"`
}

type createRes struct {
	*user.User
	Token string `json:"token,omitempty"`
}

func create(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	req := &createReq{}
	req.User = user.New(db)

	// Default these fields to exotic unicode character to test if they are set to empty
	req.Email = "\u263A"
	req.FirstName = "\u263A"
	req.LastName = "\u263A"

	log.Info("Decoding User Creation Request", c)
	// Decode response body to create new user
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if org.Recaptcha.Enabled && !recaptcha.Challenge(db.Context, org.Recaptcha.SecretKey, req.Captcha) {
		http.Fail(c, 400, "Captcha needs to be completed", errors.New("Captcha needs to be completed"))
		return
	}

	// Pull out user
	usr := req.User

	log.Info("Fetching User Request: %v", usr, c)
	// Email is required
	if usr.Email == "" || usr.Email == "\u263A" {
		http.Fail(c, 400, "Email is required", errors.New("Email is required"))
		return
	}

	usr.Email = strings.ToLower(strings.TrimSpace(usr.Email))

	usr2 := user.New(db)
	// Email can't already exist or if it does, can't have a password
	if err := usr2.GetByEmail(usr.Email); err == nil {
		if len(usr2.PasswordHash) > 0 {
			http.Fail(c, 400, "Email is in use", errors.New("Email is in use"))
			return
		} else {
			// Transfer name from request user to queried out user if successful
			req.User = usr2
			if usr.FirstName != "" && usr.FirstName != "\u263A" {
				usr2.FirstName = usr.FirstName
			}
			if usr.LastName != "" && usr.FirstName != "\u263A" {
				usr2.LastName = usr.LastName
			}

			usr = usr2
		}
	}

	if !org.SignUpOptions.NoNameRequired {
		log.Info("Sign up does require Name: %s/%s", usr.FirstName, usr.LastName, c)
		if usr.FirstName == "" || usr.FirstName == "\u263A" {
			http.Fail(c, 400, "First name cannot be blank", errors.New("First name cannot be blank"))
			return
		}

		if usr.LastName == "" || usr.LastName == "\u263A" {
			http.Fail(c, 400, "Last name cannot be blank", errors.New("Last name cannot be blank"))
			return
		}
	} else {
		log.Info("Sign up does not require Name", c)
	}

	if usr.Email == "\u263A" {
		usr.Email = ""
	}

	if usr.FirstName == "\u263A" {
		usr.FirstName = ""
	}

	if usr.LastName == "\u263A" {
		usr.LastName = ""
	}

	// Email must be valid
	log.Info("Checking if User email is valid", c)
	if ok := emailRegex.MatchString(usr.Email); !ok {
		http.Fail(c, 400, "Email '"+usr.Email+"' is not valid", errors.New("Email '"+usr.Email+"' is not valid"))
		return
	}

	if !org.SignUpOptions.NoPasswordRequired {
		log.Info("Sign up requires password", c)
		// Password should be at least 6 characters long
		if len(req.Password) < 6 {
			http.Fail(c, 400, "Password needs to be atleast 6 characters", errors.New("Password needs to be atleast 6 characters"))
			return
		}

		// Password confirm must match
		if req.Password != req.PasswordConfirm {
			http.Fail(c, 400, "Passwords need to match", errors.New("Passwords need to match"))
			return
		}

		// Hash password
		if hash, err := password.Hash(req.Password); err != nil {
			http.Fail(c, 400, "Failed to hash user password", err)
		} else {
			usr.PasswordHash = hash
		}
	} else {
		log.Info("Sign up does not require password", c)
	}

	ctx := org.Db.Context
	if err := counter.IncrUsers(ctx, org, time.Now()); err != nil {
		log.Warn("Redis Error %s", err, ctx)
	}

	// Test key users are automatically confirmed
	if !org.Live {
		usr.Enabled = true
	}

	log.Info("User is enabled? %v", usr.Enabled, c)
	usr.Enabled = org.SignUpOptions.AccountsEnabledByDefault

	// Determine store to use
	storeId := req.StoreId
	if storeId == "" {
		storeId = org.DefaultStore
	}

	usr.StoreId = storeId

	// Save new user
	log.Info("User is attributed to store: %v", storeId, c)
	if err := usr.Put(); err != nil {
		http.Fail(c, 400, "Failed to create user", err)
	}

	ref := referrer.New(usr.Db)

	// if ReferrerId refers to non-existing token, then remove from order
	if usr.ReferrerId != "" {
		log.Info("User is attributed to Referrer %s: %v", usr.ReferrerId, c)
		if err := ref.GetById(usr.ReferrerId); err != nil {
			usr.ReferrerId = ""
		} else {
			// Try to save referral, save updated referrer
			if _, err := ref.SaveReferral(org.Db.Context, org.Id(), referral.NewUser, usr); err != nil {
				log.Warn("Unable to save referral: %v", err, c)
			}
		}
	}

	tokStr := ""

	if org.SignUpOptions.ImmediateLogin {
		log.Info("User is being immediately logged in", c)
		loginTok := middleware.GetToken(c)
		loginTok.Set("user-id", usr.Id())
		loginTok.Set("exp", time.Now().Add(time.Hour*24*7))
		tokStr = loginTok.String()
	}

	counter.IncrUser(usr.Context(), usr.CreatedAt)
	// Render user
	http.Render(c, 201, createRes{User: usr, Token: tokStr})

	// Don't send email confirmation if test key is used
	// if org.Live {
	log.Info("Sending Emails", c)
	// Send welcome, email confirmation emails
	emails.SendAccountCreationConfirmationEmail(ctx, org, usr)
	emails.SendUserWelcome(ctx, org, usr)
	// } else {
	// 	log.Info("Organization %v is not live.  No emails sent.", org.Name, c)
	// }

	// Save user as customer in Mailchimp if configured
	if org.Mailchimp.APIKey != "" {
		log.Info("Saving User to Mailchimp: %s", usr, c)
		// Create new mailchimp client
		client := mailchimp.New(ctx, org.Mailchimp.APIKey)

		// Create customer in mailchimp for this user
		if err := client.CreateCustomer(storeId, usr); err != nil {
			log.Warn("Failed to create Mailchimp customer: %v", err, ctx)
		}
	} else {
		log.Info("Skip saving User to Mailchimp: %s", usr, c)
	}
}
