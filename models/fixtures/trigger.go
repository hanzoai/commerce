package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/trigger"
)

var Trigger = New("trigger", func(c *gin.Context) *trigger.Trigger {
	db := getNamespaceDb(c)

	trig := trigger.New(db)
	trig.Name = "default trigger"
	trig.GetOrCreate("Name=", trig.Name)

	seg := Segment(c)

	trig.Checks["user.created"] = true
	trig.Action = trigger.Action{
		Task: "add-to-segment",
		Args: map[string]string{
			"SegmentId": seg.Id(),
		},
	}

	trig.MustUpdate()

	return trig
})
