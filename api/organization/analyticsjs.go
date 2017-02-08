package organization

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/util/fs"
)

var (
	analyticsTemplate = ""
	requireRegex      = regexp.MustCompile(`require\(['"]./index['"]\)|,\w\(['"]./index['"]\)`)
)

func Render(org *organization.Organization) string {
	if analyticsTemplate == "" {
		var cwd, _ = os.Getwd()
		bundleJs := string(fs.ReadFile(cwd + "/resources/analytics/bundle.js"))
		analyticsTemplate = string(fs.ReadFile(cwd + "/resources/analytics/analytics.js"))
		analyticsTemplate = requireRegex.ReplaceAllString(analyticsTemplate, ";"+bundleJs)
		analyticsTemplate = strings.Replace(analyticsTemplate, "analytics.initialize({})", "analytics.initialize(%s)", 1)
	}

	return fmt.Sprintf(analyticsTemplate, org.Analytics.SnippetJSON())
}

func analyticsJs(c *gin.Context) {
	id := c.Params.ByName("organizationid")

	// Passed organizationid as part of organization.js, strip extension.
	if strings.Contains(id, ".") {
		id = strings.Split(id, ".")[0]
	}

	db := datastore.New(c)

	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		c.String(404, fmt.Sprintf("Failed to retrieve organization '%v': %v", id, err))
		return
	}

	c.Writer.Header().Add("Content-Type", "application/javascript")
	c.String(200, Render(org))
}
