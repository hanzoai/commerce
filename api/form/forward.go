package form

import (
	"fmt"

	"appengine"

	"hanzo.io/config"
	"hanzo.io/models/mailinglist"
	"hanzo.io/models/organization"
	"hanzo.io/models/submission"
	"hanzo.io/models/subscriber"

	. "hanzo.io/models"

	mandrill "hanzo.io/thirdparty/mandrill/tasks"
)

// Add subscriber to mailing list
func forward(ctx context.Context, org *organization.Organization, ml *mailinglist.MailingList, s interface{}) {
	if !ml.Forward.Enabled {
		return
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
	toEmail := ml.Forward.Email
	toName := ml.Forward.Name
	fromEmail := "noreply@hanzo.io"
	fromName := "Hanzo"
	subject := "New submission for form " + ml.Name

	html := ""
	for k, v := range metadata {
		html += fmt.Sprintf("<b>%s</b>: %s<br><br>", k, v)
	}

	mandrill.Forward.Call(ctx, config.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, replyTo, subject, html)
}
