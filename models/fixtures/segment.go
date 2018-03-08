package fixtures

import (
	"github.com/gin-gonic/gin"

	"hanzo.io/models/segment"
)

var Segment = New("segment", func(c *gin.Context) *segment.Segment {
	db := getNamespaceDb(c)

	seg := segment.New(db)
	seg.Name = "default list"
	seg.GetOrCreate("Name=", seg.Name)

	return seg
})
