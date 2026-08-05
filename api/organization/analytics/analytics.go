package analytics

import (
	"errors"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/types/analytics"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

func Get(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	integrations := org.Analytics.UpdateShownDisabledStatus()
	return http.Render(c, 200, integrations)
}

func Set(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)
	id := c.Param("organizationid")

	if id != org.Id() && id != org.Name && id != org.FullName {
		return http.Fail(c, 403, "Organization Id does not match key", errors.New("Organization Id does not match key"))
	}

	integrations := analytics.Analytics{}

	// Decode response body for listing
	if err := json.DecodeBytes(c.Body(), &integrations); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	integrations.UpdateStoredDisabledStatus()

	// Update integrations
	org.Analytics = integrations

	// Save organization
	if err := org.Put(); err != nil {
		return http.Fail(c, 500, "Failed to save analytics integrations", err)
	}
	c.SetHeader("Location", c.Path())
	return http.Render(c, 201, integrations)
}

func Update(c *zip.Ctx) error {
	// Get organization
	org := middleware.GetOrganization(c)
	id := c.Param("organizationid")

	if id != org.Id() && id != org.Name && id != org.FullName {
		return http.Fail(c, 403, "Organization Id does not match key", errors.New("Organization Id does not match key"))
	}

	// Decode response body for listing
	if err := json.DecodeBytes(c.Body(), &org.Analytics); err != nil {
		return http.Fail(c, 400, "Failed decode request body", err)
	}

	org.Analytics.UpdateStoredDisabledStatus()

	if err := org.Put(); err != nil {
		return http.Fail(c, 500, "Failed to save organization integrations", err)
	}
	c.SetHeader("Location", c.Path())
	return http.Render(c, 201, org.Analytics)
}
