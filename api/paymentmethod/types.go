package paymentmethod

import "encoding/json"

type CreateReq struct {
	PublicToken string          `json:"public_token"`
	AccountId   string          `json:"accountId"`
	Metadata    json.RawMessage `json:"metadata"`
}
