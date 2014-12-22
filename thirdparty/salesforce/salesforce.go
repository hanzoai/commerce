package salesforce

import (
	"github.com/davidtai/go-force/force"
)

type Api struct {
	api *force.ForceApi
}

func Init(accessToken, instanceUrl, id, issuedAt, signature string) (*Api, error) {
	api, err := force.Set(accessToken, instanceUrl, id, issuedAt, signature)
	return &Api{api: api}, err
}

func (a *Api) GetSObject(id string, out force.SObject) error {
	return a.api.GetSObject(id, out)
}
