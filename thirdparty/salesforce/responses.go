package salesforce

import "fmt"

// Api Data Container
type SalesforceTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	InstanceUrl  string `json:"instance_url"`
	Id           string `json:"id"`
	IssuedAt     string `json:"issued_at"`
	Signature    string `json:"signature"`

	ErrorDescription string `json:"error_description"`
	Error            string `json:"error"`
}

// Salesforce Structs
// These represent non-sobject responses received from salesforce
type ErrorFromSalesforce struct {
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}

func (e *ErrorFromSalesforce) Error() string {
	return fmt.Sprintf("%v: %v", e.ErrorCode, e.Message)
}

type DescribeResponse struct {
	SObjects     string `json:"sobjects"`
	Connect      string `json:"connect"`
	Query        string `json:"query"`
	Theme        string `json:"theme"`
	QueryAll     string `json:"queryAll"`
	Tooling      string `json:"tooling"`
	Chatter      string `json:"chatter"`
	Analytics    string `json:"analytics"`
	Recent       string `json:"recent"`
	Commerce     string `json:"commerce"`
	Licensing    string `json:"licensing"`
	Identity     string `json:"identity"`
	FlexiPage    string `json:"flexiPage"`
	Search       string `json:"search"`
	QuickActions string `json:"quickActions"`
	AppMenu      string `json:"appMenu"`
}

type SObjectUrls struct {
	SObjectMetadata string `json:"sobject"`
	Describe        string `json:"describe"`
	RowTemplate     string `json:"rowTemplate"`
}

type SObjectMetaData struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	KeyPrefix   string `json:"keyPrefix"`
	LabelPlural string `json:"labelPlural"`

	Urls SObjectUrls `json:"urls"`

	// grammatically annoying bools
	Custom              bool `json:"custom"`
	Layoutable          bool `json:"layoutable"`
	Activateable        bool `json:"activateable"`
	Searchable          bool `json:"searchable"`
	Updateable          bool `json:"updateable"`
	Createable          bool `json:"createable"`
	DeprecatedAndHidden bool `json:"deprecatedAndHidden"`
	CustomSetting       bool `json:"customSetting"`
	Deletable           bool `json:"deletable"`
	FeedEnable          bool `json:"feedEnabled"`
	Mergeable           bool `json:"mergeable"`
	Queryable           bool `json:"queryable"`
	Replicateable       bool `json:"replicateable"`
	Retrieveable        bool `json:"retrieveable"`
	Undeleteable        bool `json:"undeleteable"`
	Triggerable         bool `json:"triggerable"`
}

type SObjectDescribeResponse struct {
	Encoding     string    `json:"encoding"`
	MaxBatchSize string    `json:"maxBatchSize"`
	SObjects     []SObject `json:"sobjects"`
}

type Attribute struct {
	Type string `json:"type"`
	Url  string `json:"url"`
}

type QueryAttributes struct {
	Id         string    `json:"Id"`
	Attributes Attribute `json:"attributes"`
}

type QueryResponse struct {
	TotalSize int               `json:"totalSize"`
	Done      bool              `json:"done"`
	Records   []QueryAttributes `json:"attributes"`
}

type UpsertResponse struct {
	Id      string                `json:"id"`
	Success bool                  `json:"success"`
	Errors  []ErrorFromSalesforce `json:"errors"`
}

type UpdatedRecordsResponse struct {
	Ids               []string `json:"ids"`
	LatestDateCovered string   `json:"latestDateCovered"`
}
