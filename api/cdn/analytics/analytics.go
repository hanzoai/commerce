package analytics

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/fs"
)

var (
	jsTemplate   = ""
	requireRegex = regexp.MustCompile(`require\(['"]./index['"]\)|,\w\(['"]./index['"]\)`)
)

func Render(org *organization.Organization) string {
	if jsTemplate == "" {
		var cwd, _ = os.Getwd()
		bundleJs := string(fs.ReadFile(cwd + "/resources/analytics/bundle.js"))
		jsTemplate = string(fs.ReadFile(cwd + "/resources/analytics/analytics.js"))
		jsTemplate = requireRegex.ReplaceAllString(jsTemplate, ";"+bundleJs)
		jsTemplate = strings.Replace(jsTemplate, "analytics.initialize({})", "analytics.initialize(%s)", 1)
	}

	return fmt.Sprintf(jsTemplate, org.Analytics.SnippetJSON())
}

func Js(c *zip.Ctx) error {
	id := c.Param("organizationid")

	// Passed organizationid as part of organization.js, strip extension.
	if strings.Contains(id, ".") {
		id = strings.Split(id, ".")[0]
	}

	db := datastore.New(c.Context())

	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		return c.String(404, fmt.Sprintf("Failed to retrieve organization '%v': %v", id, err))
	}

	c.SetHeader("Content-Type", "application/javascript")
	return c.String(200, Render(org))
}
