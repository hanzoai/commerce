package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/referrer"
	"crowdstart.com/models/types/currency"
)

var Referrer = New("referrer", func(c *gin.Context) *referrer.Referrer {
	// Get namespaced db
	db := getNamespaceDb(c)

	u := User(c)

	ref := referrer.New(db)
	ref.UserId = u.Id()
	ref.GetOrCreate("UserId=", ref.UserId)
	ref.Program.Triggers = []int{0}
	ref.Program.Actions = []referrer.Action{referrer.Action{Type: referrer.StoreCredit}}
	ref.Program.Actions[0].Credit = referrer.Credit{Currency: currency.USD, Amount: currency.Cents(1000)}
	ref.MustPut()

	return ref
})
