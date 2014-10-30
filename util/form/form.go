package form

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/schema"
)

func Parse(c *gin.Context, form interface{}) error {
	decoder := schema.NewDecoder()
	c.Request.ParseForm()
	return decoder.Decode(form, c.Request.PostForm)
}
