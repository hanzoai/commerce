package integrations

import (
	"errors"

	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/types/analytics"
	"hanzo.io/models/types/integrations"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"
)

func Get(c *gin.Context) {
	org := middleware.GetOrganization(c)
	id := c.Params.ByName("organizationid")

	if id != org.Id() && id != org.Name && id != org.FullName {
		http.Fail(c, 403, "Organization Id does not match key", errors.New("Organization Id does not match key"))
		return
	}

	ins := org.Integrations
	for i, in := range ins {
		if err := integrations.Encode(&in, &in); err != nil {
			log.Warn("Could not encode integration: %s", err, c)
			continue
		}
		ins[i] = in
	}
	http.Render(c, 200, ins)
}

func Upsert(c *gin.Context) {
	org := middleware.GetOrganization(c)
	id := c.Params.ByName("organizationid")

	if id != org.Id() && id != org.Name && id != org.FullName {
		http.Fail(c, 403, "Organization Id does not match key", errors.New("Organization Id does not match key"))
		return
	}

	ins := org.Integrations
	in := integrations.Integration{}

	// Decode response body
	if err := json.Decode(c.Request.Body, &in); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Update integration
	if ins, err := ins.Update(&in); err != nil {
		http.Fail(c, 500, "Failed to save integrations", err)
	} else {
		org.Integrations = ins
	}

	ans := analytics.Analytics{}

	for _, in_ := range ins {
		an := analytics.Integration{}
		switch in_.Type {
		case integrations.AnalyticsCustomType:
			an.Type = "custom"
			an.Event = in_.AnalyticsCustom.Event
			an.Sampling = in_.AnalyticsCustom.Sampling
			an.Code = in_.AnalyticsCustom.Code
		case integrations.AnalyticsFacebookConversionsType:
			an.Type = "facebook-conversions"
			an.Event = in_.AnalyticsFacebookConversions.Event
			an.Sampling = in_.AnalyticsFacebookConversions.Sampling
			an.Value = in_.AnalyticsFacebookConversions.Value
			an.Currency = in_.AnalyticsFacebookConversions.Currency
		case integrations.AnalyticsFacebookPixelType:
			an.Type = "facebook-pixel"
			an.Event = in_.AnalyticsFacebookPixel.Event
			an.Sampling = in_.AnalyticsFacebookPixel.Sampling
			an.Values = analytics.Values(in_.AnalyticsFacebookPixel.Values)
		case integrations.AnalyticsGoogleAdwordsType:
			an.Type = "google-adwords"
		case integrations.AnalyticsGoogleAnalyticsType:
			an.Type = "google-analytics"
		case integrations.AnalyticsHeapType:
			an.Type = "heap"
		default:
			continue
		}
		an.Disabled = !in_.Enabled
		an.IntegrationId = in.Id
		ans.Integrations = append(ans.Integrations, an)
	}

	org.Analytics = ans

	// Synchronize integrations
	if mailchimps := org.Integrations.FilterByType(integrations.MailchimpType); len(mailchimps) > 0 {
		m := mailchimps[0]
		org.Mailchimp = m.Mailchimp
	}

	if mandrills := org.Integrations.FilterByType(integrations.MandrillType); len(mandrills) > 0 {
		m := mandrills[0]
		org.Mandrill = m.Mandrill
	}

	if netlifies := org.Integrations.FilterByType(integrations.NetlifyType); len(netlifies) > 0 {
		n := netlifies[0]
		org.Netlify = n.Netlify
	}

	if reamazes := org.Integrations.FilterByType(integrations.ReamazeType); len(reamazes) > 0 {
		r := reamazes[0]
		org.Reamaze = r.Reamaze
	}

	if recaptchas := org.Integrations.FilterByType(integrations.RecaptchaType); len(recaptchas) > 0 {
		r := recaptchas[0]
		org.Recaptcha = r.Recaptcha
	}

	if shipwires := org.Integrations.FilterByType(integrations.ShipwireType); len(shipwires) > 0 {
		s := shipwires[0]
		org.Shipwire = s.Shipwire
	}

	if stripes := org.Integrations.FilterByType(integrations.StripeType); len(stripes) > 0 {
		s := stripes[0]
		org.Stripe = s.Stripe
	}

	// Save organization
	if err := org.Update(); err != nil {
		http.Fail(c, 500, "Failed to save integrations", err)
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path)
		http.Render(c, 201, ins)
	}
}
