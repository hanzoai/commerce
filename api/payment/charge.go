package payment

import (
	"github.com/gin-gonic/gin"

	"crowdstart.io/models2/order"
)

func charge(c *gin.Context) (*order.Order, error) {
	order, err := authorize(c)
	if err != nil {
		return order, err
	}

	return capture(c)
}
