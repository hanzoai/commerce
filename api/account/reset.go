package account

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/token"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/log"
	"crowdstart.com/util/template"

	mandrill "crowdstart.com/thirdparty/mandrill/tasks"
)

func SendPasswordReset(c *gin.Context, org *organization.Organization, usr *user.User) {
	conf := org.Email.User.PasswordReset.Config(org)
	if !conf.Enabled || org.Mandrill.APIKey == "" {
		return
	}

	// Create token
	tok := token.New(usr.Db)
	tok.Email = usr.Email
	tok.UserId = usr.Id()
	tok.Expires = time.Now().Add(time.Hour * 72)

	err := tok.Put()
	if err != nil {
		panic(err)
	}

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := usr.Email
	toName := usr.Name()

	// Subject
	subject := conf.Subject

	// Render email
	html := template.RenderStringFromString(conf.Template, "user", usr, "token", tok)

	// Send Email
	ctx := middleware.GetAppEngine(c)
	mandrill.Send.Call(ctx, org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, html)
}

func reset(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))
	usr := user.New(db)

	query := c.Request.URL.Query()
	email := query.Get("email")

	if err := usr.GetByEmail(email); err == nil {
		SendPasswordReset(c, org, usr)
	}

	http.Render(c, 200, gin.H{"status": "ok"})
}

type ConfirmPassword struct {
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
}

func resetConfirm(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))

	usr := user.New(db)
	tok := token.New(db)

	// Get Token
	id := c.Params.ByName("tokenid")
	if err := tok.GetById(id); err != nil {
		panic(err)
	}

	// Get user associated with token
	if err := usr.GetById(tok.UserId); err != nil {
		panic(err)
	}

	if tok.Expired() {
		http.Fail(c, 403, "Token expired", errors.New("Token expired"))
		return
	}

	// Get new password
	confirm := &ConfirmPassword{}
	if err := json.Decode(c.Request.Body, confirm); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	// Enable user in case this user has never confirmed account
	usr.Enabled = true

	// Validate password
	if len(confirm.Password) < 6 {
		http.Fail(c, 400, "Password needs to be atleast 6 characters", errors.New("Password needs to be atleast 6 characters"))
		return
	}

	if confirm.Password != confirm.ConfirmPassword {
		http.Fail(c, 400, "Passwords need to match", errors.New("Passwords need to match"))
		return
	}

	// Update password
	if err := usr.SetPassword(confirm.Password); err != nil {
		http.Fail(c, 500, "Failed to set password", err)
		return
	}

	if err := usr.Put(); err != nil {
		http.Fail(c, 500, "Failed to update password", err)
		return
	}

	// Save token
	tok.Used = true
	if err := tok.Put(); err != nil {
		log.Warn("Unable to update token", err, c)
	}

	http.Render(c, 200, gin.H{"status": "ok"})
}
