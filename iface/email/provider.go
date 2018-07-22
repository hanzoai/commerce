package email

import (
	"hanzo.io/types/email"
)

type Provider interface {
	Send(message email.Message, subs []email.Substitution) error
	SendTemplate(message email.Message, subs []email.Substitution) error
}
