package email

import (
	"hanzo.io/types/email"
)

type Provider interface {
	Send(message *email.Message) error
}

type ListProvider interface {
	Subscribe(listid string, s *email.Subscriber) error
}
