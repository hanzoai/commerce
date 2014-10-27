package template

import (
	"github.com/flosch/pongo2"
	"github.com/gin-gonic/gin"
	"os"
)

var cwd, _ = os.Getwd()

type Context map[string]interface{}

func Render(c *gin.Context, path string, pairs ...interface{}) (err error) {
	// All templates are expected to be in templates dir
	path = cwd + "/../templates/" + path

	// Get template from cache
	template, err := pongo2.FromCache(path)
	if err != nil {
		c.Fail(500, err)
		return err
	}

	// Create context from pairs
	ctx := pongo2.Context{}

	for i := 0; i < len(pairs); i=i+2 {
		ctx[pairs[i].(string)] = pairs[i+1]
	}

	// Render template
	if err := template.ExecuteWriter(ctx, c.Writer); err != nil {
		c.Fail(500, err)
		return err
	}

	return
}
