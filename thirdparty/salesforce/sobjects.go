package salesforce

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"appengine/datastore"

	"crowdstart.io/models"
	"crowdstart.io/util/log"
)

var ErrorUserTypeRequired = errors.New("Parameter needs to be of type User")
var ErrorOrderTypeRequired = errors.New("Parameter needs to be of type Order")
var ErrorShouldNotCall = errors.New("Function should not be called")

type Currency string

func ToCurrency(centicents int64) Currency {
	return Currency(fmt.Sprintf("%.2f", float64(centicents)/10000.0))
}

// For crowdstart models/mixins to be salesforce compatible in future
type SObjectCompatible interface {
	SetSalesforceId(string)
	SalesforceId() string
	SetSalesforceId2(string)
	SalesforceId2() string
	SetLastSync()
	LastSync() time.Time
}

type SObjectSyncable interface {
	Push(SalesforceClient) error
	PullId(SalesforceClient, string) error
	PullExternalId(SalesforceClient, string) error
}

type SObjectSerializeable interface {
	SetExternalId(string)
	ExternalId() string
	// Should be SObjectCompatible in the future instead of models.User
	Read(SObjectCompatible) error
	Write(SObjectCompatible) error

	// SObjectCompatible proxies
	SetSalesforceId(string)
	SalesforceId() string
	SetLastSync()
	LastSync() time.Time
}

// Reference to the struct/datastore model for an SObject
type ModelReference struct {
	Ref SObjectCompatible
}

func (s *ModelReference) SetSalesforceId(id string) {
	if s.Ref != nil {
		s.Ref.SetSalesforceId(id)
	}
}

func (s *ModelReference) SalesforceId() string {
	if s.Ref != nil {
		return s.Ref.SalesforceId()
	}
	return ""
}

func (s *ModelReference) SetLastSync() {
	if s.Ref != nil {
		s.Ref.SetLastSync()
	}
}

func (s *ModelReference) LastSync() time.Time {
	if s.Ref != nil {
		return s.Ref.LastSync()
	}

	return time.Now()
}

// Also a reference like above but some of these structs/models refer to multiple sobjects
type ModelSecondaryReference struct {
	Ref SObjectCompatible
}

func (s *ModelSecondaryReference) SetSalesforceId(id string) {
	if s.Ref != nil {
		s.Ref.SetSalesforceId2(id)
	}
}

func (s *ModelSecondaryReference) SalesforceId() string {
	if s.Ref != nil {
		return s.Ref.SalesforceId()
	}
	return ""
}

func (s *ModelSecondaryReference) SetLastSync() {
	if s.Ref != nil {
		s.Ref.SetLastSync()
	}
}

func (s *ModelSecondaryReference) LastSync() time.Time {
	if s.Ref != nil {
		return s.Ref.LastSync()
	}

	return time.Now()
}

//SObject Definitions
type Contact struct {
	ModelSecondaryReference `json:"-"` // Struct this sobject refers to

	// Don't manually specify these

	// Response Only Fields
	Attributes     Attribute `json:"attributes,omitempty"`
	Id             string    `json:"Id,omitempty"`
	IsDeleted      bool      `json:"IsDeleted,omitempty"`
	MasterRecordId string    `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using Id (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__C,omitempty"`

	// Read Only
	Name             string `json:"Name,omitempty"`
	AccountId        string `json:"AccountId,omitempty"`
	CreatedById      string `json:"CreatedById,omitempty"`
	LastModifiedById string `json:"LastModifiedById,omitempty"`

	// You can manually specify these

	// Data Fields
	LastName           string  `json:"LastName,omitempty"`
	FirstName          string  `json:"FirstName,omitempty"`
	Salutation         string  `json:"Salutation,omitempty"`
	MailingStreet      string  `json:"MailingStreet,omitempty"`
	MailingCity        string  `json:"MailingCity,omitempty"`
	MailingState       string  `json:"MailingState,omitempty"`
	MailingPostalCode  string  `json:"MailingPostalCode,omitempty"`
	MailingCountry     string  `json:"MailingCountry,omitempty"`
	MailingStateCode   string  `json:"MailingStateCode,omitempty"`
	MailingCountryCode string  `json:"MailingCountryCode,omitempty"`
	MailingLatitude    string  `json:"MailingLatitude,omitempty"`
	MailingLongitude   string  `json:"MailingLongitude,omitempty"`
	Phone              string  `json:"Phone,omitempty"`
	Fax                string  `json:"Fax,omitempty"`
	MobilePhone        string  `json:"MobilePhone,omitempty"`
	ReportsToId        string  `json:"ReportsToId,omitempty"`
	Email              string  `json:"Email,omitempty"`
	Title              string  `json:"Title,omitempty"`
	Department         string  `json:"Department,omitempty"`
	OwnerId            string  `json:"OwnerId,omitempty"`
	CreatedDate        string  `json:"CreatedDate,omitempty"`
	LastModifiedDate   string  `json:"LastModifiedDate,omitempty"`
	SystemModstamp     string  `json:"SystemModstamp,omitempty"`
	LastActivityDate   string  `json:"LastActivityDate,omitempty"`
	LastCURequestDate  string  `json:"LastCURequestDate,omitempty"`
	LastCUUpdateDate   string  `json:"LastCUUpdateDate,omitempty"`
	LastViewedDate     string  `json:"LastViewedDate,omitempty"`
	LastReferencedDate string  `json:"LastReferencedDate,omitempty"`
	EmailBouncedReason string  `json:"EmailBouncedReason,omitempty"`
	EmailBouncedDate   string  `json:"EmailBouncedDate,omitempty"`
	IsEmailBounced     bool    `json:"IsEmailBounced,omitempty"`
	JigsawContactId    string  `json:"JigsawContactId,omitempty"`
	Account            Account `json:"Account,omitempty"`

	// Skully Custom fields
	UniquePreorderLinkC string `json:"Unique_Preorder_Link__C,omitempty"`
	FullfillmentStatusC string `json:"Fulfillment_Status__C,omitempty"`
	PreorderC           string `json:"Preorder__C,omitempty"`
	ShippingAddressC    string `json:"Shipping_Address__C,omitempty"`
	ShippingCityC       string `json:"Shipping_City__C,omitempty"`
	ShippingStateC      string `json:"Shipping_State__C,omitempty"`
	ShippingPostalZipC  string `json:"Shipping_Postal_Zip__C,omitempty"`
	ShippingCountryC    string `json:"Shipping_Country__C,omitempty"`
	MC4SFMCSubscriberC  string `json:"MC4SF__MC_Subscriber__C,omitempty"`

	// Zendesk Custom fields
	ZendeskLastSyncDateC         string `json:"Zendesk__Last_Sync_Date__C,omitempty"`
	ZendeskLastSyncStatusC       string `json:"Zendesk__Last_Sync_Status__C,omitempty"`
	ZendeskResultC               string `json:"Zendesk__Result__C,omitempty"`
	ZendeskTagsC                 string `json:"Zendesk__Tags__C,omitempty"`
	ZendeskZendeskOutofSyncC     string `json:"Zendesk__Zendesk_OutofSync__C,omitempty"`
	ZendeskZendeskOldTagsC       string `json:"Zendesk__Zendesk_oldTags__C,omitempty"`
	ZendeskIsCreatedUpdatedFlagC string `json:"Zendesk__isCreatedUpdatedFlag__C,omitempty"`
	ZendeskNotesC                string `json:"Zendesk__notes__C,omitempty"`
	ZendeskZendeskIdC            string `json:"Zendesk__zendesk_id__C,omitempty"`
}

func (c *Contact) Read(so SObjectCompatible) error {
	c.Ref = so

	u, ok := so.(*models.User)
	if !ok {
		return ErrorUserTypeRequired
	}

	c.CrowdstartIdC = u.Id
	c.LastName = u.LastName
	if c.LastName == "" {
		c.LastName = "-"
	}

	c.FirstName = u.FirstName
	if c.FirstName == "" {
		c.FirstName = "-"
	}

	c.Email = u.Email
	c.Phone = u.Phone

	c.Account = Account{CrowdstartIdC: u.Id}

	return nil
}

func (c *Contact) Write(so SObjectCompatible) error {
	c.Ref = so

	u, ok := so.(*models.User)
	if !ok {
		return ErrorUserTypeRequired
	}

	u.Id = c.CrowdstartIdC
	u.Email = c.Email

	u.LastName = c.LastName
	if u.LastName == "-" {
		u.LastName = ""
	}

	u.FirstName = c.FirstName
	if u.FirstName == "-" {
		u.FirstName = ""
	}

	u.Phone = c.Phone
	return nil
}

func (c *Contact) SetExternalId(id string) {
	c.CrowdstartIdC = id
}

func (c *Contact) ExternalId() string {
	return c.CrowdstartIdC
}

func (c *Contact) Push(api SalesforceClient) error {
	return push(api, ContactExternalIdPath, c)
}
func (c *Contact) PullExternalId(api SalesforceClient, id string) error {
	return pull(api, ContactExternalIdPath, id, c)
}

func (c *Contact) PullId(api SalesforceClient, id string) error {
	return pull(api, ContactPath, id, c)
}

type Account struct {
	ModelReference `json:"-"` // Struct this sobject refers to

	// Don't manually specify these

	// Response Only Fields
	Attributes     Attribute `json:"attributes,omitempty"`
	Id             string    `json:"Id,omitempty"`
	IsDeleted      bool      `json:"IsDeleted,omitempty"`
	MasterRecordId string    `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using Id (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__C,omitempty"`

	// Read Only
	CreatedById      string `json:"CreatedById,omitempty"`
	LastModifiedById string `json:"LastModifiedById,omitempty"`

	// You can manually specify these
	// Data Fields
	Name               string `json:"Name,omitempty"`
	Type               string `json:"Type,omitempty"`
	ParentId           string `json:"ParentId,omitempty"`
	BillingStreet      string `json:"BillingStreet,omitempty"`
	BillingCity        string `json:"BillingCity,omitempty"`
	BillingState       string `json:"BillingState,omitempty"`
	BillingPostalCode  string `json:"BillingPostalCode,omitempty"`
	BillingCountry     string `json:"BillingCountry,omitempty"`
	BillingLatitude    string `json:"BillingLatitude,omitempty"`
	BillingLongitude   string `json:"BillingLongitude,omitempty"`
	ShippingStreet     string `json:"ShippingStreet,omitempty"`
	ShippingCity       string `json:"ShippingCity,omitempty"`
	ShippingState      string `json:"ShippingState,omitempty"`
	ShippingPostalCode string `json:"ShippingPostalCode,omitempty"`
	ShippingCountry    string `json:"ShippingCountry,omitempty"`
	ShippingLatitude   string `json:"ShippingLatitude,omitempty"`
	ShippingLongitude  string `json:"ShippingLongitude,omitempty"`
	Phone              string `json:"Phone,omitempty"`
	Fax                string `json:"Fax,omitempty"`
	AccountNumber      string `json:"AccountNumber,omitempty"`
	Website            string `json:"Website,omitempty"`
	Sic                string `json:"Sic,omitempty"`
	Industry           string `json:"Industry,omitempty"`
	AnnualRevenue      string `json:"AnnualRevenue,omitempty"`
	NumberOfEmployees  string `json:"NumberOfEmployees,omitempty"`
	Ownership          string `json:"Ownership,omitempty"`
	TickerSymbol       string `json:"TickerSymbol,omitempty"`
	Description        string `json:"Description,omitempty"`
	Rating             string `json:"Rating,omitempty"`
	Site               string `json:"Site,omitempty"`
	OwnerId            string `json:"OwnerId,omitempty"`
	CreatedDate        string `json:"CreatedDate,omitempty"`
	LastModifiedDate   string `json:"LastModifiedDate,omitempty"`
	SystemModstamp     string `json:"SystemModstamp,omitempty"`
	LastActivityDate   string `json:"LastActivityDate,omitempty"`
	LastViewedDate     string `json:"LastViewedDate,omitempty"`
	LastReferencedDate string `json:"LastReferencedDate,omitempty"`
	Jigsaw             string `json:"Jigsaw,omitempty"`
	JigsawCompanyId    string `json:"JigsawCompanyId,omitempty"`
	CleanStatus        string `json:"CleanStatus,omitempty"`
	AccountSource      string `json:"AccountSource,omitempty"`
	DunsNumber         string `json:"DunsNumber,omitempty"`
	Tradestyle         string `json:"Tradestyle,omitempty"`
	NaicsCode          string `json:"NaicsCode,omitempty"`
	NaicsDesc          string `json:"NaicsDesc,omitempty"`
	YearStarted        string `json:"YearStarted,omitempty"`
	SicDesc            string `json:"SicDesc,omitempty"`
	DandbCompanyId     string `json:"DandbCompanyId,omitempty"`
	CustomerPriorityC  string `json:"CustomerPriority__C,omitempty"`
	SlaC               string `json:"SLA__C,omitempty"`
	ActiveC            string `json:"Active__C,omitempty"`
	NumberofLocationsC string `json:"NumberofLocations__C,omitempty"`
	UpsellOpportunityC string `json:"UpsellOpportunity__C,omitempty"`
	SLASerialNumberC   string `json:"SLASerialNumber__C,omitempty"`
	SLAExpirationDateC string `json:"SLAExpirationDate__C,omitempty"`
	Account            string `json:"Account,omitempty"`
	Master             string `json:"Master,omitempty"`

	// Zendesk integration items
	ZendeskCreatedUpdatedFlagC    string `json:"Zendesk__CreatedUpdatedFlag__C,omitempty"`
	ZendeskDomainMappingC         string `json:"Zendesk__Domain_Mapping__C,omitempty"`
	ZendeskLastSyncDataC          string `json:"Zendesk__Last_Sync_Date__C,omitempty"`
	ZendeskLastSyncStatusC        string `json:"Zendesk__Last_Sync_Status__C,omitempty"`
	ZendeskNotesC                 string `json:"Zendesk__Notes__C,omitempty"`
	ZendeskTagsC                  string `json:"Zendesk__Tags__C,omitempty"`
	ZendeskZendeskOldTagsC        string `json:"Zendesk__Zendesk_oldTags__C,omitempty"`
	ZendeskZendeskOutofSyncC      string `json:"Zendesk__Zendesk_OutofSync__C,omitempty"`
	ZendeskZendeskOrganizationC   string `json:"Zendesk__Zendesk_Organization__C,omitempty"`
	ZendeskZendeskOrganizationIdC string `json:"Zendesk__Zendesk_Organization_Id__C,omitempty"`
	ZendeskZendeskResultC         string `json:"Zendesk__Result__C,omitempty"`
}

func (a *Account) Read(so SObjectCompatible) error {
	a.Ref = so

	u, ok := so.(*models.User)
	if !ok {
		return ErrorUserTypeRequired
	}

	a.CrowdstartIdC = u.Id

	if key, err := datastore.DecodeKey(u.Id); err == nil {
		a.Name = strconv.FormatInt(key.IntID(), 10)
	} else {
		// This should never happen
	}

	a.BillingStreet = u.BillingAddress.Line1 + "\n" + u.BillingAddress.Line2
	a.BillingCity = u.BillingAddress.City
	a.BillingState = u.BillingAddress.State
	a.BillingPostalCode = u.BillingAddress.PostalCode
	a.BillingCountry = u.BillingAddress.Country

	a.ShippingStreet = u.ShippingAddress.Line1 + "\n" + u.ShippingAddress.Line2
	a.ShippingCity = u.ShippingAddress.City
	a.ShippingState = u.ShippingAddress.State
	a.ShippingPostalCode = u.ShippingAddress.PostalCode
	a.ShippingCountry = u.ShippingAddress.Country
	return nil
}

func (a *Account) Write(so SObjectCompatible) error {
	a.Ref = so

	u, ok := so.(*models.User)
	if !ok {
		return ErrorUserTypeRequired
	}

	u.Id = a.CrowdstartIdC

	lines := strings.Split(a.ShippingStreet, "\n")

	// Split Street line \n to recover our data
	u.ShippingAddress.Line1 = lines[0]
	if len(lines) > 1 {
		u.ShippingAddress.Line2 = strings.Join(lines[1:], "\n")
	}

	u.ShippingAddress.City = a.ShippingCity
	u.ShippingAddress.State = a.ShippingState
	u.ShippingAddress.PostalCode = a.ShippingPostalCode
	u.ShippingAddress.Country = a.ShippingCountry

	lines = strings.Split(a.BillingStreet, "\n")

	// Split Street line \n to recover our data
	u.BillingAddress.Line1 = lines[0]
	if len(lines) > 1 {
		u.BillingAddress.Line2 = strings.Join(lines[1:], "\n")
	}

	u.BillingAddress.City = a.BillingCity
	u.BillingAddress.State = a.BillingState
	u.BillingAddress.PostalCode = a.BillingPostalCode
	u.BillingAddress.Country = a.BillingCountry

	return nil
}

func (a *Account) SetExternalId(id string) {
	a.CrowdstartIdC = id
}

func (a *Account) ExternalId() string {
	return a.CrowdstartIdC
}

func (a *Account) Push(api SalesforceClient) error {
	return push(api, AccountExternalIdPath, a)
}

func (a *Account) PullExternalId(api SalesforceClient, id string) error {
	return pull(api, AccountExternalIdPath, id, a)
}

func (a *Account) PullId(api SalesforceClient, id string) error {
	return pull(api, AccountPath, id, a)
}

type Order struct {
	ModelReference `json:"-"` // Struct this sobject refers to

	// Don't manually specify these

	// Response Only Fields
	Attributes     Attribute `json:"attributes,omitempty"`
	Id             string    `json:"Id,omitempty"`
	IsDeleted      bool      `json:"IsDeleted,omitempty"`
	MasterRecordId string    `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using Id (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__C,omitempty"`

	// Read Only
	CreatedById      string `json:"CreatedById,omitempty"`
	LastModifiedById string `json:"LastModifiedById,omitempty"`
	AccountId        string `json:"AccountId,omitempty"`

	// You can manually specify these
	// Data Fields
	Account                *Account `json:"Account,omitempty"`
	PricebookId            string   `json:"Pricebook2Id,omitempty"`
	OriginalOrderId        string   `json:"OriginalOrderId,omitempty"`
	EffectiveDate          string   `json:"EffectiveDate,omitempty"`
	EndDate                string   `json:"EndDate,omitempty"`
	IsReductionOrder       string   `json:"IsReductionOrder,omitempty"`
	Status                 string   `json:"Status,omitempty"`
	Description            string   `json:"Description,omitempty"`
	CustomerAuthorizedById string   `json:"CustomerAuthorizedById,omitempty"`
	CustomerAuthorizedDate string   `json:"CustomerAuthorizedDate,omitempty"`
	CompanyAuthorizedById  string   `json:"CompanyAuthorizedById,omitempty"`
	CompanyAuthorizedDate  string   `json:"CompanyAuthorizedDate,omitempty"`
	Type                   string   `json:"Type,omitempty"`
	BillingStreet          string   `json:"BillingStreet,omitempty"`
	BillingCity            string   `json:"BillingCity,omitempty"`
	BillingState           string   `json:"BillingState,omitempty"`
	BillingPostalCode      string   `json:"BillingPostalCode,omitempty"`
	BillingCountry         string   `json:"BillingCountry,omitempty"`
	BillingLatitude        string   `json:"BillingLatitude,omitempty"`
	BillingLongitude       string   `json:"BillingLongitude,omitempty"`
	ShippingStreet         string   `json:"ShippingStreet,omitempty"`
	ShippingCity           string   `json:"ShippingCity,omitempty"`
	ShippingState          string   `json:"ShippingState,omitempty"`
	ShippingPostalCode     string   `json:"ShippingPostalCode,omitempty"`
	ShippingCountry        string   `json:"ShippingCountry,omitempty"`
	ShippingLatitude       string   `json:"ShippingLatitude,omitempty"`
	ShippingLongitude      string   `json:"ShippingLongitude,omitempty"`
	//Name                   string   `json:"Name,omitempty"`
	PoDate               string `json:"PoDate,omitempty"`
	PoNumber             string `json:"PoNumber,omitempty"`
	OrderReferenceNumber string `json:"OrderReferenceNumber,omitempty"`
	BillToContactId      string `json:"BillToContactId,omitempty"`
	ShipToContactId      string `json:"ShipToContactId,omitempty"`
	ActivatedDate        string `json:"ActivatedDate,omitempty"`
	ActivatedById        string `json:"ActivatedById,omitempty"`
	StatusCode           string `json:"StatusCode,omitempty"`
	OrderNumber          string `json:"OrderNumber,omitempty"`
	TotalAmount          string `json:"TotalAmount,omitempty"`
	CreatedDate          string `json:"CreatedDate,omitempty"`
	SystemModstamp       string `json:"SystemModstamp,omitempty"`
	LastViewedDate       string `json:"LastViewedDate,omitempty"`
	LastReferencedDate   string `json:"LastReferencedDate,omitempty"`
	Order                string `json:"Order,omitempty"`
	Master               string `json:"Master,omitempty"`

	// Custom Crowdstart fields
	CancelledC     bool     `json:"Cancelled__C,omitempty"`
	DisputedC      bool     `json:"Disputed__C,omitempty"`
	LockedC        bool     `json:"Locked__C,omitempty"`
	PaymentIdC     string   `json:"PaymentId__C,omitempty"`
	PaymentTypeC   string   `json:"PaymentType__C,omitempty"`
	PreorderC      bool     `json:"Preorder__C,omitempty"`
	RefundedC      bool     `json:"Refunded__C,omitempty"`
	ShippedC       bool     `json:"Shipped__C,omitempty"`
	ShippingC      Currency `json:"Shipping__C,omitempty"`
	SubtotalC      Currency `json:"Subtotal__C,omitempty"`
	TaxC           Currency `json:"Tax__C,omitempty"`
	TotalC         Currency `json:"Total__C,omitempty"`
	UnconfirmedC   bool     `json:"Unconfirmed__C,omitempty"`
	OriginalEmailC string   `json:"OriginalEmail__C,omitempty"`

	// We don't use contracts
	ContractId string `json:"ContractId,omitempty"`

	// private data
	orderProducts []OrderProduct
}

func (o *Order) Read(so SObjectCompatible) error {
	o.Ref = so

	order, ok := so.(*models.Order)
	if !ok {
		return ErrorOrderTypeRequired
	}

	o.EffectiveDate = order.CreatedAt.Format(time.RFC3339)

	o.BillingStreet = order.BillingAddress.Line1 + "\n" + order.BillingAddress.Line2
	o.BillingCity = order.BillingAddress.City
	o.BillingState = order.BillingAddress.State
	o.BillingPostalCode = order.BillingAddress.PostalCode
	o.BillingCountry = order.BillingAddress.Country

	o.ShippingStreet = order.ShippingAddress.Line1 + "\n" + order.ShippingAddress.Line2
	o.ShippingCity = order.ShippingAddress.City
	o.ShippingState = order.ShippingAddress.State
	o.ShippingPostalCode = order.ShippingAddress.PostalCode
	o.ShippingCountry = order.ShippingAddress.Country

	o.Status = "Draft" // SF Rquired

	// Payment Information
	o.ShippingC = ToCurrency(order.Shipping)
	o.SubtotalC = ToCurrency(order.Subtotal)
	o.TaxC = ToCurrency(order.Tax)
	o.TotalC = ToCurrency(order.Shipping + order.Subtotal + order.Tax)

	if len(order.Charges) > 0 {
		o.PaymentTypeC = "Stripe"
		o.PaymentIdC = order.Charges[0].ID
	}

	// Status Flags
	o.CancelledC = order.Cancelled
	o.DisputedC = order.Disputed
	o.LockedC = order.Locked
	o.PreorderC = order.Preorder
	o.RefundedC = order.Refunded
	o.ShippedC = order.Shipped
	o.UnconfirmedC = order.Unconfirmed

	//SKU
	if !o.UnconfirmedC {
		o.orderProducts = make([]OrderProduct, len(order.Items))
		for i, item := range order.Items {
			orderProduct := OrderProduct{}
			orderProduct.Read(&item)
			orderProduct.Order = &Order{CrowdstartIdC: order.Id}
			o.orderProducts[i] = orderProduct
		}
	}

	// Skully salesforce is rejecting name
	// if name, err := datastore.DecodeKey(order.Id); err == nil {
	// 	o.Name = strconv.FormatInt(name.IntID(), 10)
	// }

	o.Account = &Account{CrowdstartIdC: order.UserId}
	o.CrowdstartIdC = order.Id
	o.OriginalEmailC = order.Email

	return nil
}

func (o *Order) Write(so SObjectCompatible) error {
	o.Ref = so

	// order, ok := so.(*models.Order)
	// if !ok {
	// 	return ErrorOrderTypeRequired
	// }

	return nil
}
func (o *Order) SetExternalId(id string) {
	o.CrowdstartIdC = id
}

func (o *Order) ExternalId() string {
	return o.CrowdstartIdC
}

func (o *Order) Push(api SalesforceClient) error {
	// Easiest way of clearing out the old OrderItems, ignore errors

	del(api, OrderExternalIdPath, o.CrowdstartIdC)
	if err := push(api, OrderExternalIdPath, o); err != nil {
		return err
	}

	for _, orderProduct := range o.orderProducts {
		if err := orderProduct.Push(api); err != nil {
			return err
		}
	}

	return nil
}

func (o *Order) PullExternalId(api SalesforceClient, id string) error {
	return pull(api, OrderExternalIdPath, id, o)
}

func (o *Order) PullId(api SalesforceClient, id string) error {
	return pull(api, OrderPath, id, o)
}

// func (o *Order) ToOrder(order *models.Order) error {
// 	lines := strings.Split(o.ShippingStreet, "\n")

// 	created, err := time.Parse(time.RFC3339, o.EffectiveDate)
// 	if err != nil {
// 		return err
// 	}

// 	order.CreatedAt = created

// 	// Split Street line \n to recover our data
// 	order.ShippingAddress.Line1 = lines[0]
// 	if len(lines) > 1 {
// 		order.ShippingAddress.Line2 = strings.Join(lines[1:], "\n")
// 	}

// 	order.ShippingAddress.City = o.ShippingCity
// 	order.ShippingAddress.State = o.ShippingState
// 	order.ShippingAddress.PostalCode = o.ShippingPostalCode
// 	order.ShippingAddress.Country = o.ShippingCountry

// 	lines = strings.Split(o.BillingStreet, "\n")

// 	// Split Street line \n to recover our data
// 	order.BillingAddress.Line1 = lines[0]
// 	if len(lines) > 1 {
// 		order.BillingAddress.Line2 = strings.Join(lines[1:], "\n")
// 	}

// 	order.BillingAddress.City = o.BillingCity
// 	order.BillingAddress.State = o.BillingState
// 	order.BillingAddress.PostalCode = o.BillingPostalCode
// 	order.BillingAddress.Country = o.BillingCountry

// 	lIs := strings.Split(o.Description, "\n")

// 	//Decode order info in the form of SKU,quantity\n
// 	order.Items = make([]models.LineItem, len(lIs))
// 	for _, lI := range lIs {
// 		t := strings.Split(lI, ",")
// 		if len(t) == 2 {
// 			if q, err := strconv.ParseInt(t[1], 10, 64); err == nil {
// 				lineItem := models.LineItem{
// 					SKU_:     t[0],
// 					Quantity: int(q),
// 				}
// 				order.Items = append(order.Items, lineItem)
// 			}
// 		}
// 	}

// 	return nil
// }

type OrderProduct struct {
	ModelReference `json:"-"` // Struct this sobject refers to

	// Don't manually specify these

	// Response Only Fields
	Attributes     Attribute `json:"attributes,omitempty"`
	Id             string    `json:"Id,omitempty"`
	IsDeleted      bool      `json:"IsDeleted,omitempty"`
	MasterRecordId string    `json:"MasterRecordId,omitempty"`

	// Read Only
	CreatedById        string `json:"CreatedById,omitempty"`
	LastModifiedById   string `json:"LastModifiedById,omitempty"`
	AccountId          string `json:"AccountId,omitempty"`
	OrderProductNumber int64  `json:"OrderItemNumber,omitempty"`
	ProductCode        string `json:"ProductCode,omitempty"`
	ListPrice          string `json:"ListPrice,omitempty"`

	// You can manually specify these
	// Data Fields
	AvailableQuantity    int64           `json:"AvailableQuantity,omitempty"`
	EndDate              string          `json:"EndDate,omitempty"`
	Description          string          `json:"Description,omitempty"`
	Order                *Order          `json:"Order,omitempty"`
	OriginalOrderProduct *OrderProduct   `json:"OriginalOrderItem,omitempty"`
	PricebookEntry       *PricebookEntry `json:"PricebookEntry,omitempty"`
	Quantity             int64           `json:"Quantity,omitempty"`
	StartDate            string          `json:"ServiceDate,omitempty"`
	TotalPrice           Currency        `json:"TotalPrice,omitempty"`
	UnitPrice            Currency        `json:"UnitPrice,omitempty"`
}

func (o *OrderProduct) Read(so SObjectCompatible) error {
	o.Ref = so

	li, ok := so.(*models.LineItem)
	if !ok {
		return ErrorOrderTypeRequired
	}

	o.Quantity = int64(li.Quantity)
	o.PricebookEntry = &PricebookEntry{CrowdstartIdC: li.VariantId}
	o.UnitPrice = ToCurrency(li.Variant.Price)

	return nil
}

func (o *OrderProduct) Write(so SObjectCompatible) error {
	o.Ref = so

	return nil
}

func (o *OrderProduct) SetExternalId(id string) {
}

func (o *OrderProduct) ExternalId() string {
	return ""
}

func (o *OrderProduct) Push(api SalesforceClient) error {
	return push(api, OrderProductBasePath, o)
}

func (o *OrderProduct) PullExternalId(api SalesforceClient, id string) error {
	return ErrorShouldNotCall
}

func (o *OrderProduct) PullId(api SalesforceClient, id string) error {
	return ErrorShouldNotCall
}

type Product struct {
	ModelReference `json:"-"` // Struct this sobject refers to

	// Don't manually specify these

	// Response Only Fields
	Attributes     Attribute `json:"attributes,omitempty"`
	Id             string    `json:"Id,omitempty"`
	IsDeleted      bool      `json:"IsDeleted,omitempty"`
	MasterRecordId string    `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using Id (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__C,omitempty"`

	// Read Only
	CreatedById      string `json:"CreatedById,omitempty"`
	LastModifiedById string `json:"LastModifiedById,omitempty"`

	// You can manually specify these
	// Data Fields
	Name        string `json:"Name,omitempty"`
	Description string `json:"Description,omitempty"`
	ProductCode string `json:"ProductCode,omitempty"`
	IsActive    bool   `json:"IsActive,omitempty"`
	Family      string `json:"Family,omitempty"`

	// private data
	pricebook2Id string
}

func (p *Product) Read(so SObjectCompatible) error {
	p.Ref = so

	v, ok := so.(*models.ProductVariant)
	if !ok {
		return ErrorUserTypeRequired
	}

	p.CrowdstartIdC = v.Id
	p.Name = v.SKU
	p.ProductCode = v.SKU
	p.IsActive = true

	return nil
}

func (p *Product) Write(so SObjectCompatible) error {
	p.Ref = so

	v, ok := so.(*models.ProductVariant)
	if !ok {
		return ErrorUserTypeRequired
	}

	v.Id = p.CrowdstartIdC
	v.SKU = p.ProductCode

	return nil
}

func (p *Product) SetExternalId(id string) {
	p.CrowdstartIdC = id
}

func (p *Product) ExternalId() string {
	return p.CrowdstartIdC
}

func (p *Product) Push(api SalesforceClient) error {
	return push(api, ProductExternalIdPath, p)
}

func (p *Product) PullExternalId(api SalesforceClient, id string) error {
	return pull(api, ProductExternalIdPath, id, p)
}

func (p *Product) PullId(api SalesforceClient, id string) error {
	return pull(api, ProductPath, id, p)
}

type PricebookEntry struct {
	ModelSecondaryReference `json:"-"` // Struct this sobject refers to

	// Don't manually specify these

	// Response Only Fields
	Attributes     Attribute `json:"attributes,omitempty"`
	Id             string    `json:"Id,omitempty"`
	IsDeleted      bool      `json:"IsDeleted,omitempty"`
	MasterRecordId string    `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using Id (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__C,omitempty"`

	// Read Only
	CreatedById      string `json:"CreatedById,omitempty"`
	LastModifiedById string `json:"LastModifiedById,omitempty"`
	Name             string `json:"Name,omitempty"`
	ProductCode      string `json:"ProductCode,omitempty"`

	// You can manually specify these
	// Data Fields
	CurrencyIsoCode  string   `json:"CurrencyIsoCode,omitempty"`
	PricebookId      string   `json:"Pricebook2Id,omitempty"`
	Product          *Product `json:"Product2,omitempty"`
	UnitPrice        Currency `json:"UnitPrice,omitempty"`
	UseStandardPrice bool     `json:"UseStandardPrice,omitempty"`
	IsActive         bool     `json:"IsActive,omitempty"`
}

func (p *PricebookEntry) Read(so SObjectCompatible) error {
	v, ok := so.(*models.ProductVariant)
	if !ok {
		return ErrorUserTypeRequired
	}

	p.CrowdstartIdC = v.Id
	p.Product = &Product{CrowdstartIdC: v.Id}
	p.UseStandardPrice = false
	p.UnitPrice = ToCurrency(v.Price)
	p.IsActive = true

	return nil
}

func (p *PricebookEntry) Write(so SObjectCompatible) error {
	v, ok := so.(*models.ProductVariant)
	if !ok {
		return ErrorUserTypeRequired
	}

	v.Id = p.CrowdstartIdC
	//v.UnitPrice =

	return nil
}

func (p *PricebookEntry) SetExternalId(id string) {
	p.CrowdstartIdC = id
}

func (p *PricebookEntry) ExternalId() string {
	return p.CrowdstartIdC
}

func (p *PricebookEntry) Push(api SalesforceClient) error {
	return push(api, PricebookEntryExternalIdPath, p)
}

func (p *PricebookEntry) PullExternalId(api SalesforceClient, id string) error {
	return pull(api, PricebookEntryExternalIdPath, id, p)
}

func (p *PricebookEntry) PullId(api SalesforceClient, id string) error {
	return pull(api, PricebookEntryPath, id, p)
}

func del(api SalesforceClient, path, id string) error {
	p := fmt.Sprintf(path, id)
	if err := api.Request("DELETE", p, "", nil, true); err != nil {
		return err
	}

	return nil
}

// Helper functions
func push(api SalesforceClient, p string, s SObjectSerializeable) error {
	id := s.ExternalId()

	// nee to set UserId to blank to prevent serialization
	s.SetExternalId("")
	bytes, err := json.Marshal(s)
	if err != nil {
		return err
	}
	s.SetExternalId(id)

	// If no ID, then we must create a record instead of upsert
	path := p
	method := "POST"
	if id != "" {
		path = fmt.Sprintf(path, strings.Replace(id, ".", "_", -1))
		method = "PATCH"
	}

	data := string(bytes[:])
	log.Debug("Pushing Json: %v", data, api.GetContext())

	// Set the last sync date on the object
	s.SetLastSync()
	if err := api.Request(method, path, data, &map[string]string{"Content-Type": "application/json"}, true); err != nil {
		return err
	}

	// Debug in production only
	// log.Warn("Receiving Json: %v", string(api.GetBody()[:]), api.GetContext())

	body := api.GetBody()
	status := api.GetStatusCode()
	if len(body) == 0 {
		if status == 201 || status == 204 {
			return nil
		} else {
			return &ErrorUnexpectedStatusCode{StatusCode: status, Body: body}
		}
	}

	response := new(UpsertResponse)

	if err := json.Unmarshal(body, response); err != nil {
		return err
	}

	if !response.Success {
		return &response.Errors[0]
	}

	// Set the Id of the struct that the sobject refrences
	s.SetSalesforceId(response.Id)

	return nil
}

func pull(api SalesforceClient, path, id string, s SObjectSerializeable) error {
	p := fmt.Sprintf(path, id)
	if err := api.Request("GET", p, "", nil, true); err != nil {
		return err
	}

	return json.Unmarshal(api.GetBody(), s)
}

func getUpdated(api SalesforceClient, p string, start, end time.Time, response *UpdatedRecordsResponse) error {
	path := fmt.Sprintf(p, start.Format(time.RFC3339), end.Format(time.RFC3339))

	if err := api.Request("GET", path, "", nil, true); err != nil {
		return err
	}

	if err := json.Unmarshal(api.GetBody(), response); err != nil {
		return err
	}

	return nil
}

func GetUpdatedContacts(api *Api, start, end time.Time, response *UpdatedRecordsResponse) error {
	return getUpdated(api, ContactsUpdatedPath, start, end, response)
}

func GetUpdatedAccounts(api *Api, start, end time.Time, response *UpdatedRecordsResponse) error {
	return getUpdated(api, AccountsUpdatedPath, start, end, response)
}
