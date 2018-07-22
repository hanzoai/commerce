package organization

import (
	"context"
	"strings"
	"time"

	"google.golang.org/appengine"
	aeds "google.golang.org/appengine/datastore"

	"github.com/gin-gonic/gin"
	"github.com/ryanuber/go-glob"

	"hanzo.io/datastore"
	"hanzo.io/log"
	"hanzo.io/models/app"
	"hanzo.io/models/mixin"
	"hanzo.io/models/oauthtoken"
	"hanzo.io/models/store"
	"hanzo.io/models/types/analytics"
	"hanzo.io/models/types/currency"
	"hanzo.io/models/types/integrations"
	"hanzo.io/models/types/pricing"
	"hanzo.io/models/user"
	"hanzo.io/models/wallet"
	"hanzo.io/types/email"
	"hanzo.io/util/json"
	"hanzo.io/util/permission"
	"hanzo.io/util/val"

	. "hanzo.io/models"
)

type Organization struct {
	mixin.Model
	mixin.AccessTokens
	wallet.WalletHolder

	Name       string   `json:"name"`
	FullName   string   `json:"fullName"`
	Owners     []string `json:"owners,omitempty" datastore:",noindex"`
	Admins     []string `json:"admins,omitempty" datastore:",noindex"`
	Moderators []string `json:"moderators,omitempty" datastore:",noindex"`
	Enabled    bool     `json:"enabled"`

	BillingEmail     string  `json:"billingEmail,omitempty"`
	Phone            string  `json:"phone,omitempty"`
	Address          Address `json:"address,omitempty"`
	Website          string  `json:"website,omitempty"`
	WalletPassphrase string  `json:"-"`

	Timezone string `json:"timezone"`

	Country string `json:"country"`
	TaxId   string `json:"taxId"`

	// Fee structure for this organization
	Fees pricing.Fees `json:"fees" datastore:",noindex"`

	// Partner fees (private, should be up to partner to disclose)
	Partners []pricing.Partner `json:"-" datastore:",noindex"`

	// Email settings
	Email email.Settings `json:"email" datastore:",noindex"`

	// Default Store
	DefaultStore string `json:"defaultStore"`

	// Default App
	DefaultApp string `json:"defaultApp"`

	// Plan settings
	Plan struct {
		PlanId    string
		StartDate time.Time
	} `json:"-"`

	// Affiliate configuration
	Affiliate integrations.Affiliate `json:"-" datastore:",noindex"`

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

		UsernameRequired bool `json:"usernameRequired"`
	} `json:"signUpOptions" datastore:",noindex"`

	// Whether we use live or test tokens, mostly applicable to stripe
	Live bool `json:"-" datastore:"-"`

	// TODO: Remain to PaymentWhitelist for clarity
	// List of comma deliminated email globs that result in charges of 50 cents
	EmailWhitelist string `json:"emailWhitelist" datastore:",noindex"`

	// Integrations
	Integrations  integrations.Integrations `json:"integrations" datastore:"-"`
	Integrations_ string                    `json:"-" datastore:",noindex"`

	// Integrations (deprecated)

	// Analytics config
	Analytics analytics.Analytics `json:"analytics" datastore:",noindex"`

	// Bitcoi settings
	Bitcoin integrations.Bitcoin `json:"-"`

	// Ethereum settings
	Ethereum integrations.Ethereum `json:"-"`

	// Mailchimp settings
	Mailchimp integrations.Mailchimp `json:"-"`

	// Mandrill settings
	Mandrill integrations.Mandrill `json:"-"`

	// Netlify settings
	Netlify integrations.Netlify `json:"-"`

	// Paypal connection
	Paypal integrations.Paypal `json:"-"`

	Reamaze integrations.Reamaze `json:"-"`

	Recaptcha integrations.Recaptcha `json:"-" datastore:",noindex"`

	// Salesforce settings
	Salesforce integrations.Salesforce `json:"-"`

	// Shipwire settings
	Shipwire integrations.Shipwire `json:"-"`

	// Stripe connection
	Stripe integrations.Stripe `json:"-"`

	// AuthorizeNet connection
	AuthorizeNet integrations.AuthorizeNet `json:"-"`

	Currency currency.Type `json:"currency"`
}

func (o *Organization) Load(ps []aeds.Property) (err error) {
	// Ensure we're initialized
	o.Defaults()

	// Load supported properties
	if err = datastore.LoadStruct(o, ps); err != nil {
		return err
	}

	if len(o.Integrations_) > 0 {
		err = json.DecodeBytes([]byte(o.Integrations_), &o.Integrations)
	}

	for i, in := range o.Integrations {
		err = integrations.Decode(&in, &in)
		o.Integrations[i] = in
		if err != nil {
			return err
		}
	}

	return err
}

func (o *Organization) Save() (ps []aeds.Property, err error) {
	// Serialize unsupported properties
	o.Integrations_ = string(json.EncodeBytes(o.Integrations))

	// Save properties
	return datastore.SaveStruct(o)
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

// Old JWT / AccessToken AUTH
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

// New JWT / OAUTH
func (o *Organization) ResetReferenceToken(usr *user.User, claims oauthtoken.Claims) (*oauthtoken.Token, error) {
	if usr.Key().Namespace() != "" {
		return nil, UserNotTopLevel
	}

	if _, _, err := o.RevokeReferenceToken(usr); err != nil {
		return nil, err
	}

	tok := oauthtoken.New(o.Db)

	log.Info("Creating New Reference Token for user '%s' from org '%s'.", usr.Id(), o.Name, o.Context())

	claims.OrganizationName = o.Name
	claims.UserId = usr.Id()
	claims.Type = oauthtoken.Reference
	claims.JTI = tok.Id()
	claims.IssuedAt = time.Now().Unix()

	tok.Claims = claims
	tok.AccessPeriod = 24 * 30

	if _, err := tok.Encode(o.SecretKey); err != nil {
		log.Info("Failed to create New Reference Token for user '%s' from org '%s': %s", usr.Id(), o.Name, err, o.Context())
		return nil, err
	}

	tok.MustCreate()

	return tok, nil
}

func (o *Organization) GetReferenceToken(usr *user.User) (*oauthtoken.Token, bool, error) {
	if usr.Key().Namespace() != "" {
		return nil, false, UserNotTopLevel
	}

	log.Info("Getting Reference Token for user '%s' from org '%s'.", usr.Id(), o.Name, o.Context())

	tok := oauthtoken.New(o.Db)

	if ok, err := tok.Query().Filter("Claims.OrganizationName=", o.Name).Filter("Claims.Type=", oauthtoken.Reference).Filter("Revoked=", false).Filter("Claims.UserId=", usr.Id()).Get(); !ok {
		log.Info("Failed to get Reference Token for user '%s' from org '%s': %s", usr.Id(), o.Name, err, o.Context())
		return nil, false, err
	}

	return tok, true, nil
}

func (o *Organization) RevokeReferenceToken(usr *user.User) (*oauthtoken.Token, bool, error) {
	if usr.Key().Namespace() != "" {
		return nil, false, UserNotTopLevel
	}

	log.Info("Revoking Reference Token for user '%s' from org '%s'.", usr.Id(), o.Name, o.Context())

	if tok, ok, err := o.GetReferenceToken(usr); !ok {
		log.Info("Failed to revoke Reference Token for user '%s' from org '%s': %s", usr.Id(), o.Name, err, o.Context())
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
func (o Organization) Namespaced(ctx context.Context) context.Context {
	if c, ok := ctx.(*gin.Context); ok {
		ctx = c.MustGet("appengine").(context.Context)
	}

	var err error
	ctx, err = appengine.Namespace(ctx, o.Name)
	if err != nil {
		panic(err)
	}
	return ctx
}

func (o Organization) StripeToken() string {
	if o.Live {
		return o.Stripe.Live.AccessToken
	}

	return o.Stripe.Test.AccessToken
}

func (o Organization) AuthorizeNetTokens() integrations.AuthorizeNetConnection {
	if o.Live {
		return o.AuthorizeNet.Live
	}

	return o.AuthorizeNet.Sandbox
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

func (o Organization) Pricing() (*pricing.Fees, []pricing.Partner) {
	// Ensure our id is set on fees used
	fees := o.Fees
	fees.Id = o.Id()
	return &fees, o.Partners
}

// Return DefaultStore
func (o Organization) GetDefaultStore() (*store.Store, error) {
	db := datastore.New(o.Namespaced(o.Context()))
	s := store.New(db)
	if err := s.GetById(o.DefaultStore); err != nil {
		return nil, err
	}

	return s, nil
}

// Return DefaultApp
func (o Organization) GetDefaultApp() (*app.App, error) {
	db := datastore.New(o.Namespaced(o.Context()))
	a := app.New(db)
	if err := a.GetById(o.DefaultApp); err != nil {
		return nil, err
	}

	return a, nil
}
