package emails

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/middleware"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/product"
	"crowdstart.com/models/user"
	"crowdstart.com/util/template"

	mandrill "crowdstart.com/thirdparty/mandrill/tasks"
)

func SendOrderConfirmationEmail(c *gin.Context, org *organization.Organization, ord *order.Order, usr *user.User) {
	conf := org.Email.OrderConfirmation.Config(org)
	if !conf.Enabled || org.Mandrill.APIKey == "" {
		return
	}

	ctx := middleware.GetAppEngine(c)

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := usr.Email
	toName := usr.Name()

	prod := product.New(ord.Db)
	prod.GetById(ord.Items[0].ProductId)

	// Subject, HTML
	subject := conf.Subject
	html := template.RenderStringFromString(conf.Template,
		"order", ord,
		"orderId", ord.Id(),
		"user", usr,
		"estimatedDelivery", prod.EstimatedDelivery)

	mandrill.Send.Call(ctx, org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, html)
}
