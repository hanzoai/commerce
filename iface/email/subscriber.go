package email

import (
	"context"

	"hanzo.io/models/mailinglist"
	"hanzo.io/models/subscriber"
	"hanzo.io/types/email"
)

type Subscriber interface {
	Subscribe(ml *mailinglist.MailingList, s *subscriber.Subscriber) error
}
