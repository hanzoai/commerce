package form

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/schema"
)

var decoder = schema.NewDecoder()

func Parse(c *gin.Context, form interface{}) error {
	c.Request.ParseForm()
	return decoder.Decode(form, c.Request.PostForm)
}
