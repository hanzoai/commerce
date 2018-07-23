package sendgrid

import (
	"context"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"

	"hanzo.io/log"
	"hanzo.io/types/email"
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

func addSubsitutions(message *email.Message, address string, personalization *mail.Personalization) {
	if v, ok := message.Personalizations[address]; ok {
		for k, v := range v.Substitutions {
			personalization.SetSubstitution("-"+k+"-", v)
		}
	}
}

func newMessage(message *email.Message) *mail.SGMailV3 {
	// New SendGrid message
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
		addSubsitutions(message, to.Address, p)
		m.AddPersonalizations(p)
	}

	for _, cc := range message.CC {
		p := mail.NewPersonalization()
		p.AddCCs(newEmail(cc))
		m.AddPersonalizations(p)
	}

	for _, bcc := range message.BCC {
		p := mail.NewPersonalization()
		p.AddBCCs(newEmail(bcc))
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

// Send a single email w/o template
func (c *Client) Send(message *email.Message) error {
	res, err := c.client.Send(newMessage(message))
	if err != nil {
		log.Error(err)
		return err
	}
	log.Info(res.StatusCode)
	log.Info(res.Body)
	log.Info(res.Headers)
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
