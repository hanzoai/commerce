package form

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/schema"
	"log"
)

var decoder = schema.NewDecoder()

func Parse(c *gin.Context, form interface{}) error {
	c.Request.ParseForm()
	err := decoder.Decode(form, c.Request.PostForm)
	log.Printf("%#v", form)

	return err
}
