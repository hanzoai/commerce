package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/referral"
	"crowdstart.com/models/referrer"
)

var Referrer = New("referrer", func(c *gin.Context) *referrer.Referrer {
	// Get namespaced db
	db := getNamespaceDb(c)

	ref := Referral(c).(*referral.Referral)
	ord := Order(c)
	u := User(c)

	refIn := referrer.New(db)
	refIn.UserId = u.Id()
	refIn.GetOrCreate("UserId=", refIn.UserId)
	refIn.Referral = *ref
	refIn.OrderId = ord.Id()
	refIn.MustPut()

	return refIn
})
