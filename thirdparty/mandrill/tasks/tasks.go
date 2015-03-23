package tasks

import (
	"appengine"
	"appengine/delay"

	"crowdstart.io/config"
	"crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/util/log"
	"crowdstart.io/util/template"
)

var SendTemplateAsync = delay.Func("send-template-email", func(ctx appengine.Context, template, toEmail, toName, subject string, vars ...mandrill.Var) {
	req := mandrill.NewSendTemplateReq()
	req.AddRecipient(toEmail, toName)

	req.Message.FromEmail = config.Mandrill.FromEmail
	req.Message.FromName = config.Mandrill.FromName
	req.Message.Subject = subject
	req.TemplateName = template

	for _, v := range vars {
		req.AddMergeVar(v)
	}

	log.Debug("Sending email to %s", toEmail, ctx)

	// Send template
	if err := mandrill.SendTemplate(ctx, &req); err != nil {
		log.Error("Failed to send email: %v", err, ctx)
	}
})

// Helper that will render a template and send it as body for
// transactional-template email.
var SendTransactionalSkully = delay.Func("send-template-email", func(ctx appengine.Context, templateName, toEmail, toName, subject string, args ...interface{}) {
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

// Helper that will render a template and send it as body for
// transactional-template email.
var SendTransactional = delay.Func("send-template-email", func(ctx appengine.Context, templateName, toEmail, toName, subject string, args ...interface{}) {
	req := mandrill.NewSendTemplateReq()
	req.AddRecipient(toEmail, toName)

	req.Message.FromEmail = config.Mandrill.FromEmail
	req.Message.FromName = config.Mandrill.FromName
	req.Message.Subject = subject
	req.TemplateName = "crowdstart-base"

	log.Debug("Sending email to %s", toEmail, ctx)

	// Render body
	body := template.RenderString(templateName, args...)

	req.AddMergeVar(mandrill.Var{"BODY", body})

	// Send template
	if err := mandrill.SendTemplate(ctx, &req); err != nil {
		log.Error("Failed to send email: %v", err, ctx)
	}
})
