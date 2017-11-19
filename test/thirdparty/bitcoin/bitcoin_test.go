package test

import (
	"hanzo.io/thirdparty/bitcoin"
	"testing"

	. "hanzo.io/util/test/ginkgo"
)

func Test(t *testing.T) {
	Setup("thirdparty/bitcoin", t)
}

var _ = Describe("thirdparty.bitcoin", func() {
	It("should generate appropriate key pairs", func() {
		priv, pub, err := bitcoin.GenerateKeyPair()
		Expect(err).To(BeNil())
		Expect(len(priv)).To(Equal(64))
		Expect(len(pub)).To(Equal(130))
	})

	It("should generate appropriate addresses", func() {
		straddr, byteaddr, err := bitcoin.PubKeyToAddress("0450863AD64A87AE8A2FE83C1AF1A8403CB53F53E486D8511DAD8A04887E5B23522CD470243453A299FA9E77237716103ABC11A1DF38855ED6F2EE187E9C582BA6")
		testaddr, testbyteaddr, err := bitcoin.PubKeyToTestNetAddress("0450863AD64A87AE8A2FE83C1AF1A8403CB53F53E486D8511DAD8A04887E5B23522CD470243453A299FA9E77237716103ABC11A1DF38855ED6F2EE187E9C582BA6")
		Expect(err).To(BeNil())
		Expect(len(byteaddr)).To(Equal(25))
		Expect(len(straddr)).To(Equal(33))
		Expect(len(testbyteaddr)).To(Equal(25))
		Expect(len(testaddr)).To(Equal(34))
	})
})
