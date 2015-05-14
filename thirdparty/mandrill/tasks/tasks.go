package tasks

import (
	"appengine"
	"appengine/delay"

	"crowdstart.io/config"
	"crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

// Helper that will render a template as body for one of our transactional
// template emails
var SendTransactional = delay.Func("mandrill-send-transactional", func(ctx appengine.Context, templateName, toEmail, toName, subject string, args ...interface{}) {
	req := mandrill.NewSendTemplateReq()
	req.AddRecipient(toEmail, toName)

	req.Message.FromEmail = config.Mandrill.FromEmail
	req.Message.FromName = config.Mandrill.FromName
	req.Message.Subject = subject
	req.TemplateName = "transactional-template"

	log.Debug("Sending email to %s", toEmail, ctx)

	// Render body
	body := template.RenderString(templateName, args...)

	req.AddMergeVar(mandrill.Var{"BODY", body})

	// Send template
	if err := mandrill.SendTemplate(ctx, &req); err != nil {
		log.Error("Failed to send email: %v", err, ctx)
	}
})

// Helper that will render a template and uses it for complete email
var Send = delay.Func("mandrill-send-email", func(ctx appengine.Context, templateName, toEmail, toName, subject string, args ...interface{}) {
	req := mandrill.NewSendReq()
	req.AddRecipient(toEmail, toName)

	req.Message.FromEmail = config.Mandrill.FromEmail
	req.Message.FromName = config.Mandrill.FromName
	req.Message.Subject = subject

	log.Debug("Sending email to %s", toEmail, ctx)

	// Render body
	req.Message.Html = template.RenderString(templateName, args...)

	// Send template
	if err := mandrill.Send(ctx, &req); err != nil {
		log.Error("Failed to send email: %v", err, ctx)
	}
})
