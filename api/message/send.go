package message

import (
	"errors"
	
	"github.com/gin-gonic/gin"

	"hanzo.io/log"
	"hanzo.io/util/json"
	"hanzo.io/util/json/http"
	"hanzo.io/thirdparty/twilio"
	"hanzo.io/middleware"
)

type sendReq struct {
	ToNumber    string `json:"toNumber"`
	Message		string `json:"message"`
}

type sendRes struct {
	Status      string  `json:"status"`
}

func send(c *gin.Context) {
	req := &sendReq{}

	// Decode response body to create new user
	if err := json.Decode(c.Request.Body, req); err != nil {
		http.Fail(c, 400, "Failed decode request body", err)
		return
	}

	//db := datastore.New(c)

	ctx := middleware.GetAppEngine(c)

	org := middleware.GetOrganization(c)

	cli := twilio.New(ctx, org.Twilio.AccountSid, org.Twilio.AuthToken)

	status, err := cli.SendSms("", req.ToNumber, req.Message);
	if err != nil {
		http.Fail(c, 401, "Failure to send SMS", errors.New("Failure to send SMS"))
		log.Debug("Failure from Twilio", c)
		log.Debug("Error from Twilio: ", err)
	}

	res := &sendRes{
		Status: status,
	}

	http.Render(c, 200, res)
	return
}
