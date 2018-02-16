package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/funnel"
)

var Funnel = New("espy-test-funnel", func(c *context.Context) *funnel.Funnel {
	db := getNamespaceDb(c)

	f := funnel.New(db)
	f.Name = "Boring Funnel"
	f.Events = [][]string{
		[]string{
			"click_1",
		},
		[]string{
			"click_2",
		},
		[]string{
			"click_3",
		},
	}

	f.MustPut()

	return f
})
