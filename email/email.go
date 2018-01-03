package email

type Email struct {
	Html string `json:"html"`
	// Text      string      `json:"text"`
	Subject   string `json:"subject"`
	FromEmail string `json:"from_email"`
	FromName  string `json:"from_name"`
	// To        []Recipient `json:"to"`

	Headers struct {
		ReplyTo string `json:"Reply-To"`
	} `json:"headers"`

	// Important bool `json:"important"`
	// TrackOpens         interface{} `json:"track_opens"`
	// TrackClicks        interface{} `json:"track_clicks"`
	// AutoText           interface{} `json:"auto_text"`
	AutoHtml interface{} `json:"auto_html"`
	// InlineCss          interface{} `json:"inline_css"`
	// UrlStripQs         interface{} `json:"url_strip_qs"`
	// PreserveRecipients interface{} `json:"preserve_recipients"`
	// ViewContentLink    interface{} `json:"view_content_link"`
	// BccAddress string `json:"bcc_address"`
	// TrackingDomain     interface{} `json:"tracking_domain"`
	// SigningDomain    interface{} `json:"signing_domain"`
	// ReturnPathDomain interface{} `json:"return_path_domain"`

	Merge         bool   `json:"merge"`
	MergeLanguage string `json:"merge_language,omitempty"`
	// MergeVars     []Var           `json:"global_merge_vars"`
	// RcptMergeVars []RcptMergeVars `json:"merge_vars"`

	// Tags []string `json:"tags"`
	// Subaccount      string            `json:"subaccount"`
	// GoogleAnalyticsDomains  []string `json:"google_analytics_domains"`
	// GoogleAnalyticsCampaign string   `json:"google_analytics_campaign"`

	// Metadata struct {
	//	Website string `json:"website"`
	// } `json:"metadata"`

	// RecipientMetadata []struct {
	// 	Rcpt   string `json:"rcpt"`
	// 	Values struct {
	// 		UserId int `json:"user_id"`
	// 	} `json:"values"`
	// } `json:"recipient_metadata"`

	// Attachments []struct {
	// 	Type    string `json:"type"`
	// 	Name    string `json:"name"`
	// 	Content string `json:"content"`
	// } `json:"attachments"`

	// Images []struct {
	// 	Type    string `json:"type"`
	// 	Name    string `json:"name"`
	// 	Content string `json:"content"`
	// } `json:"images"`
}
