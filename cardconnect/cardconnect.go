package cardconnect

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type AuthorizationRequest struct {
	MerchId  string
	AcctType string
	OrderId  string
	Account  string
	Expiry   string
	Amount   string
	Currency string
	Name     string
	Address  string
	City     string
	Region   string
	Country  string
	Postal   string
	Ecomind  string
	CVV2     string // confirmation code
	Track    string
	Tokenize string
}

type AuthorizationResponse struct {
	RespStat string
	Account  string
	Token    string
	RetRef   string
	Amount   string
	MerchId  string
	RespCode string
	RespText string

	// If successful
	AVSResp  string
	CVVResp  string
	AuthCode string
	CommCard string
}

var base_url = "fts.prinpay.com:6443/cardconnect/rest" //496160873888-CardConnect - USD - NORTH
var authCode = base64.StdEncoding.EncodeToString([]byte("testing:testing123"))

func Authorize(areq AuthorizationRequest) (aresp AuthorizationResponse, err error) {
	var err error
	var aresp AuthorizationResponse

	if reqJson, err := json.Marshal(areq); err == nil {
		reqJson := string(reqJson)
		client := &http.Client{}

		if req, err := http.NewRequest("POST", base_url, nil); err == nil {
			req.Header.Add("Authorization: Basic " + authCode)
			resp, err := client.Do(req)
			defer resp.Body.Close()

			if err == nil {
				if body, err := ioutil.ReadAll(resp.Body); err == nil {
					if err := json.Unmarshal(body, &aresp); err == nil {
						return aresp, nil
					}
				}
			}
		}
	}
}
