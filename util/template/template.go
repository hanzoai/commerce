package template

import (
	"os"

	"crowdstart.io/config"
	"github.com/flosch/pongo2"
	"github.com/gin-gonic/gin"
)

var cwd, _ = os.Getwd()

func TemplateSet() *pongo2.TemplateSet {
	set := pongo2.NewSet("default")
	set.Debug = config.IsDevelopment

	set.Globals["staticUrl"] = config.StaticUrl
	set.Globals["siteTitle"] = config.SiteTitle
	set.Globals["moduleUrl"] = func(moduleName string) string {
		return config.ModuleUrl(moduleName)
	}

	return set
}

var templateSet = TemplateSet()

func Render(c *gin.Context, path string, pairs ...interface{}) (err error) {
	// All templates are expected to be in templates dir
	path = cwd + "/templates/" + path

	// Get template from cache
	template, err := templateSet.FromCache(path)
	if err != nil {
		c.Fail(500, err)
		return err
	}

	// Create context from pairs
	ctx := pongo2.Context{}

	for i := 0; i < len(pairs); i = i + 2 {
		ctx[pairs[i].(string)] = pairs[i+1]
	}

	// Render template
	if err := template.ExecuteWriter(ctx, c.Writer); err != nil {
		c.Fail(500, err)
		return err
	}

	// Set content type
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")

	return
}
