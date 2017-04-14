package integrations

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/middleware"
	"hanzo.io/models/types/integrations"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
)

func List(c *gin.Context) {
	org := middleware.GetOrganization(c)
	http.Render(c, 200, org.Integrations)
}

func Upsert(c *gin.Context) {
	org := middleware.GetOrganization(c)
	ins := org.Integrations
	in := integrations.Integration{}

	// Decode response body
	if err := json.Decode(c.Request.Body, &in); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Update integration
	if ins, err := ins.Update(in); err != nil {
		http.Fail(c, 500, "Failed to save integrations", err)
	} else {
		org.Integrations = ins
	}

	// Save organization
	if err := org.Update(); err != nil {
		http.Fail(c, 500, "Failed to save integrations", err)
	} else {
		c.Writer.Header().Add("Location", c.Request.URL.Path)
		http.Render(c, 201, ins)
	}
}
