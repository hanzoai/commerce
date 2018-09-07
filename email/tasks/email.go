package tasks

import (
	"context"
	"errors"

	"hanzo.io/delay"

	"hanzo.io/log"
	"hanzo.io/thirdparty/mandrill"
	"hanzo.io/thirdparty/sendgrid"
	"hanzo.io/thirdparty/smtprelay"
	"hanzo.io/types/email"
	"hanzo.io/types/integration"
)

func getEmailSender(c context.Context, in integration.Integration) (email.Sender, error) {
	switch in.Type {
	case integration.MandrillType:
		log.Info("Using Mandrill", c)
		return mandrill.New(c, in.Mandrill), nil
	case integration.SendGridType:
		log.Info("Using SendGrid", c)
		return sendgrid.New(c, in.SendGrid), nil
	case integration.SMTPRelayType:
		log.Info("Using SMTPRelay", c)
		return smtprelay.New(c, in.SMTPRelay), nil
	default:
		log.Error("Invalid Email Provider", c)
		return nil, errors.New("Invalid Email Provider")
	}
}

// Send email with appropriate provider
var Send = delay.Func("email-send", func(c context.Context, in integration.Integration, message *email.Message) error {
	log.Debug("Sending email to %s, %v", message.To[0], message, c)

	provider, err := getEmailSender(c, in)
	if err != nil {
		return log.Error("Email provider integration not found: %v", err, c)
	}

	err = provider.Send(message)
	if err != nil {
		return log.Error("Email provider error: %v", err, c)
	}

	return nil
})
