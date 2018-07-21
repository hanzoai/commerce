package email

// Email name and address for sender, recpient, etc
type Email struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address`
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

// Represents a single email message
type Message struct {
	Subject     string       `json:"subject,omitempty"`
	From        Email        `json:"from"`
	ReplyTo     Email        `json:"replyTo,omitempty"`
	To          []Email      `json:"to"`
	Bcc         []Email      `json:"bcc,omitempty"`
	Html        string       `json:"html,omitempty"`
	Text        string       `json:"text,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty`
}
