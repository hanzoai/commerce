package mixin

import "time"

type Salesforce struct {
	PrimarySalesforceId_   string
	SecondarySalesforceId_ string
	LastSync_              time.Time
}

func (so *Salesforce) SetSalesforceId(id string) {
	so.PrimarySalesforceId_ = id
}

func (so *Salesforce) SalesforceId() string {
	return so.PrimarySalesforceId_
}

func (so *Salesforce) SetSalesforceId2(id string) {
	so.SecondarySalesforceId_ = id
}

func (so *Salesforce) SalesforceId2() string {
	return so.SecondarySalesforceId_
}

func (so *Salesforce) SetLastSync() {
	// Add 1 more minute to the Last Sync date due to sf resolution being nearest minute
	so.LastSync_ = time.Now().Add(1 * time.Minute)
}

func (so *Salesforce) LastSync() time.Time {
	return so.LastSync_
}
