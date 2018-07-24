package login

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/auth"
	"hanzo.io/auth/password"
	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/email"
	"hanzo.io/log"
	"hanzo.io/middleware"
	"hanzo.io/models/token"
	"hanzo.io/models/user"
	"hanzo.io/util/template"
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
	usr := user.New(db)
	if err := usr.GetByEmail(form.Email); err != nil {
		template.Render(c, "login/password-reset.html", "error", "No account associated with that email.")
		return
	}

	// Save reset token
	token := token.New(db)
	token.UserId = usr.Id()
	token.Email = usr.Email
	if err := token.Put(); err != nil {
		template.Render(c, "login/password-reset.html", "error", "Failed to create reset token, please try again later.")
		return
	}

	resetUrl := config.AbsoluteUrlFor("dash", "/password-reset/") + token.Id()

	message := email.NewMessage()
	message.Subject = "Reset your Hanzo password"
	message.AddTos(email.Email{
		Address: usr.Email,
		Name:    usr.Name(),
	})
	message.Substitutions["resetUrl"] = resetUrl
	email.SendResetPassword(c, nil, usr, token)
	template.Render(c, "login/password-reset-sent.html")
}

// GET /password-reset/:token
func PasswordResetConfirm(c *gin.Context) {
	db := datastore.New(c)
	tokenId := c.Params.ByName("token")

	// Verify token is valid.
	token := token.New(db)
	if err := token.GetById(tokenId); err != nil {
		log.Warn("Invalid reset token: %v", err)
		template.Render(c, "login/password-reset-confirm.html", "invalidCode", true)
		return
	}

	user := user.New(db)
	if err := user.GetById(token.UserId); err != nil {
		log.Warn("Reset token has invalid UserId: %v", err)
		template.Render(c, "login/password-reset-confirm.html", "invalidCode", true)
		return
	}

	template.Render(c, "login/password-reset-confirm.html", "email", user.Email)
}

// POST /password-reset/:token
func PasswordResetConfirmSubmit(c *gin.Context) {
	tokenId := c.Params.ByName("token")
	ctx := middleware.GetAppEngine(c)
	db := datastore.New(ctx)

	// Verify token is valid.
	token := token.New(db)
	if err := token.GetById(tokenId); err != nil {
		log.Warn("Invalid reset token: %v", err)
		template.Render(c, "login/password-reset-confirm.html", "invalidCode", true)
		return
	}

	// Lookup user by email
	usr := user.New(db)
	if err := usr.GetById(token.UserId); err != nil {
		template.Render(c, "login/password-reset-confirm.html", "invalidEmail", true)
		return
	}

	// Parse reset form
	form := new(PasswordResetConfirmForm)
	if err := form.Parse(c); err != nil {
		template.Render(c, "login/password-reset-confirm.html", "error", "Please enter your new password.")
		return
	}

	if form.NewPassword == form.ConfirmPassword {
		usr.PasswordHash, _ = password.Hash(form.NewPassword)
	} else {
		template.Render(c, "login/password-reset-confirm.html", "error", "Passwords to not match")
		return
	}

	// Update user
	if err := usr.Put(); err != nil {
		log.Panic("Failed to save user: %v", err)
	}

	// Notify user of password reset
	message := email.NewMessage()
	message.Subject = "Hanzo password changed"
	message.AddTos(email.Email{
		Address: usr.Email,
		Name:    usr.Name(),
	})
	email.SendUpdatePassword(ctx, nil, usr, token)

	// Login user
	auth.Login(c, usr)

	// Redirect to profile
	c.Redirect(302, config.UrlFor("dash"))
}
