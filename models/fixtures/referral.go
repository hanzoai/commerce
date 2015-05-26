package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/referral"
	"crowdstart.com/models/types/currency"
)

var Referral = New("referral", func(c *gin.Context) *referral.Referral {
	// Get namespaced db
	db := getNamespaceDb(c)

	// Referral
	ref := referral.New(db)
	ref.Name = "Such Referral"
	ref.GetOrCreate("Name=", ref.Name)
	ref.Triggers = []int{0}
	ref.Actions = []referral.Action{referral.Action{Type: referral.StoreCredit}}
	ref.Actions[0].Credit = referral.Credit{Currency: currency.USD, Amount: currency.Cents(1000)}
	ref.MustPut()

	return ref
})
