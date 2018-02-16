package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/referralprogram"
	"hanzo.io/models/referrer"
	"hanzo.io/models/types/currency"
)

var Referrer = New("referrer", func(c *context.Context) *referrer.Referrer {
	// Get namespaced db
	db := getNamespaceDb(c)

	u := User(c)

	ref := referrer.New(db)
	ref.UserId = u.Id()
	ref.GetOrCreate("UserId=", ref.UserId)
	ref.Program.Triggers = []int{0}
	ref.Program.Actions = []referralprogram.Action{referralprogram.Action{Type: referralprogram.StoreCredit}}
	ref.Program.Actions[0].CreditAction = referralprogram.CreditAction{Currency: currency.USD, Amount: currency.Cents(1000)}
	ref.MustPut()

	return ref
})
