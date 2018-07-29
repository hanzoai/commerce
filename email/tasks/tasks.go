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
	"hanzo.io/util/json"
)

func getProvider(c context.Context, in integration.Integration) (iface.Provider, error) {
	switch in.Type {
	case integration.MandrillType:
		log.Info("Using Mandrill: %v", json.Encode(in.Mandrill), c)
		return mandrill.New(c, in.Mandrill), nil
	case integration.SendGridType:
		log.Info("Using SendGrid: %v", json.Encode(in.SendGrid), c)
		return sendgrid.New(c, in.SendGrid), nil
	case integration.SMTPRelayType:
		log.Info("Using SMTPRelay: %v", json.Encode(in.SMTPRelay), c)
		return smtprelay.New(c, in.SMTPRelay), nil
	default:
		log.Error("Invalid Email Provider", c)
		return nil, errors.New("Invalid Email Provider")
	}
}

// Send email with appropriate provider
var Send = delay.Func("send-email", func(c context.Context, in integration.Integration, message email.Message) {
	log.Debug("Sending email to %s, %v", message.To[0], c)
	provider, err := getProvider(c, in)
	if err != nil {
		log.Error("Email provider integration not found: %v", err, c)
		panic(err)
	}
	err = provider.Send(&message)
	if err != nil {
		log.Error("Email provider error: %v", err, c)
		panic(err)
	}
})
