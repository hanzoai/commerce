package native

import (
	"os"
	"strings"

	"appengine"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/fs"
	"crowdstart.com/util/json/http"
)

var jsTemplate string

func Js(c *gin.Context) {
	db := datastore.New(c)

	id := c.Params.ByName("organizationid")
	org := organization.New(db)
	if err := org.GetById(id); err != nil {
		http.Fail(c, 400, "Failed to get organization", err)
		return
	}

	if jsTemplate == "" {
		var cwd, _ = os.Getwd()
		jsTemplate = string(fs.ReadFile(cwd + "/js/native.js"))
	}

	// Endpoint for subscription
	endpoint := config.UrlFor("analytics", "/"+org.Id())
	if appengine.IsDevAppServer() {
		endpoint = "http://localhost:8080" + endpoint
	} else {
		endpoint = "https:" + endpoint
	}

	c.Writer.Header().Add("Content-Type", "application/javascript")

	script := strings.Replace(jsTemplate, "%%%%%url%%%%%", config.UrlFor("analytics", "/"+org.Id()+"/"), -1)

	c.String(200, script)
}
