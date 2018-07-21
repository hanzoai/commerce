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

func newMessage(message email.Message) *mail.SGMailV3 {
	// New SendGrid message
	m := new(mail.SGMailV3)

	// Set From
	from := newEmail(message.From)
	m.SetFrom(from)

	// Set subject
	m.Subject = message.Subject

	// Add recipients
	p := mail.NewPersonalization()
	for _, to := range message.To {
		p.AddTos(newEmail(to))
	}
	m.AddPersonalizations(p)

	// Add content
	m.AddContent(newContent("text/plain", message.Text), newContent("text/html", message.Html))

	return m
}

func (c *Client) Send(message email.Message) {
	res, err := c.client.Send(newMessage(message))
	if err != nil {
		log.Error(err)
	} else {
		log.Info(res.StatusCode)
		log.Info(res.Body)
		log.Info(res.Headers)
	}
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
