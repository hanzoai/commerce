package redis

import (
	"strconv"
	"time"

	"crowdstart.com/config"
	"crowdstart.com/models/organization"
	"crowdstart.com/models/payment"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/log"

	redis "gopkg.in/redis.v3"
)

var (
	sep            string
	salesKey       string
	currencySetKey string
	ordersKey      string
	subsKey        string
	client         *redis.Client
)

func init() {
	var err error

	sep = "_"
	salesKey = "sales"
	ordersKey = "orders"
	subsKey = "subscribers"
	currencySetKey = "currencies"

	client, err = New(config.Redis.Url, config.Redis.Password)
	if err != nil {
		log.Error("redis client could not connect")
	}
}

type TimeFunc func(t time.Time) string

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

func AllTime(t time.Time) string {
	return "all"
}

func Hourly(t time.Time) string {
	t2 := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	return strconv.FormatInt(t2.Unix(), 10)
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

func setName(org *organization.Organization, name string) string {
	name = org.Name + sep + name
	name = addEnvironment(name)

	return name
}

func totalKey(org *organization.Organization, key string, timeStamp string) string {
	key = org.Name + sep + key
	key = addEnvironment(key)
	key = key + sep + timeStamp

	return key
}

func storeKey(org *organization.Organization, storeId, key string, timeStamp string) string {
	key = org.Name + sep + storeId + sep + key
	key = addEnvironment(key)
	key = key + sep + timeStamp

	return key
}

func subKey(org *organization.Organization, key string, timeStamp string) string {
	key = org.Name + sep + subsKey + sep + key
	key = addEnvironment(key)
	key = key + sep + timeStamp

	return key
}

func salesKeyId(cur currency.Type) string {
	return salesKey + sep + string(cur)
}

func IncrTotalSales(tf TimeFunc, org *organization.Organization, pays []*payment.Payment) error {
	var total currency.Cents
	var currency currency.Type

	for _, pay := range pays {
		if pay.CurrencyTransferred != "" {
			// This is first because we care about it more :p
			total += pay.AmountTransferred
			if currency == "" {
				currency = pay.CurrencyTransferred
			} else if currency != pay.CurrencyTransferred {
				log.Error("Multiple currencies in a single payment set should not happen", org.Db.Context)
			}
		} else {
			total += pay.Amount
			if currency == "" {
				currency = pay.Currency
			} else if currency != pay.Currency {
				log.Error("Multiple currencies in a single payment set should not happen", org.Db.Context)
			}
		}
	}

	keyId := salesKeyId(currency)
	key := totalKey(org, keyId, tf(time.Now()))
	log.Debug("%v incremented by %v", key, int64(total), org.Db.Context)
	err := client.IncrBy(key, int64(total)).Err()
	if err != nil {
		return err
	}

	currencySet := setName(org, currencySetKey)
	err = client.SAdd(currencySet, string(currency)).Err()
	if err != nil {
		return err
	}

	key = totalKey(org, keyId, AllTime(time.Now()))
	log.Debug("%v incremented by %v", key, int64(total), org.Db.Context)
	return client.IncrBy(key, int64(total)).Err()
}

func IncrStoreSales(tf TimeFunc, org *organization.Organization, storeId string, pays []*payment.Payment) error {
	var total currency.Cents
	var currency currency.Type

	for _, pay := range pays {
		// This is first because we care about it more :p
		if pay.Type == payment.Stripe && pay.CurrencyTransferred != "" {
			total += pay.AmountTransferred
			if currency == "" {
				currency = pay.CurrencyTransferred
			} else if currency != pay.CurrencyTransferred {
				log.Error("Multiple currencies in a single payment set should not happen", org.Db.Context)
			}
		} else {
			total += pay.Amount
			if currency == "" {
				currency = pay.Currency
			} else if currency != pay.Currency {
				log.Error("Multiple currencies in a single payment set should not happen", org.Db.Context)
			}
		}
	}

	keyId := salesKeyId(currency)
	key := storeKey(org, storeId, keyId, tf(time.Now()))
	log.Debug("%v incremented by %v", key, int64(total), org.Db.Context)
	err := client.IncrBy(key, int64(total)).Err()
	if err != nil {
		return err
	}

	currencySet := setName(org, currencySetKey)
	err = client.SAdd(currencySet, string(currency)).Err()
	if err != nil {
		return err
	}

	key = storeKey(org, storeId, keyId, AllTime(time.Now()))
	log.Debug("%v incremented by %v", key, int64(total), org.Db.Context)
	return client.IncrBy(key, int64(total)).Err()
}

func IncrTotalOrders(tf TimeFunc, org *organization.Organization) error {
	key := totalKey(org, ordersKey, tf(time.Now()))
	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
	err := client.Incr(key).Err()
	if err != nil {
		return err
	}

	key = totalKey(org, ordersKey, AllTime(time.Now()))
	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
	return client.Incr(key).Err()
}

func IncrStoreOrders(tf TimeFunc, org *organization.Organization, storeId string) error {
	key := storeKey(org, storeId, ordersKey, tf(time.Now()))
	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
	err := client.Incr(key).Err()
	if err != nil {
		return err
	}

	key = storeKey(org, storeId, ordersKey, AllTime(time.Now()))
	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
	return client.Incr(key).Err()
}

func IncrSubscribers(tf TimeFunc, org *organization.Organization, mailinglistId string) error {
	key := subKey(org, mailinglistId, tf(time.Now()))
	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
	err := client.Incr(key).Err()
	if err != nil {
		return err
	}

	key = subKey(org, mailinglistId, AllTime(time.Now()))
	log.Debug("%v incremented by %v", key, 1, org.Db.Context)
	return client.Incr(key).Err()
}
