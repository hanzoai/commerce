package test

import (
	"hanzo.io/models/wallet"

	. "hanzo.io/util/test/ginkgo"
)

var _ = Describe("Account", func() {
	var acc wallet.Account
	var password string

	BeforeEach(func() {
		_, acc, password = wallet.Fake(db)
	})

	Context("Encrypt/Decrypt", func() {
		It("should Encrypt/Decrypt properly", func() {
			enc := acc.Encrypted
			priv := acc.PrivateKey

			acc.PrivateKey = ""

			// Get the private key from the encrypted data
			err := acc.Decrypt([]byte(password))
			Expect(err).ToNot(HaveOccurred())
			Expect(acc.PrivateKey).To(Equal(priv))

			acc.Encrypted = ""

			err = acc.Encrypt([]byte(password))
			Expect(err).ToNot(HaveOccurred())

			// Should have random IV block each encryption system
			Expect(acc.Encrypted).ToNot(Equal(enc))

			acc.PrivateKey = ""

			err = acc.Decrypt([]byte(password))
			Expect(err).ToNot(HaveOccurred())
			Expect(acc.PrivateKey).To(Equal(priv))
		})

		It("should error with NoPrivateKeySetError", func() {
			acc.PrivateKey = ""

			err := acc.Encrypt([]byte(password))
			Expect(err).To(Equal(wallet.NoPrivateKeySetError))
		})

		It("should error with NoEncryptedKeyFound", func() {
			acc.Encrypted = ""

			err := acc.Decrypt([]byte(password))
			Expect(err).To(Equal(wallet.NoEncryptedKeyFound))
		})
	})

	Context("Delete", func() {
		It("should work properly", func() {
			Expect(acc.Deleted).To(Equal(false))
			acc.Delete()
			Expect(acc.Deleted).To(Equal(true))
		})
	})
})
