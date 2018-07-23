package email

import (
	"encoding/gob"
)

// Types of system-defined emails
type Type string

const (
	OrderConfirmation     Type = "order.confirmation"
	OrderRefunded         Type = "order.refunded"
	OrderShipped          Type = "order.shipped"
	OrderUpdated          Type = "order.updated"
	UserWelcome           Type = "user.welcome"
	UserEmailConfirmation Type = "user.emailConfirmation"
	UserEmailConfirmed    Type = "user.emailConfirmed"
	UserPasswordReset     Type = "user.passwordReset"
	SubscriberWelcome     Type = "subscriber.welcome"
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
		Confirmation Setting `json:"confirmation"`
		Refunded     Setting `json:"refunded"`
		Shipped      Setting `json:"shipped"`
		Updated      Setting `json:"updated"`
	} `json:"order"`

	User struct {
		Welcome           Setting `json:"welcome`
		EmailConfirmation Setting `json:"emailConfirmation"`
		EmailConfirmed    Setting `json:"emailConfirmed"`
		PasswordReset     Setting `json:"PasswordReset"`
	} `json:"user"`

	Subscriber struct {
		Welcome Setting `json:"welcome`
	} `json:"subscriber"`
}

// Return email settings updated from defaults
func (s Settings) Get(typ Type) Setting {
	setting := Setting{}

	switch typ {
	case OrderConfirmation:
		setting = s.Order.Confirmation
	case UserWelcome:
		setting = s.User.Welcome
	case UserEmailConfirmation:
		setting = s.User.EmailConfirmation
	case UserEmailConfirmed:
		setting = s.User.EmailConfirmed
	case UserPasswordReset:
		setting = s.User.PasswordReset
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
