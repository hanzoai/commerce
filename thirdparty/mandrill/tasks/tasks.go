package tasks

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/delay"

	"hanzo.io/config"
	"hanzo.io/thirdparty/mandrill"
	"hanzo.io/log"
	"hanzo.io/util/template"
)

// Helper that will render a template and uses it for complete email
var Send = delay.Func("send-email", func(ctx context.Context, apiKey, toEmail, toName, fromEmail, fromName, subject, html string) {
	req := mandrill.NewSendReq()
	req.AddRecipient(toEmail, toName)
	req.Key = apiKey

	req.Message.FromEmail = fromEmail
	req.Message.FromName = fromName
	req.Message.Subject = subject

	log.Debug("Sending email to %s, %v", toEmail, req, ctx)

	// Render body
	req.Message.Html = html

	// Send template
	if err := mandrill.Send(ctx, &req); err != nil {
		log.Panic("Failed to send email: %v", err, ctx)
	}
})

func SendTemplate(ctx context.Context, template, apiKey, toEmail, toName, fromEmail, fromName, subject string, vars map[string]interface{}) {
	sendTemplate.Call(ctx, template, apiKey, toEmail, toName, fromEmail, fromName, subject, vars)
}

var sendTemplate = delay.Func("send-email-template", func(ctx context.Context, template, apiKey, toEmail, toName, fromEmail, fromName, subject string, vars map[string]interface{}) {
	req := mandrill.NewSendTemplateReq()
	req.AddRecipient(toEmail, toName)
	req.Key = apiKey

	req.Message.FromEmail = fromEmail
	req.Message.FromName = fromName
	req.Message.Subject = subject
	req.TemplateName = template

	for k, v := range vars {
		req.AddMergeVar(mandrill.Var{k, v})
	}

	log.Debug("Sending '%s' email to '%s'", template, toEmail, ctx)

	// Send template
	if err := mandrill.SendTemplate(ctx, &req); err != nil {
		log.Panic("Failed to send email: %v", err, ctx)
	}
})

// Helper that will render a template and send it as body for
// transactional-template email.
var SendTransactional = delay.Func("send-email-transactional", func(ctx context.Context, templateName, toEmail, toName, subject string, args ...interface{}) {
	req := mandrill.NewSendTemplateReq()
	req.AddRecipient(toEmail, toName)

	req.Message.FromEmail = config.Mandrill.FromEmail
	req.Message.FromName = config.Mandrill.FromName
	req.Message.Subject = subject
	req.TemplateName = "crowdstart-base"

	log.Debug("Sending email to %s", toEmail, ctx)

	// Render body
	body := template.RenderString(nil, templateName, args...)

	req.AddMergeVar(mandrill.Var{"BODY", body})

	// Send template
	if err := mandrill.SendTemplate(ctx, &req); err != nil {
		log.Panic("Failed to send email: %v", err, ctx)
	}
})

// Helper to forward emails using custom reply-to address
var Forward = delay.Func("forward-email", func(ctx context.Context, apiKey, toEmail, toName, fromEmail, fromName, replyTo, subject, html string) {
	req := mandrill.NewSendReq()
	req.AddRecipient(toEmail, toName)
	req.Key = apiKey

	req.Message.FromEmail = fromEmail
	req.Message.FromName = fromName
	req.Message.Subject = subject
	req.Message.Headers.ReplyTo = replyTo

	log.Debug("Sending email to %s, %v", toEmail, req, ctx)

	// Render body
	req.Message.Html = html

	// Send template
	if err := mandrill.Send(ctx, &req); err != nil {
		log.Panic("Failed to send email: %v", err, ctx)
	}
})
