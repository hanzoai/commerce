package email

import (
	"time"
)

// Email name and address for sender, recpient, etc
type Email struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address"`
}

// Custom header
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Attachment holds attachement information
type Attachment struct {
	Content     string `json:"content,omitempty"`
	Type        string `json:"type,omitempty"`
	Name        string `json:"name,omitempty"`
	Filename    string `json:"filename,omitempty"`
	Disposition string `json:"disposition,omitempty"`
	ContentID   string `json:"contentId,omitempty"`
}

// Represents a single dynamic variable to be used when sending an email
type Substitution struct {
	Name      string `json:"name"`
	Content   string `json:"content"`
	Recipient string `json:"recipient,omitempty"`
}

// A tag to associate with a message
type Tag string

type Tracking struct {
	Opens  bool `json:"opens`
	Clicks bool `json:"clicks`
}

// Represents a single email message
type Message struct {
	Subject       string         `json:"subject,omitempty"`
	From          Email          `json:"from"`
	ReplyTo       Email          `json:"replyTo,omitempty"`
	To            []Email        `json:"to"`
	Cc            []Email        `json:"cc,omitempty"`
	Bcc           []Email        `json:"bcc,omitempty"`
	Html          string         `json:"html,omitempty"`
	Text          string         `json:"text,omitempty"`
	Attachments   []Attachment   `json:"attachments,omitempty"`
	Substitutions []Substitution `json:"substitutions,omitempty"`
	Headers       []Header       `json:"headers,omitempty"`
	TemplateID    string         `json:"templateId,omitempty"`
	SendAt        time.Time      `json:"sendAt,omitempty"`
	Tags          []Tag          `json:"tags,omitempty"`
	Tracking      Tracking       `json:"tracking,omitempty"`
}
