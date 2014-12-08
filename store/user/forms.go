package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/util/form"
)

type ForgotPasswordForm struct {
	Email string
}

func (f *ForgotPasswordForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}
