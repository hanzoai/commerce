package form

import (
	"fmt"

	"appengine"

	"crowdstart.com/config"
	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/submission"
	"crowdstart.com/models/subscriber"

	. "crowdstart.com/models"

	mandrill "crowdstart.com/thirdparty/mandrill/tasks"
)

// Add subscriber to mailing list
func forward(ctx appengine.Context, org *organization.Organization, ml *mailinglist.MailingList, s interface{}) {
	if !ml.Forward.Enabled {
		return
	}

	replyTo := ""
	metadata := make(Metadata)

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
	fromEmail := "noreply@crowdstart.com"
	fromName := "Crowdstart"
	subject := "New submission for form " + ml.Name

	html := fmt.Sprintf("Form submission from: %v\n", replyTo)

	for k, v := range metadata {
		html += fmt.Sprintf("%s: %s\n", k, v)
	}

	mandrill.Send.Call(ctx, config.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, html)
}
