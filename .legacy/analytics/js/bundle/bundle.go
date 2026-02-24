package bundle

import (
	"fmt"
	"os"
	"strings"

	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/fs"
)

var js = ""

func Render(o *organization.Organization) string {
	if js == "" {
		var cwd, _ = os.Getwd()
		bundlejs := string(fs.ReadFile(cwd + "/resources/analytics/bundle.js"))
		js = string(fs.ReadFile(cwd + "/resources/analytics/analytics.js"))
		js = strings.Replace(js, "require(\"./index\")", bundlejs, 1)
		js = strings.Replace(js, "e(\"./index\")", bundlejs, 1)
		js = strings.Replace(js, "analytics.initialize({})", "analytics.initialize(%s)", 1)
	}

	return fmt.Sprintf(js, o.Analytics.SnippetJSON())
}
