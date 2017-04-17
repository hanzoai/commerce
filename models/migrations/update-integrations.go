package migrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/organization"
	"hanzo.io/models/types/integrations"
	"hanzo.io/util/json"
	// "hanzo.io/util/log"

	ds "hanzo.io/datastore"
)

var _ = New("update-integrations",
	func(c *gin.Context) []interface{} {
		c.Set("namespace", "")
		return NoArgs
	},
	func(db *ds.Datastore, org *organization.Organization) {
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
			}

			in.BasicIntegration.Id = an.IntegrationId
			in.BasicIntegration.Data = json.EncodeBytes(an)

			org.Integrations.MustUpdate(&in)

			org.Analytics.Integrations[i].IntegrationId = in.BasicIntegration.Id
		}

		if mailchimps := org.Integrations.FilterByType(integrations.MailchimpType); len(mailchimps) > 0 {
			m := mailchimps[0]
			m.Mailchimp = org.Mailchimp
			org.Integrations.MustUpdate(&m)
		} else {
			m := integrations.Integration{
				Mailchimp: org.Mailchimp,
			}
			org.Integrations.MustUpdate(&m)
		}

		if mandrills := org.Integrations.FilterByType(integrations.MandrillType); len(mandrills) > 0 {
			m := mandrills[0]
			m.Mandrill = org.Mandrill
			org.Integrations.MustUpdate(&m)
		} else {
			m := integrations.Integration{
				Mandrill: org.Mandrill,
			}
			org.Integrations.MustUpdate(&m)
		}

		if netlifies := org.Integrations.FilterByType(integrations.NetlifyType); len(netlifies) > 0 {
			n := netlifies[0]
			n.Netlify = org.Netlify
			org.Integrations.MustUpdate(&n)
		} else {
			n := integrations.Integration{
				Netlify: org.Netlify,
			}
			org.Integrations.MustUpdate(&n)
		}

		if reamazes := org.Integrations.FilterByType(integrations.ReamazeType); len(reamazes) > 0 {
			r := reamazes[0]
			r.Reamaze = org.Reamaze
			org.Integrations.MustUpdate(&r)
		} else {
			r := integrations.Integration{
				Reamaze: org.Reamaze,
			}
			org.Integrations.MustUpdate(&r)
		}

		if recaptchas := org.Integrations.FilterByType(integrations.RecaptchaType); len(recaptchas) > 0 {
			r := recaptchas[0]
			r.Recaptcha = org.Recaptcha
			org.Integrations.MustUpdate(&r)
		} else {
			r := integrations.Integration{
				Recaptcha: org.Recaptcha,
			}
			org.Integrations.MustUpdate(&r)
		}

		if stripes := org.Integrations.FilterByType(integrations.MailchimpType); len(stripes) > 0 {
			s := stripes[0]
			s.Stripe = org.Stripe
			org.Integrations.MustUpdate(&s)
		} else {
			s := integrations.Integration{
				Stripe: org.Stripe,
			}
			org.Integrations.MustUpdate(&s)
		}

		org.Update()
	},
)
