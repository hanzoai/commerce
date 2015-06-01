package emails

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
	"crowdstart.com/util/template"

	mandrill "crowdstart.com/thirdparty/mandrill/tasks"
)

func SendOrderConfirmationEmail(c *gin.Context, org *organization.Organization, ord *order.Order, usr *user.User) {
	if !org.Email.Enabled || !org.Email.OrderConfirmation.Enabled || org.Mandrill.APIKey == "" {
		return
	}

	ctx := middleware.GetAppEngine(c)

	ordConf := org.Email.OrderConfirmation

	// From
	fromEmail := org.Email.FromEmail
	fromName := org.Email.FromName
	if ordConf.FromEmail != "" {
		fromEmail = ordConf.FromEmail
	}
	if ordConf.FromName != "" {
		fromName = ordConf.FromName
	}

	// To
	toEmail := usr.Email
	toName := usr.Name()

	// Subject, HTML
	subject := ordConf.Subject
	html := template.RenderStringFromString(ordConf.Template,
		"order", ord,
		"orderId", ord.Id(),
		"user", usr)

	mandrill.Send.Call(ctx, org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, html)
}
