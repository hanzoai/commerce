package counter

import (
	"strconv"
	"time"

	"appengine"

	"hanzo.io/models/lineitem"
	"hanzo.io/models/order"
	"hanzo.io/models/product"
	"hanzo.io/models/return"
	"hanzo.io/util/log"
)

var incrementSep = "."

func IncrementByAll(ctx appengine.Context, tag, storeId, geo string, value int, t time.Time) error {
	t2 := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	key := tag + incrementSep
	if storeId != "" {
		key += storeId + incrementSep
	}
	if geo != "" {
		key += geo + incrementSep
	}
	key += string(Hourly) + incrementSep + strconv.FormatInt(t2.Unix(), 10)
	log.Debug("%v incremented by %v", key, 1, ctx)
	if err := Increment(ctx, key, tag, storeId, geo, Hourly, t); err != nil {
		return err
	}

	t2 = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	key = tag + incrementSep
	if storeId != "" {
		key += storeId + incrementSep
	}
	if geo != "" {
		key += geo + incrementSep
	}
	key += string(Monthly) + incrementSep + strconv.FormatInt(t2.Unix(), 10)
	log.Debug("%v incremented by %v", key, 1, ctx)
	if err := Increment(ctx, key, tag, storeId, geo, Monthly, t); err != nil {
		return err
	}

	key = tag + incrementSep
	if storeId != "" {
		key += storeId + incrementSep
	}
	if geo != "" {
		key += geo + incrementSep
	}
	key += string(Total)
	log.Debug("%v incremented by %v", key, 1, ctx)
	if err := Increment(ctx, key, tag, storeId, geo, Total, t); err != nil {
		return err
	}

	return nil
}

func IncrUser(ctx appengine.Context, t time.Time) error {
	return IncrementByAll(ctx, "user.count", "", "", 1, t)
}

func IncrSubscriber(ctx appengine.Context, t time.Time) error {
	return IncrementByAll(ctx, "subscriber.count", "", "", 1, t)
}

func IncrOrder(ctx appengine.Context, ord *order.Order) error {
	if ord.StoreId != "" {
		if err := IncrementByAll(ctx, "order.count", ord.StoreId, ord.ShippingAddress.Country, 1, ord.CreatedAt); err != nil {
			return err
		}
		if err := IncrementByAll(ctx, "order.revenue", ord.StoreId, ord.ShippingAddress.Country, int(ord.Total), ord.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "order.count", "", ord.ShippingAddress.Country, 1, ord.CreatedAt); err != nil {
		return err
	}
	if err := IncrementByAll(ctx, "order.revenue", "", ord.ShippingAddress.Country, int(ord.Total), ord.CreatedAt); err != nil {
		return err
	}
	for _, item := range ord.Items {
		prod := product.New(ord.Db)
		if err := prod.GetById(item.ProductId); err != nil {
			return err
		}
		for i := 0; i < item.Quantity; i++ {
			if err := IncrProduct(ctx, prod, ord); err != nil {
				log.Error("IncrProduct Error %v", err, ctx)
				return err
			}
		}
	}

	return nil
}

func IncrProduct(ctx appengine.Context, prod *product.Product, ord *order.Order) error {
	if ord.StoreId != "" {
		if err := IncrementByAll(ctx, "product."+prod.Id()+".sold", ord.StoreId, ord.ShippingAddress.Country, 1, ord.CreatedAt); err != nil {
			return err
		}
		if err := IncrementByAll(ctx, "product."+prod.Id()+".revenue", ord.StoreId, ord.ShippingAddress.Country, int(prod.Price), ord.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "product."+prod.Id()+".sold", "", ord.ShippingAddress.Country, 1, ord.CreatedAt); err != nil {
		return err
	}
	if err := IncrementByAll(ctx, "product."+prod.Id()+".revenue", "", ord.ShippingAddress.Country, int(prod.Price), ord.CreatedAt); err != nil {
		return err
	}

	if prod.InventoryCost == 0 {
		return nil
	}

	if ord.StoreId != "" {
		if err := IncrementByAll(ctx, "product."+prod.Id()+".inventory.cost", ord.StoreId, ord.ShippingAddress.Country, int(prod.InventoryCost), ord.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "product."+prod.Id()+".inventory.cost", "", ord.ShippingAddress.Country, int(prod.InventoryCost), ord.CreatedAt); err != nil {
		return err
	}
	return nil
}

func IncrOrderRefund(ctx appengine.Context, ord *order.Order, refund int, t time.Time) error {
	if ord.Refunded == 0 {
		return nil
	}
	if ord.StoreId != "" {
		if err := IncrementByAll(ctx, "order.refunded", ord.StoreId, ord.ShippingAddress.Country, refund, t); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "order.refunded", "", ord.ShippingAddress.Country, refund, t); err != nil {
		return err
	}
	if ord.Refunded != ord.Total {
		return nil
	}
	if ord.StoreId != "" {
		if err := IncrementByAll(ctx, "order.refunded.count", ord.StoreId, ord.ShippingAddress.Country, 1, t); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "order.refunded.count", "", ord.ShippingAddress.Country, 1, t); err != nil {
		return err
	}
	for _, item := range ord.Items {
		prod := product.New(ord.Db)
		if err := prod.GetById(item.ProductId); err != nil {
			return err
		}
		for i := 0; i < item.Quantity; i++ {
			if err := IncrProductRefund(ctx, prod, ord); err != nil {
				log.Error("IncrProduct Error %v", err, ctx)
				return err
			}
		}
	}

	return nil
}

func IncrOrderShip(ctx appengine.Context, ord *order.Order, t time.Time) error {
	if ord.Fulfillment.Pricing == 0 {
		return nil
	}
	if ord.StoreId != "" {
		if err := IncrementByAll(ctx, "order.shipped.count", ord.StoreId, ord.ShippingAddress.Country, 1, t); err != nil {
			return err
		}
		if err := IncrementByAll(ctx, "order.shipped", ord.StoreId, ord.ShippingAddress.Country, int(ord.Fulfillment.Pricing), t); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "order.shipped.count", "", ord.ShippingAddress.Country, 1, t); err != nil {
		return err
	}
	if err := IncrementByAll(ctx, "order.shipped", "", ord.ShippingAddress.Country, int(ord.Fulfillment.Pricing), t); err != nil {
		return err
	}
	for _, item := range ord.Items {
		prod := product.New(ord.Db)
		if err := prod.GetById(item.ProductId); err != nil {
			return err
		}
		for i := 0; i < item.Quantity; i++ {
			if err := IncrProductShip(ctx, prod, ord); err != nil {
				log.Error("IncrProduct Error %v", err, ctx)
				return err
			}
		}
	}

	return nil
}

func IncrProductShip(ctx appengine.Context, prod *product.Product, ord *order.Order) error {
	if ord.StoreId != "" {
		if err := IncrementByAll(ctx, "product."+prod.Id()+".shipped.count", ord.StoreId, ord.ShippingAddress.Country, 1, ord.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "product."+prod.Id()+".shipped.count", "", ord.ShippingAddress.Country, 1, ord.CreatedAt); err != nil {
		return err
	}

	return nil
}

func IncrProductRefund(ctx appengine.Context, prod *product.Product, ord *order.Order) error {
	if ord.StoreId != "" {
		if err := IncrementByAll(ctx, "product."+prod.Id()+".refunded.count", ord.StoreId, ord.ShippingAddress.Country, 1, ord.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "product."+prod.Id()+".refunded.count", "", ord.ShippingAddress.Country, 1, ord.CreatedAt); err != nil {
		return err
	}

	return nil
}

func IncrOrderReturn(ctx appengine.Context, items []lineitem.LineItem, rtn *return_.Return) error {
	ord := order.New(rtn.Db)
	if err := ord.GetById(rtn.OrderId); err != nil {
		return err
	}
	if rtn.StoreId != "" {
		if err := IncrementByAll(ctx, "order.returned.count", rtn.StoreId, ord.ShippingAddress.Country, 1, rtn.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "order.returned.count", "", ord.ShippingAddress.Country, 1, rtn.CreatedAt); err != nil {
		return err
	}
	for _, item := range items {
		prod := product.New(rtn.Db)
		if err := prod.GetById(item.ProductId); err != nil {
			return err
		}
		for i := 0; i < item.Quantity; i++ {
			if err := IncrProductReturn(ctx, prod, ord, rtn); err != nil {
				log.Error("IncrProduct Error %v", err, ctx)
				return err
			}
		}
	}

	return nil
}

func IncrProductReturn(ctx appengine.Context, prod *product.Product, ord *order.Order, rtn *return_.Return) error {
	if rtn.StoreId != "" {
		if err := IncrementByAll(ctx, "product."+prod.Id()+".returned.count", rtn.StoreId, ord.ShippingAddress.Country, 1, rtn.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "product."+prod.Id()+".returned.count", "", ord.ShippingAddress.Country, 1, rtn.CreatedAt); err != nil {
		return err
	}

	return nil
}
