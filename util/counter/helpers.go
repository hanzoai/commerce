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

func IncrementByAll(ctx appengine.Context, tag, storeId string, value int, t time.Time) error {
	t2 := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	key := tag + "-"
	if storeId != "" {
		key += storeId + "-"
	}
	key += string(Hourly) + "-" + strconv.FormatInt(t2.Unix(), 10) + "-"
	key = addEnvironment(key)
	log.Debug("%v incremented by %v", key, 1, ctx)
	if err := Increment(ctx, key, tag, storeId, Hourly, t); err != nil {
		return err
	}

	t2 = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	key = tag + "-"
	if storeId != "" {
		key += storeId + "-"
	}
	key += string(Monthly) + "-" + strconv.FormatInt(t2.Unix(), 10) + "-"
	key = addEnvironment(key)
	log.Debug("%v incremented by %v", key, 1, ctx)
	if err := Increment(ctx, key, key, storeId, Monthly, t); err != nil {
		return err
	}

	key = tag + "-"
	if storeId != "" {
		key += storeId + "-"
	}
	key += string(Total) + "-"
	key = addEnvironment(key)
	log.Debug("%v incremented by %v", key, 1, ctx)
	if err := Increment(ctx, key, key, storeId, Total, t); err != nil {
		return err
	}

	return nil
}

func IncrUser(ctx appengine.Context, t time.Time) error {
	return IncrementByAll(ctx, "user-count", "", 1, t)
}

func IncrOrder(ctx appengine.Context, ord *order.Order) error {
	if ord.StoreId != "" {
		if err := IncrementByAll(ctx, "order-count", ord.StoreId, 1, ord.CreatedAt); err != nil {
			return err
		}
		if err := IncrementByAll(ctx, "order-revenue", ord.StoreId, int(ord.Total), ord.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "order-count", "", 1, ord.CreatedAt); err != nil {
		return err
	}
	if err := IncrementByAll(ctx, "order-revenue", "", int(ord.Total), ord.CreatedAt); err != nil {
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
		if err := IncrementByAll(ctx, "product-"+prod.Id()+"-sold", ord.StoreId, 1, ord.CreatedAt); err != nil {
			return err
		}
		if err := IncrementByAll(ctx, "product-"+prod.Id()+"-revenue", ord.StoreId, int(prod.Price), ord.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "product-"+prod.Id()+"-sold", "", 1, ord.CreatedAt); err != nil {
		return err
	}
	if err := IncrementByAll(ctx, "product-"+prod.Id()+"-revenue", "", int(prod.Price), ord.CreatedAt); err != nil {
		return err
	}

	if prod.InventoryCost == 0 {
		return nil
	}

	if ord.StoreId != "" {
		if err := IncrementByAll(ctx, "product-"+prod.Id()+"-inventory-cost", ord.StoreId, int(prod.InventoryCost), ord.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "product-"+prod.Id()+"-inventory-cost", "", int(prod.InventoryCost), ord.CreatedAt); err != nil {
		return err
	}
	return nil
}

func IncrOrderRefund(ctx appengine.Context, ord *order.Order, refund int, t time.Time) error {
	if ord.StoreId != "" {
		if err := IncrementByAll(ctx, "order-refunds", ord.StoreId, refund, t); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "order-refunds", "", refund, t); err != nil {
		return err
	}
	if ord.Refunded != ord.Total {
		return nil
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

func IncrProductRefund(ctx appengine.Context, prod *product.Product, ord *order.Order) error {
	if ord.StoreId != "" {
		if err := IncrementByAll(ctx, "product-"+prod.Id()+"-refunded", ord.StoreId, 1, ord.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "product-"+prod.Id()+"-refunded", "", 1, ord.CreatedAt); err != nil {
		return err
	}

	return nil
}

func IncrOrderReturn(ctx appengine.Context, items []lineitem.LineItem, rtn *return_.Return) error {
	if rtn.StoreId != "" {
		if err := IncrementByAll(ctx, "order-returns", rtn.StoreId, 1, rtn.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "order-returns", "", 1, rtn.CreatedAt); err != nil {
		return err
	}
	for _, item := range items {
		prod := product.New(rtn.Db)
		if err := prod.GetById(item.ProductId); err != nil {
			return err
		}
		for i := 0; i < item.Quantity; i++ {
			if err := IncrProductReturn(ctx, prod, rtn); err != nil {
				log.Error("IncrProduct Error %v", err, ctx)
				return err
			}
		}
	}

	return nil
}

func IncrProductReturn(ctx appengine.Context, prod *product.Product, rtn *return_.Return) error {
	if rtn.StoreId != "" {
		if err := IncrementByAll(ctx, "product-"+prod.Id()+"-returns", rtn.StoreId, 1, rtn.CreatedAt); err != nil {
			return err
		}
	}
	if err := IncrementByAll(ctx, "product-"+prod.Id()+"-returns", "", 1, rtn.CreatedAt); err != nil {
		return err
	}

	return nil
}
