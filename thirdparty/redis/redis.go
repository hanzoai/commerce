package redis

import (
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
	client    *redis.Client
)

func init() {
	var err error

	sep = "_"
	salesKey = "sales"
	client, err = New(config.Redis.Url, config.Redis.Password)
	if err != nil {
		log.Error("redis client could not connect")
	}
}

type TimeFunc func(t time.Time) time.Time

func New(addr, pw string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:       addr,
		Password:   pw, // no password set
		DB:         0,  // use default DB
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
	return key + sep + tf(time.Now()).String()
}

func totalKey(org *organization.Organization, key string) string {
	key = org.Name + sep + key
	key = addEnvironment(key)

	return key
}

func storeKey(org *organization.Organization, storeId, key string) string {
	key = org.Name + sep + storeId + sep + key
	key = addEnvironment(key)

	return key
}

func IncrTotalSales(tf TimeFunc, org *organization.Organization, ord *order.Order) {
	key := totalKey(org, salesKey) + sep + string(ord.Currency)
	key = addTimestamp(key, tf)

	client.IncrBy(key, int64(ord.Total))
}

func IncrStoreSales(tf TimeFunc, org *organization.Organization, storeId string, ord *order.Order) {
	key := storeKey(org, storeId, salesKey) + sep + string(ord.Currency)
	key = addTimestamp(key, tf)

	client.IncrBy(key, int64(ord.Total))
}

func IncrTotalOrders(tf TimeFunc, org *organization.Organization) {
	key := totalKey(org, ordersKey)
	key = addTimestamp(key, tf)

	client.Incr(key)
}

func IncrStoreOrders(tf TimeFunc, org *organization.Organization, storeId string) {
	key := storeKey(org, storeId, ordersKey)
	key = addTimestamp(key, tf)

	client.Incr(key)
}
