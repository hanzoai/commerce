package integrations

import (
	"errors"
	"time"

	"hanzo.io/util/json"
	"hanzo.io/util/log"
	"hanzo.io/util/rand"
)

var (
	ErrorNotFound     = errors.New("Integration not found")
	ErrorInvalidType  = errors.New("Invalid Type")
	ErrorTooMany      = errors.New("Too many of this Integration found")
	ErrorIdTypeNotSet = errors.New("Integration ID and Type not set")
)

type Integrations []Integration

func Encode(src *Integration, dst *Integration) error {
	switch src.Type {
	case AnalyticsCustomType:
		dst.Data = json.EncodeBytes(src.AnalyticsCustom)
	case AnalyticsFacebookPixelType:
		dst.Data = json.EncodeBytes(src.AnalyticsFacebookPixel)
	case AnalyticsFacebookConversionsType:
		dst.Data = json.EncodeBytes(src.AnalyticsFacebookConversions)
	case AnalyticsGoogleAdwordsType:
		dst.Data = json.EncodeBytes(src.AnalyticsGoogleAdwords)
	case AnalyticsGoogleAnalyticsType:
		dst.Data = json.EncodeBytes(src.AnalyticsGoogleAnalytics)
	case AnalyticsHeapType:
		dst.Data = json.EncodeBytes(src.AnalyticsHeap)
	case AnalyticsSentryType:
		dst.Data = json.EncodeBytes(src.AnalyticsSentry)
	case MailchimpType:
		dst.Data = json.EncodeBytes(src.Mailchimp)
	case MandrillType:
		dst.Data = json.EncodeBytes(src.Mandrill)
	case NetlifyType:
		dst.Data = json.EncodeBytes(src.Netlify)
	case PaypalType:
		dst.Data = json.EncodeBytes(src.Paypal)
	case ReamazeType:
		dst.Data = json.EncodeBytes(src.Reamaze)
	case RecaptchaType:
		dst.Data = json.EncodeBytes(src.Recaptcha)
	case SalesforceType:
		dst.Data = json.EncodeBytes(src.Salesforce)
	case ShipwireType:
		dst.Data = json.EncodeBytes(src.Shipwire)
	case StripeType:
		dst.Data = json.EncodeBytes(src.Stripe)
	default:
		return ErrorInvalidType
	}

	dst.Enabled = src.Enabled
	dst.Show = src.Show

	dst.Id = src.Id
	dst.Type = src.Type

	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
	return nil
}

func Decode(src *Integration, dst *Integration) error {
	switch src.Type {
	case AnalyticsCustomType:
		dst.AnalyticsCustom = src.AnalyticsCustom
	case AnalyticsFacebookPixelType:
		dst.AnalyticsFacebookPixel = src.AnalyticsFacebookPixel
	case AnalyticsFacebookConversionsType:
		dst.AnalyticsFacebookConversions = src.AnalyticsFacebookConversions
	case AnalyticsGoogleAdwordsType:
		dst.AnalyticsGoogleAdwords = src.AnalyticsGoogleAdwords
	case AnalyticsGoogleAnalyticsType:
		dst.AnalyticsGoogleAnalytics = src.AnalyticsGoogleAnalytics
	case AnalyticsHeapType:
		dst.AnalyticsHeap = src.AnalyticsHeap
	case AnalyticsSentryType:
		dst.AnalyticsSentry = src.AnalyticsSentry
	case MailchimpType:
		dst.Mailchimp = src.Mailchimp
	case MandrillType:
		dst.Mandrill = src.Mandrill
	case NetlifyType:
		dst.Netlify = src.Netlify
	case PaypalType:
		dst.Paypal = src.Paypal
	case ReamazeType:
		dst.Reamaze = src.Reamaze
	case RecaptchaType:
		dst.Recaptcha = dst.Recaptcha
	case SalesforceType:
		dst.Salesforce = dst.Salesforce
	case ShipwireType:
		dst.Shipwire = dst.Shipwire
	case StripeType:
		dst.Stripe = dst.Stripe
	default:
		return ErrorInvalidType
	}

	if len(src.Data) > 0 {
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
	}

	dst.Enabled = src.Enabled
	dst.Show = src.Show

	dst.Type = src.Type

	dst.UpdatedAt = time.Now()

	return nil
}

func (i Integrations) Append(src *Integration) (Integrations, error) {
	switch src.Type {
	case AnalyticsCustomType:
	case AnalyticsFacebookPixelType:
	case AnalyticsFacebookConversionsType:
	case AnalyticsHeapType:
	case AnalyticsSentryType:
	default:
		if len(i.FilterByType(src.Type)) > 0 {
			return i, ErrorTooMany
		}
	}

	dst := Integration{}

	dst.Id = rand.ShortId()
	dst.Type = src.Type

	dst.CreatedAt = time.Now()

	if err := Decode(src, &dst); err != nil {
		return i, err
	}

	log.Debug("Length %v", len(i))
	// log.Warn("Appending %s", dst.Type)
	ins := append(i, dst)
	// log.Warn("List")
	// for _, in := range ins {
	// 	log.Warn("%s", in.Type)
	// }
	log.Debug("Length %v", len(i))
	src.Id = dst.Id
	src.CreatedAt = dst.CreatedAt
	src.UpdatedAt = dst.UpdatedAt
	return ins, nil
}

func (i Integrations) MustAppend(src *Integration) Integrations {
	if ins, err := i.Append(src); err != nil {
		panic(err)
	} else {
		return ins
	}
}

func (i Integrations) Update(src *Integration) (Integrations, error) {
	if src.Id != "" {
		dst, err := i.FindById(src.Id)
		if err != nil {
			return i, err
		}

		ins := Integrations{}
		for _, in := range i {
			if in.Id == src.Id {
				err = Decode(src, dst)
				ins = append(ins, *dst)
			} else {
				ins = append(ins, in)
			}
		}
		log.Debug("Before %s\nAfter %s", dst, src)
		src.UpdatedAt = dst.UpdatedAt
		return ins, err
	}

	if src.Type != "" {
		return i.Append(src)
	}

	return i, ErrorIdTypeNotSet
}

func (i Integrations) MustUpdate(in *Integration) Integrations {
	if ins, err := i.Update(in); err != nil {
		panic(err)
	} else {
		return ins
	}
}

func (i Integrations) Remove(id string) (Integrations, error) {
	ins := Integrations{}
	for _, in := range i {
		// Go is terrible
		// if in.Id == id {
		// 	ins := append(i[:j], i[j+1:]...)
		// 	return ins, nil
		// }
		if in.Id != id {
			ins = append(ins, in)
		}
	}

	if len(ins) != len(i) {
		return ins, nil
	}

	return i, ErrorNotFound
}

func (i Integrations) MustRemove(id string) Integrations {
	// log.Warn("ListB.1")
	// for _, in := range i {
	// 	log.Warn("%s - %s", in.Id, in.Type)
	// }
	if ins, err := i.Remove(id); err != nil {
		panic(err)
	} else {
		// log.Warn("ListB.2")
		// for _, in := range i {
		// 	log.Warn("%s - %s", in.Id, in.Type)
		// }
		return ins
	}
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
