package fixtures

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/models/referral"
	"crowdstart.com/models/referralinstance"
)

var ReferralInstance = New("referralinstance", func(c *gin.Context) *referralinstance.ReferralInstance {
	// Get namespaced db
	db := getNamespaceDb(c)

	ref := Referral(c).(*referral.Referral)
	ord := Order(c)
	u := User(c)

	// ReferralInstance
	refIn := referralinstance.New(db)
	refIn.UserId = u.Id()
	refIn.GetOrCreate("UserId=", refIn.UserId)
	refIn.Referral = *ref
	refIn.OrderId = ord.Id()
	refIn.MustPut()

	return refIn
})
