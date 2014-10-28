package cart

import (
	"github.com/gin-gonic/gin"
	"crowdstart.io/util/template"
)

func Get(c *gin.Context) {
	template.Render(c, "store/cart.html")
}
