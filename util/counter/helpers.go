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
	t1 := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	t2 := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	baseKey := tag + incrementSep
	if storeId != "" {
		storeKey := baseKey + storeId + incrementSep
		key := storeKey + string(Hourly) + incrementSep + strconv.FormatInt(t1.Unix(), 10)
		log.Debug("%v incremented by %v", key, 1, ctx)
		if err := Increment(ctx, key, tag, storeId, "", Hourly, t); err != nil {
			return err
		}
		key = storeKey + string(Monthly) + incrementSep + strconv.FormatInt(t2.Unix(), 10)
		log.Debug("%v incremented by %v", key, 1, ctx)
		if err := Increment(ctx, key, tag, storeId, "", Monthly, t); err != nil {
			return err
		}
		key = storeKey + string(Total)
		log.Debug("%v incremented by %v", key, 1, ctx)
		if err := Increment(ctx, key, tag, storeId, "", Monthly, t); err != nil {
			return err
		}
	}
	if geo != "" {
		geoKey := baseKey + geo + incrementSep
		key := geoKey + string(Hourly) + incrementSep + strconv.FormatInt(t1.Unix(), 10)
		log.Debug("%v incremented by %v", key, 1, ctx)
		if err := Increment(ctx, key, tag, "", geo, Hourly, t); err != nil {
			return err
		}
		key = geoKey + string(Monthly) + incrementSep + strconv.FormatInt(t2.Unix(), 10)
		log.Debug("%v incremented by %v", key, 1, ctx)
		if err := Increment(ctx, key, tag, "", geo, Monthly, t); err != nil {
			return err
		}
		key = geoKey + string(Total)
		log.Debug("%v incremented by %v", key, 1, ctx)
		if err := Increment(ctx, key, tag, "", geo, Monthly, t); err != nil {
			return err
		}
	}
	if storeId != "" && geo != "" {
		storeGeoKey := baseKey + storeId + incrementSep + geo + incrementSep
		key := storeGeoKey + string(Hourly) + incrementSep + strconv.FormatInt(t1.Unix(), 10)
		log.Debug("%v incremented by %v", key, 1, ctx)
		if err := Increment(ctx, key, tag, storeId, geo, Hourly, t); err != nil {
			return err
		}
		key = storeGeoKey + string(Monthly) + incrementSep + strconv.FormatInt(t2.Unix(), 10)
		log.Debug("%v incremented by %v", key, 1, ctx)
		if err := Increment(ctx, key, tag, storeId, geo, Monthly, t); err != nil {
			return err
		}
		key = storeGeoKey + string(Total)
		log.Debug("%v incremented by %v", key, 1, ctx)
		if err := Increment(ctx, key, tag, storeId, geo, Monthly, t); err != nil {
			return err
		}
	}

	key := baseKey + string(Hourly) + incrementSep + strconv.FormatInt(t1.Unix(), 10)
	log.Debug("%v incremented by %v", key, 1, ctx)
	if err := Increment(ctx, key, tag, "", "", Hourly, t); err != nil {
		return err
	}

	key = baseKey + string(Monthly) + incrementSep + strconv.FormatInt(t2.Unix(), 10)
	log.Debug("%v incremented by %v", key, 1, ctx)
	if err := Increment(ctx, key, tag, "", "", Monthly, t); err != nil {
		return err
	}

	key = baseKey + string(Total)
	log.Debug("%v incremented by %v", key, 1, ctx)
	if err := Increment(ctx, key, tag, "", "", Total, t); err != nil {
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
	if err := IncrementByAll(ctx, "product."+prod.Id()+".sold", "", ord.ShippingAddress.Country, 1, ord.CreatedAt); err != nil {
		return err
	}
	if err := IncrementByAll(ctx, "product."+prod.Id()+".revenue", "", ord.ShippingAddress.Country, int(prod.Price), ord.CreatedAt); err != nil {
		return err
	}

	if prod.InventoryCost == 0 {
		return nil
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
	if err := IncrementByAll(ctx, "order.refunded", "", ord.ShippingAddress.Country, refund, t); err != nil {
		return err
	}
	if ord.Refunded != ord.Total {
		return nil
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
	if err := IncrementByAll(ctx, "product."+prod.Id()+".shipped.count", "", ord.ShippingAddress.Country, 1, ord.CreatedAt); err != nil {
		return err
	}

	return nil
}

func IncrProductRefund(ctx appengine.Context, prod *product.Product, ord *order.Order) error {
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
	if err := IncrementByAll(ctx, "product."+prod.Id()+".returned.count", "", ord.ShippingAddress.Country, 1, rtn.CreatedAt); err != nil {
		return err
	}

	return nil
}
