package test

import (
	"testing"

	"appengine"
	"appengine/aetest"

	"crowdstart.io/mail"
)

func TestPingMandrill(t *testing.T) {
	instance, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()

	req, err := instance.NewRequest("", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := appengine.NewContext(req)

	if !mail.PingMandrill(ctx) {
		t.Error("Ping failed")
	}
}

func TestSendMail(t *testing.T) {
	// t.Skip("for now")
	instance, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()

	req, err := instance.NewRequest("", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := appengine.NewContext(req)
	html := mail.GetHtml("../templates/confirmation_email_escaped.html")
	err = mail.SendMail(ctx,
		"from_name",
		"noreply@skullysystems.com",
		"to_name",
		"marvel.j.mathew@gmail.com",
		"test",
		html,
		map[string]string{
			"*|SUBJECT|*":   "Test email",
			"*|FirstName|*": "Marvel",
		},
	)

	if err != nil {
		t.Error(err)
	}
}
