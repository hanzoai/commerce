package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/util/json"
)

func Authorize(c *gin.Context) {
	if order, err := authorize(c); err != nil {
		json.Fail(c, 500, err.Error(), err)
	} else {
		c.JSON(200, order)
	}
}

func Capture(c *gin.Context) {
	if order, err := capture(c); err != nil {
		json.Fail(c, 500, err.Error(), err)
	} else {
		c.JSON(200, order)
	}
}

func Charge(c *gin.Context) {
	if order, err := charge(c); err != nil {
		json.Fail(c, 500, err.Error(), err)
	} else {
		c.JSON(200, order)
	}
}
