package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/form"
	"hanzo.io/models/types/thankyou"
)

var Form = New("form", func(c *gin.Context) *form.Form {
	db := getNamespaceDb(c)

	form := form.New(db)

	form.Name = "Such Tees Newsletter"
	form.SendWelcome = true
	form.Type = "signup"

	form.Mailchimp.Id = "30d872227a"
	form.Mailchimp.APIKey = "473b358fd2972742c8ef6af581c3c0-us9"
	form.Mailchimp.DoubleOptin = false
	form.Mailchimp.UpdateExisting = true
	form.Mailchimp.ReplaceInterests = false
	form.Mailchimp.SendWelcome = false
	form.Mailchimp.Enabled = true

	form.ThankYou.Type = thankyou.Redirect
	form.ThankYou.Url = "http://suchtees.com/thanks/"
	form.Facebook.Id = "6031480185266"
	form.Facebook.Value = "0.00"
	form.Facebook.Currency = "USD"

	form.Google.Category = "Subscription"
	form.Google.Name = "Newsletter Sign-up"

	form.MustUpdate()

	return form
})
