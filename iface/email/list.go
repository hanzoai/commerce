package email

import (
	"context"

	"hanzo.io/types/email"
)

type List interface {
	ListGet(c context.Context, id string)
	ListCreate(context.Context, email.Email)
	ListUpdate(context.Context, email.Email)
	ListDelete(context.Context, email.Email)
}
