package tasks

import (
	"crowdstart.com/datastore"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/user"
	"crowdstart.com/util/delay"
	"crowdstart.com/util/log"

	"appengine"

	mandrill "crowdstart.com/thirdparty/mandrill/tasks"
	emails "crowdstart.com/util/emails"
)

// Fire webhooks
var SendUserEmail = delay.Func("referrer-send-user-email", func(ctx appengine.Context, orgId string, templateName string, usrId string) {
	db := datastore.New(ctx)
	org := organization.New(db)
	if err := org.GetById(orgId); err != nil {
		log.Error("Could not get organization '%s'", orgId, ctx)
		return
	}

	// User Welcome stuff
	conf := org.Email.User.Welcome.Config(org)
	if !emails.MandrillEnabled(ctx, org, conf) {
		return
	}

	nsCtx := org.Namespaced(ctx)
	nsDb := datastore.New(nsCtx)
	usr := user.New(nsDb)
	if err := usr.GetById(usrId); err != nil {
		log.Error("Could not get user '%s'", usrId, ctx)
		return
	}

	// From
	fromName := conf.FromName
	fromEmail := conf.FromEmail

	// To
	toEmail := usr.Email
	toName := usr.Name()

	// Subject
	subject := conf.Subject

	// Create Merge Vars
	vars := map[string]interface{}{
		"user": map[string]interface{}{
			"firstname": usr.FirstName,
			"lastname":  usr.LastName,
		},
		"USER_FIRSTNAME": usr.FirstName,
		"USER_LASTNAME":  usr.LastName,
	}

	// Send Email
	mandrill.SendTemplate(ctx, templateName, org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, subject, vars)
})
