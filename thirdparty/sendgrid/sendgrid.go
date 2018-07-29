package sendgrid

import (
	"context"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"

	"github.com/hanzoai/sendgrid-go"
	"github.com/hanzoai/sendgrid-go/helpers/mail"
	"github.com/sendgrid/rest"

	"hanzo.io/log"
	"hanzo.io/types/email"
	"hanzo.io/types/integration"
)

type Client struct {
	ctx    context.Context
	client *sendgrid.Client
}

func newContent(contentType string, value string) *mail.Content {
	return &mail.Content{
		Type:  contentType,
		Value: value,
	}
}

func newEmail(email email.Email) *mail.Email {
	return &mail.Email{
		Name:    email.Name,
		Address: email.Address,
	}
}

func addSubsitutions(p *mail.Personalization, message *email.Message, address string) {
	if v, ok := message.Personalizations[address]; ok {
		for k, v := range v.Substitutions {
			p.SetSubstitution("-"+k+"-", v)
		}
	}
}

// Convert from our message type to sendgrid type
func newMessage(message *email.Message) *mail.SGMailV3 {
	m := new(mail.SGMailV3)

	// Set From
	from := newEmail(message.From)
	m.SetFrom(from)

	// Set subject
	m.Subject = message.Subject

	// Add recipients + personalizations
	for _, to := range message.To {
		p := mail.NewPersonalization()
		p.AddTos(newEmail(to))
		p.DynamicTemplateData = message.TemplateData
		addSubsitutions(p, message, to.Address)
		m.AddPersonalizations(p)
	}

	for _, cc := range message.CC {
		p := mail.NewPersonalization()
		p.AddCCs(newEmail(cc))
		p.DynamicTemplateData = message.TemplateData
		addSubsitutions(p, message, cc.Address)
		m.AddPersonalizations(p)
	}

	for _, bcc := range message.BCC {
		p := mail.NewPersonalization()
		p.AddBCCs(newEmail(bcc))
		p.DynamicTemplateData = message.TemplateData
		addSubsitutions(p, message, bcc.Address)
		m.AddPersonalizations(p)
	}

	// Add section substitutions
	for k, v := range message.Substitutions {
		m.AddSection("-"+k+"-", v)
	}

	// Set tracking
	ts := mail.NewTrackingSettings()

	ct := mail.NewClickTrackingSetting()
	ct.SetEnable(message.Tracking.Clicks)
	ts.SetClickTracking(ct)

	ot := mail.NewOpenTrackingSetting()
	ot.SetEnable(message.Tracking.Opens)
	ts.SetOpenTracking(ot)

	m.SetTrackingSettings(ts)

	// Add content
	if message.Text != "" {
		m.AddContent(newContent("text/plain", message.Text))

	}

	if message.HTML != "" {
		m.AddContent(newContent("text/html", message.HTML))
	}

	// Use template if set
	if message.TemplateID != "" {
		m.SetTemplateID(message.TemplateID)
	}

	return m
}

// Send email
func (c *Client) Send(message *email.Message) error {
	res, err := c.client.Send(newMessage(message))
	if err != nil {
		log.Error("SendGrid Could Not Send", err, c.ctx)
		return err
	}
	log.Info(res.StatusCode, c.ctx)
	log.Info(res.Body, c.ctx)
	log.Info(res.Headers, c.ctx)
	return nil
}

// func (c *Client) SendCampaign(id string) {

// }

// func (c *Client) SendTemplate(id string) {

// }

// func (c *Client) TemplateGet(id string)               {}
// func (c *Client) TemplateCreate(email email.Template) {}
// func (c *Client) TemplateUpdate(email email.Template) {}
// func (c *Client) TemplateDelete(email email.Template) {}

// func (c *Client) ListGet(id string)      {}
// func (c *Client) ListCreate(email.Email) {}
// func (c *Client) ListUpdate(email.Email) {}
// func (c *Client) ListDelete(email.Email) {}

func New(ctx context.Context, settings integration.SendGrid) *Client {
	// Set deadline
	ctx, _ = context.WithTimeout(ctx, time.Second*55)

	// Set HTTP Client for App engine
	httpClient := urlfetch.Client(ctx)

	httpClient.Transport = &urlfetch.Transport{
		Context: ctx,
		AllowInvalidServerCertificate: appengine.IsDevAppServer(),
	}
	rest.DefaultClient = &rest.Client{HTTPClient: httpClient}
	client := sendgrid.NewSendClient(settings.APIKey)

	return &Client{ctx, client}
}
