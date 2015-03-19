package json

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/util/log"
)

func Fail(c *gin.Context, code int, message string, err error) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Write(EncodeBytes(gin.H{"code": code, "message": message}))

	if err != nil {
		log.Error(message+": %v", err, c)
	}

	c.Abort(code)
}

func Render(c *gin.Context, code int, src interface{}) {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.Write(EncodeBytes(src))
	c.Abort(code)
}
