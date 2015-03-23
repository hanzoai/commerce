package login

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/auth"
	"crowdstart.io/config"
	"crowdstart.io/datastore"
	"crowdstart.io/middleware"
	"crowdstart.io/models2/token"
	"crowdstart.io/models2/user"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"

	mandrill "crowdstart.io/thirdparty/mandrill/tasks"
)

// GET /password-reset
func PasswordReset(c *gin.Context) {
	template.Render(c, "login/password-reset.html")
}

// POST /password-reset
func PasswordResetSubmit(c *gin.Context) {
	form := new(PasswordResetForm)
	if err := form.Parse(c); err != nil {
		template.Render(c, "login/password-reset.html", "error", "Please enter your new password.")
		return
	}

	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)

	// Lookup email
	user := user.New(db)
	if err := user.GetByEmail(form.Email); err != nil {
		template.Render(c, "login/password-reset.html", "error", "No account associated with that email.")
		return
	}

	// Save reset token
	token := token.New(db)
	token.UserId = user.Id()
	token.Generate()
	if err := token.Put(); err != nil {
		template.Render(c, "login/password-reset.html", "error", "Failed to create reset token, please try again later.")
		return
	}

	resetUrl := "https:" + config.UrlFor("platform", "/password-reset/", token.Id())
	mandrill.SendTransactional.Call(ctx, "email/password-reset.html",
		user.Email,
		user.Name(),
		"Recover your password",
		"resetUrl", resetUrl)

	template.Render(c, "login/password-reset-sent.html")
}

// GET /password-reset/:token
func PasswordResetConfirm(c *gin.Context) {
	db := datastore.New(c)
	tokenId := c.Params.ByName("token")

	// Verify token is valid.
	token := token.New(db)
	if err := token.Get(tokenId); err != nil {
		log.Warn("Invalid reset token: %v", err)
		template.Render(c, "password-reset-confirm.html", "invalidCode", true)
		return
	}

	user := user.New(db)
	if err := user.Get(token.UserId); err != nil {
		log.Warn("Reset token has invalid UserId: %v", err)
		template.Render(c, "password-reset-confirm.html", "invalidCode", true)
		return
	}

	template.Render(c, "password-reset-confirm.html", "email", user.Email)
}

// POST /password-reset/:token
func PasswordResetConfirmSubmit(c *gin.Context) {
	tokenId := c.Params.ByName("token")
	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)

	// Verify token is valid.
	token := token.New(db)
	if err := token.Get(tokenId); err != nil {
		log.Warn("Invalid reset token: %v", err)
		template.Render(c, "password-reset-confirm.html", "invalidCode", true)
		return
	}

	// Lookup user by email
	user := user.New(db)
	if err := user.Get(token.UserId); err != nil {
		template.Render(c, "password-reset-confirm.html", "invalidEmail", true)
		return
	}

	// Parse reset form
	form := new(PasswordResetConfirmForm)
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
	if err := user.Put(); err != nil {
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
