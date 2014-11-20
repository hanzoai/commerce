package preorder

import (
	"crowdstart.io/datastore"
	"crowdstart.io/models"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
	"log"
)

func Get(c *gin.Context) {
	db := datastore.New(c)
	slug := c.Params.ByName("slug")

	user := new(models.User)
	db.GetKey("user", slug, user)

	log.Printf("%#v", user)
	template.Render(c, "preorder.html", "user", user)
}
