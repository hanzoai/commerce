package form

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/schema"
)

var decoder = schema.NewDecoder()

type Form struct{}

func (f *Form) Parse(c *gin.Context) error {
	c.Request.ParseForm()
	return decoder.Decode(f, c.Request.PostForm)
}

func (f Form) Validate() (errs []string) {
	return errs
}
