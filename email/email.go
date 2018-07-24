package email

import (
	"context"

	"hanzo.io/config"
	"hanzo.io/email/tasks"
	"hanzo.io/models/organization"
	"hanzo.io/types/email"
	"hanzo.io/util/template"
)

// Alias common types from "hanzo.io/types/email"
var NewMessage = email.NewMessage
var NewPersonalization = email.NewPersonalization

type Email = email.Email

// Send email
func Send(c context.Context, message *email.Message, org *organization.Organization) (err error) {
	// Default to built-in email provider
	in := &config.Email.Provider

	// If org is provider use their email provider
	if org != nil {
		if in, err = org.Integrations.EmailProvider(); err != nil {
			return err
		}
	}

	// Fire off task to send email
	return tasks.Send.Call(c, in, message)
}

// Send email using server-side template
func SendTemplate(templatePath string, c context.Context, message *email.Message, org *organization.Organization) (err error) {
	// Built-in tempate, we should render with handlebars
	message.HTML = template.RenderEmail(templatePath, message.TemplateData)
	return Send(c, message, org)
}
