package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models/mailinglist"
	"crowdstart.io/models/types/thankyou"
)

var Mailinglist = New("mailinglist", func(c *gin.Context) *mailinglist.MailingList {
	db := getNamespaceDb(c)

	mailingList := mailinglist.New(db)

	mailingList.Name = "Teh Nooest Mailing List!"
	mailingList.SendWelcome = true
	mailingList.Mailchimp = mailinglist.Mailchimp{Id: "aowieij", APIKey: "23098fIOWJEOIJFW", DoubleOptin: false, UpdateExisting: true, ReplaceInterests: true, SendWelcome: false}
	mailingList.ThankYou.Type = thankyou.Redirect
	mailingList.ThankYou.Url = "http://suchtees.com/thanks/"

	mailingList.MustPut()

	return mailingList
})
