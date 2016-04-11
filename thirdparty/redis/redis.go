package redis

// import (
// 	"errors"
// 	"net"
// 	"strconv"
// 	"time"

// 	"github.com/hashicorp/golang-lru"

// 	"appengine"

// 	"appengine/socket"

// 	"crowdstart.com/config"
// 	"crowdstart.com/models/order"
// 	"crowdstart.com/models/organization"
// 	"crowdstart.com/models/payment"
// 	"crowdstart.com/models/types/currency"
// 	"crowdstart.com/util/log"

// 	redis "gopkg.in/redis.v3"
// )

// var (
// 	sep               = "_"
// 	soldKey           = "sold"
// 	salesKey          = "sales"
// 	ordersKey         = "orders"
// 	currencySetKey    = "currencies"
// 	subsKey           = "subscribers"
// 	usersKey          = "users"
// 	mailinglistAllKey = "ml_all"

// 	allTime = "all"

// 	maxClients        = 100
// 	redisClients, err = lru.New(maxClients)
// )

// // func init() {
// // 	var err error

// // 	client, err = New(config.Redis.Url, config.Redis.Password)
// // 	if err != nil {
// // 		log.Error("redis client could not connect")
// // 	}
// // }

// var RedisDisabled = errors.New("Redis disabled(no host url specified)")

// type TimeFunc func(t time.Time) string

// func GetClient(ctx appengine.Context) (*redis.Client, error) {
// 	if clientI, ok := redisClients.Get(ctx); ok {
// 		log.Debug("Returning existing client")

// 		client := clientI.(*redis.Client)

// 		if _, err := client.Ping().Result(); err == nil {
// 			return client, nil
// 		}

// 		// if the client is and ping has failed, try to replace with a new client
// 	}

// 	db := int64(0) // unknown db assumed to be dev

// 	// if config.IsDevelopment {
// 	// 	db = 0
// 	// } else if config.IsStaging {
// 	// 	db = 1
// 	// } else if config.IsSandbox {
// 	// 	db = 2
// 	// } else if config.IsProduction {
// 	// 	db = 3
// 	// }

// 	log.Debug("Creating new client")

// 	if config.Redis.Url == "" {
// 		return nil, RedisDisabled
// 	}

// 	var opts *redis.Options
// 	opts = &redis.Options{
// 		Addr:       config.Redis.Url,      // This needs to be the same as what you are using in the dialer
// 		Password:   config.Redis.Password, // no password set
// 		DB:         db,
// 		MaxRetries: 3,
// 		PoolSize:   1,
// 		Dialer: func() (net.Conn, error) {
// 			log.Debug("DIALING")
// 			return socket.DialTimeout(ctx, "tcp", opts.Addr, 5*time.Second)
// 		},
// 	}

// 	client := redis.NewClient(opts)
// 	redisClients.Add(ctx, client)

// 	if _, err := client.Ping().Result(); err != nil {
// 		log.Warn("Redis error: %v", err)
// 		return nil, err
// 	}

// 	return client, nil
// }

// func monthly(t time.Time) string {
// 	t2 := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
// 	return "monthly" + sep + strconv.FormatInt(t2.Unix(), 10)
// }

// func daily(t time.Time) string {
// 	t2 := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
// 	return "daily" + sep + strconv.FormatInt(t2.Unix(), 10)
// }

// func hourly(t time.Time) string {
// 	t2 := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
// 	return "hourly" + sep + strconv.FormatInt(t2.Unix(), 10)
// }

// func addEnvironment(key string) string {
// 	env := "unknown"

// 	if config.IsDevelopment {
// 		env = "dev"
// 	} else if config.IsStaging {
// 		env = "staging"
// 	} else if config.IsSandbox {
// 		env = "sandbox"
// 	} else if config.IsProduction {
// 		env = "prod"
// 	}

// 	return key + sep + env
// }

// func setName(org *organization.Organization, name string) string {
// 	name = org.Name + sep + name
// 	name = addEnvironment(name)

// 	return name
// }

// func totalKey(org *organization.Organization, key string, timeStamp string) string {
// 	key = org.Name + sep + key
// 	key = addEnvironment(key)
// 	key = key + sep + timeStamp

// 	return key
// }

// func storeKey(org *organization.Organization, storeId, key string, timeStamp string) string {
// 	key = org.Name + sep + storeId + sep + key
// 	key = addEnvironment(key)
// 	key = key + sep + timeStamp

// 	return key
// }

// func subKey(org *organization.Organization, key string, timeStamp string) string {
// 	key = org.Name + sep + subsKey + sep + key
// 	key = addEnvironment(key)
// 	key = key + sep + timeStamp

// 	return key
// }

// func userKey(org *organization.Organization, timeStamp string) string {
// 	key := org.Name + sep + usersKey
// 	key = addEnvironment(key)
// 	key = key + sep + timeStamp

// 	return key
// }

// func salesKeyId(cur currency.Type) string {
// 	return salesKey + sep + string(cur)
// }

// func productKeyId(productId string) string {
// 	return soldKey + sep + productId
// }

// func IncrTotalSales(ctx appengine.Context, org *organization.Organization, pays []*payment.Payment, t time.Time) error {
// 	client, err := GetClient(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	var total currency.Cents
// 	var currency currency.Type

// 	for _, pay := range pays {
// 		if pay.CurrencyTransferred != "" {
// 			// This is first because we care about it more :p
// 			total += pay.AmountTransferred
// 			if currency == "" {
// 				currency = pay.CurrencyTransferred
// 			} else if currency != pay.CurrencyTransferred {
// 				log.Error("Multiple currencies in a single payment set should not happen", org.Db.Context)
// 			}
// 		} else {
// 			total += pay.Amount
// 			if currency == "" {
// 				currency = pay.Currency
// 			} else if currency != pay.Currency {
// 				log.Error("Multiple currencies in a single payment set should not happen", org.Db.Context)
// 			}
// 		}
// 	}

// 	keyId := salesKeyId(currency)
// 	key := totalKey(org, keyId, hourly(t))
// 	log.Debug("%v incremented by %v", key, int64(total), org.Db.Context)
// 	err = client.IncrBy(key, int64(total)).Err()
// 	if err != nil {
// 		return err
// 	}

// 	keyId = salesKeyId(currency)
// 	key = totalKey(org, keyId, daily(t))
// 	log.Debug("%v incremented by %v", key, int64(total), org.Db.Context)
// 	err = client.IncrBy(key, int64(total)).Err()
// 	if err != nil {
// 		return err
// 	}

// 	keyId = salesKeyId(currency)
// 	key = totalKey(org, keyId, monthly(t))
// 	log.Debug("%v incremented by %v", key, int64(total), org.Db.Context)
// 	err = client.IncrBy(key, int64(total)).Err()
// 	if err != nil {
// 		return err
// 	}

// 	currencySet := setName(org, currencySetKey)
// 	err = client.SAdd(currencySet, string(currency)).Err()
// 	if err != nil {
// 		return err
// 	}

// 	key = totalKey(org, keyId, allTime)
// 	log.Debug("%v incremented by %v", key, int64(total), org.Db.Context)
// 	return client.IncrBy(key, int64(total)).Err()
// }

// func IncrStoreSales(ctx appengine.Context, org *organization.Organization, storeId string, pays []*payment.Payment, t time.Time) error {
// 	client, err := GetClient(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	var total currency.Cents
// 	var currency currency.Type

// 	for _, pay := range pays {
// 		// This is first because we care about it more :p
// 		if pay.Type == payment.Stripe && pay.CurrencyTransferred != "" {
// 			total += pay.AmountTransferred
// 			if currency == "" {
// 				currency = pay.CurrencyTransferred
// 			} else if currency != pay.CurrencyTransferred {
// 				log.Error("Multiple currencies in a single payment set should not happen", org.Db.Context)
// 			}
// 		} else {
// 			total += pay.Amount
// 			if currency == "" {
// 				currency = pay.Currency
// 			} else if currency != pay.Currency {
// 				log.Error("Multiple currencies in a single payment set should not happen", org.Db.Context)
// 			}
// 		}
// 	}

// 	keyId := salesKeyId(currency)
// 	key := storeKey(org, storeId, keyId, hourly(t))
// 	log.Debug("%v incremented by %v", key, int64(total), org.Db.Context)
// 	if err := client.IncrBy(key, int64(total)).Err(); err != nil {
// 		return err
// 	}

// 	keyId = salesKeyId(currency)
// 	key = storeKey(org, storeId, keyId, daily(t))
// 	log.Debug("%v incremented by %v", key, int64(total), org.Db.Context)
// 	if err := client.IncrBy(key, int64(total)).Err(); err != nil {
// 		return err
// 	}

// 	keyId = salesKeyId(currency)
// 	key = storeKey(org, storeId, keyId, monthly(t))
// 	log.Debug("%v incremented by %v", key, int64(total), org.Db.Context)
// 	if err := client.IncrBy(key, int64(total)).Err(); err != nil {
// 		return err
// 	}

// 	currencySet := setName(org, currencySetKey)
// 	if err := client.SAdd(currencySet, string(currency)).Err(); err != nil {
// 		return err
// 	}

// 	key = storeKey(org, storeId, keyId, allTime)
// 	log.Debug("%v incremented by %v", key, int64(total), org.Db.Context)
// 	return client.IncrBy(key, int64(total)).Err()
// }

// func IncrTotalOrders(ctx appengine.Context, org *organization.Organization, t time.Time) error {
// 	client, err := GetClient(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	key := totalKey(org, ordersKey, hourly(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)

// 	log.Debug("redis client %v", client, org.Db.Context)

// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = totalKey(org, ordersKey, daily(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = totalKey(org, ordersKey, monthly(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = totalKey(org, ordersKey, allTime)
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	return client.Incr(key).Err()
// }

// func IncrStoreOrders(ctx appengine.Context, org *organization.Organization, storeId string, t time.Time) error {
// 	client, err := GetClient(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	key := storeKey(org, storeId, ordersKey, hourly(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = storeKey(org, storeId, ordersKey, daily(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = storeKey(org, storeId, ordersKey, monthly(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = storeKey(org, storeId, ordersKey, allTime)
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	return client.Incr(key).Err()
// }

// func IncrSubscribers(ctx appengine.Context, org *organization.Organization, mailinglistId string, t time.Time) error {
// 	client, err := GetClient(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	key := subKey(org, mailinglistId, hourly(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = subKey(org, mailinglistId, daily(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = subKey(org, mailinglistId, monthly(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = subKey(org, mailinglistId, allTime)
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	return client.Incr(key).Err()

// 	key = subKey(org, mailinglistAllKey, hourly(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = subKey(org, mailinglistAllKey, daily(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = subKey(org, mailinglistAllKey, monthly(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = subKey(org, mailinglistAllKey, allTime)
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	return client.Incr(key).Err()
// }

// func IncrUsers(ctx appengine.Context, org *organization.Organization, t time.Time) error {
// 	client, err := GetClient(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	key := userKey(org, hourly(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = userKey(org, daily(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = userKey(org, monthly(t))
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	if err := client.Incr(key).Err(); err != nil {
// 		return err
// 	}

// 	key = userKey(org, allTime)
// 	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 	return client.Incr(key).Err()
// }

// func IncrTotalProductOrders(ctx appengine.Context, org *organization.Organization, ord *order.Order, t time.Time) error {
// 	client, err := GetClient(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	for _, item := range ord.Items {
// 		productsKey := productKeyId(item.ProductId)

// 		key := totalKey(org, productsKey, hourly(t))
// 		log.Debug("%v incremented by %v", key, 1, org.Db.Context)

// 		log.Debug("redis client %v", client, org.Db.Context)

// 		if err := client.Incr(key).Err(); err != nil {
// 			return err
// 		}

// 		key = totalKey(org, productsKey, daily(t))
// 		log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 		if err := client.Incr(key).Err(); err != nil {
// 			return err
// 		}

// 		key = totalKey(org, productsKey, monthly(t))
// 		log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 		if err := client.Incr(key).Err(); err != nil {
// 			return err
// 		}

// 		key = totalKey(org, productsKey, allTime)
// 		log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 		if err := client.Incr(key).Err(); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// func IncrStoreProductOrders(ctx appengine.Context, org *organization.Organization, storeId string, ord *order.Order, t time.Time) error {
// 	client, err := GetClient(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	for _, item := range ord.Items {
// 		productsKey := productKeyId(item.ProductId)

// 		key := storeKey(org, storeId, productsKey, hourly(t))
// 		log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 		if err := client.Incr(key).Err(); err != nil {
// 			return err
// 		}

// 		key = storeKey(org, storeId, productsKey, daily(t))
// 		log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 		if err := client.Incr(key).Err(); err != nil {
// 			return err
// 		}

// 		key = storeKey(org, storeId, productsKey, monthly(t))
// 		log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 		if err := client.Incr(key).Err(); err != nil {
// 			return err
// 		}

// 		key = storeKey(org, storeId, productsKey, allTime)
// 		log.Debug("%v incremented by %v", key, 1, org.Db.Context)
// 		if err := client.Incr(key).Err(); err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }
