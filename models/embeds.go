package models

type SalesforceSObject struct {
	Id string
}

func (so *SalesforceSObject) SetSalesforceId(id string) {
	so.Id = id
}

func (so *SalesforceSObject) SalesforceId() string {
	return so.Id
}
