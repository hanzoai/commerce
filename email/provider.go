package email

import (
	"context"
)

type Provider interface {
	Send(context.Context, Email)
	SendTemplate(context.Context)

	TemplateGet(context.Context, Email)
	TemplateCreate(context.Context, Email)
	TemplateUpdate(context.Context, Email)
	TemplateDelete(context.Context, Email)
}
