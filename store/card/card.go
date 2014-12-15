package card

import (
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
)

func GetCard(c *gin.Context) {
	template.Render(c, "skullycard.html")
}
