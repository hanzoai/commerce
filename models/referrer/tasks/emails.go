package tasks

import (
	"context"
	"hanzo.io/datastore"
	"hanzo.io/delay"
	"hanzo.io/log"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	// mandrill "hanzo.io/thirdparty/mandrill/tasks"
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
	if !org.Email.Enabled {
		return
	}

	nsCtx := org.Namespaced(ctx)
	nsDb := datastore.New(nsCtx)
	usr := user.New(nsDb)
	if err := usr.GetById(usrId); err != nil {
		log.Error("Could not get user '%s'", usrId, ctx)
		return
	}

	// // From
	// from := org.Email.From

	// // To
	// toEmail := usr.Email
	// toName := usr.Name()

	// // Create Merge Vars
	// vars := map[string]interface{}{
	// 	"user": map[string]interface{}{
	// 		"firstname": usr.FirstName,
	// 		"lastname":  usr.LastName,
	// 	},
	// 	"USER_FIRSTNAME": usr.FirstName,
	// 	"USER_LASTNAME":  usr.LastName,
	// }

	// Send Email
	// mandrill.SendTemplate(ctx, templateName, org.Mandrill.APIKey, toEmail, toName, fromEmail, fromName, "", vars)
})
