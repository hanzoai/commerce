package salesforce

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"appengine/datastore"

	"crowdstart.io/models"
)

//SObject Definitions
type Contact struct {
	// Don't manually specify these

	// Response Only Fields
	Attributes     Attribute `json:"attributes,omitempty"`
	Id             string    `json:"Id,omitempty"`
	IsDeleted      bool      `json:"IsDeleted,omitempty"`
	MasterRecordId string    `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using email (max length 255)
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

func (c *Contact) FromUser(u *models.User) {
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
}

func (c *Contact) ToUser(u *models.User) {
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
}

func (c *Contact) Push(api SalesforceClient, u *models.User) error {
	c.FromUser(u)

	bytes, err := json.Marshal(c)
	if err != nil {
		return err
	}

	path := fmt.Sprintf(ContactExternalIdPath, strings.Replace(u.Id, ".", "_", -1))
	data := string(bytes[:])

	if err = api.Request("PATCH", path, data, &map[string]string{"Content-Type": "application/json"}, true); err != nil {
		return err
	}

	return nil
}

func (c *Contact) pull(api SalesforceClient, path string) error {
	if err := api.Request("GET", path, "", nil, true); err != nil {
		return err
	}

	return json.Unmarshal(api.GetBody(), c)
}

func (c *Contact) PullExternalId(api SalesforceClient, id string) error {
	path := fmt.Sprintf(ContactExternalIdPath, id)
	return c.pull(api, path)
}

func (c *Contact) PullId(api SalesforceClient, id string) error {
	path := fmt.Sprintf(ContactPath, id)
	return c.pull(api, path)
}

type Account struct {
	// Don't manually specify these

	// Response Only Fields
	Attributes     Attribute `json:"attributes,omitempty"`
	Id             string    `json:"Id,omitempty"`
	IsDeleted      bool      `json:"IsDeleted,omitempty"`
	MasterRecordId string    `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using email (max length 255)
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

func (a *Account) FromUser(u *models.User) {
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
}

func (a *Account) ToUser(u *models.User) {
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
}

func (a *Account) Push(api SalesforceClient, u *models.User) error {
	a.FromUser(u)

	bytes, err := json.Marshal(a)
	if err != nil {
		return err
	}

	path := fmt.Sprintf(AccountExternalIdPath, strings.Replace(u.Id, ".", "_", -1))
	data := string(bytes[:])

	if err = api.Request("PATCH", path, data, &map[string]string{"Content-Type": "application/json"}, true); err != nil {
		return err
	}

	return nil
}

func (a *Account) pull(api SalesforceClient, path string) error {
	if err := api.Request("GET", path, "", nil, true); err != nil {
		return err
	}

	return json.Unmarshal(api.GetBody(), a)
}

func (a *Account) PullExternalId(api SalesforceClient, id string) error {
	path := fmt.Sprintf(AccountExternalIdPath, id)
	return a.pull(api, path)
}

func (a *Account) PullId(api SalesforceClient, id string) error {
	path := fmt.Sprintf(AccountPath, id)
	return a.pull(api, path)
}

type Order struct {
	// Don't manually specify these

	// Response Only Fields
	Attributes     Attribute `json:"attributes,omitempty"`
	Id             string    `json:"Id,omitempty"`
	IsDeleted      bool      `json:"IsDeleted,omitempty"`
	MasterRecordId string    `json:"MasterRecordId,omitempty"`

	// Unique External Id, currently using email (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__C,omitempty"`

	// Read Only
	CreatedById      string `json:"CreatedById,omitempty"`
	LastModifiedById string `json:"LastModifiedById,omitempty"`
	AccountId        string `json:"AccountId,omitempty"`

	// You can manually specify these
	// Data Fields
	Account                Account `json:"Account,omitempty"`
	Pricebook2Id           string  `json:"Pricebook2Id,omitempty"`
	OriginalOrderId        string  `json:"OriginalOrderId,omitempty"`
	EffectiveDate          string  `json:"EffectiveDate,omitempty"`
	EndDate                string  `json:"EndDate,omitempty"`
	IsReductionOrder       string  `json:"IsReductionOrder,omitempty"`
	Status                 string  `json:"Status,omitempty"`
	Description            string  `json:"Description,omitempty"`
	CustomerAuthorizedById string  `json:"CustomerAuthorizedById,omitempty"`
	CustomerAuthorizedDate string  `json:"CustomerAuthorizedDate,omitempty"`
	CompanyAuthorizedById  string  `json:"CompanyAuthorizedById,omitempty"`
	CompanyAuthorizedDate  string  `json:"CompanyAuthorizedDate,omitempty"`
	Type                   string  `json:"Type,omitempty"`
	BillingStreet          string  `json:"BillingStreet,omitempty"`
	BillingCity            string  `json:"BillingCity,omitempty"`
	BillingState           string  `json:"BillingState,omitempty"`
	BillingPostalCode      string  `json:"BillingPostalCode,omitempty"`
	BillingCountry         string  `json:"BillingCountry,omitempty"`
	BillingLatitude        string  `json:"BillingLatitude,omitempty"`
	BillingLongitude       string  `json:"BillingLongitude,omitempty"`
	ShippingStreet         string  `json:"ShippingStreet,omitempty"`
	ShippingCity           string  `json:"ShippingCity,omitempty"`
	ShippingState          string  `json:"ShippingState,omitempty"`
	ShippingPostalCode     string  `json:"ShippingPostalCode,omitempty"`
	ShippingCountry        string  `json:"ShippingCountry,omitempty"`
	ShippingLatitude       string  `json:"ShippingLatitude,omitempty"`
	ShippingLongitude      string  `json:"ShippingLongitude,omitempty"`
	Name                   string  `json:"Name,omitempty"`
	PoDate                 string  `json:"PoDate,omitempty"`
	PoNumber               string  `json:"PoNumber,omitempty"`
	OrderReferenceNumber   string  `json:"OrderReferenceNumber,omitempty"`
	BillToContactId        string  `json:"BillToContactId,omitempty"`
	ShipToContactId        string  `json:"ShipToContactId,omitempty"`
	ActivatedDate          string  `json:"ActivatedDate,omitempty"`
	ActivatedById          string  `json:"ActivatedById,omitempty"`
	StatusCode             string  `json:"StatusCode,omitempty"`
	OrderNumber            string  `json:"OrderNumber,omitempty"`
	TotalAmount            string  `json:"TotalAmount,omitempty"`
	CreatedDate            string  `json:"CreatedDate,omitempty"`
	SystemModstamp         string  `json:"SystemModstamp,omitempty"`
	LastViewedDate         string  `json:"LastViewedDate,omitempty"`
	LastReferencedDate     string  `json:"LastReferencedDate,omitempty"`
	Order                  string  `json:"Order,omitempty"`
	Master                 string  `json:"Master,omitempty"`

	// Custom Crowdstart fields
	Cancelled   bool   `json:"Cancelled__c,omitempty"`
	Disputed    bool   `json:"Disputed__c,omitempty"`
	Locked      bool   `json:"Locked__c,omitempty"`
	PaymentId   string `json:"PaymentId__c,omitempty"`
	PaymentType string `json:"PaymentType__c,omitempty"`
	Preorder    bool   `json:"Preorder__c,omitempty"`
	Refunded    bool   `json:"Refunded__c,omitempty"`
	Shipped     bool   `json:"Shipped__c,omitempty"`
	Shipping    string `json:"Shipping__c,omitempty"`
	Subtotal    string `json:"Subtotal__c,omitempty"`
	Tax         string `json:"Tax__c,omitempty"`
	Unconfirmed bool   `json:"Unconfirmed__c"`

	// We don't use contracts
	ContractId string `json:"ContractId,omitempty"`
}

func (o *Order) FromOrder(order *models.Order) {
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

	o.Status = "Draft" // SF Required

	// Payment Information
	o.Shipping = fmt.Sprintf("%.2f", float64(order.Shipping)/1000.0)
	o.Subtotal = fmt.Sprintf("%.2f", float64(order.Subtotal)/1000.0)
	o.Tax = fmt.Sprintf("%.2f", float64(order.Tax)/1000.0)

	if len(order.Charges) > 0 {
		o.PaymentType = "Stripe"
		o.PaymentId = order.Charges[0].ID
	}

	// Status Flags
	o.Cancelled = order.Cancelled
	o.Disputed = order.Disputed
	o.Locked = order.Locked
	o.Preorder = order.Preorder
	o.Refunded = order.Refunded
	o.Shipped = order.Shipped
	o.Unconfirmed = order.Unconfirmed

	//SKU
	desc := ""
	for _, i := range order.Items {
		desc += i.SKU_ + "," + strconv.Itoa(i.Quantity) + "\n"
	}

	o.Description = desc
	if name, err := datastore.DecodeKey(order.Id); err == nil {
		o.Name = strconv.FormatInt(name.IntID(), 10)
	}

	o.Account.CrowdstartIdC = order.UserId
}

func (o *Order) Push(api SalesforceClient, or *models.Order) error {
	o.FromOrder(or)

	bytes, err := json.Marshal(o)
	if err != nil {
		return err
	}

	path := fmt.Sprintf(OrderExternalIdPath, strings.Replace(o.Id, ".", "_", -1))
	data := string(bytes[:])

	if err = api.Request("PATCH", path, data, &map[string]string{"Content-Type": "application/json"}, true); err != nil {
		return err
	}

	return nil
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
