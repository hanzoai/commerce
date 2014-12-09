package mandrill

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"appengine"
	"appengine/delay"
	"appengine/urlfetch"

	"crowdstart.io/config"
	"crowdstart.io/util/json"
	"crowdstart.io/util/log"
)

const root = "http://mandrillapp.com/api/1.0"

type GlobalMergeVars struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type MergeVars struct {
	Rcpt string `json:"rcpt"`
	Vars []struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	} `json:"vars"`
}

type Recipient struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

type SendReq struct {
	Key     string `json:"key"`
	Message struct {
		Html string `json:"html"`
		// Text      string      `json:"text"`
		Subject   string      `json:"subject"`
		FromEmail string      `json:"from_email"`
		FromName  string      `json:"from_name"`
		To        []Recipient `json:"to"`
		// Headers   struct {
		// 	ReplyTo string `json:"Reply-To"`
		// } `json:"headers"`
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
		MergeLanguage string `json:"merge_language"`
		// GlobalMergeVars []GlobalMergeVars `json:"global_merge_vars"`
		// MergeVars       []MergeVars       `json:"merge_vars"`
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
	} `json:"message"`
	Async  bool   `json:"async"`
	IpPool string `json:"ip_pool"`
	// SendAt string `json:"send_at"`
}

type SendTemplateReq struct {
	SendReq
	TemplateName    string `json:"template_name"`
	TemplateContent []struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	} `json:"template_content"`
}

func (r *SendReq) AddRecipient(email, name string) {
	recp := Recipient{
		Email: email,
		Name:  name,
		Type:  "to",
	}

	r.Message.To = append(r.Message.To, recp)
}

func NewSendReq() (req SendReq) {
	req.Async = true
	req.IpPool = "Main Pool"
	req.Key = config.Mandrill.APIKey
	req.Message.MergeLanguage = "mailchimp"
	req.Message.AutoHtml = true
	req.Message.Merge = true
	return req
}

func NewSendTemplateReq() (req SendTemplateReq) {
	req.SendReq = NewSendReq()
	return req
}

// Gets the content of a template
func GetTemplate(filename string) string {
	wd, _ := os.Getwd()
	log.Info(wd)
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Panic(err.Error())
		return ""
	}

	return string(b)
}

// PingMandrill checks if our credentials/url are okay
// Returns true if Mandrill replies with  a 200 OK
func Ping(ctx appengine.Context) bool {
	url := root + "/users/ping.json"
	log.Debug(url)

	str := fmt.Sprintf(`{"key": "%s"}`, config.Mandrill.APIKey)
	log.Debug(str)
	body := []byte(str)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		log.Panic(err.Error())
		return false
	}

	client := urlfetch.Client(ctx)
	res, err := client.Do(req)
	defer res.Body.Close()
	if err != nil {
		log.Panic(err.Error())
		return false
	}

	log.Debug(res.Status)
	return res.StatusCode == 200
}

func SendTemplate(ctx appengine.Context, req *SendTemplateReq) error {
	// Convert the map of vars to a byte buffer of a json string
	url := root + "/messages/send-template.json"
	log.Debug(url)

	j := json.Encode(req)
	log.Debug(j)

	hreq, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(j)))
	if err != nil {
		log.Panic(err.Error())
		return err
	}

	client := urlfetch.Client(ctx)
	res, err := client.Do(hreq)
	defer res.Body.Close()
	if err != nil {
		log.Panic(err.Error())
		return err
	}

	b, _ := ioutil.ReadAll(res.Body)
	log.Debug(string(b))
	log.Debug(config.Mandrill.APIKey)

	if res.StatusCode == 200 {
		return nil
	}
	return errors.New("Email not sent")
}

func Send(ctx appengine.Context, req *SendReq) error {
	// Convert the map of vars to a byte buffer of a json string
	url := root + "/messages/send.json"
	log.Debug(url)

	j := json.Encode(req)
	log.Debug(j)

	hreq, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(j)))
	if err != nil {
		log.Panic(err.Error())
		return err
	}

	client := urlfetch.Client(ctx)
	res, err := client.Do(hreq)
	defer res.Body.Close()
	if err != nil {
		log.Panic(err.Error())
		return err
	}

	b, _ := ioutil.ReadAll(res.Body)
	log.Debug(string(b))
	log.Debug(config.Mandrill.APIKey)

	if res.StatusCode == 200 {
		return nil
	}
	return errors.New("Email not sent")
}

var SendTemplateAsync = delay.Func("send-template-email", func(ctx appengine.Context, templateName, toEmail, toName, subject string) {
	req := NewSendTemplateReq()
	req.AddRecipient(toEmail, toName)

	req.Message.FromEmail = config.Mandrill.FromEmail
	req.Message.FromName = config.Mandrill.FromName
	req.Message.Subject = subject
	req.TemplateName = templateName

	// Send template
	if err := SendTemplate(ctx, &req); err != nil {
		log.Error(err)
	}
})
