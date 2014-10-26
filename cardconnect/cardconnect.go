package cardconnect

import (
	"encoding/json"
	"net/http"
)

type AuthorizationRequest struct {
	MerchId string
	AcctType string
	OrderId string
	Account string
	Expiry string
	Amount string
	Currency string
	Name string
	Address string
	City string
	Region string
	Country string
	Postal string
	Ecomind string
	CVV2 string // confirmation code
	Track string
	Tokenize string
}

type AuthorizationResponse struct {
	RespStat string
	Account string
	Token string
	RetRef string
	Amount string
	MerchId string
	RespCode string
	RespText string

	// If successful
	AVSResp string
	CVVResp string
	AuthCode string
	CommCard string
}

var url = "fts.prinpay.com:6443/cardconnect/rest" //496160873888-CardConnect - USD - NORTH
var authCode = ""

func Authorize(areq AuthorizationRequest) (AuthorizationResponse, error) {
	if reqJson, err := json.Marshal(areq); err == nil {
		reqJson := string(reqJson)
		client := &http.Client{}
		
		if req, err := http.NewRequest("POST", url, nil); err == nil {
			req.Header.Add("Authorization: Basic " + authCode)
			if resp, err := client.Do(req); err == nil {
				
			}
		}
	}
}
