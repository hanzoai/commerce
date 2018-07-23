package email

import (
	"hanzo.io/types/email"
)

type Provider interface {
	Send(message *email.Message) error
}
