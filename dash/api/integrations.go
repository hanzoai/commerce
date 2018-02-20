package api

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
)

func Mailchimp(c *gin.Context) {
	org := middleware.GetOrganization(c)

	http.Render(c, 200, org.Mailchimp)
}

func UpdateMailchimp(c *gin.Context) {
	org := middleware.GetOrganization(c)
	if err := json.Decode(c.Request.Body, &org.Mailchimp); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	org.MustUpdate()

	c.Writer.WriteHeader(204)
}

func Mandrill(c *gin.Context) {
	org := middleware.GetOrganization(c)

	http.Render(c, 200, org.Mandrill)
}

func UpdateMandrill(c *gin.Context) {
	org := middleware.GetOrganization(c)
	if err := json.Decode(c.Request.Body, &org.Mandrill); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	org.MustUpdate()

	c.Writer.WriteHeader(204)
}

func Netlify(c *gin.Context) {
	org := middleware.GetOrganization(c)

	http.Render(c, 200, org.Netlify)
}

func UpdateNetlify(c *gin.Context) {
	org := middleware.GetOrganization(c)
	if err := json.Decode(c.Request.Body, &org.Netlify); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	org.MustUpdate()

	c.Writer.WriteHeader(204)
}

func Affiliate(c *gin.Context) {
	org := middleware.GetOrganization(c)

	http.Render(c, 200, org.Affiliate)
}

func UpdateAffiliate(c *gin.Context) {
	org := middleware.GetOrganization(c)
	if err := json.Decode(c.Request.Body, &org.Affiliate); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	org.MustUpdate()

	c.Writer.WriteHeader(204)
}

func Reamaze(c *gin.Context) {
	org := middleware.GetOrganization(c)

	http.Render(c, 200, org.Reamaze)
}

func UpdateReamaze(c *gin.Context) {
	org := middleware.GetOrganization(c)
	if err := json.Decode(c.Request.Body, &org.Reamaze); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	org.MustUpdate()

	c.Writer.WriteHeader(204)
}

func Shipwire(c *gin.Context) {
	org := middleware.GetOrganization(c)

	http.Render(c, 200, org.Shipwire)
}

func UpdateShipwire(c *gin.Context) {
	org := middleware.GetOrganization(c)
	if err := json.Decode(c.Request.Body, &org.Shipwire); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	org.MustUpdate()

	c.Writer.WriteHeader(204)
}

func Recaptcha(c *gin.Context) {
	org := middleware.GetOrganization(c)

	http.Render(c, 200, org.Recaptcha)
}

func UpdateRecaptcha(c *gin.Context) {
	org := middleware.GetOrganization(c)
	if err := json.Decode(c.Request.Body, &org.Recaptcha); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	org.MustUpdate()

	c.Writer.WriteHeader(204)
}
