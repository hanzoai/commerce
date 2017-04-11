package organization

import (
	"time"

	"hanzo.io/thirdparty/stripe/connect"

	enjson "encoding/json"
)

type IntegrationType string

const (
	MailchimpType  IntegrationType = "mailchimp"
	MandrillType   IntegrationType = "mandrill"
	NetlifyType    IntegrationType = "netlify"
	PaypalType     IntegrationType = "paypal"
	ReamazeType    IntegrationType = "reamaze"
	RecaptchaType  IntegrationType = "recaptcha"
	SalesforceType IntegrationType = "salesforce"
	ShipwireType   IntegrationType = "shipwire"
	StripeType     IntegrationType = "stripe"
)

// Analytics

// Generic fields
type AnalyticsIntegration struct {
	// Common to all integrations
	Type  string `json:"type"`
	Event string `json:"event,omitempty"`
	Id    string `json:"id,omitempty"`

	// Sampling percentage
	Sampling float64 `json:"sampling,omitempty"`
}

// Integration specific properties
type Custom struct {
	AnalyticsIntegration

	Code string `json:"code,omitempty"`
}

type FacebookPixel struct {
	AnalyticsIntegration

	Values Values `json:"values,omitempty"`
}

type FacebookConversions struct {
	AnalyticsIntegration

	Value    string `json:"value,omitempty"`
	Currency string `json:"currency,omitempty"`
}

type Heap struct {
}

type Sentry struct {
}

// Others

// Mailchimp settings
type Mailchimp struct {
	ListId string `json:"listId"`
	APIKey string `json:"apiKey"`
}

// Mandrill settings
type Mandrill struct {
	APIKey string `json:"apiKey"`
}

// Netlify settings
type Netlify struct {
	AccessToken string    `json:"accessToken"`
	CreatedAt   time.Time `json:"createdAt"`
	Email       string    `json:"email"`
	Id          string    `json:"id"`
	Uid         string    `json:"uId"`
}

// Paypal connection
type Paypal struct {
	Live struct {
		Email             string `json:"email"`
		SecurityUserId    string `json:"securityUserId"`
		SecurityPassword  string `json:"securityPassword" datastore:",noindex"`
		SecuritySignature string `json:"SecuritySignature" datastore:",noindex"`
		ApplicationId     string `json:"applicationId"`
	} `json:"live"`
	Test struct {
		Email             string `json:"email"`
		SecurityUserId    string `json:"securityUserId"`
		SecurityPassword  string `json:"securityPassword" datastore:",noindex"`
		SecuritySignature string `json:"SecuritySignature" datastore:",noindex"`
		ApplicationId     string `json:"applicationId"`
	} `json:"test"`

	ConfirmUrl string `json:"confirmUrl" datastore:",noindex"`
	CancelUrl  string `json:"cancelUrl" datastore:",noindex"`
}

// Affiliate configuration
type Affiliate struct {
	SuccessUrl string `json:"successUrl"`
	ErrorUrl   string `json:"errorUrl"`
}

type Reamaze struct {
	Secret string `json:"secret"`
}

type Recaptcha struct {
	Enabled   bool   `json:"enabled"`
	SecretKey string `json:"secretKey"`
}

// Salesforce settings
type Salesforce struct {
	Integration

	AccessToken        string `json:"accessToken"`
	DefaultPriceBookId string `json:"defaultPriceBookId"`
	// personalized login url
	Id           string `json:"id"`
	InstanceUrl  string `json:"instanceUrl"`
	IssuedAt     string `json:"issuedAt"`
	RefreshToken string `json:"refreshToken"`
	Signature    string `json:"signature" datastore:",noindex"`
}

type Shipwire struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Stripe connection
type Stripe struct {
	// For convenience duplicated
	AccessToken    string `json:"accessToken"`
	PublishableKey string `json:"publishableKey"`
	RefreshToken   string `json:"refreshToken"`
	UserId         string `json:"userId"`

	// Save entire live and test tokens
	Live connect.Token `json:"live" datastore:",noindex"`
	Test connect.Token `json:"test" datastore:",noindex"`
}

type BasicIntegration struct {
	Enabled bool            `json:"enabled"`
	Type    IntegrationType `json:"type"`

	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
	Data      enjson.RawMessage `json:"data"`
}

type Integration struct {
	BasicIntegration

	// Analytics
	CustomAnalytics CustomAnalytics `json:"-"`

	// Others
	Mailchimp  Mailchimp  `json: "-"`
	Mandrill   Mandrill   `json: "-"`
	Netlify    Netlify    `json: "-"`
	Paypal     Paypal     `json: "-"`
	Reamaze    Reamaze    `json: "-"`
	Recaptcha  Recaptcha  `json: "-"`
	Salesforce Salesforce `json: "-"`
	Shipwire   Shipwire   `json: "-"`
	Stripe     Stripe     `json: "-"`
}
