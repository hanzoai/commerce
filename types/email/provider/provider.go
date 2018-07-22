package provider

// Types of system-defined emails
type Type string

const (
	Mandrill  Type = "mandrill"
	SendGrid  Type = "sendgrid"
	SmtpRelay Type = "smtprelay"
)
