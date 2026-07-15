package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/referral"
)

var Referral = New("referral", func(c *zip.Ctx) *referral.Referral {
	// Get namespaced db
	db := getNamespaceDb(c)

	ord := Order(c)
	u := User(c)

	// Referral
	ref := referral.New(db)
	ref.UserId = u.Id()
	ref.OrderId = ord.Id()
	ref.GetOrCreate("OrderId=", ref.OrderId)
	ref.MustPut()

	return ref
})
