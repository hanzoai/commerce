package sendgrid

import (
	"hanzo.io/log"
	"hanzo.io/types/email"
)

// Add subscriber to list
func (c *Client) Subscribe(listid string, s *email.Subscriber) error {
	// Ensure contact exists
	contact, err := c.UpdateContact(s)
	if err != nil {
		return err
	}

	// Add contact to list
	res, err := c.Request("POST", "/v3/contactdb/lists/"+listid+"/recipients/"+contact.Id, nil, nil)
	if err != nil {
		return log.Error("Failed to add contact to list: %v", err, c.ctx)
	}
	log.Info(res.StatusCode, c.ctx)
	log.Info(res.Body, c.ctx)
	log.Info(res.Headers, c.ctx)

	return nil
}
