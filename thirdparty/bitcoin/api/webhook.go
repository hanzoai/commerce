package api

import (
	ej "encoding/json"
	"errors"
	"fmt"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/config"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/blockchains"
	"github.com/hanzoai/commerce/models/blockchains/blockaddress"
	"github.com/hanzoai/commerce/models/blockchains/blocktransaction"
	"github.com/hanzoai/commerce/thirdparty/bitcoin/tasks"
	"github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"

	. "github.com/hanzoai/commerce/models/blockchains"
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
func decodeEvent(c *zip.Ctx) (*Event, error) {
	event := new(Event)
	if err := json.DecodeBytes(c.Body(), event); err != nil {
		log.Error("Could not Decode:\n%s", c.Body(), c)
		return nil, fmt.Errorf("Failed to parse webhook: %v", err)
	}

	log.JSON("Received '%s'", event.Type, event)
	return event, nil
}

var AccessDeniedError = errors.New("Access Denied")
var BlockTransactionNotFound = errors.New("BlockTransaction not found, it should exist for this webhook to be received")
var CouldNotConvertToBigInt = errors.New("BlockTransaction Value could not be converted")
var ReceiverBlockShouldBeVOutBlock = errors.New("BlockTransaction marked as 'receiver' Usage should also be of 'vin' BitcoinTransactionType")

// Handle Bitcoin webhook POSTs
func Webhook(c *zip.Ctx) error {
	event, err := decodeEvent(c)
	if err != nil {
		return http.Fail(c, 500, err.Error(), err)
	}

	if event.Password != config.Bitcoin.WebhookPassword {
		return http.Fail(c, 401, AccessDeniedError.Error(), AccessDeniedError)
	}

	db := datastore.New(c.Context())
	// ctx := db.Context

	switch event.DataKind {
	case BlockTransaction:
		switch event.Name {
		case "blocktransaction.confirmed":
			// Confirm a block transaction
			bt := blocktransaction.New(db)

			// Decode event data
			if err := json.Unmarshal([]byte(event.Data), bt); err != nil {
				return http.Fail(c, 500, err.Error(), err)
			}

			// We only care about payments we receive for orders
			if bt.Usage != ReceiverUsage {
				break
			}

			// Receivers should all be VIns
			if bt.BitcoinTransactionType != blockchains.BitcoinTransactionTypeVOut {
				return http.Fail(c, 500, ReceiverBlockShouldBeVOutBlock.Error(), ReceiverBlockShouldBeVOutBlock)
			}

			// Get block address
			ba := blockaddress.New(db)
			if ok, err := ba.Query().Filter("Type=", bt.Type).Filter("Address=", bt.Address).Get(); !ok {
				if err != nil {
					return http.Fail(c, 500, err.Error(), err)
				}

				return http.Fail(c, 500, BlockTransactionNotFound.Error(), BlockTransactionNotFound)
			}

			// Ignore updates about platform wallets
			// May start listening for deposits in the future
			if ba.WalletNamespace == "" {
				break
			}

			if err := tasks.BitcoinProcessPayment.Call(
				db.Context,
				ba.WalletNamespace,
				ba.WalletId,
				bt.BitcoinTransactionTxId,
				string(bt.Type),
				bt.BitcoinTransactionVOutValue,
			); err != nil {
				return http.Fail(c, 500, err.Error(), err)
			}

		case "ping":
			return c.String(200, "pong")
		}
	}

	log.Info("Received Bitcoin Webhook: %v", event, c)
	return c.String(200, "ok")
}
