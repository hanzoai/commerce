package organization

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ryanuber/go-glob"

	"appengine"

	"crowdstart.com/models/mixin"
	"crowdstart.com/models/types/analytics"
	"crowdstart.com/models/types/pricing"
	"crowdstart.com/models/user"
	"crowdstart.com/thirdparty/stripe/connect"
	"crowdstart.com/util/permission"
	"crowdstart.com/util/val"

	. "crowdstart.com/models"
)

type Email struct {
	Enabled   bool   `json:"enabled"`
	FromEmail string `json:"fromEmail"`
	FromName  string `json:"fromName"`
	Subject   string `json:"subject"`
	Template  string `json:"template" datastore:",noindex"`
}

func (e Email) Config(org *Organization) Email {
	conf := Email{e.Enabled, e.FromName, e.FromEmail, e.Subject, e.Template}

	// Use organization defaults
	if org != nil {
		if !org.Email.Defaults.Enabled {
			conf.Enabled = false
		}

		if conf.FromEmail == "" {
			conf.FromEmail = org.Email.Defaults.FromEmail
		}

		if conf.FromName == "" {
			conf.FromName = org.Email.Defaults.FromName
		}
	}

	return conf
}

type EmailConfig struct {
	// Default email configuration
	Defaults struct {
		Enabled   bool   `json:"enabled"`
		FromName  string `json:"fromName"`
		FromEmail string `json:"fromEmail"`
	} `json:"defaults"`

	// Per-email configuration
	OrderConfirmation Email `json:"orderConfirmation"`
	User              struct {
		Welcome           Email `json:"welcome`
		EmailConfirmation Email `json:"emailConfirmation"`
		EmailConfirmed    Email `json:"emailConfirmed"`
		PasswordReset     Email `json:"PasswordReset"`
	} `json:"user"`
}

type Organization struct {
	mixin.Model
	mixin.AccessToken

	Name       string   `json:"name"`
	FullName   string   `json:"fullName"`
	Owners     []string `json:"owners,omitempty"`
	Admins     []string `json:"admins,omitempty"`
	Moderators []string `json:"moderators,omitempty"`
	Enabled    bool     `json:"enabled"`

	BillingEmail string  `json:"billingEmail,omitempty"`
	Phone        string  `json:"phone,omitempty"`
	Address      Address `json:"address,omitempty"`
	Website      string  `json:"website,omitempty"`

	Timezone string `json:"timezone"`

	Country string `json:"country"`
	TaxId   string `json:"taxId"`

	// Fee structure for this organization
	Fees pricing.Fees `json:"fees"`

	// Partner fees (private, should be up to partner to disclose)
	Partners []pricing.Partner `json:"-"`

	// Analytics config
	Analytics analytics.Analytics `json:"analytics"`

	Email EmailConfig `json:"email"`

	Plan struct {
		PlanId    string
		StartDate time.Time
	} `json:"-"`

	// Default store
	DefaultStore string `json:"defaultStore"`

	Salesforce struct {
		AccessToken        string `json:"accessToken"`
		DefaultPriceBookId string `json:"defaultPriceBookId"`
		// personalized login url
		Id           string `json:"id"`
		InstanceUrl  string `json:"instanceUrl"`
		IssuedAt     string `json:"issuedAt"`
		RefreshToken string `json:"refreshToken"`
		Signature    string `json:"signature"`
	} `json:"-"`

	Paypal struct {
		Live struct {
			Email             string `json:"paypalEmail"`
			SecurityUserId    string
			SecurityPassword  string
			SecuritySignature string
			ApplicationId     string
		}
		Test struct {
			Email             string `json:"paypalEmail"`
			SecurityUserId    string
			SecurityPassword  string
			SecuritySignature string
			ApplicationId     string
		}

		ConfirmUrl string `json:"confirmUrl"`
		CancelUrl  string `json:"cancelUrl"`
	} `json:"-"`

	Stripe struct {
		// For convenience duplicated
		AccessToken    string
		PublishableKey string
		RefreshToken   string
		UserId         string

		// Save entire live and test tokens
		Live connect.Token
		Test connect.Token
	} `json:"-"`

	Mailchimp struct {
		ListId string `json:"listId"`
		APIKey string `json:"apiKey"`
	} `json:"-"`

	Mandrill struct {
		APIKey string
	} `json:"-"`

	Netlify struct {
		AccessToken string
		CreatedAt   time.Time
		Email       string
		Id          string
		Uid         string
	} `json:"-"`

	Affilliate struct {
		SuccessUrl string
		ErrorUrl   string
	} `json:"-"`

	// TODO: Delete?
	GoogleAnalytics string `json:"googleAnalytics"`
	FacebookTag     string `json:"facebookTag"`

	// Whether we use live or test tokens, mostly applicable to stripe
	Live bool `json:"-" datastore:"-"`

	// List of comma deliminated email globs that result in charges of 50 cents
	EmailWhitelist string `json:"emailWhitelist"`
}

func (o Organization) GetStripeAccessToken(userId string) (string, error) {
	if o.Stripe.Live.UserId == userId {
		return o.Stripe.Live.AccessToken, nil
	}
	if o.Stripe.Test.UserId == userId {
		return o.Stripe.Test.AccessToken, nil
	}
	return "", StripeAccessTokenNotFound{userId, o.Stripe.Live.UserId, o.Stripe.Test.UserId}
}

func (o *Organization) Validator() *val.Validator {
	return val.New().Check("FullName").Exists()
}

func (o *Organization) AddDefaultTokens() {
	o.RemoveToken("live-secret-key")
	o.RemoveToken("live-published-key")
	o.RemoveToken("test-secret-key")
	o.RemoveToken("test-published-key")
	o.AddToken("live-secret-key", permission.Admin|permission.Live)
	o.AddToken("live-published-key", permission.Published|permission.Live|permission.ReadCoupon|permission.ReadProduct|permission.WriteReferrer)
	o.AddToken("test-secret-key", permission.Admin|permission.Test)
	o.AddToken("test-published-key", permission.Published|permission.Test|permission.ReadCoupon|permission.ReadProduct|permission.WriteReferrer)
}

func userId(userOrId interface{}) string {
	userid := ""
	switch v := userOrId.(type) {
	case *user.User:
		userid = v.Id()
	case string:
		userid = v
	}
	return userid
}

func (o Organization) IsAdmin(userOrId interface{}) bool {
	userid := userId(userOrId)

	for i := range o.Admins {
		if o.Admins[i] == userid {
			return true
		}
	}
	return false
}

func (o Organization) IsOwner(userOrId interface{}) bool {
	userid := userId(userOrId)

	for i := range o.Owners {
		if o.Owners[i] == userid {
			return true
		}
	}
	return false
}

// Add admin to organization
func (o *Organization) AddAdmin(userOrId string) {
	userid := userId(userOrId)

	if !o.IsAdmin(userid) {
		o.Admins = append(o.Admins, userid)
	}
}

// Add admin to organization
func (o *Organization) AddOwner(userOrId string) {
	userid := userId(userOrId)

	if !o.IsOwner(userid) {
		o.Owners = append(o.Owners, userid)
	}
}

// Get namespaced context for this organization
func (o Organization) Namespaced(ctx interface{}) appengine.Context {
	var _ctx appengine.Context

	switch v := ctx.(type) {
	case *gin.Context:
		_ctx = v.MustGet("appengine").(appengine.Context)
	case appengine.Context:
		_ctx = v
	}

	_ctx, err := appengine.Namespace(_ctx, o.Name)
	if err != nil {
		panic(err)
	}
	return _ctx
}

func (o Organization) StripeToken() string {
	if o.Live {
		return o.Stripe.Live.AccessToken
	}

	return o.Stripe.Test.AccessToken
}

func (o Organization) IsTestEmail(email string) bool {
	if email == "" || o.EmailWhitelist == "" {
		return false
	}

	globs := strings.Split(strings.Replace(o.EmailWhitelist, " ", "", -1), ",")

	for _, g := range globs {
		if glob.Glob(g, email) {
			return true
		}
	}

	return false
}
