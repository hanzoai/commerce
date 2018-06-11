package email

import (
	"context"

	"hanzo.io/types/email"
)

type Provider interface {
	Send(c context.Context, e email.Email)
	SendCampaign(c context.Context, id string)
	SendTemplate(d context.Context, id string)
}
