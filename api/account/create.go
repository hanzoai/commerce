package account

import (
	"errors"
	"io/ioutil"
	"net/url"
	"regexp"
	"strings"
	"time"

	"appengine"
	"appengine/urlfetch"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/mailchimp"
	"crowdstart.com/util/counter"
	"crowdstart.com/util/emails"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
)

var emailRegex = regexp.MustCompile("(\\w[-._\\w]*@\\w[-._\\w]*\\w\\.\\w{2,4})")

type createReq struct {
	*user.User
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
	Captcha         string `json:"g-recaptcha-response"`
}

type createRes struct {
	*user.User
	Token string `json:"token,omitempty"`
}

type RecaptchaResponse struct {
	Success     bool      `json:"success"`
	ChallengeTS time.Time `json:"challenge_ts"`
	Hostname    string    `json:"hostname"`
	ErrorCodes  []int     `json:"error-codes"`
}

func recaptcha(ctx appengine.Context, privateKey, response string) bool {
	log.Warn("Captcha:\n\n%s\n\n%s\n\n%s", privateKey, response, ctx)
	client := urlfetch.Client(ctx)
	r := RecaptchaResponse{}
	resp, err := client.PostForm("https://www.google.com/recaptcha/api/siteverify",
		url.Values{
			"secret":   {privateKey},
			"response": {response},
			// "remoteip": {remoteIp},
		})
	if err != nil {
		log.Error("Captcha post error: %s", err, ctx)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	log.Warn("Captcha %s", body, ctx)
	if err != nil {
		log.Error("Read error: could not read body: %s", err, ctx)
		return false
	}
	err = json.Unmarshal(body, &r)
	log.Warn("Captcha %v", r, ctx)
	if err != nil {
		log.Error("Read error: got invalid JSON: %s", err, ctx)
		return false
	}

	return r.Success
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

	log.Warn("Request:\n%s", c.Request.Body, db.Context)

	// Decode response body to create new user
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if org.Recaptcha.Enabled && !recaptcha(db.Context, org.Recaptcha.SecretKey, req.Captcha) {
		http.Fail(c, 400, "Captcha needs to be completed", errors.New("Captcha needs to be completed"))
		return
	}

	// Pull out user
	usr := req.User

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
		if usr.FirstName == "" || usr.FirstName == "\u263A" {
			http.Fail(c, 400, "First name cannot be blank", errors.New("First name cannot be blank"))
			return
		}

		if usr.LastName == "" || usr.LastName == "\u263A" {
			http.Fail(c, 400, "Last name cannot be blank", errors.New("Last name cannot be blank"))
			return
		}
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
	if ok := emailRegex.MatchString(usr.Email); !ok {
		http.Fail(c, 400, "Email '"+usr.Email+"' is not valid", errors.New("Email is not valid"))
		return
	}

	if !org.SignUpOptions.NoPasswordRequired {
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
	}

	ctx := org.Db.Context
	if err := counter.IncrUsers(ctx, org, time.Now()); err != nil {
		log.Warn("Redis Error %s", err, ctx)
	}

	// Test key users are automatically confirmed
	if !org.Live {
		usr.Enabled = true
	}

	usr.Enabled = org.SignUpOptions.AccountsEnabledByDefault

	// Save new user
	if err := usr.Put(); err != nil {
		http.Fail(c, 400, "Failed to create user", err)
	}

	ref := referrer.New(usr.Db)

	// if ReferrerId refers to non-existing token, then remove from order
	if usr.ReferrerId != "" {
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
		loginTok := middleware.GetToken(c)
		loginTok.Set("user-id", usr.Id())
		loginTok.Set("exp", time.Now().Add(time.Hour*24*7))
		tokStr = loginTok.String()
	}

	// Render user
	http.Render(c, 201, createRes{User: usr, Token: tokStr})

	// Don't send email confirmation if test key is used
	if org.Live {
		// Send welcome, email confirmation emails
		ctx := middleware.GetAppEngine(c)
		emails.SendAccountCreationConfirmationEmail(ctx, org, usr)
		emails.SendWelcomeEmail(ctx, org, usr)
	}

	// Save user as customer in Mailchimp if configured
	if org.Mailchimp.APIKey != "" {
		// Create new mailchimp client
		client := mailchimp.New(ctx, org.Mailchimp.APIKey)

		// Create customer in mailchimp for this user
		if err := client.CreateCustomer(org.DefaultStore, usr); err != nil {
			log.Warn("Failed to create Mailchimp customer: %v", err, ctx)
		}
	}
}
