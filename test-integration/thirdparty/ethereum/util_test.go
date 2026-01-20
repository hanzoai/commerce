package test

import (
	. "github.com/hanzoai/commerce/util/test/ginkgo"

	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/user"
	"github.com/hanzoai/commerce/thirdparty/ethereum/util"
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
