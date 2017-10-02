package test

import (
	"hanzo.io/models/wallet"

	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("Account", func() {
	Context("CreateAccount", func() {
		var wal *wallet.Wallet

		Before(func() {
			wal = wallet.New(db)
		})

		It("should create an account and save it automatically if wallet is in the datastore", func() {
			wal.MustCreate()

			password := "Th1$1s@b@dp@$$w0rd"
			acc, err := wal.CreateAccount(wallet.Ethereum, []byte(password))

			Expect(err).ToNot(HaveOccurred())

			Expect(len(wal.Accounts)).To(Equal(1))
			Expect(wal.Accounts[0]).To(Equal(acc))

			enc := acc.Encrypted
			priv := acc.PrivateKey
			pub := acc.PublicKey
			add := acc.Address

			w2 := wallet.New(db)
			w2.MustGetById(wal.Id())

			Expect(len(w2.Accounts)).To(Equal(1))

			acc2 := w2.Accounts[0]
			Expect(acc2.Encrypted).To(Equal(enc))
			Expect(acc2.PrivateKey).To(Equal(""))
			Expect(acc2.PublicKey).To(Equal(pub))
			Expect(acc2.Address).To(Equal(add))

			err = acc2.Decrypt([]byte(password))
			Expect(err).ToNot(HaveOccurred())
			Expect(acc2.PrivateKey).To(Equal(priv))
		})

		It("should throw errors for unknown types", func() {
			wal.MustCreate()

			password := "Th1$1s@b@dp@$$w0rd"
			acc, err := wal.CreateAccount(wallet.Type("nopecoin"), []byte(password))

			Expect(err).To(Equal(wallet.InvalidTypeSpecified))
			Expect(acc).To(Equal(wallet.Account{}))
		})
	})
})
