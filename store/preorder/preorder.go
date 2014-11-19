package preorder

import (
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
)

func Get(c *gin.Context) {
	template.Render(c, "preorder.html")
}
