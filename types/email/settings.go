package email

import (
	"encoding/gob"
)

// Types of system-defined emails
type Type string

const (
	OrderConfirmation  Type = "order.confirmation"
	OrderPartialRefund Type = "order.partialrefund"
	OrderRefund        Type = "order.refund"
	OrderShipped       Type = "order.shipped"
	OrderUpdated       Type = "order.updated"
	UserActivated      Type = "user.activated"
	UserConfirmEmail   Type = "user.confirmemail"
	UserResetPassword  Type = "user.resetPassword"
	UserUpdatePassword Type = "user.updatePassword"
	UserWelcome        Type = "user.welcome"
	SubscriberWelcome  Type = "subscriber.welcome"
)

type Setting struct {
	Enabled    bool    `json:"enabled"`
	From       Email   `json:"from"`
	ReplyTo    Email   `json:"replyTo"`
	Subject    string  `json:"subject"`
	CC         []Email `json:"cc,omitempty"`
	BCC        []Email `json:"bcc,omitempty"`
	HTML       string  `json:"html,omitempty" datastore:",noindex"`
	Text       string  `json:"text,omitempty" datastore:",noindex"`
	TemplateId string  `json:"templateId,omitempty"`
	ProviderId string  `json:"providerId"`
}

// System-wide email settings
type Settings struct {
	// Global enable/disable of email
	Enabled bool `json:"enabled"`

	// Defaults for all email settings
	Defaults struct {
		From       Email   `json:"from"`
		ReplyTo    Email   `json:"replyTo"`
		CC         []Email `json:"cc,omitempty"`
		BCC        []Email `json:"bcc,omitempty"`
		ProviderId string  `json:"providerId"`
	} `json:"defaults`

	// Per-email configuration
	Order struct {
		Confirmation  Setting `json:"confirmation"`
		Refund        Setting `json:"refund"`
		PartialRefund Setting `json:"partialRefund"`
		Shipped       Setting `json:"shipped"`
		Updated       Setting `json:"updated"`
	} `json:"order"`

	User struct {
		Welcome        Setting `json:"welcome`
		ConfirmEmail   Setting `json:"confirmEmail"`
		Activated      Setting `json:"activated"`
		ResetPassword  Setting `json:"resetPassword"`
		UpdatePassword Setting `json:"updatePassword"`
	} `json:"user"`

	Subscriber struct {
		Welcome Setting `json:"welcome`
	} `json:"subscriber"`
}

// Return email settings updated from defaults
func (s Settings) Get(typ Type) Setting {
	setting := Setting{}

	switch typ {
	// Order emails
	case OrderConfirmation:
		setting = s.Order.Confirmation
	case OrderShipped:
		setting = s.Order.Shipped
	case OrderRefund:
		setting = s.Order.Refund
	case OrderPartialRefund:
		setting = s.Order.PartialRefund

	// User emails
	case UserWelcome:
		setting = s.User.Welcome
	case UserConfirmEmail:
		setting = s.User.ConfirmEmail
	case UserActivated:
		setting = s.User.Activated
	case UserResetPassword:
		setting = s.User.ResetPassword
	case UserUpdatePassword:
		setting = s.User.UpdatePassword

	// Subscriber emails
	case SubscriberWelcome:
		setting = s.Subscriber.Welcome
	}

	// Use organization defaults
	if !s.Enabled {
		setting.Enabled = false
	}

	if setting.From.Address == "" {
		setting.From.Address = s.Defaults.From.Address
	}

	if setting.From.Name == "" {
		setting.From.Name = s.Defaults.From.Name
	}

	if len(setting.CC) == 0 {
		setting.CC = s.Defaults.CC
	}

	if len(setting.BCC) == 0 {
		setting.BCC = s.Defaults.BCC
	}

	if setting.ProviderId == "" {
		setting.ProviderId = s.Defaults.ProviderId
	}

	return setting
}

func init() {
	gob.Register(Setting{})
	gob.Register(Settings{})
}
