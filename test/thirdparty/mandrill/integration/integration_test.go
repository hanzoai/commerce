package test

import (
	"testing"

	"appengine"

	"github.com/zeekay/aetest"

	"crowdstart.io/config"
	"crowdstart.io/thirdparty/mandrill"
	"crowdstart.io/util/log"
)

func TestPing(t *testing.T) {
	log.SetVerbose(testing.Verbose())
	if config.Mandrill.APIKey == "" {
		t.Skip()
	}
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

	if !mandrill.Ping(ctx) {
		t.Error("Ping failed")
	}
}

func TestSendTemplate(t *testing.T) {
	if config.Mandrill.APIKey == "" {
		t.Skip()
	}

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

	req := mandrill.NewSendTemplateReq()
	// req.AddRecipient("dev@hanzo.ai", "Zach Kelling")
	// req.AddRecipient("dev@hanzo.ai", "Michael W")
	// req.AddRecipient("dev@hanzo.ai", "Marvel Mathew")
	// req.AddRecipient("dev@hanzo.ai", "David Tai")
	req.AddRecipient("dev@hanzo.ai", "Test Mandrill")

	req.Message.Subject = "Test subject"
	req.Message.FromEmail = "dev@hanzo.ai"
	req.Message.FromName = "Tester"
	req.TemplateName = "preorder-confirmation-template"

	err = mandrill.SendTemplate(ctx, &req)
	if err != nil {
		t.Error(err)
	}
}

func TestSend(t *testing.T) {
	if config.Mandrill.APIKey == "" {
		t.Skip()
	}

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

	html := mandrill.GetTemplate("../templates/confirmation_email.html")
	req := mandrill.NewSendReq()
	req.AddRecipient("dev@hanzo.ai", "Test Mandrill")

	req.Message.Subject = "Test subject"
	req.Message.FromEmail = "dev@hanzo.ai"
	req.Message.FromName = "Tester"
	req.Message.Html = html

	err = mandrill.Send(ctx, &req)

	if err != nil {
		t.Error(err)
	}
}
