package sendgrid

import (
	"context"
	"github.com/sendgrid/sendgrid-go"

	"hanzo.io/types/email"
)

type Client struct {
	ctx    context.Context
	client *sendgrid.Client
}

func (c *Client) Send(email.Email) {

}

func (c *Client) SendCampaign(id string) {

}

func (c *Client) SendTemplate(id string) {

}

func (c *Client) TemplateGet(id string)            {}
func (c *Client) TemplateCreate(email email.Email) {}
func (c *Client) TemplateUpdate(email email.Email) {}
func (c *Client) TemplateDelete(email email.Email) {}

func (c *Client) ListGet(id string)      {}
func (c *Client) ListCreate(email.Email) {}
func (c *Client) ListUpdate(email.Email) {}
func (c *Client) ListDelete(email.Email) {}
