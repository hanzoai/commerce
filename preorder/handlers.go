package preorder

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
)

func Get(c *gin.Context) {
	db := datastore.New(c)
	token := c.Params.ByName("token")

	// Should use token to lookup email
	user := new(models.User)
	db.GetKey("user", token, user)

	log.Debug("%#v", user)
	template.Render(c, "preorder.html", "user", user)
}

func Login(c *gin.Context) {
	template.Render(c, "login.html")
}
