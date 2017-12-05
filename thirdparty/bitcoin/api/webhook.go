package api

import (
	ej "encoding/json"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	"hanzo.io/config"
	"hanzo.io/datastore"
	"hanzo.io/models/blockchains/blockaddress"
	"hanzo.io/models/blockchains/blocktransaction"
	// "hanzo.io/models/wallet"
	//"hanzo.io/thirdparty/ethereum/tasks"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/util/log"

	. "hanzo.io/models/blockchains"
)

type Kind string

const (
	BlockKind        Kind = "block"
	BlockAddress     Kind = "blockaddress"
	BlockTransaction Kind = "blocktransaction"
)

type Event struct {
	Name     string `json:"name"`
	Type     Type   `json:"type"`
	Password string `json:"password"`

	DataId   string        `json:"dataId"`
	DataKind Kind          `json:"dataKind"`
	Data     ej.RawMessage `json:"data"`
}

// Decode Bitcoin payload
func decodeEvent(c *gin.Context) (*Event, error) {
	event := new(Event)
	if err := json.Decode(c.Request.Body, event); err != nil {
		log.Error("Could not Decode:\n%s", c.Request.Body, c)
		return nil, fmt.Errorf("Failed to parse Stripe webhook: %v", err)
	}

	log.JSON("Received '%s'", event.Type, event)
	return event, nil
}

var AccessDeniedError = errors.New("Access Denied")
var BlockTransactionNotFound = errors.New("BlockTransaction not found, it should exist for this webhook to be received")
var CouldNotConvertToBigInt = errors.New("BlockTransaction Value could not be converted")

// Handle stripe webhook POSTs
func Webhook(c *gin.Context) {
	event, err := decodeEvent(c)
	if err != nil {
		http.Fail(c, 500, err.Error(), err)
		return
	}

	if event.Password != config.Bitcoin.WebhookPassword {
		http.Fail(c, 401, AccessDeniedError.Error(), AccessDeniedError)
		return
	}

	db := datastore.New(c)
	// ctx := db.Context

	switch event.DataKind {
	case BlockTransaction:
		switch event.Name {
		case "blocktransaction.confirmed":
			// Confirm a block transaction
			bt := blocktransaction.New(db)

			// Decode event data
			if err := json.Unmarshal([]byte(event.Data), bt); err != nil {
				http.Fail(c, 500, err.Error(), err)
				panic(err)
			}

			// We only care about payments we receive for orders
			if bt.Usage != ReceiverUsage {
				break
			}

			// Get block address
			ba := blockaddress.New(db)
			if ok, err := ba.Query().Filter("Type=", bt.Type).Filter("Address=", bt.Address).Get(); !ok {
				if err != nil {
					http.Fail(c, 500, err.Error(), err)
					panic(err)
				}

				http.Fail(c, 500, BlockTransactionNotFound.Error(), BlockTransactionNotFound)
				panic(err)
			}

			// Ignore updates about platform wallets
			// May start listening for deposits in the future
			if ba.WalletNamespace == "" {
				break
			}

			// bi, ok := big.NewInt(0).SetString(string(bt.EthereumTransactionValue), 10)
			// if !ok {
			// 	http.Fail(c, 500, CouldNotConvertToBigInt.Error(), CouldNotConvertToBigInt)
			// 	panic(CouldNotConvertToBigInt)
			// }

			// if err := tasks.EthereumProcessPayment.Call(
			// 	ctx, ba.WalletNamespace,
			// 	ba.WalletId,
			// 	bt.EthereumTransactionHash,
			// 	string(bt.Type),
			// 	bi,
			// ); err != nil {
			// 	http.Fail(c, 500, err.Error(), err)
			// 	panic(err)
			// }

		case "ping":
			c.String(200, "pong")
			return
		}
	}

	log.Info("Received Bitcoin Webhook: %v", event, c)
	c.String(200, "ok")
}
