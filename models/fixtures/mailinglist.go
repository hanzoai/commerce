package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/mailinglist"
	"crowdstart.com/models/types/thankyou"
)

var Mailinglist = New("mailinglist", func(c *gin.Context) *mailinglist.MailingList {
	db := getNamespaceDb(c)

	mailingList := mailinglist.New(db)

	mailingList.Name = "Such Tees Newsletter"
	mailingList.SendWelcome = true
	mailingList.Type = "signup"

	mailingList.Mailchimp.ListId = "cc383800a7"
	mailingList.Mailchimp.APIKey = "4a241ef914c2b098a3965d718c8f7f7e-us13"
	mailingList.Mailchimp.DoubleOptin = false
	mailingList.Mailchimp.UpdateExisting = true
	mailingList.Mailchimp.ReplaceInterests = false
	mailingList.Mailchimp.SendWelcome = false
	mailingList.Mailchimp.Enabled = true

	mailingList.ThankYou.Type = thankyou.Redirect
	mailingList.ThankYou.Url = "http://suchtees.com/thanks/"
	mailingList.Facebook.Id = "6031480185266"
	mailingList.Facebook.Value = "0.00"
	mailingList.Facebook.Currency = "USD"

	mailingList.Google.Category = "Subscription"
	mailingList.Google.Name = "Newsletter Sign-up"

	mailingList.MustPut()

	return mailingList
})
