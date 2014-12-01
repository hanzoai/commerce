package test

import (
	"testing"

	"appengine"
	"appengine/aetest"

	mail "crowdstart.io/thirdparty/mandrill"
)

func TestPing(t *testing.T) {
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

	if !mail.Ping(ctx) {
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
	html := mail.GetHtml("../templates/confirmation_email.html")
	err = mail.SendMail(ctx,
		"from_name",
		"noreply@skullysystems.com",
		"to_name",
		"dev@hanzo.ai",
		"test",
		html,
		nil,
	)

	if err != nil {
		t.Error(err)
	}
}
