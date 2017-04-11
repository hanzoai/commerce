package integrations

import (
	"errors"
	"time"

	"hanzo.io/util/json"
	"hanzo.io/util/rand"
)

var (
	ErrorNotFound     = errors.New("Integration not found")
	ErrorInvalidType  = errors.New("Invalid Type")
	ErrorTooMany      = errors.New("Too many of this Integration found")
	ErrorIdTypeNotSet = errors.New("Integration ID and Type not set")
)

type Integrations []Integration

func Update(src, dst *Integration) error {
	switch src.Type {
	case AnalyticsCustomType:
		json.DecodeBytes(src.Data, &dst.AnalyticsCustom)
	case AnalyticsFacebookPixelType:
		json.DecodeBytes(src.Data, &dst.AnalyticsFacebookPixel)
	case AnalyticsFacebookConversionsType:
		json.DecodeBytes(src.Data, &dst.AnalyticsFacebookConversions)
	case AnalyticsHeapType:
		json.DecodeBytes(src.Data, &dst.AnalyticsHeap)
	case AnalyticsSentryType:
		json.DecodeBytes(src.Data, &dst.AnalyticsSentry)
	case MailchimpType:
		json.DecodeBytes(src.Data, &dst.Mailchimp)
	case MandrillType:
		json.DecodeBytes(src.Data, &dst.Mandrill)
	case NetlifyType:
		json.DecodeBytes(src.Data, &dst.Netlify)
	case PaypalType:
		json.DecodeBytes(src.Data, &dst.Paypal)
	case ReamazeType:
		json.DecodeBytes(src.Data, &dst.Reamaze)
	case RecaptchaType:
		json.DecodeBytes(src.Data, &dst.Recaptcha)
	case SalesforceType:
		json.DecodeBytes(src.Data, &dst.Salesforce)
	case ShipwireType:
		json.DecodeBytes(src.Data, &dst.Shipwire)
	case StripeType:
		json.DecodeBytes(src.Data, &dst.Stripe)
	default:
		return ErrorInvalidType
	}

	dst.Enabled = src.Enabled
	dst.UpdatedAt = time.Now()

	return nil
}

func (i Integrations) Create(src Integration) error {
	switch src.Type {
	case AnalyticsCustomType:
	case AnalyticsFacebookPixelType:
	case AnalyticsFacebookConversionsType:
	case AnalyticsHeapType:
	case AnalyticsSentryType:
	default:
		if len(i.FilterByType(src.Type)) > 0 {
			return ErrorTooMany
		}
	}

	dst := Integration{}
	dst.Id = rand.ShortId()
	dst.Type = src.Type
	dst.CreatedAt = time.Now()

	if err := Update(&src, &dst); err != nil {
		return err
	}

	i = append(i, dst)
	return nil
}

func (i Integrations) Update(in Integration) error {
	if in.Id != "" {
		dst, err := i.FindById(in.Id)
		if err != nil {
			return err
		}
		err = Update(&in, dst)
		return err
	}

	if in.Type != "" {
		return i.Create(in)
	}

	return ErrorIdTypeNotSet
}

func (i Integrations) FilterByType(typ IntegrationType) Integrations {
	ins := Integrations{}
	for _, in := range i {
		if in.Type == typ {
			ins = append(ins, in)
		}
	}
	return ins
}

func (i Integrations) FindById(id string) (*Integration, error) {
	for _, in := range i {
		if in.Id == id {
			return &in, nil
		}
	}

	return nil, ErrorNotFound
}
