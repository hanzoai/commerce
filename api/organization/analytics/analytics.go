package analytics

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/types/analytics"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
)

func Get(c *gin.Context) {
	org := middleware.GetOrganization(c)
	integrations := org.Analytics.UpdateShownDisabledStatus()
	http.Render(c, 200, integrations)
}

func Set(c *gin.Context) {
	org := middleware.GetOrganization(c)
	integrations := analytics.Analytics{}

	// Decode response body for listing
	if err := json.Decode(c.Request.Body, &integrations); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	integrations.UpdateStoredDisabledStatus()

	// Update integrations
	org.Analytics = integrations

	// Save organization
	if err := org.Put(); err != nil {
		http.Fail(c, 500, "Failed to save analytics integrations", err)
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path)
		http.Render(c, 201, integrations)
	}
}

func Update(c *gin.Context) {
	// Get organization
	org := middleware.GetOrganization(c)

	// Decode response body for listing
	if err := json.Decode(c.Request.Body, &org.Analytics); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	org.Analytics.UpdateStoredDisabledStatus()

	if err := org.Put(); err != nil {
		http.Fail(c, 500, "Failed to save organization integrations", err)
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path)
		http.Render(c, 201, org.Analytics)
	}
}
