package checkout

import (
	"crowdstart.io/util/template"
	"github.com/gin-gonic/gin"
	"github.com/mholt/binding"
)

func showCheckout(c *gin.Context) {
	template.Render(c, "checkout.html", nil)
}

func processCheckout(c *gin.Context) {
	checkoutForm := new(CheckoutForm)
	errs := binding.Bind(c.Request, checkoutForm)
	if len(errs) > 0 {
		// Failed, show errors
		template.Render(c, "checkout.html", nil)
	} else {
		// Success! show that
		template.Render(c, "checkout.html", nil)
	}

}
