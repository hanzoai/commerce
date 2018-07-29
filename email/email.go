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

// Send email
func Send(c context.Context, message *email.Message, org *organization.Organization) (err error) {
	// Default to built-in email provider
	in := &config.Email.Provider

	// If org is provider use their email provider
	if org != nil {
		if in, err = org.Integrations.EmailProvider(); err != nil {
			log.Error("Could not get Email Provider from org %v: %v", org.Name, err, c)
			return err
		}
	}

	// Fire off task to send email
	return tasks.Send.Call(c, in, message)
}

// Send email using server-side template
func SendTemplate(templatePath string, c context.Context, message *email.Message, org *organization.Organization) (err error) {
	if(message.HTML == "" && message.TemplateID == "") {
		// Built-in tempate, we should render with handlebars
		log.Info("Using built in template %v", templatePath, c)
		message.HTML = template.RenderEmail(templatePath, message.TemplateData)
	}
	return Send(c, message, org)
}
