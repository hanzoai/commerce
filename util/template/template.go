package template

import (
	"github.com/flosch/pongo2"
	"github.com/gin-gonic/gin"
	"os"
)

var cwd, _ = os.Getwd()

func Render(c *gin.Context, path string, ctx pongo2.Context) (err error) {
	// All templates are expected to be in templates dir
	path = cwd + "/../templates/" + path

	template, err := pongo2.FromCache(path)
	if err != nil {
		c.Fail(500, err)
		return err
	}

	if err := template.ExecuteWriter(ctx, c.Writer); err != nil {
		c.Fail(500, err)
		return err
	}

	return
}
