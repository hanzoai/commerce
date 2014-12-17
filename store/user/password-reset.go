package user

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models"
	"crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// GET /password-reset
func PasswordReset(c *gin.Context) {
	template.Render(c, "password-reset.html")
}

// POST /password-reset
func PasswordResetSubmit(c *gin.Context) {
	form := new(ResetPasswordForm)
	if err := form.Parse(c); err != nil {
		template.Render(c, "password-reset.html", "error", "Please enter your new password.")
		return
	}

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)

	// Lookup email
	user := new(models.User)
	if err := db.GetKey("user", form.Email, user); err != nil {
		template.Render(c, "password-reset.html", "error", "No account associated with that email.")
		return
	}

	// Save reset token
	token := new(models.Token)
	token.Email = user.Email
	token.GenerateId()
	if _, err := db.PutKey("reset-token", token.Id, token); err != nil {
		template.Render(c, "password-reset.html", "error", "Failed to create reset token, please try again later.")
		return
	}

	resetUrl := "https:" + config.UrlFor("store", "/password-reset/", token.Id)
	mandrill.SendTransactional.Call(ctx, "email/password-reset.html",
		user.Email,
		user.Name(),
		"Recover your password",
		"resetUrl", resetUrl)

	template.Render(c, "password-reset-sent.html")
}

// GET /password-reset/:token
func PasswordResetConfirm(c *gin.Context) {
	db := datastore.New(c)
	tokenId := c.Params.ByName("token")

	// Verify token is valid.
	token := new(models.Token)
	err := db.GetKey("reset-token", tokenId, token)
	if err != nil {
		log.Warn("Invalid reset token: %v", err)
		template.Render(c, "password-reset-confirm.html", "invalidCode", true)
		return
	}

	template.Render(c, "password-reset-confirm.html", "email", token.Email)
}

// POST /password-reset/:token
func PasswordResetConfirmSubmit(c *gin.Context) {
	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)
	tokenId := c.Params.ByName("token")

	// Verify token is valid.
	token := new(models.Token)
	err := db.GetKey("reset-token", tokenId, token)
	if err != nil {
		log.Warn("Invalid reset token: %v", err)
		template.Render(c, "password-reset-confirm.html", "invalidCode", true)
		return
	}

	// Lookup user by email
	user := new(models.User)
	if err := db.GetKey("user", token.Email, user); err != nil {
		template.Render(c, "password-reset-confirm.html", "invalidEmail", true)
		return
	}

	// Parse reset form
	form := new(ResetPasswordConfirmForm)
	if err := form.Parse(c); err != nil {
		template.Render(c, "password-reset-confirm.html", "error", "Please enter your new password.")
		return
	}

	if form.NewPassword == form.ConfirmPassword {
		user.PasswordHash = auth.HashPassword(form.NewPassword)
	} else {
		template.Render(c, "password-reset-confirm.html", "error", "Passwords to not match")
		return
	}

	// Update user
	if _, err = db.PutKey("user", user.Email, user); err != nil {
		log.Panic("Failed to save user: %v", err)
	}

	// Notify user of password reset
	mandrill.SendTransactional.Call(ctx, "email/password-updated.html",
		user.Email,
		user.Name(),
		"SKULLY password changed")

	// Redirect to profile
	c.Redirect(302, config.UrlFor("store", "/profile"))
}
