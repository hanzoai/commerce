package integrations

import (
	"errors"
	"time"
	// "net/http/httputil"

	"github.com/gin-gonic/gin"

	"hanzo.io/log"
	"hanzo.io/middleware"
	"hanzo.io/models/types/analytics"
	"hanzo.io/types/integration"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/rand"
)

func Get(c *gin.Context) {
	org := middleware.GetOrganization(c)
	id := c.Params.ByName("organizationid")

	if id != org.Id() && id != org.Name && id != org.FullName {
		http.Fail(c, 403, "Organization Id does not match key", errors.New("Organization Id does not match key"))
		return
	}

	ins := org.Integrations

	// Add a read only stripe integration (will need to do this with all other
	// oauths)
	if org.Stripe.AccessToken != "" {
		in := integration.Integration{Stripe: org.Stripe}
		in.Enabled = true
		in.Show = true
		in.Id = rand.ShortId()
		in.Type = integration.StripeType
		in.CreatedAt = time.Now()
		in.UpdatedAt = in.CreatedAt
		ins = append(ins, in)
	}

	for i, in := range ins {
		if err := integration.Encode(&in, &in); err != nil {
			log.Warn("Could not encode integration: %s", err, c)
			continue
		}
		ins[i] = in
	}

	http.Render(c, 200, ins)
}

func Delete(c *gin.Context) {
	org := middleware.GetOrganization(c)
	id := c.Params.ByName("organizationid")

	if id != org.Id() && id != org.Name && id != org.FullName {
		http.Fail(c, 403, "Organization Id does not match key", errors.New("Organization Id does not match key"))
		return
	}

	iId := c.Params.ByName("integrationid")
	org.Integrations = org.Integrations.MustRemove(iId)

	// Save organization
	if err := org.Update(); err != nil {
		http.Fail(c, 500, "Failed to save integrations", err)
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path)
		http.Render(c, 201, org.Integrations)
	}
}

func Upsert(c *gin.Context) {
	org := middleware.GetOrganization(c)
	id := c.Params.ByName("organizationid")

	if id != org.Id() && id != org.Name && id != org.FullName {
		http.Fail(c, 403, "Organization Id does not match key", errors.New("Organization Id does not match key"))
		return
	}

	ins := org.Integrations
	updateIns := integration.Integrations{}

	// dump, _ := httputil.DumpRequestOut(c.Request, true)
	// log.Warn("Request %s", dump, c)

	// Decode response body
	if err := json.Decode(c.Request.Body, &updateIns); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// log.Warn("Received %s", string(json.EncodeBytes(in.Data)), c)

	org.RunInTransaction(func() error {
		// Update integration
		for _, in := range updateIns {
			if ins_, err := ins.Update(&in); err != nil {
				http.Fail(c, 500, "Failed to save integrations", err)
				return err
			} else {
				ins = ins_
				org.Integrations = ins_
			}
		}

		log.Warn("Saved %s", json.Encode(org.Integrations), c)

		ans := analytics.Analytics{}

		for _, in := range org.Integrations {
			an := analytics.Integration{}
			switch in.Type {
			case integration.AnalyticsCustomType:
				an.Type = "custom"
				an.Id = in.AnalyticsCustom.Id
				an.Event = in.AnalyticsCustom.Event
				an.Sampling = in.AnalyticsCustom.Sampling
				an.Code = in.AnalyticsCustom.Code
				an.Disabled = !in.Enabled
			case integration.AnalyticsFacebookConversionsType:
				an.Type = "facebook-conversions"
				an.Id = in.AnalyticsFacebookConversions.Id
				an.Event = in.AnalyticsFacebookConversions.Event
				an.Sampling = in.AnalyticsFacebookConversions.Sampling
				an.Value = in.AnalyticsFacebookConversions.Value
				an.Currency = in.AnalyticsFacebookConversions.Currency
				an.Disabled = !in.Enabled
			case integration.AnalyticsFacebookPixelType:
				an.Type = "facebook-pixel"
				an.Id = in.AnalyticsFacebookPixel.Id
				an.Event = in.AnalyticsFacebookPixel.Event
				an.Sampling = in.AnalyticsFacebookPixel.Sampling
				an.Values = analytics.Values(in.AnalyticsFacebookPixel.Values)
				an.Disabled = !in.Enabled
			case integration.AnalyticsGoogleAdwordsType:
				an.Type = "google-adwords"
				an.Id = in.AnalyticsGoogleAdwords.Id
				an.Event = in.AnalyticsGoogleAdwords.Event
				an.Sampling = in.AnalyticsGoogleAdwords.Sampling
				an.Disabled = !in.Enabled
			case integration.AnalyticsGoogleAnalyticsType:
				an.Type = "google-analytics"
				an.Id = in.AnalyticsGoogleAnalytics.Id
				an.Event = in.AnalyticsGoogleAnalytics.Event
				an.Sampling = in.AnalyticsGoogleAnalytics.Sampling
				an.Disabled = !in.Enabled
			case integration.AnalyticsHeapType:
				an.Type = "heap"
				an.Id = in.AnalyticsHeap.Id
				an.Event = in.AnalyticsHeap.Event
				an.Sampling = in.AnalyticsHeap.Sampling
				an.Disabled = !in.Enabled
			default:
				continue
			}
			an.Disabled = !in.Enabled
			an.IntegrationId = in.Id
			ans.Integrations = append(ans.Integrations, an)
		}

		org.Analytics = ans

		// Synchronize integrations
		if eth := org.Integrations.FilterByType(integration.BitcoinType); len(eth) > 0 {
			m := eth[0]
			org.Bitcoin = m.Bitcoin
		}

		// Synchronize integrations
		if eth := org.Integrations.FilterByType(integration.EthereumType); len(eth) > 0 {
			m := eth[0]
			org.Ethereum = m.Ethereum
		}

		// Synchronize integrations
		if mailchimps := org.Integrations.FilterByType(integration.MailchimpType); len(mailchimps) > 0 {
			m := mailchimps[0]
			org.Mailchimp = m.Mailchimp
		}

		if mandrills := org.Integrations.FilterByType(integration.MandrillType); len(mandrills) > 0 {
			m := mandrills[0]
			org.Mandrill = m.Mandrill
		}

		if netlifies := org.Integrations.FilterByType(integration.NetlifyType); len(netlifies) > 0 {
			n := netlifies[0]
			org.Netlify = n.Netlify
		}

		if reamazes := org.Integrations.FilterByType(integration.ReamazeType); len(reamazes) > 0 {
			r := reamazes[0]
			org.Reamaze = r.Reamaze
		}

		if recaptchas := org.Integrations.FilterByType(integration.RecaptchaType); len(recaptchas) > 0 {
			r := recaptchas[0]
			org.Recaptcha = r.Recaptcha
		}

		if shipwires := org.Integrations.FilterByType(integration.ShipwireType); len(shipwires) > 0 {
			s := shipwires[0]
			org.Shipwire = s.Shipwire
		}

		if stripes := org.Integrations.FilterByType(integration.StripeType); len(stripes) > 0 {
			s := stripes[0]
			org.Stripe = s.Stripe
		}

		// Save organization
		if err := org.Update(); err != nil {
			http.Fail(c, 500, "Failed to save integrations", err)
		} else {
			c.Writer.Header().Add("Location", c.Request.URL.Path)
			http.Render(c, 201, org.Integrations)
		}
		return nil
	}, nil)
}
