package fixtures

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/models/submission"
)

var Submission = New("submission", func(c *zip.Ctx) *submission.Submission {
	db := getNamespaceDb(c)

	sub := submission.New(db)
	sub.Email = "fan@suchfan.com"
	sub.Metadata["message"] = "Hi I am a fan!"

	sub.MustPut()

	return sub
})
