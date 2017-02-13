package types

import (
	"hanzo.io/util/json"
)

type Decoder interface {
	Decode(json.RawMessage) error
}

type Response struct {
	Error            string `json:"error,omitempty"`
	Status           int    `json:"status"`
	Message          string `json:"message"`
	ResourceLocation string `json:"resourceLocation"`
	Resource         struct {
		Items []struct {
			ResourceLocation string          `json:"resourceLocation"`
			Resource         json.RawMessage `json:"resource"`
		} `json:"items"`
	} `json:"resource"`
}
