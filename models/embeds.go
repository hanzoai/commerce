package models

import "time"

type SalesforceSObject struct {
	PrimarySalesforceId_   string
	SecondarySalesforceId_ string
	LastSync_              time.Time
}

func (so *SalesforceSObject) SetSalesforceId(id string) {
	so.PrimarySalesforceId_ = id
}

func (so *SalesforceSObject) SalesforceId() string {
	return so.PrimarySalesforceId_
}

func (so *SalesforceSObject) SetSalesforceId2(id string) {
	so.SecondarySalesforceId_ = id
}

func (so *SalesforceSObject) SalesforceId2() string {
	return so.SecondarySalesforceId_
}

func (so *SalesforceSObject) SetLastSync() {
	so.LastSync_ = time.Now()
}

func (so *SalesforceSObject) LastSync() time.Time {
	return so.LastSync_
}
