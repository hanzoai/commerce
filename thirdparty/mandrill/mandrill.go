package mandrill

import (
	"context"
	"time"

	"github.com/keighl/mandrill"
	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"hanzo.io/log"
	"hanzo.io/types/email"
	"hanzo.io/types/integration"
)

type Client struct {
	client  *mandrill.Client
	context context.Context
}

// Convert our message to mandrill message type
func newMessage(message *email.Message) *mandrill.Message {
	m := new(mandrill.Message)

	m.FromEmail = message.From.Address
	m.FromName = message.From.Name
	m.Subject = message.Subject

	// Add recipients
	for _, to := range message.To {
		m.AddRecipient(to.Address, to.Name, "to")
	}
	for _, cc := range message.CC {
		m.AddRecipient(cc.Address, cc.Name, "cc")
	}
	for _, bcc := range message.BCC {
		m.AddRecipient(bcc.Address, bcc.Name, "bcc")
	}

	// Add global mail merge variables
	m.GlobalMergeVars = mandrill.MapToVars(message.Substitutions)

	// Add recpient unique merge variables
	mergeVars := make([]*mandrill.RcptMergeVars, 0)

	for k, v := range message.Personalizations {
		vars := mandrill.MapToRecipientVars(k, v.Substitutions)
		mergeVars = append(mergeVars, vars)
	}

	m.MergeVars = mergeVars

	// Add content
	if message.HTML != "" {
		m.HTML = message.HTML
	}

	if message.Text != "" {
		m.Text = message.Text
	}
	return m
}

// Send email
func (c *Client) Send(message *email.Message) error {
	var (
		res []*mandrill.Response
		err error
	)

	if message.TemplateID != "" {
		res, err = c.client.MessagesSend(newMessage(message))
	} else {
		res, err = c.client.MessagesSend(newMessage(message))
	}

	if err != nil {
		log.Error(err)
		return err
	}

	log.Info("%v", res)

	return nil
}

func New(c context.Context, in integration.Mandrill) *Client {
	// Set deadline
	c, _ = context.WithTimeout(c, time.Second*55)

	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(c)

	httpClient.Transport = &urlfetch.Transport{
		Context: c,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}

	client := mandrill.ClientWithKey(in.APIKey)
	client.HTTPClient = httpClient
	return &Client{
		client:  client,
		context: c,
	}
}
