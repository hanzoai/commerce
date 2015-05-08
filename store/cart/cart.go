package cart

import (
	"crowdstart.com/util/template"
	"github.com/gin-gonic/gin"
)

func Get(c *gin.Context) {
	template.Render(c, "cart.html")
}
