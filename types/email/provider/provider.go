package provider

// Available email providers
type Type string

const (
	Mandrill  Type = "mandrill"
	SendGrid  Type = "sendgrid"
	SmtpRelay Type = "smtprelay"
)
