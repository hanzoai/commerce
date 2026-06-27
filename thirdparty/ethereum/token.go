package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/luxfi/crypto"
	"github.com/luxfi/geth/common"
	"github.com/luxfi/geth/common/hexutil"
	"github.com/luxfi/geth/core/types"
	"github.com/luxfi/geth/rlp"

	"github.com/hanzoai/commerce/log"
	blockchainutil "github.com/hanzoai/commerce/util/blockchain"
)

// init wires the EVM ERC-20 token-transfer implementation into the parent
// commerce module (mirrors the native-payment registration in register.go).
// Only this sub-module links luxfi/geth; the parent calls through the
// blockchainutil.TransferToken seam.
func init() {
	blockchainutil.RegisterTokenTransfer(transferToken)
}

// erc20TransferSelector is the 4-byte selector for the ERC-20 method
// transfer(address,uint256) — keccak256("transfer(address,uint256)")[:4].
var erc20TransferSelector = []byte{0xa9, 0x05, 0x9c, 0xbb}

// DefaultTokenTransferGas is a safe gas limit for a standard ERC-20 transfer.
const DefaultTokenTransferGas uint64 = 100_000

// EncodeERC20Transfer builds the calldata for transfer(to, amount):
// selector ++ left-padded(to, 32) ++ left-padded(amount, 32).
func EncodeERC20Transfer(to common.Address, amount *big.Int) []byte {
	data := make([]byte, 0, 4+32+32)
	data = append(data, erc20TransferSelector...)
	data = append(data, common.LeftPadBytes(to.Bytes(), 32)...)
	data = append(data, common.LeftPadBytes(amount.Bytes(), 32)...)
	return data
}

// transferToken signs an ERC-20 transfer with the treasury key and submits it
// to the configured EVM chain, returning the transaction hash. Works for any
// chain id (e.g. Hanzo 36900) — it does not gate on the legacy chain switch.
//
// The treasury private key is decoded in-memory and never logged.
func transferToken(ctx context.Context, t blockchainutil.TokenTransfer) (string, error) {
	if t.AmountWei == nil || t.AmountWei.Sign() <= 0 {
		return "", fmt.Errorf("ethereum: transfer amount must be positive")
	}

	privKey, err := crypto.HexToECDSA(strings.TrimPrefix(t.TreasuryKey, "0x"))
	if err != nil {
		return "", fmt.Errorf("ethereum: decode treasury key: %w", err)
	}
	from := crypto.PubkeyToAddress(privKey.PublicKey)

	client := New(ctx, t.RPCURL)
	client.Chain = ChainId(t.ChainID)

	nonce, err := client.NonceAt(from.Hex(), "pending")
	if err != nil {
		return "", fmt.Errorf("ethereum: get nonce: %w", err)
	}

	gasPrice, err := client.GasPrice()
	if err != nil || gasPrice == nil || gasPrice.Sign() == 0 {
		gasPrice = big.NewInt(DefaultGasPrice)
	}

	gasLimit := t.GasLimit
	if gasLimit == 0 {
		gasLimit = DefaultTokenTransferGas
	}

	data := EncodeERC20Transfer(common.HexToAddress(t.To), t.AmountWei)

	// ERC-20 transfer: value is 0, recipient is the token contract, the real
	// transfer is encoded in calldata.
	tx := types.NewTransaction(nonce, common.HexToAddress(t.TokenAddress), big.NewInt(0), gasLimit, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(t.ChainID)), privKey)
	if err != nil {
		return "", fmt.Errorf("ethereum: sign tx: %w", err)
	}

	raw, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return "", fmt.Errorf("ethereum: rlp encode: %w", err)
	}

	txHash, err := client.SendRawTransaction(hexutil.Encode(raw))
	if err != nil {
		return "", fmt.Errorf("ethereum: send raw tx: %w", err)
	}

	// Log the receipt — never the key.
	log.Info("ethereum: ERC-20 transfer chain=%d token=%s from=%s to=%s amountWei=%s tx=%s",
		t.ChainID, t.TokenAddress, from.Hex(), t.To, t.AmountWei.String(), txHash)
	return txHash, nil
}
