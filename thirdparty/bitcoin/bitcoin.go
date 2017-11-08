package bitcoin

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/ripemd160"

	"github.com/btcsuite/btcutil/base58"
	secp256k1 "hanzo.io/thirdparty/bitcoin/crypto"
	"hanzo.io/thirdparty/ethereum/go-ethereum/crypto"
	"hanzo.io/util/log"
)

// The steps notated in the variable names here relate to the steps outlined in
// https://en.bitcoin.it/wiki/Technical_background_of_version_1_Bitcoin_addresses
func PubKeyToAddress(pubKey string, testNet bool) (string, []byte, error) {
	ripe := ripemd160.New()
	step2decode, err := hex.DecodeString(pubKey)
	if err != nil {
		return "", nil, err
	}
	step2 := sha256.Sum256(step2decode)

	log.Debug("public key: %v", pubKey)
	log.Debug("public key hex decode: %v", step2decode)
	log.Debug("Step 2 hex: %v", hex.EncodeToString(step2[:]))
	if len(step2) != 32 {
		return "", nil, fmt.Errorf("Step 2: Invalid length. %v", len(step2))
	}

	log.Debug("Step 2: %v", step2)
	ripe.Write(step2[:])
	step3 := ripe.Sum(nil)

	log.Debug("Step 3 hex: %v", hex.EncodeToString(step3))
	if len(step3) != 20 {
		return "", nil, fmt.Errorf("Step 3: Invalid length. %v", len(step3))
	}

	step4 := append([]byte{byte(0)}, step3...)

	log.Debug("Step 4 hex: %v", hex.EncodeToString(step4))
	if len(step4) != 21 {
		return "", nil, fmt.Errorf("Step 4: Invalid length. %v", len(step4))
	}

	step5 := sha256.Sum256(step4)

	log.Debug("Step 5 hex: %v", hex.EncodeToString(step5[:]))
	if len(step5) != 32 {
		return "", nil, fmt.Errorf("Step 5: Invalid length. %v", len(step5))
	}

	step6 := sha256.Sum256(step5[:])

	log.Debug("Step 6 hex: %v", hex.EncodeToString(step6[:]))
	if len(step6) != 32 {
		return "", nil, fmt.Errorf("Step 6: Invalid length. %v", len(step6))
	}
	step7 := step6[0:4]

	log.Debug("Step 7 hex: %v", hex.EncodeToString(step7[:]))
	if len(step7) != 4 {
		return "", nil, fmt.Errorf("Step 7: Invalid length. %v", len(step7))
	}
	step8 := append(step4, step7...)

	log.Debug("Step 8 hex: %v", hex.EncodeToString(step8[:]))
	log.Debug("Step 8 Base58 encode: %v", base58.Encode(step8))
	if len(step8) != 25 {
		return "", nil, fmt.Errorf("Step 8: Invalid length. %v", len(step8))
	}

	return base58.Encode(step8), step8, nil
}

func GenerateKeyPair() (string, string, error) {
	priv, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	// Remove the extra pubkey byte before serializing hex (drop the first 0x04)
	return hex.EncodeToString(crypto.FromECDSA(priv)), hex.EncodeToString(crypto.FromECDSAPub(&priv.PublicKey)), nil
}

func signRawTransaction(rawTransaction []byte, privateKeyBase58 string) []byte {
	//Here we start the process of signing the raw transaction.

	secp256k1.Start()
	privateKeyBytes := base58check.Decode(privateKeyBase58)
	var privateKeyBytes32 [32]byte

	for i := 0; i < 32; i++ {
		privateKeyBytes32[i] = privateKeyBytes[i]
	}

	//Get the raw public key
	publicKeyBytes, success := secp256k1.Pubkey_create(privateKeyBytes32, false)
	if !success {
		log.Fatal("Failed to convert private key to public key")
	}

	//Hash the raw transaction twice before the signing
	shaHash := sha256.New()
	shaHash.Write(rawTransaction)
	var hash []byte = shaHash.Sum(nil)

	shaHash2 := sha256.New()
	shaHash2.Write(hash)
	rawTransactionHashed := shaHash2.Sum(nil)

	//Sign the raw transaction
	signedTransaction, success := secp256k1.Sign(rawTransactionHashed, privateKeyBytes32, generateNonce())
	if !success {
		log.Fatal("Failed to sign transaction")
	}

	//Verify that it worked.
	verified := secp256k1.Verify(rawTransactionHashed, signedTransaction, publicKeyBytes)
	if !verified {
		log.Fatal("Failed to sign transaction")
	}

	secp256k1.Stop()

	hashCodeType, err := hex.DecodeString("01")
	if err != nil {
		log.Fatal(err)
	}

	//+1 for hashCodeType
	signedTransactionLength := byte(len(signedTransaction) + 1)

	var publicKeyBuffer bytes.Buffer
	publicKeyBuffer.Write(publicKeyBytes)
	pubKeyLength := byte(len(publicKeyBuffer.Bytes()))

	var buffer bytes.Buffer
	buffer.WriteByte(signedTransactionLength)
	buffer.Write(signedTransaction)
	buffer.WriteByte(hashCodeType[0])
	buffer.WriteByte(pubKeyLength)
	buffer.Write(publicKeyBuffer.Bytes())

	scriptSig := buffer.Bytes()

	//Return the final transaction
	return createRawTransaction(flagInputTransaction, flagInputIndex, flagDestination, flagSatoshis, scriptSig)
}
func createScriptPubKey(publicKeyBase58 string) []byte {
	publicKeyBytes := base58check.Decode(publicKeyBase58)

	var scriptPubKey bytes.Buffer
	scriptPubKey.WriteByte(byte(118))                 //OP_DUP
	scriptPubKey.WriteByte(byte(169))                 //OP_HASH160
	scriptPubKey.WriteByte(byte(len(publicKeyBytes))) //PUSH
	scriptPubKey.Write(publicKeyBytes)
	scriptPubKey.WriteByte(byte(136)) //OP_EQUALVERIFY
	scriptPubKey.WriteByte(byte(172)) //OP_CHECKSIG
	return scriptPubKey.Bytes()
}

func generateNonce() [32]byte {
	var bytes [32]byte
	for i := 0; i < 32; i++ {
		//This is not "cryptographically random"
		bytes[i] = byte(randInt(0, math.MaxUint8))
	}
	return bytes
}

func randInt(min int, max int) uint8 {
	rand.Seed(time.Now().UTC().UnixNano())
	return uint8(min + rand.Intn(max-min))
}

func createRawTransaction(inputTransactionHash string, inputTransactionIndex int, publicKeyBase58Destination string, satoshis int, scriptSig []byte) []byte {
	//Create the raw transaction.

	//Version field
	version, err := hex.DecodeString("01000000")
	if err != nil {
		log.Fatal(err)
	}

	//# of inputs (always 1 in our case)
	inputs, err := hex.DecodeString("01")
	if err != nil {
		log.Fatal(err)
	}

	//Input transaction hash
	inputTransactionBytes, err := hex.DecodeString(inputTransactionHash)
	if err != nil {
		log.Fatal(err)
	}

	//Convert input transaction hash to little-endian form
	inputTransactionBytesReversed := make([]byte, len(inputTransactionBytes))
	for i := 0; i < len(inputTransactionBytes); i++ {
		inputTransactionBytesReversed[i] = inputTransactionBytes[len(inputTransactionBytes)-i-1]
	}

	//Output index of input transaction
	outputIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(outputIndexBytes, uint32(inputTransactionIndex))

	//Script sig length
	scriptSigLength := len(scriptSig)

	//sequence_no. Normally 0xFFFFFFFF. Always in this case.
	sequence, err := hex.DecodeString("ffffffff")
	if err != nil {
		log.Fatal(err)
	}

	//Numbers of outputs for the transaction being created. Always one in this example.
	numOutputs, err := hex.DecodeString("01")
	if err != nil {
		log.Fatal(err)
	}

	//Satoshis to send.
	satoshiBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(satoshiBytes, uint64(satoshis))

	//Script pub key
	scriptPubKey := createScriptPubKey(publicKeyBase58Destination)
	scriptPubKeyLength := len(scriptPubKey)

	//Lock time field
	lockTimeField, err := hex.DecodeString("00000000")
	if err != nil {
		log.Fatal(err)
	}

	var buffer bytes.Buffer
	buffer.Write(version)
	buffer.Write(inputs)
	buffer.Write(inputTransactionBytesReversed)
	buffer.Write(outputIndexBytes)
	buffer.WriteByte(byte(scriptSigLength))
	buffer.Write(scriptSig)
	buffer.Write(sequence)
	buffer.Write(numOutputs)
	buffer.Write(satoshiBytes)
	buffer.WriteByte(byte(scriptPubKeyLength))
	buffer.Write(scriptPubKey)
	buffer.Write(lockTimeField)

	return buffer.Bytes()
}
