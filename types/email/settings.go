package email

import (
	"encoding/gob"
)

// Types of system-defined emails
type Type string

const (
	OrderConfirmation     Type = "order.confirmation"
	UserWelcome           Type = "user.welcome"
	UserEmailConfirmation Type = "user.emailConfirmation"
	UserEmailConfirmed    Type = "user.emailConfirmed"
	UserPasswordReset     Type = "user.passwordReset"
	SubscriberWelcome     Type = "subscriber.welcome"
)

type Setting struct {
	Enabled    bool     `json:"enabled"`
	FromEmail  string   `json:"fromEmail"`
	FromName   string   `json:"fromName"`
	Subject    string   `json:"subject"`
	Template   string   `json:"template" datastore:",noindex"`
	Cc         []string `json:"cc,omitempty"`
	Bcc        []string `json:"cc,omitempty"`
	ProviderId string   `json:"providerId"`
}

// System-wide email settings
type Settings struct {

	// Default email configuration
	Defaults struct {
		Enabled    bool     `json:"enabled"`
		ProviderId string   `json:"providerId"`
		FromName   string   `json:"fromName"`
		FromEmail  string   `json:"fromEmail"`
		Cc         []string `json:"cc,omitempty"`
		Bcc        []string `json:"bcc,omitempty"`
	} `json:"defaults"`

	// Per-email configuration
	Order struct {
		Confirmation Setting `json:"confirmation"`
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
	if !s.Defaults.Enabled {
		setting.Enabled = false
	}

	if setting.FromEmail == "" {
		setting.FromEmail = s.Defaults.FromEmail
	}

	if setting.FromName == "" {
		setting.FromName = s.Defaults.FromName
	}

	if len(setting.Cc) == 0 {
		setting.Cc = s.Defaults.Cc
	}

	if len(setting.Bcc) == 0 {
		setting.Bcc = s.Defaults.Bcc
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
