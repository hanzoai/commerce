package salesforce

import (
	"github.com/davidtai/go-force/force"
)

func Init(accessToken, instanceUrl, id, issuedAt, signature string) (*force.ForceApi, error) {
	api, err := force.Set(accessToken, instanceUrl, id, issuedAt, signature)
	return api, err
}
