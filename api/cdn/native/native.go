package native

import (
	"os"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/fs"
	"github.com/hanzoai/commerce/util/json/http"
)

var jsTemplate string

func Js(c *zip.Ctx) error {
	db := datastore.New(c.Context())

	id := c.Param("organizationid")
	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		return http.Fail(c, 400, "Failed to get organization", err)
	}

	if jsTemplate == "" {
		var cwd, _ = os.Getwd()
		jsTemplate = string(fs.ReadFile(cwd + "/js/native.js"))
	}

	c.SetHeader("Content-Type", "application/javascript")

	script := strings.Replace(jsTemplate, "%%%%%url%%%%%", config.UrlFor("analytics", "/"+org.Id()+"/"), -1)

	return c.String(200, script)
}
