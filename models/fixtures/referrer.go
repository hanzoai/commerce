package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/referralprogram"
	"github.com/hanzoai/commerce/models/referrer"
	"github.com/hanzoai/commerce/models/types/currency"
)

var Referrer = New("referrer", func(c *zip.Ctx) *referrer.Referrer {
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
