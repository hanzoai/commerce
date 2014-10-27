package checkout

import (
	"crowdstart.io/cardconnect"
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
	"github.com/mholt/binding"
)

func checkout(c *gin.Context) {
	template.Render(c, "checkout.html", nil)
}

func checkoutComplete(c *gin.Context) {
	template.Render(c, "checkout-complete.html", nil)
}

func submitOrder(c *gin.Context) {
	checkoutForm := new(CheckoutForm)
	if binding.Bind(c.Request, checkoutForm).Handle(c.Writer) {
		// Failed, binding will return errors
		return
	}

	// Authorize order
	ares, err := cardconnect.Authorize(checkoutForm.Order); switch {
	case err != nil:
		c.JSON(500, gin.H{"status": "Unable to authorize payment."})
	case ares.Status == "A":
		c.JSON(200, gin.H{"status": "ok"})
	case ares.Status == "B":
		c.JSON(200, gin.H{"status": "retry"})
	case ares.Status == "C":
		c.JSON(200, gin.H{"status": "declined"})
	}
}
