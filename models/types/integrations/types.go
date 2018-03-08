package integrations

import (
	"time"

	"hanzo.io/models/types/analytics"
	"hanzo.io/thirdparty/stripe/connect"

	enjson "encoding/json"
)

type IntegrationType string

const (
	// Analytics
	AnalyticsCustomType              IntegrationType = "analytics-custom"
	AnalyticsFacebookPixelType       IntegrationType = "analytics-facebook-pixel"
	AnalyticsFacebookConversionsType IntegrationType = "analytics-facebook-conversions"
	AnalyticsGoogleAdwordsType       IntegrationType = "analytics-google-adwords"
	AnalyticsGoogleAnalyticsType     IntegrationType = "analytics-google-analytics"
	AnalyticsHeapType                IntegrationType = "analytics-heap"
	AnalyticsSentryType              IntegrationType = "analytics-sentry"

	// Others
	BitcoinType    IntegrationType = "bitcoin"
	EthereumType   IntegrationType = "ethereum"
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
	Event string `json:"event,omitempty"`
	Id    string `json:"id,omitempty"`

	// Sampling percentage
	Sampling float64 `json:"sampling,omitempty"`
}

// Integration specific properties

// Override value for a given event
type Value analytics.Value

type Values analytics.Values

type AnalyticsCustom struct {
	AnalyticsIntegration

	Code string `json:"code,omitempty"`
}

type AnalyticsFacebookPixel struct {
	AnalyticsIntegration

	Values Values `json:"values,omitempty"`
}

type AnalyticsFacebookConversions struct {
	AnalyticsIntegration

	Value    string `json:"value,omitempty"`
	Currency string `json:"currency,omitempty"`
}

type AnalyticsGoogleAdwords struct {
	AnalyticsIntegration
}

type AnalyticsGoogleAnalytics struct {
	AnalyticsIntegration
}

type AnalyticsHeap struct {
	AnalyticsIntegration
}

type AnalyticsSentry struct {
	AnalyticsIntegration
}

// Others

// Mailchimp settings
type Mailchimp struct {
	ListId      string `json:"listId,omitempty"`
	APIKey      string `json:"apiKey,omitempty"`
	CheckoutUrl string `json:"checkoutUrl,omitempty"`
}

// Mandrill settings
type Mandrill struct {
	APIKey string `json:"apiKey,omitempty"`
}

// Netlify settings
type Netlify struct {
	AccessToken string    `json:"accessToken,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
	Email       string    `json:"email,omitempty"`
	Id          string    `json:"id,omitempty"`
	Uid         string    `json:"uId,omitempty"`
}

// Paypal connection
type Paypal struct {
	Live struct {
		Email             string `json:"email,omitempty"`
		SecurityUserId    string `json:"securityUserId,omitempty"`
		SecurityPassword  string `json:"securityPassword,omitempty" datastore:",noindex"`
		SecuritySignature string `json:"SecuritySignature,omitempty" datastore:",noindex"`
		ApplicationId     string `json:"applicationId,omitempty"`
	} `json:"live,omitempty"`
	Test struct {
		Email             string `json:"email,omitempty"`
		SecurityUserId    string `json:"securityUserId,omitempty"`
		SecurityPassword  string `json:"securityPassword,omitempty" datastore:",noindex"`
		SecuritySignature string `json:"SecuritySignature,omitempty" datastore:",noindex"`
		ApplicationId     string `json:"applicationId,omitempty"`
	} `json:"test,omitempty"`

	ConfirmUrl string `json:"confirmUrl,omitempty" datastore:",noindex"`
	CancelUrl  string `json:"cancelUrl,omitempty" datastore:",noindex"`
}

// Affiliate configuration
type Affiliate struct {
	SuccessUrl string `json:"successUrl,omitempty"`
	ErrorUrl   string `json:"errorUrl,omitempty"`
}

type Reamaze struct {
	Secret string `json:"secret,omitempty"`
}

type Recaptcha struct {
	Enabled   bool   `json:"enabled,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
}

// Salesforce settings
type Salesforce struct {
	AccessToken        string `json:"accessToken,omitempty"`
	DefaultPriceBookId string `json:"defaultPriceBookId,omitempty"`
	// personalized login url
	Id           string `json:"id,omitempty"`
	InstanceUrl  string `json:"instanceUrl,omitempty"`
	IssuedAt     string `json:"issuedAt,omitempty"`
	RefreshToken string `json:"refreshToken,omitempty"`
	Signature    string `json:"signature,omitempty" datastore:",noindex"`
}

type Shipwire struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// Stripe connection
type Stripe struct {
	// For convenience duplicated
	AccessToken    string `json:"accessToken,omitempty"`
	PublishableKey string `json:"publishableKey,omitempty"`
	RefreshToken   string `json:"refreshToken,omitempty"`
	UserId         string `json:"userId,omitempty"`

	// Save entire live and test tokens
	Live connect.Token `json:"live,omitempty" datastore:",noindex"`
	Test connect.Token `json:"test,omitempty" datastore:",noindex"`
}

// Bitcoin
type Bitcoin struct {
	Address     string `json:"address,omitempty"`
	TestAddress string `json:"testAddress,omitempty"`
}

// Ethereum
type Ethereum struct {
	Address     string `json:"address,omitempty"`
	TestAddress string `json:"testAddress,omitempty"`
}

type BasicIntegration struct {
	Enabled bool `json:"enabled,omitempty"`
	Show    bool `json:"show,omitempty"`

	Id   string            `json:"id,omitempty"`
	Data enjson.RawMessage `json:"data,omitempty"`
	Type IntegrationType   `json:"type,omitempty"`

	CreatedAt time.Time `json:"createdAt,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

type Integration struct {
	BasicIntegration

	// Analytics
	AnalyticsCustom              AnalyticsCustom              `json:"-"`
	AnalyticsFacebookPixel       AnalyticsFacebookPixel       `json:"-"`
	AnalyticsFacebookConversions AnalyticsFacebookConversions `json:"-"`
	AnalyticsGoogleAdwords       AnalyticsGoogleAdwords       `json:"-"`
	AnalyticsGoogleAnalytics     AnalyticsGoogleAnalytics     `json:"-"`
	AnalyticsHeap                AnalyticsHeap                `json:"-"`
	AnalyticsSentry              AnalyticsSentry              `json:"-"`

	// Others
	Bitcoin    Bitcoin    `json:"-"`
	Ethereum   Ethereum   `json:"-"`
	Mailchimp  Mailchimp  `json:"-"`
	Mandrill   Mandrill   `json:"-"`
	Netlify    Netlify    `json:"-"`
	Paypal     Paypal     `json:"-"`
	Reamaze    Reamaze    `json:"-"`
	Recaptcha  Recaptcha  `json:"-"`
	Salesforce Salesforce `json:"-"`
	Shipwire   Shipwire   `json:"-"`
	Stripe     Stripe     `json:"-"`
}
