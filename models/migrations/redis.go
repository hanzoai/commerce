package migrations

// import (
// 	"github.com/gin-gonic/gin"

// 	"crowdstart.com/models/order"
// 	"crowdstart.com/models/organization"
// 	"crowdstart.com/models/payment"
// 	"crowdstart.com/models/subscriber"
// 	"crowdstart.com/models/user"
// 	"crowdstart.com/thirdparty/redis"
// 	"crowdstart.com/util/hashid"
// 	"crowdstart.com/util/log"

// 	ds "crowdstart.com/datastore"
// )

// var _ = New("load-redis-orders",
// 	func(c *gin.Context) []interface{} {
// 		return NoArgs
// 	},
// 	func(db *ds.Datastore, ord *order.Order) {
// 		if ord.Test {
// 			return
// 		}

// 		var pays []*payment.Payment

// 		for _, pid := range ord.PaymentIds {
// 			pay := payment.New(db)
// 			if err := pay.GetById(pid); err != nil {
// 				break
// 			}

// 			pays = append(pays, pay)
// 		}

// 		ctx := db.Context
// 		ns, err := hashid.GetNamespace(db.Context, ord.Id())
// 		if err != nil {
// 			log.Warn("hash id decode error %v", err, ctx)
// 		}

// 		org := organization.New(db)
// 		org.Name = ns
// 		log.Debug("org name is %v", ns)

// 		t := ord.CreatedAt

// 		if err := redis.IncrTotalOrders(ctx, org, t); err != nil {
// 			log.Warn("Redis Error %s", err, ctx)
// 		}

// 		if err := redis.IncrTotalSales(ctx, org, pays, t); err != nil {
// 			log.Warn("Redis Error %s", err, ctx)
// 		}

// 		if ord.StoreId != "" {
// 			if err := redis.IncrStoreOrders(ctx, org, ord.StoreId, t); err != nil {
// 				log.Warn("Redis Error %s", err, ctx)
// 			}

// 			if err := redis.IncrStoreSales(ctx, org, ord.StoreId, pays, t); err != nil {
// 				log.Warn("Redis Error %s", err, ctx)
// 			}
// 		}
// 	},
// )

// var _ = New("load-redis-product-orders",
// 	func(c *gin.Context) []interface{} {
// 		return NoArgs
// 	},
// 	func(db *ds.Datastore, ord *order.Order) {
// 		if ord.Test {
// 			return
// 		}

// 		ctx := db.Context
// 		ns, err := hashid.GetNamespace(db.Context, ord.Id())
// 		if err != nil {
// 			log.Warn("hash id decode error %v", err, ctx)
// 		}

// 		org := organization.New(db)
// 		org.Name = ns
// 		log.Debug("org name is %v", ns)

// 		t := ord.CreatedAt

// 		if err := redis.IncrTotalProductOrders(ctx, org, ord, t); err != nil {
// 			log.Warn("Redis Error %s", err, ctx)
// 		}

// 		if ord.StoreId != "" {
// 			if err := redis.IncrStoreProductOrders(ctx, org, ord.StoreId, ord, t); err != nil {
// 				log.Warn("Redis Error %s", err, ctx)
// 			}
// 		}
// 	},
// )

// var _ = New("load-redis-users",
// 	func(c *gin.Context) []interface{} {
// 		return NoArgs
// 	},
// 	func(db *ds.Datastore, usr *user.User) {
// 		ctx := db.Context
// 		ns, err := hashid.GetNamespace(db.Context, usr.Id())
// 		if err != nil {
// 			log.Warn("hash id decode error %v", err, ctx)
// 		}

// 		org := organization.New(db)
// 		org.Name = ns
// 		log.Debug("org name is %v", ns)

// 		t := usr.CreatedAt

// 		if err := redis.IncrUsers(ctx, org, t); err != nil {
// 			log.Warn("Redis Error %s", err, ctx)
// 		}
// 	},
// )

// var _ = New("load-redis-subscribers",
// 	func(c *gin.Context) []interface{} {
// 		return NoArgs
// 	},
// 	func(db *ds.Datastore, sub *subscriber.Subscriber) {
// 		ctx := db.Context
// 		ns, err := hashid.GetNamespace(db.Context, sub.Id())
// 		if err != nil {
// 			log.Warn("hash id decode error %v", err, ctx)
// 		}

// 		org := organization.New(db)
// 		org.Name = ns
// 		log.Debug("org name is %v", ns)

// 		t := sub.CreatedAt

// 		if err := redis.IncrUsers(ctx, org, t); err != nil {
// 			log.Warn("Redis Error %s", err, ctx)
// 		}
// 	},
// )
