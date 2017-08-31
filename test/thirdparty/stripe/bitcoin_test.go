package test

import (
	"hanzo.io/models/payment"
	"hanzo.io/models/user"
	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("thirdparty.stripe.client.CreateSource", func() {

	Context("When CreateSource is called", func() {
		It("Should do a thing", func() {
			pay := payment.Payment{Amount: 20420}
			usr := user.User{Email: "dev@hanzo.ai"}

			client.CreateSource(&pay, &usr)
		})
	})
})
