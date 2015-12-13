package form

import (
	"fmt"

	"appengine"

	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/organization"

	mandrill "crowdstart.com/thirdparty/mandrill/tasks"
)

// Add subscriber to mailing list
func forward(ctx appengine.Context, org *organization.Organization, ml *mailinglist.MailingList, s interface{}) {
	if !ml.Forward.Enabled {
		return
	}

	// Forward form submission
	toEmail := ml.Forward.Email
	toName := ml.Forward.Name
	fromEmail := "noreply@crowdstart.com"
	fromName := "Crowdstart"
	subject := "New submission for form " + ml.Name
	html := fmt.Sprintf("%v", s)
	mandrill.Send.Call(ctx, org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, html)
}
