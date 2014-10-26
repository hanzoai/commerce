package template

import (
	"os"
	"github.com/flosch/pongo2"
	"github.com/gin-gonic/gin"
	"crowdstart.io/middleware"
)

var templateCache map[string]*pongo2.Template
var cwd, _ = os.Getwd()

func Render(c *gin.Context, path string, ctx pongo2.Context) (err error) {
	// All templates are expected to be in templates dir
	path = cwd + "/templates/" + path

	template, ok := templateCache[path]
	if !ok {
		template, err = pongo2.FromFile(path)
		if err != nil {
			ctx := middleware.GetAppEngine(c)
			ctx.Errorf("Failed to fetch template: %v", err)
			return err
		}
	}

	if err := template.ExecuteWriter(ctx, c.Writer); err != nil {
		ctx := middleware.GetAppEngine(c)
		ctx.Errorf("Failed to render template: %v", err)
		return err
	}

	return
}
