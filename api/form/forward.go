package form

import (
	"fmt"

	"golang.org/x/net/context"

	"hanzo.io/config"
	"hanzo.io/models/form"
	"hanzo.io/models/organization"
	"hanzo.io/models/submission"
	"hanzo.io/models/subscriber"

	. "hanzo.io/models"

	mandrill "hanzo.io/thirdparty/mandrill/tasks"
)

// Forward subscriber
func forward(ctx context.Context, org *organization.Organization, f *form.Form, s interface{}) error {
	if !f.Forward.Enabled {
		return nil
	}

	replyTo := ""
	metadata := make(Map)

	switch v := s.(type) {
	case *subscriber.Subscriber:
		replyTo = v.Email
		metadata = v.Metadata
	case *submission.Submission:
		replyTo = v.Email
		metadata = v.Metadata
	}

	// Forward form submission
	toEmail := f.Forward.Email
	toName := f.Forward.Name
	fromEmail := "noreply@hanzo.io"
	fromName := "Hanzo"
	subject := "New submission for form " + f.Name

	html := ""
	for k, v := range metadata {
		html += fmt.Sprintf("<b>%s</b>: %s<br><br>", k, v)
	}

	return mandrill.Forward.Call(ctx, config.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, replyTo, subject, html)
}
