package organization

import (
	"os"
	"strings"

	"google.golang.org/appengine"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/util/fs"
	"hanzo.io/util/json/http"
)

var nativeTemplate string

func nativeJs(c *gin.Context) {
	db := datastore.New(c)

	id := c.Params.ByName("organizationid")
	org := organization.New(db)
	if err := org.Get(id); err != nil {
		http.Fail(c, 400, "Failed to get organization", err)
		return
	}

	if nativeTemplate == "" {
		var cwd, _ = os.Getwd()
		nativeTemplate = string(fs.ReadFile(cwd + "/js/native.js"))
	}

	// Endpoint for subscription
	endpoint := config.UrlFor("analytics", "/"+org.Id())
	if appengine.IsDevAppServer() {
		endpoint = "http://localhost:8080" + endpoint
	} else {
		endpoint = "https:" + endpoint
	}

	c.Writer.Header().Add("Content-Type", "application/javascript")

	script := strings.Replace(nativeTemplate, "%%%%%url%%%%%", config.UrlFor("analytics", "/"+org.Id()+"/"), -1)

	c.String(200, script)
}
