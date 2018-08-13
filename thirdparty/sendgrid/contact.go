package sendgrid

import (
	"hanzo.io/log"
	"hanzo.io/types/email"
	"hanzo.io/util/json"
)

type Error struct {
	Message      string `json:"message"`
	ErrorIndices []int  `json:"error_indices"`
}

type ContactResponse struct {
	NewCount            int      `json:"new_count"`
	UpdatedCounti       int      `json:"unmodified_count"`
	ErrorCount          int      `json:"error_count'`
	ErrorIndices        []int    `json:"error_indices"`
	Errors              []Error  `json:"errors"`
	PersistedRecipients []string `json:"persisted_recipients"`
}

type Contact struct {
	Id string `json:"id"`
}

func newContact(s *email.Subscriber) []byte {
	m := s.Metadata
	m["email"] = s.Email.Address

	return json.EncodeBytes(m)
}

// Update or create contact if it doesn't exist
func (c *Client) UpdateContact(s *email.Subscriber) (*Contact, error) {
	res, err := c.Request("PATCH", "/v3/contactdb/recipients", nil, newContact(s))
	if err != nil {
		return nil, log.Error("Failed to create contact: %v", err)
	}
	log.Info(res.StatusCode)
	log.Info(res.Body)
	log.Info(res.Headers)

	// Decode response and get contact details
	contactRes := new(ContactResponse)
	json.DecodeBytes([]byte(res.Body), contactRes)

	return &Contact{contactRes.PersistedRecipients[0]}, nil
}
