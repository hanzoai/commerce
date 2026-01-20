package form

import (
	"context"
	"fmt"

	"github.com/hanzoai/commerce/email"
	"github.com/hanzoai/commerce/models/form"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/submission"
	"github.com/hanzoai/commerce/models/subscriber"

	. "github.com/hanzoai/commerce/types"
)

var hanzoEmail = email.Email{Address: "noreplay@hanzo.io", Name: "Hanzo"}

// Forward email to configured recpeients
func forward(c context.Context, org *organization.Organization, f *form.Form, s interface{}) {
	if !f.Forward.Enabled {
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
	message.Subject = "New submission for form " + f.Name
	message.From = hanzoEmail
	message.AddTos(email.Email{Address: f.Forward.Email, Name: f.Forward.Name})
	message.ReplyTo = email.Email{Address: replyTo}
	message.HTML = html
	email.Send(c, message, nil)
}
