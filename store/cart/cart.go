package cart

import (
	"github.com/gin-gonic/gin"
	"crowdstart.io/util/template"
)

func Get(c *gin.Context) {
	if err := template.Render(c, "cart.html", nil); err != nil {
		c.String(500, "Unable to render template")
	}
}
