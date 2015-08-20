package redis

import (
	"strconv"
	"time"

	"crowdstart.com/config"
	"crowdstart.com/models/order"
	"crowdstart.com/models/organization"
	"crowdstart.com/util/log"

	redis "gopkg.in/redis.v3"
)

var (
	sep       string
	salesKey  string
	ordersKey string
	subsKey   string
	client    *redis.Client
)

func init() {
	var err error

	sep = "_"
	salesKey = "sales"
	ordersKey = "orders"
	subsKey = "subscribers"

	client, err = New(config.Redis.Url, config.Redis.Password)
	if err != nil {
		log.Error("redis client could not connect")
	}
}

type TimeFunc func(t time.Time) time.Time

func New(addr string, pw string) (*redis.Client, error) {
	db := int64(0) // unknown db assumed to be dev

	if config.IsDevelopment {
		db = 0
	} else if config.IsStaging {
		db = 1
	} else if config.IsSandbox {
		db = 2
	} else if config.IsProduction {
		db = 3
	}

	client := redis.NewClient(&redis.Options{
		Addr:       addr,
		Password:   pw, // no password set
		DB:         db, // use default DB
		MaxRetries: 3,
	})

	if _, err := client.Ping().Result(); err != nil {
		return nil, err
	}

	return client, nil
}

func Hourly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}

func addEnvironment(key string) string {
	env := "unknown"

	if config.IsDevelopment {
		env = "dev"
	} else if config.IsStaging {
		env = "staging"
	} else if config.IsSandbox {
		env = "sandbox"
	} else if config.IsProduction {
		env = "prod"
	}

	return key + sep + env
}

func addTimestamp(key string, tf TimeFunc) string {
	return key + sep + strconv.FormatInt(tf(time.Now()).Unix(), 10)
}

func totalKey(org *organization.Organization, key string, tf TimeFunc) string {
	key = org.Name + sep + key
	key = addEnvironment(key)
	key = addTimestamp(key, tf)

	return key
}

func storeKey(org *organization.Organization, storeId, key string, tf TimeFunc) string {
	key = org.Name + sep + storeId + sep + key
	key = addEnvironment(key)
	key = addTimestamp(key, tf)

	return key
}

func subKey(org *organization.Organization, key string, tf TimeFunc) string {
	key = org.Name + sep + subsKey + sep + key
	key = addEnvironment(key)
	key = addTimestamp(key, tf)

	return key
}

func IncrTotalSales(tf TimeFunc, org *organization.Organization, ord *order.Order) error {
	key := totalKey(org, salesKey+sep+string(ord.Currency), tf)

	log.Debug("%v incremented by %v", key, int64(ord.Total), org.Db.Context)

	return client.IncrBy(key, int64(ord.Total)).Err()
}

func IncrStoreSales(tf TimeFunc, org *organization.Organization, storeId string, ord *order.Order) error {
	key := storeKey(org, storeId, salesKey+sep+string(ord.Currency), tf)

	log.Debug("%v incremented by %v", key, int64(ord.Total), org.Db.Context)

	return client.IncrBy(key, int64(ord.Total)).Err()
}

func IncrTotalOrders(tf TimeFunc, org *organization.Organization) error {
	key := totalKey(org, ordersKey, tf)

	log.Debug("%v incremented by %v", key, 1, org.Db.Context)

	return client.Incr(key).Err()
}

func IncrStoreOrders(tf TimeFunc, org *organization.Organization, storeId string) error {
	key := storeKey(org, storeId, ordersKey, tf)

	log.Debug("%v incremented by %v", key, 1, org.Db.Context)

	return client.Incr(key).Err()
}

func IncrSubscribers(tf TimeFunc, org *organization.Organization, mailinglistId string) error {
	key := subKey(org, mailinglistId, tf)

	log.Debug("%v incremented by %v", key, 1, org.Db.Context)

	return client.Incr(key).Err()
}
