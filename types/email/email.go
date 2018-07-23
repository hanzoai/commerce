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

// Tags to substitute in an email body
type Substitutions map[string]string

// Tags to replace in an email for a specific recipient
type Personalization struct {
	Substitutions map[string]Substitutions `json:"substitution"`
	Subject       string                   `json:"subject,omitempty"`
	Headers       []Header                 `json:"headers,omitempty"`
	To            []Email                  `json:"to,omitempty"`
	CC            []Email                  `json:"cc,omitempty"`
	BCC           []Email                  `json:"bcc,omitempty"`
	SendAt        time.Time                `json:"sendAt,omitempty"`
}

// Tracking settings for a given message
type Tracking struct {
	Opens  bool `json:"opens`
	Clicks bool `json:"clicks`
}

// Represents a single email message
type Message struct {
	Subject          string            `json:"subject,omitempty"`
	From             Email             `json:"from"`
	ReplyTo          Email             `json:"replyTo,omitempty"`
	To               []Email           `json:"to"`
	CC               []Email           `json:"cc,omitempty"`
	BCC              []Email           `json:"bcc,omitempty"`
	Html             string            `json:"html,omitempty"`
	Text             string            `json:"text,omitempty"`
	TemplateID       string            `json:"templateId,omitempty"`
	Attachments      []Attachment      `json:"attachments,omitempty"`
	Substitutions    Substitutions     `json:"substitutions,omitempty"`
	Personalizations []Personalization `json:"personalizations,omitempty"`
	Headers          []Header          `json:"headers,omitempty"`
	SendAt           time.Time         `json:"sendAt,omitempty"`
	Tags             []string          `json:"tags,omitempty"`
	Tracking         Tracking          `json:"tracking,omitempty"`
}

func (m *Message) AddAttachments(ats ...Attachment) {
	m.Attachments = append(m.Attachments, ats...)
}

func (m *Message) AddTos(tos ...Email) {
	m.To = append(m.To, tos...)
}

func (m *Message) AddCCs(ccs ...Email) {
	m.CC = append(m.CC, ccs...)
}

func (m *Message) AddBCCs(bccs ...Email) {
	m.BCC = append(m.BCC, bccs...)
}

func (m *Message) AddSubsitutions(subs Substitutions) {
	for k, v := range subs {
		m.Substitutions[k] = v
	}
}

func (m *Message) AddPersonalizations(personalizations ...Personalization) {
	m.Personalizations = append(m.Personalizations, personalizations...)
}
