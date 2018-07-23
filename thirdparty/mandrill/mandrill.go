package mandrill

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"google.golang.org/appengine/urlfetch"

	"hanzo.io/config"
	iface "hanzo.io/iface/email"
	"hanzo.io/log"
	"hanzo.io/types/email"
	"hanzo.io/types/integration"
	"hanzo.io/util/json"
)

const root = "http://mandrillapp.com/api/1.0"

func init() {
	gob.Register(Var{})
}

type Var struct {
	Name    string      `json:"name"`
	Content interface{} `json:"content"`
}

type RcptMergeVars struct {
	Rcpt string `json:"rcpt"`
	Vars []Var  `json:"vars"`
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

		Merge         bool            `json:"merge"`
		MergeLanguage string          `json:"merge_language,omitempty"`
		MergeVars     []Var           `json:"global_merge_vars"`
		RcptMergeVars []RcptMergeVars `json:"merge_vars"`

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

func (r *SendReq) AddRecipient(email, name string) {
	rcpt := Recipient{
		Email: email,
		Name:  name,
		Type:  "to",
	}

	r.Message.To = append(r.Message.To, rcpt)
}

func (r *SendReq) AddMergeVar(v Var) {
	r.Message.MergeVars = append(r.Message.MergeVars, v)
}

type SendTemplateReq struct {
	SendReq
	TemplateName    string `json:"template_name"`
	TemplateContent []struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	} `json:"template_content"`
}

func NewSendReq() (req SendReq) {
	req.Async = true
	req.IpPool = "Main Pool"
	req.Key = config.Mandrill.APIKey
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
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return ""
	}

	return string(bytes)
}

// PingMandrill checks if our credentials/url are okay
// Returns true if Mandrill replies with  a 200 OK
func Ping(ctx context.Context) bool {
	url := root + "/users/ping.json"

	str := fmt.Sprintf(`{"key": "%s"}`, config.Mandrill.APIKey)
	body := []byte(str)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return false
	}

	client := urlfetch.Client(ctx)
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()

	return res.StatusCode == 200
}

func SendTemplate(ctx context.Context, req *SendTemplateReq) error {
	// Convert the map of vars to a byte buffer of a json string
	url := root + "/messages/send-template.json"

	j := json.Encode(req)

	hreq, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(j)))
	if err != nil {
		return err
	}

	// Set timeout
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	client := urlfetch.Client(ctx)
	client.Transport = &urlfetch.Transport{Context: ctx}

	res, err := client.Do(hreq)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		return nil
	}

	// Failed to send
	b, _ := ioutil.ReadAll(res.Body)
	return errors.New(fmt.Sprintf("Invalid response from Mandrill: %s", b))
}

func Send(ctx context.Context, req *SendReq) error {
	// Convert the map of vars to a byte buffer of a json string
	url := root + "/messages/send.json"

	j := json.Encode(req)

	hreq, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(j)))
	if err != nil {
		return err
	}

	client := urlfetch.Client(ctx)
	res, err := client.Do(hreq)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		return nil
	}

	// Failed to send
	b, _ := ioutil.ReadAll(res.Body)
	return errors.New(fmt.Sprintf("Invalid response from Mandrill: %s", b))
}

// TODO: Update Mandrill
type placeholder struct{}

func (p *placeholder) Send(message *email.Message) error {
	return errors.New("Send is not implemented")
}

func New(c context.Context, in integration.Mandrill) iface.Provider {
	return new(placeholder)
}
