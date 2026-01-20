package snippet

import (
	"fmt"
	"os"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/fs"
)

var js = ""

func Render(org *organization.Organization) string {
	if js == "" {
		var cwd, _ = os.Getwd()
		js = string(fs.ReadFile(cwd + "/resources/analytics/snippet.js"))
	}

	endpoint := config.UrlFor("cdn", "/a/", org.Id(), "/analytics.js")
	if config.IsDevelopment {
		endpoint = "http://localhost:8080" + endpoint
	} else {
		endpoint = "https:" + endpoint
	}

	return fmt.Sprintf(js, endpoint)
}
