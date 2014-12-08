package test

import (
	"testing"

	"appengine"
	"appengine/aetest"

	mail "crowdstart.io/thirdparty/mandrill"
)

func TestPing(t *testing.T) {
	t.Skip()
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

func TestSendTemplate(t *testing.T) {
	t.Skip()
	instance, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()

	areq, err := instance.NewRequest("", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := appengine.NewContext(areq)

	req := mail.NewSendTemplateReq()
	// req.AddRecipient("dev@hanzo.ai", "Zach Kelling")
	// req.AddRecipient("dev@hanzo.ai", "Michael W")
	// req.AddRecipient("dev@hanzo.ai", "Marvel Mathew")
	// req.AddRecipient("dev@hanzo.ai", "David Tai")
	req.AddRecipient("dev@hanzo.ai", "Test Mandrill")

	req.Message.Subject = "Test subject"
	req.Message.FromEmail = "noreply@skullysystems.com"
	req.Message.FromName = "Tester"
	req.TemplateName = "preorder-confirmation-template"

	err = mail.SendTemplate(ctx, &req)

	if err != nil {
		t.Error(err)
	}
}

func TestSend(t *testing.T) {
	t.Skip()
	instance, err := aetest.NewInstance(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()

	areq, err := instance.NewRequest("", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := appengine.NewContext(areq)

	html := mail.GetTemplate("../templates/confirmation_email.html")
	req := mail.NewSendReq()
	req.AddRecipient("dev@hanzo.ai", "Test Mandrill")

	req.Message.Subject = "Test subject"
	req.Message.FromEmail = "noreply@skullysystems.com"
	req.Message.FromName = "Tester"
	req.Message.Html = html

	err = mail.Send(ctx, &req)

	if err != nil {
		t.Error(err)
	}
}
