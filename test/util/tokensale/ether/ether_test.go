package test

import (
	"testing"

	"hanzo.io/util/log"
	"hanzo.io/util/tokensale/ether"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("util/tokensale/ether", t)
}

// PubkeyToAddress is covered in this test too
var _ = Describe("ether.GenerateKeyPair", func() {
	It("should work", func() {
		priv, pub, add, err := ether.GenerateKeyPair()
		Expect(err).ToNot(HaveOccurred())

		log.Debug("\nPrivate: %v\nPublic: %v\nAddress: %v\n", priv, pub, add)

		Expect(len(priv)).To(Equal(64))
		Expect(len(pub)).To(Equal(128))
		Expect(len(add)).To(Equal(40))
	})

	It("should be random", func() {
		priv, pub, add, err := ether.GenerateKeyPair()
		Expect(err).ToNot(HaveOccurred())

		priv2, pub2, add2, err := ether.GenerateKeyPair()
		Expect(err).ToNot(HaveOccurred())

		Expect(priv).ToNot(Equal(priv2))
		Expect(pub).ToNot(Equal(pub2))
		Expect(add).ToNot(Equal(add2))
	})
})
