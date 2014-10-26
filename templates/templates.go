package templates

import (
	"github.com/flosch/pongo2"
	"github.com/gin-gonic/gin"
)

var templateCache map[string]*pongo2.Template

func Render(c *gin.Context, path string, ctx pongo2.Context) (err error) {
	// All templates are expected to be in templates dir
	path = "templates/" + path

	template, ok := templateCache[path]
	if !ok {
		template, err = pongo2.FromFile(path)
		if err != nil {
			return err
		}
	}

	if err := template.ExecuteWriter(ctx, c.Writer); err != nil {
		return err
	}

	return
}
