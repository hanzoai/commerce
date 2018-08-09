package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/form"
	"hanzo.io/models/types/thankyou"
)

var Form = New("form", func(c *gin.Context) *form.Form {
	db := getNamespaceDb(c)

	f := form.New(db)

	f.Name = "Such Tees Newsletter"
	f.SendWelcome = true
	f.Type = "signup"

	f.Mailchimp.ListId = "cc383800a7"
	f.Mailchimp.APIKey = "4a241ef914c2b098a3965d718c8f7f7e-us13"
	f.Mailchimp.DoubleOptin = false
	f.Mailchimp.UpdateExisting = true
	f.Mailchimp.ReplaceInterests = false
	f.Mailchimp.SendWelcome = false
	f.Mailchimp.Enabled = true

	f.ThankYou.Type = thankyou.Redirect
	f.ThankYou.Url = "http://suchtees.com/thanks/"
	f.Facebook.Id = "6031480185266"
	f.Facebook.Value = "0.00"
	f.Facebook.Currency = "USD"

	f.Google.Category = "Subscription"
	f.Google.Name = "Newsletter Sign-up"

	f.MustPut()

	return f
})
