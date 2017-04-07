package organization

import (
	"time"

	"hanzo.io/thirdparty/stripe/connect"
)

// Salesforce settings
type Salesforce struct {
	AccessToken        string `json:"accessToken"`
	DefaultPriceBookId string `json:"defaultPriceBookId"`
	// personalized login url
	Id           string `json:"id"`
	InstanceUrl  string `json:"instanceUrl"`
	IssuedAt     string `json:"issuedAt"`
	RefreshToken string `json:"refreshToken"`
	Signature    string `json:"signature" datastore:",noindex"`
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

// Affiliate configuration
type Affiliate struct {
	SuccessUrl string `json:"successUrl"`
	ErrorUrl   string `json:"errorUrl"`
}

type Reamaze struct {
	Secret string `json:"secret"`
}

type Shipwire struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Recaptcha struct {
	Enabled   bool   `json:"enabled"`
	SecretKey string `json:"secretKey"`
}
