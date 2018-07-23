package email

import (
	"context"

	"hanzo.io/email/tasks"
	"hanzo.io/models/organization"
	"hanzo.io/types/email"
)

func SendEmail(c context.Context, org *organization.Organization, message *email.Message) error {
	in, err := org.Integrations.EmailProvider()
	if err != nil {
		return err
	}

	tasks.Send.Call(c, in, message)
	return nil
}
