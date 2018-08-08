package form

import (
	"context"
	"fmt"

	"hanzo.io/email"
	"hanzo.io/models/mailinglist"
	"hanzo.io/models/organization"
	"hanzo.io/models/submission"
	"hanzo.io/models/subscriber"

	. "hanzo.io/types"
)

var hanzoEmail = email.Email{Address: "noreplay@hanzo.io", Name: "Hanzo"}

// Add subscriber to mailing list
func forward(c context.Context, org *organization.Organization, ml *mailinglist.MailingList, s interface{}) {
	if !ml.Forward.Enabled {
		return
	}

	metadata := make(Map)

	// Determine where to send replies
	replyTo := ""

	switch v := s.(type) {
	case *subscriber.Subscriber:
		replyTo = v.Email
		metadata = v.Metadata
	case *submission.Submission:
		replyTo = v.Email
		metadata = v.Metadata
	}

	// Forward form submission
	html := ""
	for k, v := range metadata {
		html += fmt.Sprintf("<b>%s</b>: %s<br><br>", k, v)
	}

	// Setup email message
	message := email.NewMessage()
	message.Subject = "New submission for form " + ml.Name
	message.From = hanzoEmail
	message.AddTos(email.Email{Address: ml.Forward.Email, Name: ml.Forward.Name})
	message.ReplyTo = email.Email{Address: replyTo}
	message.HTML = html
	email.Send(c, message, nil)
}
