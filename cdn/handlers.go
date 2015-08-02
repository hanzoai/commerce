package cdn

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

	"fmt"

	"github.com/gin-gonic/gin"

	"crowdstart.com/config"
	"crowdstart.com/datastore"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"

	. "crowdstart.com/util/aggregate/tasks"
	. "crowdstart.com/util/analytics/tasks"
)

var jsTemplate string

var subscriberEndpoint = config.UrlFor("api", "/subscriber/")

func mailingListJs(c *gin.Context) {
	db := datastore.New(c)

	id := c.Params.ByName("orgid")
	org := organization.New(db)
	if err := org.Get(id); err != nil {
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

func js(c *gin.Context) {
	id := c.Params.ByName("mailinglistid")
	db := datastore.New(c)

	ml := mailinglist.New(db)

	// Set key and namespace correctly
	ml.SetKey(id)
	log.Debug("mailinglist: %v", ml)
	log.Debug("key: %v", ml.Key())
	log.Debug("namespace: %v", ml.Key().Namespace())
	ml.SetNamespace(ml.Key().Namespace())

	if err := ml.Get(); err != nil {
		c.String(404, fmt.Sprintf("Failed to retrieve mailing list '%v': %v", id, err))
		return
	}

	c.Writer.Header().Add("Content-Type", "application/javascript")
	c.String(200, ml.Js())
}
