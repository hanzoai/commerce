package tasks

import (
	"context"
	"errors"

	"hanzo.io/delay"

	iface "hanzo.io/iface/email"
	"hanzo.io/log"
	"hanzo.io/thirdparty/mandrill"
	"hanzo.io/thirdparty/sendgrid"
	"hanzo.io/thirdparty/smtprelay"
	"hanzo.io/types/email"
	"hanzo.io/types/integration"
)

func getProvider(c context.Context, in integration.Integration) (iface.Provider, error) {
	switch in.Type {
	case integration.MandrillType:
		return mandrill.New(c, in.Mandrill), nil
	case integration.SendGridType:
		return sendgrid.New(c, in.SendGrid), nil
	case integration.SMTPRelayType:
		return smtprelay.New(c, in.SMTPRelay), nil
	default:
		return nil, errors.New("Invalid Email Provider")
	}
}

// Send email with appropriate provider
var Send = delay.Func("send-email", func(c context.Context, in integration.Integration, message email.Message) {
	log.Debug("Sending email to %s, %v", message.To[0], c)
	provider, err := getProvider(c, in)
	if err != nil {
		log.Error("Email provider integration not found")
	}
	provider.Send(message)
})
