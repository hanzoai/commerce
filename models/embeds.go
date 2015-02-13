package models

type SalesforceSObject struct {
	_SalesforceId string
}

func (so *SalesforceSObject) SetSalesforceId(id string) {
	so._SalesforceId = id
}

func (so *SalesforceSObject) SalesforceId() string {
	return so._SalesforceId
}
