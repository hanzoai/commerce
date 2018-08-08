package email

import (
	"context"

	"hanzo.io/config"
	"hanzo.io/email/tasks"
	"hanzo.io/models/organization"
	"hanzo.io/types/email"
	"hanzo.io/util/template"

	"hanzo.io/log"
)

// Alias common types from "hanzo.io/types/email"
var NewMessage = email.NewMessage
var NewPersonalization = email.NewPersonalization

type Email = email.Email
type Setting email.Setting

const AffiliateWelcome = email.AffiliateWelcome
const OrderConfirmation = email.OrderConfirmation
const OrderRefund = email.OrderRefund
const OrderRefundPartial = email.OrderRefundPartial
const OrderShipped = email.OrderShipped
const OrderUpdated = email.OrderUpdated
const ReferralSignup = email.ReferralSignup
const SubscriberWelcome = email.SubscriberWelcome
const UserActivated = email.UserActivated
const UserConfirmEmail = email.UserConfirmEmail
const UserPasswordUpdated = email.UserPasswordUpdated
const UserResetPassword = email.UserResetPassword
const UserUpdated = email.UserUpdated
const UserWelcome = email.UserWelcome

// Send email
func Send(c context.Context, message *email.Message, org *organization.Organization) (err error) {
	// Default to built-in email provider
	in := &config.Email.Provider

	// If org is provider use their email provider
	if org != nil {
		if in, err = org.Integrations.EmailProvider(); err != nil {
			log.Error("Could not get Email Provider from org %v: %v", org.Name, err, c)
			return err
		} else if in == nil {
			return IntegrationShouldNotBeNilError
		}
	}

	// Fire off task to send email
	return tasks.Send.Call(c, *in, *message)
}

// Send email using server-side template
func SendTemplate(templatePath string, c context.Context, message *email.Message, org *organization.Organization) (err error) {
	if message.HTML == "" && message.TemplateID == "" {
		// Built-in tempate, we should render with handlebars
		log.Info("Using built in template %v", templatePath, c)
		message.HTML = template.RenderEmail(templatePath, message.TemplateData)
	}

	log.Info("Sending template %v", templatePath+"/"+message.TemplateID, c)
	return Send(c, message, org)
}
