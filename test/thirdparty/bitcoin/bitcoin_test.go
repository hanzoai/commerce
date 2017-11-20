package test

import (
	"bytes"
	"encoding/hex"
	"hanzo.io/thirdparty/bitcoin"
	"testing"

	"hanzo.io/util/log"
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
	It("should not screw up during transaction creation", func() {
		senderPubKey, senderPrivKey, _ := bitcoin.GenerateKeyPair()
		receiver1PubKey, _, _ := bitcoin.GenerateKeyPair()
		receiver2PubKey, _, _ := bitcoin.GenerateKeyPair()

		senderTestNetAddress, _, _ := bitcoin.PubKeyToTestNetAddress(senderPubKey)
		receiver1TestNetAddress, _, _ := bitcoin.PubKeyToTestNetAddress(receiver1PubKey)
		receiver2TestNetAddress, _, _ := bitcoin.PubKeyToTestNetAddress(receiver2PubKey)

		tempScript := bitcoin.CreateScriptPubKey(senderTestNetAddress)
		rawTransaction := bitcoin.CreateRawTransaction([]string{"5b60d0684a8201ddac20f713782a1f03682b508e90d99d0887b4114ad4ccfd2c"}, []int{0}, []string{receiver1TestNetAddress, receiver2TestNetAddress}, []int{1000, 5000}, tempScript)
		log.Info("initial raw transaction created.")
		hashCodeType, err := hex.DecodeString("01000000")
		log.Info("Hash code type created.")
		Expect(err).To(BeNil())
		var rawTransactionBuffer bytes.Buffer
		rawTransactionBuffer.Write(rawTransaction)
		rawTransactionBuffer.Write(hashCodeType)
		rawTransactionWithHashCodeType := rawTransactionBuffer.Bytes()
		log.Info("Raw transaction appended with hash code. %v", len(rawTransactionWithHashCodeType))
		finalSignature, err := bitcoin.GetRawTransactionSignature(rawTransactionWithHashCodeType, senderPrivKey)
		Expect(err).To(BeNil())
		rawTrx := bitcoin.CreateRawTransaction([]string{"5b60d0684a8201ddac20f713782a1f03682b508e90d99d0887b4114ad4ccfd2c"}, []int{0}, []string{receiver1TestNetAddress, receiver2TestNetAddress}, []int{1000, 5000}, finalSignature)
		log.Info("Final trx: %v", hex.EncodeToString(rawTrx))
		Expect(rawTrx).ToNot(BeNil())
	})
})
