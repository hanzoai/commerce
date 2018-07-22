package tasks

import (
	"context"

	"hanzo.io/delay"

	"hanzo.io/config"
	"hanzo.io/log"
	"hanzo.io/thirdparty/mandrill"
	"hanzo.io/thirdparty/sendgrid"
	"hanzo.io/types/email"
)

// Send email with appropriate provider
var Send = delay.Func("send-email", func(ctx context.Context, provider email.Provider, apiKey string, message email.Message) {
	log.Debug("Sending email to %s, %v", message.To[0], ctx)

	if provider == 

})

// Send email template with appropriate provider
func SendTemplate(ctx context.Context, template, apiKey, toEmail, toName, fromEmail, fromName, subject string, vars map[string]interface{}) {
})
