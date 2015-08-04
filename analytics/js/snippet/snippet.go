package snippet

import (
	"fmt"
	"os"

	"appengine"

	"crowdstart.com/config"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/fs"
)

var js = ""

func Render(org *organization.Organization) string {
	if js == "" {
		var cwd, _ = os.Getwd()
		js = string(fs.ReadFile(cwd + "/resources/analytics/snippet.js"))
	}

	endpoint := config.UrlFor("cdn", "/a/", org.Id(), "/analytics.js")
	if appengine.IsDevAppServer() {
		endpoint = "http://localhost:8080" + endpoint
	} else {
		endpoint = "https:" + endpoint
	}

	return fmt.Sprintf(js, endpoint)
}
