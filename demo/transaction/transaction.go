package transaction

import (
	"hanzo.io/models/mixin"
	"time"
)

// Datastructure for Bitcoin Transaction
type Transaction struct {
	mixin.Model

	Timestamp             time.Time `json:"timestamp"`
	SendingAddress        string    `json:"sendingAddress"`
	ReceivingAddress      string    `json:"receivingAddress"`
	SendingName           string    `json:"sendingName"`
	ReceivingName         string    `json:"receivingName"`
	SenderFlagged         bool      `json:"senderFlagged"`
	ReceiverFlagged       bool      `json:"receiverFlagged"`
	JuristictionSending   string    `json:"juristictionSending"`
	JuristictionReceiving string    `json:"juristictionReceiving"`
	Protocol              string    `json:"protocol"`
	TransactionHash       string    `json:"transactionHash"`
}
