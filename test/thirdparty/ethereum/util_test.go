package test

import (
	. "hanzo.io/util/test/ginkgo"

	"hanzo.io/models/order"
	"hanzo.io/models/user"
	"hanzo.io/thirdparty/ethereum/util"
)

var _ = Describe("client.GasPrice2", func() {
	It("should work", func() {
		ord := order.New(db)
		wal, err := ord.GetOrCreateWallet(db)
		Expect(err).NotTo(HaveOccurred())

		ord.MustCreate()

		usr := user.New(db)
		usr.MustCreate()

		u, o, w, err := util.GetUserOrderByWallet(db, wal.Id())

		Expect(err).NotTo(HaveOccurred())

		Expect(u.Id()).To(Equal(usr.Id()))
		Expect(o.Id()).To(Equal(ord.Id()))
		Expect(w.Id()).To(Equal(wal.Id()))
	})
})
