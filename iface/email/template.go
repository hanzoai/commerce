package email

import (
	"context"

	"hanzo.io/types/email"
)

type Template interface {
	TemplateGet(c context.Context, id string)
	TemplateCreate(context.Context, email.Email)
	TemplateUpdate(context.Context, email.Email)
	TemplateDelete(context.Context, email.Email)
}
