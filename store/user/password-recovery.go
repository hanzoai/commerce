package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/util/template"
)

// GET /forgotpassword
func ForgotPassword(c *gin.Context) {
	template.Render(c, "forgot-password.html")
}

// POST /forgotpassword
func SubmitForgotPassword(c *gin.Context) {
	ctx := middleware.GetAppEngine(c)

	form := new(ForgotPasswordForm)
	err := form.Parse(c)
	if err != nil {
		template.Render(c, "forgot-password.html",
			"error", "Please enter your email.")
		return
	}

	db := datastore.New(ctx)
	var user models.User
	err = db.GetKey("user", form.Email, &user)
	if err != nil {
		template.Render(c, "forgot-password.html",
			"error", "No account associated with that email.")
		return
	}

	mandrill.SendTemplateAsync.Call(ctx, "password-recovery", user.Email, user.Name(), "Recover your password")

	template.Render(c, "forgot-password-sent.html")
}
