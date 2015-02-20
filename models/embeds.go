package models

type SalesforceSObject struct {
	SalesforceId_ string
}

func (so *SalesforceSObject) SetSalesforceId(id string) {
	so.SalesforceId_ = id
}

func (so *SalesforceSObject) SalesforceId() string {
	return so.SalesforceId_
}
