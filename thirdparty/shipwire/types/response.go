package types

import (
	"hanzo.io/util/json"
)

type Response struct {
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
