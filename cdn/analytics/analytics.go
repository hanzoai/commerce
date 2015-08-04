package analytics

import (
	"fmt"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/fs"
)

var jsTemplate = ""

func Render(org *organization.Organization) string {
	if jsTemplate == "" {
		var cwd, _ = os.Getwd()
		bundleJs := string(fs.ReadFile(cwd + "/resources/analytics/bundle.js"))
		jsTemplate = string(fs.ReadFile(cwd + "/resources/analytics/analytics.js"))
		jsTemplate = strings.Replace(jsTemplate, "require('./index')", bundleJs, 1)
		jsTemplate = strings.Replace(jsTemplate, "e('./index')", bundleJs, 1)
		jsTemplate = strings.Replace(jsTemplate, "analytics.initialize({})", "analytics.initialize(%s)", 1)
	}

	return fmt.Sprintf(jsTemplate, org.Analytics.JSON())
}

func Js(c *gin.Context) {
	id := c.Params.ByName("organizationid")
	db := datastore.New(c)

	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		c.String(404, fmt.Sprintf("Failed to retrieve organization '%v': %v", id, err))
		return
	}

	c.Writer.Header().Add("Content-Type", "application/javascript")
	c.String(200, Render(org))
}
