package salesforce

//SObject Definitions
type Contact struct {
	// Don't manually specify these

	// Response Only Fields
	Attributes     Attribute `json:"attributes,omitempty"`
	Id             string    `json:"Id,omitempty"`
	IsDeleted      bool      `json:"IsDeleted,omitempty"`
	MasterRecordId string    `json:"MasterRecordId,omitempty"`
	AccountId      string    `json:"AccountId,omitempty"`

	// Unique External Id, currently using email (max length 255)
	CrowdstartIdC string `json:"CrowdstartId__C,omitempty"`

	// Read Only
	Name             string `json:"Name,omitempty"`
	CreatedById      string `json:"CreatedById,omitempty"`
	LastModifiedById string `json:"LastModifiedById,omitempty"`

	// You can manually specify these

	// Data Fields
	LastName           string `json:"LastName,omitempty"`
	FirstName          string `json:"FirstName,omitempty"`
	Salutation         string `json:"Salutation,omitempty"`
	MailingStreet      string `json:"MailingStreet,omitempty"`
	MailingCity        string `json:"MailingCity,omitempty"`
	MailingState       string `json:"MailingState,omitempty"`
	MailingPostalCode  string `json:"MailingPostalCode,omitempty"`
	MailingCountry     string `json:"MailingCountry,omitempty"`
	MailingStateCode   string `json:"MailingStateCode,omitempty"`
	MailingCountryCode string `json:"MailingCountryCode,omitempty"`
	MailingLatitude    string `json:"MailingLatitude,omitempty"`
	MailingLongitude   string `json:"MailingLongitude,omitempty"`
	Phone              string `json:"Phone,omitempty"`
	Fax                string `json:"Fax,omitempty"`
	MobilePhone        string `json:"MobilePhone,omitempty"`
	ReportsToId        string `json:"ReportsToId,omitempty"`
	Email              string `json:"Email,omitempty"`
	Title              string `json:"Title,omitempty"`
	Department         string `json:"Department,omitempty"`
	OwnerId            string `json:"OwnerId,omitempty"`
	CreatedDate        string `json:"CreatedDate,omitempty"`
	LastModifiedDate   string `json:"LastModifiedDate,omitempty"`
	SystemModstamp     string `json:"SystemModstamp,omitempty"`
	LastActivityDate   string `json:"LastActivityDate,omitempty"`
	LastCURequestDate  string `json:"LastCURequestDate,omitempty"`
	LastCUUpdateDate   string `json:"LastCUUpdateDate,omitempty"`
	LastViewedDate     string `json:"LastViewedDate,omitempty"`
	LastReferencedDate string `json:"LastReferencedDate,omitempty"`
	EmailBouncedReason string `json:"EmailBouncedReason,omitempty"`
	EmailBouncedDate   string `json:"EmailBouncedDate,omitempty"`
	IsEmailBounced     bool   `json:"IsEmailBounced,omitempty"`
	JigsawContactId    string `json:"JigsawContactId,omitempty"`

	// Skully Custom fields
	ZendeskLastSyncDateC         string `json:"Zendesk__Last_Sync_Date__c,omitempty"`
	ZendeskLastSyncStatusC       string `json:"Zendesk__Last_Sync_Status__c,omitempty"`
	ZendeskResultC               string `json:"Zendesk__Result__c,omitempty"`
	ZendeskTagsC                 string `json:"Zendesk__Tags__c,omitempty"`
	ZendeskZendeskOutofSyncC     string `json:"Zendesk__Zendesk_OutofSync__c,omitempty"`
	ZendeskZendeskOldTagsC       string `json:"Zendesk__Zendesk_oldTags__c,omitempty"`
	ZendeskIsCreatedUpdatedFlagC string `json:"Zendesk__isCreatedUpdatedFlag__c,omitempty"`
	ZendeskNotesC                string `json:"Zendesk__notes__c,omitempty"`
	ZendeskZendeskIdC            string `json:"Zendesk__zendesk_id__c,omitempty"`
	UniquePreorderLinkC          string `json:"Unique_Preorder_Link__c,omitempty"`
	FullfillmentStatusC          string `json:"Fulfillment_Status__c,omitempty"`
	PreorderC                    string `json:"Preorder__c,omitempty"`
	ShippingAddressC             string `json:"Shipping_Address__c,omitempty"`
	ShippingCityC                string `json:"Shipping_City__c,omitempty"`
	ShippingStateC               string `json:"Shipping_State__c,omitempty"`
	ShippingPostalZipC           string `json:"Shipping_Postal_Zip__c,omitempty"`
	ShippingCountryC             string `json:"Shipping_Country__c,omitempty"`
	MC4SFMCSubscriberC           string `json:"MC4SF__MC_Subscriber__c,omitempty"`
}
