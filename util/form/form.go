package form

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/schema"

	"crowdstart.io/util/log"
)

var decoder = schema.NewDecoder()

func Parse(c *gin.Context, form interface{}) error {
	decoder.IgnoreUnknownKeys(true)
	c.Request.ParseForm()
	err := decoder.Decode(form, c.Request.PostForm)
	if err != nil {
		log.Panic("Parsing form %#v", err)
	}
	return err
}
