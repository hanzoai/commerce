package emails

import (
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/product"
	"hanzo.io/models/user"
	"hanzo.io/util/log"
	"hanzo.io/util/template"

	"golang.org/x/net/context"

	mandrill "hanzo.io/thirdparty/mandrill/tasks"
)

func SendOrderConfirmationEmail(ctx context.Context, org *organization.Organization, ord *order.Order, usr *user.User) {
	conf := org.Email.OrderConfirmation.Config(org)
	if !conf.Enabled || org.Mandrill.APIKey == "" {
		log.Debug("Skip Email", ctx)
		return
	}

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
