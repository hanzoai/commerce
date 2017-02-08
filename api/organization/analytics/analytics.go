package analytics

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/types/analytics"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"
)

func Get(c *gin.Context) {
	id := c.Params.ByName("organizationid")
	db := datastore.New(c)

	// Get organization
	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		log.Warn("Failed to retrieve organization '%v': %v", id, err, c)
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve organization '%v': %v", id, err), err)
		return
	}

	integrations := org.Analytics.UpdateShownDisabledStatus()
	http.Render(c, 200, integrations)
}

func Set(c *gin.Context) {
	id := c.Params.ByName("organizationid")
	db := datastore.New(c)

	// Get organization
	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		log.Warn("Failed to retrieve organization '%v': %v", id, err, c)
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve organization '%v': %v", id, err), err)
		return
	}

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
	id := c.Params.ByName("organizationid")
	db := datastore.New(c)

	// Get organization
	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		log.Warn("Failed to retrieve organization '%v': %v", id, err, c)
		http.Fail(c, 404, fmt.Sprintf("Failed to retrieve organization '%v': %v", id, err), err)
		return
	}

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
