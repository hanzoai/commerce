package account

import (
	"errors"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"

	"crowdstart.com/auth/password"
	"crowdstart.com/datastore"
	"crowdstart.com/middleware"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/token"
	"crowdstart.com/models/user"
	"crowdstart.com/util/json"
	"crowdstart.com/util/json/http"
	"crowdstart.com/util/template"

	mandrill "crowdstart.com/thirdparty/mandrill/tasks"
)

func sendEmailConfirmation(c *gin.Context, org *organization.Organization, usr *user.User) {
	conf := org.Email.User.EmailConfirmation.Config(org)
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

func sendEmailConfirmed(c *gin.Context, org *organization.Organization, usr *user.User) {
	conf := org.Email.User.EmailConfirmed.Config(org)
	if !conf.Enabled || org.Mandrill.APIKey == "" {
		return
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
	html := template.RenderStringFromString(conf.Template, "user", usr)

	// Send Email
	ctx := middleware.GetAppEngine(c)
	mandrill.Send.Call(ctx, org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, html)
}

func sendWelcome(c *gin.Context, org *organization.Organization, usr *user.User) {
	conf := org.Email.User.Welcome.Config(org)
	if !conf.Enabled || org.Mandrill.APIKey == "" {
		return
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
	html := template.RenderStringFromString(conf.Template, "user", usr)

	// Send Email
	ctx := middleware.GetAppEngine(c)
	mandrill.Send.Call(ctx, org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, html)
}

func create(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespace(c))
	usr := user.New(db)

	usrIn := &userIn{User: usr}

	// Default these fields to exotic unicode character to test if they are set to empty
	usr.FirstName = "\u263A"
	usr.LastName = "\u263A"

	// Decode response body to create new user
	if err := json.Decode(c.Request.Body, usrIn); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	if usr.FirstName == "" {
		http.Fail(c, 400, "First name cannot be blank", errors.New("First name cannot be blank"))
		return
	} else if usr.FirstName == "\u263A" {
		usr.FirstName = ""
	}

	if usr.LastName == "" {
		http.Fail(c, 400, "Last name cannot be blank", errors.New("Last name cannot be blank"))
		return
	} else if usr.LastName == "\u263A" {
		usr.LastName = ""
	}

	if err := usr.GetByEmail(usr.Email); err == nil {
		http.Fail(c, 400, "Email is in use", errors.New("Email is in use"))
		return
	}

	if ok, _ := regexp.MatchString("(\\w[-._\\w]*\\w@\\w[-._\\w]*\\w\\.\\w{2,3})", usr.Email); !ok {
		http.Fail(c, 400, "Email is not valid", errors.New("Email is not valid"))
		return
	}

	// Check for required fields
	if usr.Email == "" {
		http.Fail(c, 400, "Email is required", errors.New("Email is required"))
		return
	}

	if len(usrIn.Password) < 6 {
		http.Fail(c, 400, "Password needs to be atleast 6 characters", errors.New("Password needs to be atleast 6 characters"))
		return
	}

	if usrIn.Password != usrIn.PasswordConfirm {
		http.Fail(c, 400, "Passwords need to match", errors.New("Passwords need to match"))
		return
	}

	if hash, err := password.Hash(usrIn.Password); err != nil {
		http.Fail(c, 400, "Failed to hash user password", err)
	} else {
		usr.PasswordHash = hash
	}

	if err := usr.Put(); err != nil {
		http.Fail(c, 400, "Failed to create user", err)
	}

	// Send welcome, email confirmation emails
	sendEmailConfirmation(c, org, usr)
	sendWelcome(c, org, usr)
}

func createConfirm(c *gin.Context) {
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

	// Set user as enabled
	usr.Enabled = true
	err := usr.Put()
	if err != nil {
		panic(err)
	}

	// Send account confirmed email
	sendEmailConfirmed(c, org, usr)

	http.Render(c, 200, gin.H{"status": "ok"})
}
