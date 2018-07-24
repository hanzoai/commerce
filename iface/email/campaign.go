package email

import (
	"context"

	"hanzo.io/types/email"
)

type Campaign interface {
	CampaignGet(c context.Context, id string)
	CampaignCreate(context.Context, email.Email)
	CampaignUpdate(context.Context, email.Email)
	CampaignDelete(context.Context, email.Email)
}
