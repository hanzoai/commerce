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
	form := new(ForgotPasswordForm)
	if err := form.Parse(c); err != nil {
		template.Render(c, "forgot-password.html",
			"error", "Please enter your email.")
		return
	}

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)

	// Lookup email
	user := new(models.User)
	if err := db.GetKey("user", form.Email, user); err != nil {
		template.Render(c, "forgot-password.html",
			"error", "No account associated with that email.")
		return
	}

	// Save reset token
	token := new(models.Token)
	token.Email = user.Email
	token.GenerateId()
	if _, err := db.PutKey("reset-token", token.Id, token); err != nil {
		template.Render(c, "forgot-password.html",
			"error", "Failed to create reset token, please try again later.")
		return
	}

	mandrill.SendTemplateAsync.Call(ctx, "password-recovery", user.Email, user.Name(), "Recover your password")

	template.Render(c, "forgot-password-sent.html")
}
