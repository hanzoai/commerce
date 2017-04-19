package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/organization"
	"hanzo.io/models/types/integrations"
	"hanzo.io/util/json"
	"hanzo.io/util/log"

	ds "hanzo.io/datastore"
)

var _ = New("update-integrations",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "")
		return NoArgs
	},
	func(db *ds.Datastore, org *organization.Organization) {
		org.Integrations = integrations.Integrations{}
		for i, an := range org.Analytics.Integrations {
			in := integrations.Integration{}
			switch an.Type {
			case "custom":
				in.Type = integrations.AnalyticsCustomType
			case "facebook-conversions":
				in.Type = integrations.AnalyticsFacebookConversionsType
			case "facebook-pixel":
				in.Type = integrations.AnalyticsFacebookPixelType
			case "google-adwords":
				in.Type = integrations.AnalyticsGoogleAdwordsType
			case "google-analytics":
				in.Type = integrations.AnalyticsGoogleAnalyticsType
			case "heap":
				in.Type = integrations.AnalyticsHeapType
			default:
				log.Warn("Analytics Type not supported: %s", an.Type, db.Context)
				continue
			}

			in.Id = an.IntegrationId
			in.Enabled = !an.Disabled
			in.Data = json.EncodeBytes(an)

			log.Debug("Updating Integration\nId: '%s'\nType: '%s'\nData: '%s'", in.Id, in.Type, in.Data, db.Context)
			if ins, err := org.Integrations.Update(&in); err == integrations.ErrorNotFound {
				in.Id = ""
				org.Integrations = org.Integrations.MustUpdate(&in)
			} else if err != nil {
				panic(err)
			} else {
				org.Integrations = ins
			}

			org.Analytics.Integrations[i].IntegrationId = in.Id
		}

		if mailchimps := org.Integrations.FilterByType(integrations.MailchimpType); len(mailchimps) > 0 {
			m := mailchimps[0]
			m.Mailchimp = org.Mailchimp
			org.Integrations = org.Integrations.MustUpdate(&m)
		} else {
			m := integrations.Integration{
				BasicIntegration: integrations.BasicIntegration{
					Type:    integrations.MailchimpType,
					Enabled: org.Mailchimp.APIKey != "",
				},
				Mailchimp: org.Mailchimp,
			}
			org.Integrations = org.Integrations.MustUpdate(&m)
		}

		if mandrills := org.Integrations.FilterByType(integrations.MandrillType); len(mandrills) > 0 {
			m := mandrills[0]
			m.Mandrill = org.Mandrill
			org.Integrations = org.Integrations.MustUpdate(&m)
		} else {
			m := integrations.Integration{
				BasicIntegration: integrations.BasicIntegration{
					Type:    integrations.MandrillType,
					Enabled: org.Mandrill.APIKey != "",
				},
				Mandrill: org.Mandrill,
			}
			org.Integrations = org.Integrations.MustUpdate(&m)
		}

		if netlifies := org.Integrations.FilterByType(integrations.NetlifyType); len(netlifies) > 0 {
			n := netlifies[0]
			n.Netlify = org.Netlify
			org.Integrations = org.Integrations.MustUpdate(&n)
		} else {
			n := integrations.Integration{
				BasicIntegration: integrations.BasicIntegration{
					Type:    integrations.NetlifyType,
					Enabled: org.Netlify.AccessToken != "",
				},
				Netlify: org.Netlify,
			}
			org.Integrations = org.Integrations.MustUpdate(&n)
		}

		if reamazes := org.Integrations.FilterByType(integrations.ReamazeType); len(reamazes) > 0 {
			r := reamazes[0]
			r.Reamaze = org.Reamaze
			org.Integrations = org.Integrations.MustUpdate(&r)
		} else {
			r := integrations.Integration{
				BasicIntegration: integrations.BasicIntegration{
					Type:    integrations.ReamazeType,
					Enabled: org.Reamaze.Secret != "",
				},
				Reamaze: org.Reamaze,
			}
			org.Integrations = org.Integrations.MustUpdate(&r)
		}

		if recaptchas := org.Integrations.FilterByType(integrations.RecaptchaType); len(recaptchas) > 0 {
			r := recaptchas[0]
			r.Recaptcha = org.Recaptcha
			org.Integrations = org.Integrations.MustUpdate(&r)
		} else {
			r := integrations.Integration{
				BasicIntegration: integrations.BasicIntegration{
					Type:    integrations.RecaptchaType,
					Enabled: org.Recaptcha.Enabled,
				},
				Recaptcha: org.Recaptcha,
			}
			org.Integrations = org.Integrations.MustUpdate(&r)
		}

		if shipwires := org.Integrations.FilterByType(integrations.ShipwireType); len(shipwires) > 0 {
			s := shipwires[0]
			s.Shipwire = org.Shipwire
			org.Integrations = org.Integrations.MustUpdate(&s)
		} else {
			s := integrations.Integration{
				BasicIntegration: integrations.BasicIntegration{
					Type:    integrations.ShipwireType,
					Enabled: org.Shipwire.Username != "",
				},
				Shipwire: org.Shipwire,
			}
			org.Integrations = org.Integrations.MustUpdate(&s)
		}

		// Stripe has third party structs where json is not set to omit empty,
		// therefore we have to use Data instead
		if stripes := org.Integrations.FilterByType(integrations.StripeType); len(stripes) > 0 {
			s := stripes[0]
			s.Data = json.EncodeBytes(org.Stripe)
			org.Integrations = org.Integrations.MustUpdate(&s)
			// log.Warn("Updating Stripe1 '%s'", string(json.EncodeBytes(org.Stripe)), db.Context)
			// log.Warn("Updating Stripe2 '%s'", string(json.EncodeBytes(s.Stripe)), db.Context)
		} else {
			s := integrations.Integration{
				BasicIntegration: integrations.BasicIntegration{
					Type:    integrations.StripeType,
					Enabled: org.Stripe.AccessToken != "",
					Data:    json.EncodeBytes(org.Stripe),
				},
			}
			org.Integrations = org.Integrations.MustUpdate(&s)
			// log.Warn("Updating Stripe3 '%s'", string(json.EncodeBytes(org.Stripe)), db.Context)
			// log.Warn("Updating Stripe4 '%s'", string(json.EncodeBytes(s.Stripe)), db.Context)
		}

		// log.Warn("Updating Integrations '%s'", string(json.Encode(org.Integrations)), db.Context)
		org.MustUpdate()
	},
)
