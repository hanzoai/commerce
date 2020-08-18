package tasks

import (
	"context"
	"net/url"
	"strconv"

	"hanzo.io/datastore"
	"hanzo.io/delay"
	"hanzo.io/log"
	"hanzo.io/models/order"
	"hanzo.io/models/organization"
	"hanzo.io/models/user"
	// mandrill "hanzo.io/thirdparty/mandrill/tasks"
	"hanzo.io/thirdparty/woopra"
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

var SendWoopraEvent = delay.Func("referrer-send-woopra-event", func(ctx context.Context, orgId, domain, usrId, id, kind string) {
	db := datastore.New(ctx)
	org := organization.New(db)

	if err := org.GetById(orgId); err != nil {
		log.Error("Could not get organization '%s', %s", orgId, err, ctx)
		return
	}

	nsCtx := org.Namespaced(ctx)
	nsDb := datastore.New(nsCtx)
	usr := user.New(nsDb)
	if err := usr.GetById(usrId); err != nil {
		log.Panic("Could not get referring user '%s'", usrId, ctx)
		return
	}

	if err := usr.LoadReferrals(); err != nil {
		log.Panic("Could not load referring user's referrals '%s'", usrId, ctx)
		return
	}

	usrId2 := ""
	orderId := ""
	orderNumber := ""

	switch kind {
	case "order":
		log.Debug("Loading order referrent")
		ord := order.New(nsDb)
		orderId = id
		if err := ord.GetById(id); err != nil {
			log.Panic("Could not get referrent user '%s'", usrId, ctx)
			return
		}
		usrId2 = ord.UserId
		orderNumber = strconv.Itoa(ord.Number)
	case "user":
		log.Debug("Loading user referrent")
		usrId2 = id
	default:
		log.Panic("unknown kind %s", kind, ctx)
		return
	}

	usr2 := user.New(nsDb)
	if err := usr2.GetById(usrId2); err != nil {
		log.Panic("Could not get referrent user '%s'", usrId, ctx)
		return
	}

	wt, _ := woopra.NewTracker(map[string]string{

		// `host` is domain as registered in Woopra, it identifies which
		// project environment to receive the tracking request
		"host": domain,

		// In milliseconds, defaults to 30000 (equivalent to 30 seconds)
		// after which the event will expire and the visit will be marked
		// as offline.
		"timeout": "30000",
	})

	values := make(url.Values)
	values.Add("firstName", usr.FirstName)
	values.Add("lastName", usr.LastName)
	values.Add("city", usr.ShippingAddress.City)
	values.Add("country", usr.ShippingAddress.Country)
	values.Add("referred_by", usr.ReferrerId)
	values.Add("referrals", strconv.Itoa(len(usr.Referrals)))

	person := woopra.Person{
		Id:     usr.Id(),
		Name:   usr.Name(),
		Email:  usr.Email,
		Values: values,
	}

	// identifying current visitor in Woopra
	ident := wt.Identify(
		ctx,
		person,
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/601.7.7 (KHTML, like Gecko) Version/9.1.2 Safari/601.7.7",
	)

	// Tracking custom event in Woopra. Each event can has additional data
	ident.Track(
		"newReferral", // event name
		map[string]string{ // custom data
			"referredId":          usr2.Id(),
			"referredName":        usr2.Name(),
			"referredEmail":       usr2.Email,
			"referredOrderId":     orderId,
			"referredOrderNumber": orderNumber,
		})

	// it's possible to send only visitor's data to Woopra, without sending
	// any custom event and/or data
	ident.Push()
})
