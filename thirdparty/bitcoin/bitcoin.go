package bitcoin

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/ripemd160"
	"math"
	mathRand "math/rand"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"hanzo.io/thirdparty/ethereum/go-ethereum/crypto"
	"hanzo.io/util/log"
)

var flagPrivateKey string = "private-key"
var flagPublicKey string = "public-key"
var flagDestination string = "destination"
var flagInputTransaction string = "input-transaction"
var flagInputIndex int = 0
var flagSatoshis int = 0

// The steps notated in the variable names here relate to the steps outlined in
// https://en.bitcoin.it/wiki/Technical_background_of_version_1_Bitcoin_addresses
func PubKeyToAddress(pubKey string) (string, []byte, error) {
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

func PubKeyToTestNetAddress(pubKey string) (string, []byte, error) {
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

	testNetPrefix, _ := hex.DecodeString("6F")
	step4 := append(testNetPrefix, step3...)

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

func GetRawTransactionSignature(rawTransaction []byte, privateKeyBase58 string) ([]byte, error) {
	//Here we start the process of signing the raw transaction.

	log.Debug("Private key base 58, prior to decode: %v", privateKeyBase58)
	privateKeyBytes := base58.Decode(privateKeyBase58)
	log.Debug("Private key bytes decoded. %v", len(privateKeyBytes))
	privateKey, err := crypto.ToECDSA(privateKeyBytes)
	if err != nil {
		return nil, err
	}
	log.Info("Private key decoding successful.")
	publicKey := privateKey.PublicKey
	log.Info("Public key derived: %v", publicKey)
	publicKeyBytes := crypto.FromECDSAPub(&publicKey)
	var privateKeyBytes32 [32]byte

	for i := 0; i < 32; i++ {
		privateKeyBytes32[i] = privateKeyBytes[i]
	}

	//Hash the raw transaction twice before the signing
	shaHash := sha256.New()
	shaHash.Write(rawTransaction)
	var hash []byte = shaHash.Sum(nil)

	shaHash2 := sha256.New()
	shaHash2.Write(hash)
	rawTransactionHashed := shaHash2.Sum(nil)

	//Sign the raw transaction
	signedTransaction, success := crypto.Sign(rawTransactionHashed, privateKey)
	if success != nil {
		log.Fatal("Failed to sign transaction")
	}
	// TODO: Should verify this when we get a second to work around a solution
	// that maintains R and S.

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

	//Return the final transaction signature
	return scriptSig, nil
}
func CreateScriptPubKey(publicKeyBase58 string) []byte {
	publicKeyBytes := base58.Decode(publicKeyBase58)

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
	mathRand.Seed(time.Now().UTC().UnixNano())
	return uint8(min + mathRand.Intn(max-min))
}

/* NOTE: This function presumes you're doing a pay to public key hash
* transaction and using a single script to authenticate the entire thing. More
* complex stuff will come later. */
func CreateRawTransaction(inputTransactionHashes []string, inputTransactionIndeces []int, publicKeyBase58Destinations []string, satoshisToOutput []int, scriptSig []byte) []byte {
	//Create the raw transaction.

	//Version field
	version, err := hex.DecodeString("01000000")
	if err != nil {
		log.Fatal(err)
	}

	in := ""
	//# of inputs (always 1 in our case)
	if len(inputTransactionHashes) < 15 {
		in = "0" + fmt.Sprintf("%x", len(inputTransactionHashes))
	} else {
		in = fmt.Sprintf("%x", len(inputTransactionHashes))
	}
	inputs, err := hex.DecodeString(in)
	if err != nil {
		log.Error("String representation of length: %v", string(len(inputTransactionHashes)))
		log.Fatal("Could not decode hash %s, %v", in, err)
	}

	//Input transaction hash

	inputTransactionLittleEndian := make([][]byte, len(inputTransactionHashes))
	for index, inputTransactionHash := range inputTransactionHashes {
		inputTransactionBytes, err := hex.DecodeString(inputTransactionHash)
		if err != nil {
			log.Fatal(err)
		}

		//Convert input transaction hash to little-endian form
		inputTransactionBytesReversed := make([]byte, len(inputTransactionBytes))
		for i := 0; i < len(inputTransactionBytes); i++ {
			inputTransactionBytesReversed[i] = inputTransactionBytes[len(inputTransactionBytes)-i-1]
		}
		inputTransactionLittleEndian[index] = inputTransactionBytesReversed
	}

	//Output index of input transaction
	outputIndeces := make([][]byte, len(inputTransactionIndeces))
	for index, inputTransactionIndex := range inputTransactionIndeces {
		outputIndexBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(outputIndexBytes, uint32(inputTransactionIndex))
		outputIndeces[index] = outputIndexBytes
	}

	//Script sig length
	scriptSigLength := len(scriptSig)

	//sequence_no. Normally 0xFFFFFFFF. Always in this case.
	sequence, err := hex.DecodeString("ffffffff")
	if err != nil {
		log.Fatal(err)
	}

	//Numbers of outputs for the transaction being created. Always one in this example.
	out := ""
	if len(publicKeyBase58Destinations) < 15 {
		out = "0" + fmt.Sprintf("%x", len(publicKeyBase58Destinations))
	} else {
		out = fmt.Sprintf("%x", len(publicKeyBase58Destinations))
	}

	numOutputs, err := hex.DecodeString(out)
	if err != nil {
		log.Fatal(err)
	}
	//Satoshis to send.

	satoshisToOutputBytes := make([][]byte, len(satoshisToOutput))
	for _, satoshis := range satoshisToOutput {
		satoshiBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(satoshiBytes, uint64(satoshis))
	}

	//Script pub key
	scripts := make([][]byte, len(publicKeyBase58Destinations))
	for index, publicKeyBase58 := range publicKeyBase58Destinations {
		scriptPubKey := CreateScriptPubKey(publicKeyBase58)
		scripts[index] = scriptPubKey
	}

	//Lock time field
	lockTimeField, err := hex.DecodeString("00000000")
	if err != nil {
		log.Fatal(err)
	}

	var buffer bytes.Buffer
	buffer.Write(version)
	buffer.Write(inputs)
	for index, bytes := range inputTransactionLittleEndian {
		buffer.Write(bytes)
		buffer.Write(outputIndeces[index])
		buffer.WriteByte(byte(scriptSigLength))
		buffer.Write(scriptSig)
		buffer.Write(sequence)
	}
	buffer.Write(numOutputs)
	for index, script := range scripts {
		buffer.Write(satoshisToOutputBytes[index])
		buffer.WriteByte(byte(len(script)))
		buffer.Write(script)
	}
	buffer.Write(lockTimeField)

	return buffer.Bytes()
}
