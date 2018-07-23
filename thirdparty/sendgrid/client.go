package sendgrid

import (
	"context"
	"errors"
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

func newMessage(message email.Message) *mail.SGMailV3 {
	// New SendGrid message
	m := new(mail.SGMailV3)

	// Set From
	from := newEmail(message.From)
	m.SetFrom(from)

	// Set subject
	m.Subject = message.Subject

	// Add Recipients
	p := mail.NewPersonalization()
	for _, to := range message.To {
		p.AddTos(newEmail(to))
	}

	for _, cc := range message.CC {
		p.AddCCs(newEmail(cc))
	}

	for _, bcc := range message.BCC {
		p.AddBCCs(newEmail(bcc))
	}

	m.AddPersonalizations(p)

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

	if message.Html != "" {
		m.AddContent(newContent("text/html", message.Html))
	}

	// Use template if set
	if message.TemplateID != "" {
		m.SetTemplateID(message.TemplateID)
	}

	return m
}

// Send a single email w/o template
func (c *Client) Send(message email.Message) error {
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

// Send a single email, specifying a given template
func (c *Client) SendTemplate(message email.Message) error {
	if message.TemplateID == "" {
		return errors.New("Template not specified")
	}
	return c.Send(message)
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
