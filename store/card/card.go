package card

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/util/template"
)

func GetCard(c *gin.Context) {
	user, _ := auth.GetUser(c)
	template.Render(c, "skullycard.html", "user", user)
}

func GetGiftCard(c *gin.Context) {
	user, _ := auth.GetUser(c)
	template.Render(c, "skullygiftcard.html", "from", user, "to", "Placeholder name")
}
