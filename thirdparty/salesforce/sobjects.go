package salesforce

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"hanzo.io/datastore"
	// "hanzo.io/models"
	"hanzo.io/models/lineitem"
	"hanzo.io/models/order"
	"hanzo.io/models/user"
	"hanzo.io/models/variant"
	"hanzo.io/util/log"
)

var ErrorUserTypeRequired = errors.New("Parameter needs to be of type User")
var ErrorOrderTypeRequired = errors.New("Parameter needs to be of type Order")
var ErrorShouldNotCall = errors.New("Function should not be called")

type Currency float64

func ToCurrency(centicents int64) Currency {
	return Currency(float64(centicents) / 10000.0)
}

func FromCurrency(dollars Currency) int64 {
	return int64(dollars * 10000.0)
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

// SObjects represent Salesforce SObjects that can be pushed and pull from/to Salesforce
type SObject interface {
	// Send model to salesforce
	Push(SalesforceClient) error

	// Get Model from salesforce
	PullId(SalesforceClient, string) error

	// Get Model using CrowdstartId from salesforce
	PullExternalId(SalesforceClient, string) error
}

type SObjectIDable interface {
	// Set CrowdstartId
	SetExternalId(string)
	// Get CrowdstartId
	ExternalId() string
}

type SObjectSerializeable interface {
	// Loads data from an SObjectCompatible
	Read(SObjectCompatible) error
	// Writes its data into an SObjectCompatible
	Write(SObjectCompatible) error
}

type SObjectSyncable interface {
	SObjectIDable
	SObjectSerializeable

	// SObjectCompatible proxies
	SetSalesforceId(string)
	SalesforceId() string
	SetLastSync()
	LastSync() time.Time
}

type SObjectLoadable interface {
	SObject
	SObjectSyncable

	LoadSalesforceId(*datastore.Datastore, string) SObjectCompatible
	Load(*datastore.Datastore) SObjectCompatible
}

// Reference to the datastore model for an SObject
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

// Also a reference like above but for models that refer to multiple sobjects which need to use a second id field
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

// SObject foreign key reference so we can use Crowdstart Id instead of Salesforce ID to reference an object
type ForeignKey struct {
	Attributes    *Attribute `json:"attributes,omitempty"`
	CrowdstartIdC string     `json:"CrowdstartId__c,omitempty"`
}

//SObject Definitions
type Contact struct {
	ModelSecondaryReference `json:"-"` // Struct this sobject refers to

	// Don't manually specify these

	// Response Only Fields
	Attributes     *Attribute `json:"attributes,omitempty"`
	Id             string     `json:"Id,omitempty"`
	IsDeleted      bool       `json:"IsDeleted,omitempty"`
	MasterRecordId string     `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using Id (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__c,omitempty"`

	// Read Only
	Name             string `json:"Name,omitempty"`
	AccountId        string `json:"AccountId,omitempty"`
	CreatedById      string `json:"CreatedById,omitempty"`
	LastModifiedById string `json:"LastModifiedById,omitempty"`

	// You can manually specify these

	// Data Fields
	LastName           string     `json:"LastName,omitempty"`
	FirstName          string     `json:"FirstName,omitempty"`
	Salutation         string     `json:"Salutation,omitempty"`
	MailingStreet      string     `json:"MailingStreet,omitempty"`
	MailingCity        string     `json:"MailingCity,omitempty"`
	MailingState       string     `json:"MailingState,omitempty"`
	MailingPostalCode  string     `json:"MailingPostalCode,omitempty"`
	MailingCountry     string     `json:"MailingCountry,omitempty"`
	MailingStateCode   string     `json:"MailingStateCode,omitempty"`
	MailingCountryCode string     `json:"MailingCountryCode,omitempty"`
	MailingLatitude    string     `json:"MailingLatitude,omitempty"`
	MailingLongitude   string     `json:"MailingLongitude,omitempty"`
	Phone              string     `json:"Phone,omitempty"`
	Fax                string     `json:"Fax,omitempty"`
	MobilePhone        string     `json:"MobilePhone,omitempty"`
	ReportsToId        string     `json:"ReportsToId,omitempty"`
	Email              string     `json:"Email,omitempty"`
	Title              string     `json:"Title,omitempty"`
	Department         string     `json:"Department,omitempty"`
	OwnerId            string     `json:"OwnerId,omitempty"`
	CreatedDate        string     `json:"CreatedDate,omitempty"`
	LastModifiedDate   string     `json:"LastModifiedDate,omitempty"`
	SystemModstamp     string     `json:"SystemModstamp,omitempty"`
	LastActivityDate   string     `json:"LastActivityDate,omitempty"`
	LastCURequestDate  string     `json:"LastCURequestDate,omitempty"`
	LastCUUpdateDate   string     `json:"LastCUUpdateDate,omitempty"`
	LastViewedDate     string     `json:"LastViewedDate,omitempty"`
	LastReferencedDate string     `json:"LastReferencedDate,omitempty"`
	EmailBouncedReason string     `json:"EmailBouncedReason,omitempty"`
	EmailBouncedDate   string     `json:"EmailBouncedDate,omitempty"`
	IsEmailBounced     bool       `json:"IsEmailBounced,omitempty"`
	JigsawContactId    string     `json:"JigsawContactId,omitempty"`
	Account            ForeignKey `json:"Account,omitempty"`

	// Skully Custom fields
	UniquePreorderLinkC string `json:"Unique_Preorder_Link__c,omitempty"`
	FullfillmentStatusC string `json:"Fulfillment_Status__c,omitempty"`
	PreorderC           string `json:"Preorder__c,omitempty"`
	ShippingAddressC    string `json:"Shipping_Address__c,omitempty"`
	ShippingCityC       string `json:"Shipping_City__c,omitempty"`
	ShippingStateC      string `json:"Shipping_State__c,omitempty"`
	ShippingPostalZipC  string `json:"Shipping_Postal_Zip__c,omitempty"`
	ShippingCountryC    string `json:"Shipping_Country__c,omitempty"`
	MC4SFMCSubscriberC  string `json:"MC4SF__MC_Subscriber__c,omitempty"`

	// Zendesk Custom fields
	ZendeskLastSyncDateC         string `json:"Zendesk__Last_Sync_Date__c,omitempty"`
	ZendeskLastSyncStatusC       string `json:"Zendesk__Last_Sync_Status__c,omitempty"`
	ZendeskResultC               string `json:"Zendesk__Result__c,omitempty"`
	ZendeskTagsC                 string `json:"Zendesk__Tags__c,omitempty"`
	ZendeskZendeskOutofSyncC     string `json:"Zendesk__Zendesk_OutofSync__c,omitempty"`
	ZendeskZendeskOldTagsC       string `json:"Zendesk__Zendesk_oldTags__c,omitempty"`
	ZendeskIsCreatedUpdatedFlagC string `json:"Zendesk__isCreatedUpdatedFlag__c,omitempty"`
	ZendeskNotesC                string `json:"Zendesk__notes__c,omitempty"`
	ZendeskZendeskIdC            string `json:"Zendesk__zendesk_id__c,omitempty"`
}

func (c *Contact) Read(so SObjectCompatible) error {
	return nil
	// c.Ref = so

	// u, ok := so.(*models.User)
	// if !ok {
	// 	return ErrorUserTypeRequired
	// }

	// c.CrowdstartIdC = u.Id
	// c.LastName = u.LastName
	// if c.LastName == "" {
	// 	c.LastName = "-"
	// }

	// c.FirstName = u.FirstName
	// if c.FirstName == "" {
	// 	c.FirstName = "-"
	// }

	// c.Email = u.Email
	// c.Phone = u.Phone

	// c.Account.CrowdstartIdC = u.Id

	// return nil
}

func (c *Contact) Write(so SObjectCompatible) error {
	return nil
	// c.Ref = so

	// u, ok := so.(*models.User)
	// if !ok {
	// 	return ErrorUserTypeRequired
	// }

	// c.SetSalesforceId(c.Id)

	// u.Id = c.CrowdstartIdC
	// u.Email = c.Email

	// u.LastName = c.LastName
	// if u.LastName == "-" {
	// 	u.LastName = ""
	// }

	// u.FirstName = c.FirstName
	// if u.FirstName == "-" {
	// 	u.FirstName = ""
	// }

	// u.Phone = c.Phone
	// return nil
}

func (c *Contact) SetExternalId(id string) {
	c.CrowdstartIdC = id
}

func (c *Contact) ExternalId() string {
	return c.CrowdstartIdC
}

func (c *Contact) Load(db *datastore.Datastore) SObjectCompatible {
	c.Ref = user.New(db)
	db.GetById(c.ExternalId(), c.Ref)
	return c.Ref
}

func (c *Contact) LoadSalesforceId(db *datastore.Datastore, id string) SObjectCompatible {
	objects := make([]*user.User, 0)
	db.Query("user").Filter("SecondarySalesforceId_=", id).Limit(1).GetAll(&objects)
	if len(objects) == 0 {
		return nil
	}
	return objects[0]
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
	Attributes     *Attribute `json:"attributes,omitempty"`
	Id             string     `json:"Id,omitempty"`
	IsDeleted      bool       `json:"IsDeleted,omitempty"`
	MasterRecordId string     `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using Id (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__c,omitempty"`

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
	CustomerPriorityC  string `json:"CustomerPriority__c,omitempty"`
	SlaC               string `json:"SLA__c,omitempty"`
	ActiveC            string `json:"Active__c,omitempty"`
	NumberofLocationsC string `json:"NumberofLocations__c,omitempty"`
	UpsellOpportunityC string `json:"UpsellOpportunity__c,omitempty"`
	SLASerialNumberC   string `json:"SLASerialNumber__c,omitempty"`
	SLAExpirationDateC string `json:"SLAExpirationDate__c,omitempty"`
	Account            string `json:"Account,omitempty"`
	Master             string `json:"Master,omitempty"`

	// Zendesk integration items
	ZendeskCreatedUpdatedFlagC    string `json:"Zendesk__createdUpdatedFlag__c,omitempty"`
	ZendeskDomainMappingC         string `json:"Zendesk__Domain_Mapping__c,omitempty"`
	ZendeskLastSyncDataC          string `json:"Zendesk__Last_Sync_Date__c,omitempty"`
	ZendeskLastSyncStatusC        string `json:"Zendesk__Last_Sync_Status__c,omitempty"`
	ZendeskNotesC                 string `json:"Zendesk__Notes__c,omitempty"`
	ZendeskTagsC                  string `json:"Zendesk__Tags__c,omitempty"`
	ZendeskZendeskOldTagsC        string `json:"Zendesk__Zendesk_oldTags__c,omitempty"`
	ZendeskZendeskOutofSyncC      string `json:"Zendesk__Zendesk_OutofSync__c,omitempty"`
	ZendeskZendeskOrganizationC   string `json:"Zendesk__Zendesk_Organization__c,omitempty"`
	ZendeskZendeskOrganizationIdC string `json:"Zendesk__Zendesk_Organization_Id__c,omitempty"`
	ZendeskZendeskResultC         string `json:"Zendesk__Result__c,omitempty"`
}

func (a *Account) Read(so SObjectCompatible) error {
	a.Ref = so

	u, ok := so.(*user.User)
	if !ok {
		return ErrorUserTypeRequired
	}

	a.CrowdstartIdC = u.Id()

	// if key, err := aeds.DecodeKey(u.Id); err == nil {
	// 	a.Name = u.Key()strconv.FormatInt(key.IntID(), 10)
	// } else {
	// 	// This should never happen
	// }

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
	return nil
	// a.Ref = so

	// u, ok := so.(*user.User)
	// if !ok {
	// 	return ErrorUserTypeRequired
	// }

	// a.SetSalesforceId(a.Id)

	// u.Id = a.CrowdstartIdC

	// lines := strings.Split(a.ShippingStreet, "\n")

	// // Split Street line \n to recover our data
	// u.ShippingAddress.Line1 = strings.TrimSpace(lines[0])
	// if len(lines) > 1 {
	// 	u.ShippingAddress.Line2 = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	// }

	// u.ShippingAddress.City = a.ShippingCity
	// u.ShippingAddress.State = a.ShippingState
	// u.ShippingAddress.PostalCode = a.ShippingPostalCode
	// u.ShippingAddress.Country = a.ShippingCountry

	// lines = strings.Split(a.BillingStreet, "\n")

	// // Split Street line \n to recover our data
	// u.BillingAddress.Line1 = strings.TrimSpace(lines[0])
	// if len(lines) > 1 {
	// 	u.BillingAddress.Line2 = strings.TrimSpace(strings.Join(lines[1:], "\n"))
	// }

	// u.BillingAddress.City = a.BillingCity
	// u.BillingAddress.State = a.BillingState
	// u.BillingAddress.PostalCode = a.BillingPostalCode
	// u.BillingAddress.Country = a.BillingCountry

	// return nil
}

func (a *Account) SetExternalId(id string) {
	a.CrowdstartIdC = id
}

func (a *Account) ExternalId() string {
	return a.CrowdstartIdC
}

func (a *Account) Load(db *datastore.Datastore) SObjectCompatible {
	a.Ref = user.New(db)
	db.GetById(a.ExternalId(), a.Ref)
	return a.Ref
}

func (a *Account) LoadSalesforceId(db *datastore.Datastore, id string) SObjectCompatible {
	objects := make([]*user.User, 0)
	db.Query("user").Filter("PrimarySalesforceId_=", id).Limit(1).GetAll(&objects)
	if len(objects) == 0 {
		return nil
	}
	return objects[0]
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

// Place Order metadata junk things
type PlaceOrderWrapper struct {
	TotalSize int64 `json:"totalSize"`
	Done      bool  `json:"done"`
}

type PlaceOrderOrderWrapper struct {
	PlaceOrderWrapper
	Records []*Order `json:records`
}

type PlaceOrderOrderProductWrapper struct {
	PlaceOrderWrapper
	Records []*OrderProduct `json:records`
}

type Order struct {
	ModelReference `json:"-"` // Struct this sobject refers to

	// Don't manually specify these

	// Response Only Fields
	Attributes     *Attribute `json:"attributes,omitempty"`
	Id             string     `json:"Id,omitempty"`
	IsDeleted      bool       `json:"IsDeleted,omitempty"`
	MasterRecordId string     `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using Id (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__c,omitempty"`

	// Read Only
	CreatedById      string `json:"CreatedById,omitempty"`
	LastModifiedById string `json:"LastModifiedById,omitempty"`
	AccountId        string `json:"AccountId,omitempty"`

	// You can manually specify these
	// Data Fields
	Account                *ForeignKey `json:"Account,omitempty"`
	PricebookId            string      `json:"Pricebook2Id,omitempty"`
	OriginalOrderId        string      `json:"OriginalOrderId,omitempty"`
	EffectiveDate          string      `json:"EffectiveDate,omitempty"`
	EndDate                string      `json:"EndDate,omitempty"`
	IsReductionOrder       bool        `json:"IsReductionOrder,omitempty"`
	Status                 string      `json:"Status,omitempty"`
	Description            string      `json:"Description,omitempty"`
	CustomerAuthorizedById string      `json:"CustomerAuthorizedById,omitempty"`
	CustomerAuthorizedDate string      `json:"CustomerAuthorizedDate,omitempty"`
	CompanyAuthorizedById  string      `json:"CompanyAuthorizedById,omitempty"`
	CompanyAuthorizedDate  string      `json:"CompanyAuthorizedDate,omitempty"`
	Type                   string      `json:"Type,omitempty"`
	BillingStreet          string      `json:"BillingStreet,omitempty"`
	BillingCity            string      `json:"BillingCity,omitempty"`
	BillingState           string      `json:"BillingState,omitempty"`
	BillingPostalCode      string      `json:"BillingPostalCode,omitempty"`
	BillingCountry         string      `json:"BillingCountry,omitempty"`
	BillingLatitude        string      `json:"BillingLatitude,omitempty"`
	BillingLongitude       string      `json:"BillingLongitude,omitempty"`
	ShippingStreet         string      `json:"ShippingStreet,omitempty"`
	ShippingCity           string      `json:"ShippingCity,omitempty"`
	ShippingState          string      `json:"ShippingState,omitempty"`
	ShippingPostalCode     string      `json:"ShippingPostalCode,omitempty"`
	ShippingCountry        string      `json:"ShippingCountry,omitempty"`
	ShippingLatitude       string      `json:"ShippingLatitude,omitempty"`
	ShippingLongitude      string      `json:"ShippingLongitude,omitempty"`
	//Name                   string   `json:"Name,omitempty"`
	PoDate               string   `json:"PoDate,omitempty"`
	PoNumber             string   `json:"PoNumber,omitempty"`
	OrderReferenceNumber string   `json:"OrderReferenceNumber,omitempty"`
	BillToContactId      string   `json:"BillToContactId,omitempty"`
	ShipToContactId      string   `json:"ShipToContactId,omitempty"`
	ActivatedDate        string   `json:"ActivatedDate,omitempty"`
	ActivatedById        string   `json:"ActivatedById,omitempty"`
	StatusCode           string   `json:"StatusCode,omitempty"`
	OrderNumber          string   `json:"OrderNumber,omitempty"`
	TotalAmount          Currency `json:"TotalAmount,omitempty"`
	CreatedDate          string   `json:"CreatedDate,omitempty"`
	SystemModstamp       string   `json:"SystemModstamp,omitempty"`
	LastViewedDate       string   `json:"LastViewedDate,omitempty"`
	LastReferencedDate   string   `json:"LastReferencedDate,omitempty"`
	Order                string   `json:"Order,omitempty"`
	Master               string   `json:"Master,omitempty"`

	// Custom Crowdstart fields
	CancelledC     bool     `json:"Cancelled__c,omitempty"`
	DisputedC      bool     `json:"Disputed__c,omitempty"`
	LockedC        bool     `json:"Locked__c,omitempty"`
	PaymentIdC     string   `json:"PaymentId__c,omitempty"`
	PaymentTypeC   string   `json:"PaymentType__c,omitempty"`
	PreorderC      bool     `json:"Preorder__c,omitempty"`
	RefundedC      bool     `json:"Refunded__c,omitempty"`
	ShippedC       bool     `json:"Shipped__c,omitempty"`
	ShippingC      Currency `json:"Shipping__c,omitempty"`
	SubtotalC      Currency `json:"Subtotal__c,omitempty"`
	TaxC           Currency `json:"Tax__c,omitempty"`
	TotalC         Currency `json:"Total__c,omitempty"`
	UnconfirmedC   bool     `json:"Unconfirmed__c,omitempty"`
	OriginalEmailC string   `json:"OriginalEmail__c,omitempty"`

	// We don't use contracts
	ContractId string `json:"ContractId,omitempty"`

	// PlaceOrder API requirement
	OrderProducts *PlaceOrderOrderProductWrapper `json:"OrderItems,omitempty"`
	// private data
	orderProducts []*OrderProduct
}

func (o *Order) Read(so SObjectCompatible) error {
	o.Ref = so

	order, ok := so.(*order.Order)
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

	// // Payment Information
	// o.ShippingC = ToCurrency(order.Shipping)
	// o.SubtotalC = ToCurrency(order.Subtotal)
	// o.TaxC = ToCurrency(order.Tax)
	// o.TotalC = ToCurrency(order.Shipping + order.Subtotal + order.Tax)

	// if len(order.Charges) > 0 {
	// 	o.PaymentTypeC = "Stripe"
	// 	o.PaymentIdC = order.Charges[0].ID
	// }

	// // Status Flags
	// o.CancelledC = order.Cancelled
	// o.DisputedC = order.Disputed
	// o.LockedC = order.Locked
	// o.PreorderC = order.Preorder
	// o.RefundedC = order.Refunded
	// o.ShippedC = order.Shipped
	// o.UnconfirmedC = order.Unconfirmed

	// //SKU
	// if !o.UnconfirmedC {
	// 	o.orderProducts = make([]*OrderProduct, len(order.Items))
	// 	for i, _ := range order.Items {
	// 		item := &order.Items[i]
	// 		orderProduct := &OrderProduct{CrowdstartIdC: order.Id + fmt.Sprintf("_%d", i)}
	// 		orderProduct.Read(item)
	// 		orderProduct.Order = &ForeignKey{CrowdstartIdC: order.Id}
	// 		o.orderProducts[i] = orderProduct
	// 	}
	// }

	// // Skully salesforce is rejecting name
	// // if name, err := datastore.DecodeKey(order.Id); err == nil {
	// // 	o.Name = strconv.FormatInt(name.IntID(), 10)
	// // }

	// o.Account = &ForeignKey{CrowdstartIdC: order.UserId}
	// o.CrowdstartIdC = order.Id
	// o.OriginalEmailC = order.Email

	return nil
}

func (o *Order) Write(so SObjectCompatible) error {
	o.Ref = so

	order, ok := so.(*order.Order)
	if !ok {
		return ErrorOrderTypeRequired
	}

	o.SetSalesforceId(o.Id)

	// We shouldn't update a read only value like this
	// order.CreatedAt = time.Parse(time.RFC3339, o.EffectiveDate)

	lines := strings.Split(o.BillingStreet, "\r\n")
	order.BillingAddress.Line1 = lines[0]
	if len(lines) > 1 {
		order.BillingAddress.Line2 = strings.Join(lines[1:], "\r\n")
	}

	order.BillingAddress.City = o.BillingCity
	order.BillingAddress.State = o.BillingState
	order.BillingAddress.PostalCode = o.BillingPostalCode
	order.BillingAddress.Country = o.BillingCountry

	lines = strings.Split(o.ShippingStreet, "\r\n")
	order.ShippingAddress.Line1 = lines[0]
	if len(lines) > 1 {
		order.ShippingAddress.Line2 = strings.Join(lines[1:], "\r\n")
	}

	order.ShippingAddress.City = o.ShippingCity
	order.ShippingAddress.State = o.ShippingState
	order.ShippingAddress.PostalCode = o.ShippingPostalCode
	order.ShippingAddress.Country = o.ShippingCountry

	// Payment Information
	// order.Shipping = FromCurrency(o.ShippingC)
	// order.Subtotal = FromCurrency(o.SubtotalC)
	// order.Tax = FromCurrency(o.TaxC)

	// // We shouldn't update a read only value like this
	// // if len(order.Charges) > 0 {
	// // 	o.PaymentTypeC = "Stripe"
	// // 	o.PaymentIdC = order.Charges[0].ID
	// // }

	// // Status Flags
	// order.Cancelled = o.CancelledC
	// order.Disputed = o.DisputedC
	// order.Locked = o.LockedC
	// order.Preorder = o.PreorderC
	// order.Refunded = o.RefundedC
	// order.Shipped = o.ShippedC
	// order.Unconfirmed = o.UnconfirmedC

	// //SKU
	// lineItems := make([]models.LineItem, len(o.orderProducts))
	// order.Items = lineItems
	// for i, op := range o.orderProducts {
	// 	lineItems[i] = models.LineItem{}
	// 	op.Write(&lineItems[i])
	// }

	// // Skully salesforce is rejecting name
	// // if name, err := datastore.DecodeKey(order.Id); err == nil {
	// // 	o.Name = strconv.FormatInt(name.IntID(), 10)
	// // }

	// // We shouldn't update a read only value like this
	// // o.OriginalEmailC = order.Email

	// order.Id = o.CrowdstartIdC

	return nil
}

func (o *Order) SetExternalId(id string) {
	o.CrowdstartIdC = id
}

func (o *Order) ExternalId() string {
	return o.CrowdstartIdC
}

func (o *Order) Load(db *datastore.Datastore) SObjectCompatible {
	o.Ref = order.New(db)
	db.GetById(o.ExternalId(), o.Ref)
	return o.Ref
}

func (o *Order) LoadSalesforceId(db *datastore.Datastore, id string) SObjectCompatible {
	objects := make([]*order.Order, 0)
	db.Query("order").Filter("PrimarySalesforceId_=", id).Limit(1).GetAll(&objects)
	if len(objects) == 0 {
		return nil
	}
	return objects[0]
}

func (o *Order) Push(api SalesforceClient) error {
	// Easiest way of clearing out the old OrderItems, ignore errors

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

var variantCache map[string]variant.Variant

// Helper for getting a Order's Order Products
func pullOrderProduct(api SalesforceClient, o *Order) error {
	if variantCache == nil {
		variantCache = make(map[string]variant.Variant)
	}
	// Get Order Products as well.  Use the place order product since it is likely faster than a filter
	poow := PlaceOrderOrderWrapper{}
	if err := pull(api, PlaceOrderOrderPath, o.Id, &poow); err != nil {
		return err
	}

	// If no orders, then leave
	if len(poow.Records) == 0 {
		return nil
	}

	// If no product orders, then leave
	if poow.Records[0].OrderProducts == nil {
		return nil
	}

	// Otherwise being the process of loading order product into orders
	db := datastore.New(api.GetContext())
	ops := poow.Records[0].OrderProducts.Records
	o.orderProducts = ops
	for _, op := range ops {
		if err := op.PullId(api, op.Id); err != nil {
			return err
		}

		if op.PricebookEntryId == "" {
			continue
		}

		pv, ok := variantCache[op.PricebookEntryId]
		if !ok {
			variants := make([]variant.Variant, 0)
			db.Query("variant").Filter("SecondarySalesforceId_=", op.PricebookEntryId).Limit(1).GetAll(&variants)
			variantCache[op.PricebookEntryId] = variants[0]
			pv = variants[0]
		}

		op.variant = &pv
	}

	return nil
}

func (o *Order) PullExternalId(api SalesforceClient, id string) error {
	if err := pull(api, OrderExternalIdPath, id, o); err != nil {
		return err
	}

	return pullOrderProduct(api, o)
}

func (o *Order) PullId(api SalesforceClient, id string) error {
	if err := pull(api, OrderPath, id, o); err != nil {
		log.Warn("Order? %v", o)

		return err
	}
	log.Warn("Order? %v", o)

	return pullOrderProduct(api, o)
}

type OrderProduct struct {
	ModelReference `json:"-"` // Struct this sobject refers to

	// Don't manually specify these

	// Response Only Fields
	Attributes     *Attribute `json:"attributes,omitempty"`
	Id             string     `json:"Id,omitempty"`
	IsDeleted      bool       `json:"IsDeleted,omitempty"`
	MasterRecordId string     `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using Id (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__c,omitempty"`

	// Read Only
	CreatedById        string   `json:"CreatedById,omitempty"`
	LastModifiedById   string   `json:"LastModifiedById,omitempty"`
	AccountId          string   `json:"AccountId,omitempty"`
	OrderProductNumber string   `json:"OrderItemNumber,omitempty"`
	ProductCode        string   `json:"ProductCode,omitempty"`
	ListPrice          Currency `json:"ListPrice,omitempty"`

	// You can manually specify these
	// Data Fields
	AvailableQuantity    float64     `json:"AvailableQuantity,omitempty"`
	EndDate              string      `json:"EndDate,omitempty"`
	Description          string      `json:"Description,omitempty"`
	Order                *ForeignKey `json:"Order,omitempty"`
	OriginalOrderProduct *ForeignKey `json:"OriginalOrderItem,omitempty"`
	PricebookEntry       *ForeignKey `json:"PricebookEntry,omitempty"`
	PricebookEntryId     string      `json:"PricebookEntryId,omitempty"`
	Quantity             float64     `json:"Quantity,omitempty"`
	StartDate            string      `json:"ServiceDate,omitempty"`
	TotalPrice           Currency    `json:"TotalPrice,omitempty"`
	UnitPrice            Currency    `json:"UnitPrice,omitempty"`

	// Private data
	variant *variant.Variant
}

func (o *OrderProduct) Read(so SObjectCompatible) error {
	o.Ref = so

	li, ok := so.(*lineitem.LineItem)
	if !ok {
		return ErrorOrderTypeRequired
	}

	o.Quantity = float64(li.Quantity)
	o.PricebookEntry = &ForeignKey{CrowdstartIdC: li.Variant.Id()}
	o.UnitPrice = ToCurrency(int64(li.Variant.Price))

	return nil
}

func (o *OrderProduct) Write(so SObjectCompatible) error {
	o.Ref = so

	li, ok := so.(*lineitem.LineItem)
	if !ok {
		return ErrorOrderTypeRequired
	}

	o.SetSalesforceId(o.Id)

	li.Quantity = int(o.Quantity)
	// li.SKU_ = o.variant.SKU

	return nil
}

func (o *OrderProduct) SetExternalId(id string) {
	o.CrowdstartIdC = id
}

func (o *OrderProduct) ExternalId() string {
	return o.CrowdstartIdC
}

func (o *OrderProduct) Push(api SalesforceClient) error {
	del(api, OrderProductPath, o.ExternalId())
	return push(api, OrderProductExternalIdPath, o)
}

func (o *OrderProduct) PullExternalId(api SalesforceClient, id string) error {
	return pull(api, OrderProductExternalIdPath, id, o)
}

func (o *OrderProduct) PullId(api SalesforceClient, id string) error {
	return pull(api, OrderProductPath, id, o)
}

type Product struct {
	ModelReference `json:"-"` // Struct this sobject refers to

	// Don't manually specify these

	// Response Only Fields
	Attributes     *Attribute `json:"attributes,omitempty"`
	Id             string     `json:"Id,omitempty"`
	IsDeleted      bool       `json:"IsDeleted,omitempty"`
	MasterRecordId string     `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using Id (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__c,omitempty"`

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
}

func (p *Product) Read(so SObjectCompatible) error {
	p.Ref = so

	v, ok := so.(*variant.Variant)
	if !ok {
		return ErrorUserTypeRequired
	}

	p.CrowdstartIdC = v.Id()
	p.Name = v.SKU
	p.ProductCode = v.SKU
	p.IsActive = true

	return nil
}

func (p *Product) Write(so SObjectCompatible) error {
	p.Ref = so

	v, ok := so.(*variant.Variant)
	if !ok {
		return ErrorUserTypeRequired
	}

	p.SetSalesforceId(p.Id)

	// v.Id = p.CrowdstartIdC
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
	Attributes     *Attribute `json:"attributes,omitempty"`
	Id             string     `json:"Id,omitempty"`
	IsDeleted      bool       `json:"IsDeleted,omitempty"`
	MasterRecordId string     `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using Id (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__c,omitempty"`

	// Read Only
	CreatedById      string `json:"CreatedById,omitempty"`
	LastModifiedById string `json:"LastModifiedById,omitempty"`
	Name             string `json:"Name,omitempty"`
	ProductCode      string `json:"ProductCode,omitempty"`

	// You can manually specify these
	// Data Fields
	CurrencyIsoCode  string      `json:"CurrencyIsoCode,omitempty"`
	PricebookId      string      `json:"Pricebook2Id,omitempty"`
	Product          *ForeignKey `json:"Product2,omitempty"`
	UnitPrice        Currency    `json:"UnitPrice,omitempty`
	UseStandardPrice bool        `json:"UseStandardPrice,omitempty"`
	IsActive         bool        `json:"IsActive,omitempty"`
}

func (p *PricebookEntry) Read(so SObjectCompatible) error {
	p.Ref = so

	v, ok := so.(*variant.Variant)
	if !ok {
		return ErrorUserTypeRequired
	}

	p.CrowdstartIdC = v.Id()
	p.Product = &ForeignKey{CrowdstartIdC: v.Id()}
	p.UseStandardPrice = false
	p.UnitPrice = ToCurrency(int64(v.Price))
	p.IsActive = true

	return nil
}

func (p *PricebookEntry) Write(so SObjectCompatible) error {
	p.Ref = so

	// v, ok := so.(*variant.Variant)
	// if !ok {
	// 	return ErrorUserTypeRequired
	// }

	p.SetSalesforceId(p.Id)

	// v.Id = p.CrowdstartIdC
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
func push(api SalesforceClient, p string, s SObjectSyncable) error {
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
	body := api.GetBody()
	status := api.GetStatusCode()

	log.Debug("Receiving Json: %v", string(body), api.GetContext())
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

func pull(api SalesforceClient, path, id string, s interface{}) error {
	p := fmt.Sprintf(path, id)
	if err := api.Request("GET", p, "", nil, true); err != nil {
		return err
	}

	body := api.GetBody()

	log.Debug("Receiving Json: %v", string(body), api.GetContext())
	err := json.Unmarshal(body, s)

	return err
}

func getUpdated(api SalesforceClient, p string, start, end time.Time, response *UpdatedRecordsResponse) error {
	path := fmt.Sprintf(p, start.Format(time.RFC3339), end.Format(time.RFC3339))

	if err := api.Request("GET", path, "", nil, true); err != nil {
		return err
	}

	body := api.GetBody()
	if err := json.Unmarshal(body, response); err != nil {
		return err
	}

	log.Debug("Receiving Json: %v", string(body), api.GetContext())
	return nil
}

func GetUpdatedContacts(api *Api, start, end time.Time, response *UpdatedRecordsResponse) error {
	return getUpdated(api, ContactsUpdatedPath, start, end, response)
}

func GetUpdatedAccounts(api *Api, start, end time.Time, response *UpdatedRecordsResponse) error {
	return getUpdated(api, AccountsUpdatedPath, start, end, response)
}

func GetUpdatedOrders(api *Api, start, end time.Time, response *UpdatedRecordsResponse) error {
	return getUpdated(api, OrdersUpdatedPath, start, end, response)
}
