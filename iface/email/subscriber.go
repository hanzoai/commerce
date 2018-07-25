package email

import (
	"context"

	"hanzo.io/types/email"
)

type Subscriber interface {
	SubscriberGet(c context.Context, id string)
	SubscriberCreate(context.Context, email.Email)
	SubscriberUpdate(context.Context, email.Email)
	SubscriberDelete(context.Context, email.Email)
}
