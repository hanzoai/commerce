package user

import (
	"crowdstart.io/util/form"
	"crowdstart.io/models"
)

type ForgotPasswordForm struct {
	Email string
}

func (f *ForgotPasswordForm) Parse(c *gin.Context) error {
	return form.Parse(c, f)
}
