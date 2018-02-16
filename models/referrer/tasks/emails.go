package tasks

import (
	"hanzo.io/datastore"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	"hanzo.io/util/delay"
	"hanzo.io/util/log"

	"google.golang.org/appengine"

	mandrill "hanzo.io/thirdparty/mandrill/tasks"
)

// Fire webhooks
var SendUserEmail = delay.Func("referrer-send-user-email", func(ctx context.Context, orgId string, templateName string, usrId string) {
	db := datastore.New(ctx)
	org := organization.New(db)
	if err := org.GetById(orgId); err != nil {
		log.Error("Could not get organization '%s', %s", orgId, err, ctx)
		return
	}

	// User Welcome stuff
	if !org.Email.Defaults.Enabled {
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
	fromEmail := org.Email.Defaults.FromEmail
	fromName := org.Email.Defaults.FromName

	// To
	toEmail := usr.Email
	toName := usr.Name()

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
	mandrill.SendTemplate(ctx, templateName, org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, "", vars)
})
