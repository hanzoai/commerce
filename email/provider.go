package email

import (
	"context"
)

type Provider interface {
	Send(c context.Context, e Email)
	SendCampaign(c context.Context, id string)
	SendTemplate(d context.Context, id string)
}

type Campaigns interface {
	CampaignGet(c context.Context, id string)
	CampaignCreate(context.Context, Email)
	CampaignUpdate(context.Context, Email)
	CampaignDelete(context.Context, Email)
}

type Subscribers interface {
	SubscriberGet(c context.Context, id string)
	SubscriberCreate(context.Context, Email)
	SubscriberUpdate(context.Context, Email)
	SubscriberDelete(context.Context, Email)
}

type Templates interface {
	TemplateGet(c context.Context, id string)
	TemplateCreate(context.Context, Email)
	TemplateUpdate(context.Context, Email)
	TemplateDelete(context.Context, Email)
}

type Lists interface {
	ListGet(c context.Context, id string)
	ListCreate(context.Context, Email)
	ListUpdate(context.Context, Email)
	ListDelete(context.Context, Email)
}
