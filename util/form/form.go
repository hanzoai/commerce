package form

import (
	"github.com/gorilla/schema"
	"github.com/zap-proto/fiber/v3/middleware/adaptor"
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/log"
)

var decoder = schema.NewDecoder()

func Parse(c *zip.Ctx, form interface{}) error {
	decoder.IgnoreUnknownKeys(true)

	// Bridge to a net/http request so gorilla/schema decodes the parsed form
	// values exactly as before (ParseForm populates PostForm from the body).
	req, err := adaptor.ConvertRequest(c.Fiber(), false)
	if err != nil {
		return err
	}
	req.ParseForm()

	err = decoder.Decode(form, req.PostForm)
	if err != nil {
		log.Panic("Parsing form %#v", err)
	}
	return err
}

// // TODO: Make this go away
// func SchemaFix(order *models.Order) {
// 	// Schema creates the Order.Items slice sized to whatever is the largest
// 	// index form item. This creates a slice with a huge number of nil structs,
// 	// so we create a new slice of items and use that instead.
// 	items := make([]models.LineItem, 0)
// 	for _, item := range order.Items {
// 		if item.SKU() != "" {
// 			items = append(items, item)
// 		}
// 	}
// 	order.Items = items
// }
