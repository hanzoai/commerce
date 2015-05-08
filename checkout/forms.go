package checkout

import (
	"github.com/gin-gonic/gin"

	"crowdstart.com/datastore"
	// "crowdstart.com/models"
	"crowdstart.com/models/order"
	"crowdstart.com/models/user"
	"crowdstart.com/util/form"
	"crowdstart.com/util/log"
)

// Load order from checkout form
type CheckoutForm struct {
	Order *order.Order
}

// Parse form
func (f *CheckoutForm) Parse(c *gin.Context) error {
	// if err := form.Parse(c, f); err != nil {
	// 	return err
	// }

	// form.SchemaFix(&f.Order) // Fuck you schema

	// // Merge and sort products, client can submit form with products duplicated (bonus items).
	// items := make([]models.LineItem, 0)
	// bonus := make([]models.LineItem, 0)
	// itemMap := make(map[string]int)
	// bonusMap := make(map[string]int)

	// for i, item := range f.Order.Items {
	// 	if item.Price() > 0 {
	// 		if index, ok := itemMap[item.SKU()]; ok {
	// 			items[index].Quantity += item.Quantity
	// 		} else {
	// 			itemMap[item.SKU()] = i
	// 			items = append(items, item)
	// 		}
	// 	} else {
	// 		if index, ok := bonusMap[item.SKU()]; ok {
	// 			bonus[index].Quantity += item.Quantity
	// 		} else {
	// 			bonusMap[item.SKU()] = i
	// 			bonus = append(bonus, item)
	// 		}
	// 	}
	// }

	// // Append bonus items to end of lineitem slice
	// items = append(items, bonus...)

	// // Update Order.Items
	// f.Order.Items = items

	return nil
}

// Populate form with data from database
func (f *CheckoutForm) Populate(db *datastore.Datastore) error {
	// return f.Order.Populate(db)
	return nil
}

// Merge line items in form
func (f *CheckoutForm) Merge(c *gin.Context) {
	// // Merge and sort products, client can submit form with products duplicated (bonus items).
	// items := make([]models.LineItem, 0)
	// bonus := make([]models.LineItem, 0)
	// itemMap := make(map[string]int)
	// bonusMap := make(map[string]int)
	// for i, item := range f.Order.Items {
	// 	if item.Price() > 0 {
	// 		if index, ok := itemMap[item.SKU()]; ok {
	// 			items[index].Quantity += item.Quantity
	// 		} else {
	// 			itemMap[item.SKU()] = i
	// 			items = append(items, item)
	// 		}
	// 	} else {
	// 		if index, ok := bonusMap[item.SKU()]; ok {
	// 			bonus[index].Quantity += item.Quantity
	// 		} else {
	// 			bonusMap[item.SKU()] = i
	// 			bonus = append(bonus, item)
	// 		}
	// 	}
	// }

	// // Append bonus items to end of lineitem slice
	// items = append(items, bonus...)

	// // Update Order.Items
	// f.Order.Items = items
}

func (f CheckoutForm) Validate(c *gin.Context) {}

// Charge after successful authorization
type ChargeForm struct {
	User          *user.User
	Order         *order.Order
	ShipToBilling bool
	StripeToken   string
}

func (f *ChargeForm) Parse(c *gin.Context) error {
	if err := form.Parse(c, f); err != nil {
		log.Panic("Unable to parse form: %v", err)
		return err
	}

	// form.SchemaFix(&f.Order) // Fuck you schema

	if f.ShipToBilling {
		f.Order.ShippingAddress = f.Order.BillingAddress
	}

	return nil
}

func (f *ChargeForm) Sanitize() {
	// val.SanitizeUser(&f.User)
}

func (f *ChargeForm) Validate() []string {
	var errs []string
	// errs = val.ValidateUser(&f.User, errs)
	// errs = val.ValidateAddress(&f.Order.BillingAddress, errs)
	// errs = val.ValidateAddress(&f.Order.ShippingAddress, errs)
	return errs
}
