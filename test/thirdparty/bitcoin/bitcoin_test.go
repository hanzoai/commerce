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
		straddr, byteaddr, err := bitcoin.PubKeyToAddress("0450863AD64A87AE8A2FE83C1AF1A8403CB53F53E486D8511DAD8A04887E5B23522CD470243453A299FA9E77237716103ABC11A1DF38855ED6F2EE187E9C582BA6", false)
		testaddr, testbyteaddr, err := bitcoin.PubKeyToAddress("0450863AD64A87AE8A2FE83C1AF1A8403CB53F53E486D8511DAD8A04887E5B23522CD470243453A299FA9E77237716103ABC11A1DF38855ED6F2EE187E9C582BA6", true)
		Expect(err).To(BeNil())
		Expect(len(byteaddr)).To(Equal(25))
		Expect(len(straddr)).To(Equal(33))
		Expect(len(testbyteaddr)).To(Equal(25))
		Expect(len(testaddr)).To(Equal(34))
	})
	It("should not screw up during transaction creation", func() {
		senderPubKey, senderPrivKey, _ := bitcoin.GenerateKeyPair()
		receiver1PubKey, _, _ := bitcoin.GenerateKeyPair()
		receiver2PubKey, _, _ := bitcoin.GenerateKeyPair()

		receiver1TestNetAddress, _, _ := bitcoin.PubKeyToAddress(receiver1PubKey, true)
		receiver2TestNetAddress, _, _ := bitcoin.PubKeyToAddress(receiver2PubKey, true)

		in := []bitcoin.Input{bitcoin.Input{TxId: "5b60d0684a8201ddac20f713782a1f03682b508e90d99d0887b4114ad4ccfd2c", OutputIndex: 0}}
		out := []bitcoin.Destination{bitcoin.Destination{Value: 1000, Address: receiver1TestNetAddress}, bitcoin.Destination{Value: 5000, Address: receiver2TestNetAddress}}
		senderAddress, _, _ := bitcoin.PubKeyToAddress(senderPubKey, false)
		senderTestNetAddress, _, _ := bitcoin.PubKeyToAddress(senderPubKey, true)
		senderAccount := bitcoin.Sender{
			PrivateKey:     senderPrivKey,
			PublicKey:      senderPubKey,
			Address:        senderAddress,
			TestNetAddress: senderTestNetAddress,
		}
		rawTrx, _ := bitcoin.CreateTransaction(in, out, senderAccount)
		Expect(rawTrx).ToNot(BeNil())
	})
})
