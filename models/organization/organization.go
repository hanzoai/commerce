package organization

import (
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/appengine"

	"github.com/gin-gonic/gin"
	glob "github.com/ryanuber/go-glob"

	"hanzo.io/models/mixin"
	token "hanzo.io/models/token2"
	"hanzo.io/models/types/analytics"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/stripe/connect"
	"hanzo.io/util/val"

	. "hanzo.io/models"
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

	Name       string   `json:"name"`
	FullName   string   `json:"fullName"`
	Owners     []string `json:"owners,omitempty"`
	Owners_    string   `json:"-"` // props
	Admins     []string `json:"admins,omitempty"`
	Moderators []string `json:"moderators,omitempty"`
	Enabled    bool     `json:"enabled"`

	BillingEmail string  `json:"billingEmail,omitempty"`
	Phone        string  `json:"phone,omitempty"`
	Address      Address `json:"address,omitempty"`
	Website      string  `json:"website,omitempty"`

	Timezone string `json:"timezone"`

	Country string `json:"country"`
	TaxId   string `json:"-"`

	Fee float64 `json:"fee"`

	// Analytics config
	Analytics analytics.Analytics `json:"analytics"`

	Email EmailConfig `json:"email"`

	Plan struct {
		PlanId    string
		StartDate time.Time
	} `json:"-"`

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

	// Affiliate configuration
	Affiliate struct {
		SuccessUrl string
		ErrorUrl   string
	} `json:"-" datastore:",noindex"`

	Reamaze struct {
		Secret string
	} `json:"-" datastore:"`

	// Signup options
	SignUpOptions struct {
		// Controls the enabled status of account after creation
		AccountsEnabledByDefault bool `json:"accountsEnabledByDefault"`

		// Turns off required backend checks
		NoNameRequired     bool `json:"noNameRequired"`
		NoPasswordRequired bool `json:"noPasswordRequired"`

		// Requires password set on create confirmation
		TwoStageEnabled bool `json:"twoStageEnabled"`
		ImmediateLogin  bool `json:"immediateLogin"`
	} `json:"signUpOptions" datastore:",noindex"`

	Recaptcha struct {
		Enabled   bool
		SecretKey string
	} `json:"-" datastore:",noindex"`

	// Whether we use live or test tokens, mostly applicable to stripe
	Live bool `json:"-" datastore:"-"`

	// List of comma deliminated email globs that result in charges of 50 cents
	EmailWhitelist string `json:"emailWhitelist"`

	SecretKey []byte `json:"-"`
}

func (o *Organization) Defaults() {
	o.Admins = make([]string, 0)
	o.Moderators = make([]string, 0)
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

func (o *Organization) ResetReferenceToken(usr *user.User, claims token.Claims) (*token.Token, error) {
	if usr.Key().Namespace() != "" {
		return nil, UserNotTopLevel
	}

	o.RevokeReferenceToken(usr)

	tok := token.New(o.Db)

	claims.OrganizationName = o.Name
	claims.UserId = usr.Id()
	claims.Type = token.Reference
	claims.JTI = tok.Id()
	claims.IssuedAt = time.Now().Unix()

	tok.Claims = claims
	tok.AccessPeriod = 24

	if _, err := tok.Encode(o.SecretKey); err != nil {
		return nil, err
	}

	tok.MustCreate()

	return tok, nil
}

func (o *Organization) GetReferenceToken(usr *user.User) (*token.Token, bool, error) {
	if usr.Key().Namespace() != "" {
		return nil, false, UserNotTopLevel
	}

	tok := token.New(o.Db)

	if ok, err := tok.Query().Filter("Claims.OrganizationName=", o.Name).Filter("Claims.Type=", token.Reference).Filter("Revoked=", false).Filter("Claims.UserId=", usr.Id()).First(); !ok {
		return nil, false, err
	}

	return tok, true, nil
}

func (o *Organization) RevokeReferenceToken(usr *user.User) (*token.Token, bool, error) {
	if usr.Key().Namespace() != "" {
		return nil, false, UserNotTopLevel
	}

	if tok, ok, err := o.GetReferenceToken(usr); !ok {
		return nil, false, err
	} else {
		tok.Revoke()
		return tok, true, nil
	}
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
func (o Organization) Namespaced(ctx interface{}) context.Context {
	var _ctx context.Context

	switch v := ctx.(type) {
	case *gin.Context:
		_ctx = v.MustGet("appengine").(context.Context)
	case context.Context:
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
