package redis

import (
	"strconv"
	"time"

	"appengine/memcache"

	"appengine"

	"gopkg.in/redis.v3"

	"crowdstart.com/models/organization"
	"crowdstart.com/models/types/currency"
	"crowdstart.com/util/log"
)

type currencyValue map[currency.Type]int64
type currencyValues map[currency.Type][]int64

type DashboardData struct {
	TotalSales       currencyValue
	TotalOrders      int64
	TotalUsers       int64
	TotalSubscribers int64

	DailySales       currencyValues
	DailyOrders      []int64
	DailyUsers       []int64
	DailySubscribers []int64

	// DailyStoreSales  [](map[currency.Type]int64)
	// DailyStoreOrders [](map[currency.Type]int64)
}
type Period string

const (
	Yearly  Period = "yearly"
	Monthly        = "monthly"
	Weekly         = "weekly"
	// Daily        = "Daily"
)

func GetDashboardData(ctx appengine.Context, t Period, date time.Time, org *organization.Organization) (DashboardData, error) {
	data := DashboardData{}
	dashboardKey := org.Name + sep + string(t) + sep
	switch t {
	case Monthly:
		d := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
		dashboardKey += strconv.FormatInt(d.Unix(), 10)
	case Weekly:
		weekday := int(date.Weekday())
		d := time.Date(date.Year(), date.Month(), 1, (7-weekday)+date.Day(), 0, 0, 0, date.Location())
		dashboardKey += strconv.FormatInt(d.Unix(), 10)
	}

	log.Debug("Redis memcache lookup for key: %v", dashboardKey)

	if _, err := memcache.Gob.Get(ctx, dashboardKey, &data); err == nil {
		log.Debug("Redis memcache hit for key: %v", dashboardKey)
		return data, nil
	}

	log.Debug("Redis memcache miss for key: %v", dashboardKey)
	data.TotalSales = make(currencyValue)

	client, err := GetClient(ctx)
	if err != nil {
		return data, err
	}

	var (
		newDate time.Time
		oldDate time.Time
		days    int64
		skip    bool
		key     string
		result  *redis.StringCmd
	)

	switch t {
	case Monthly:
		year := date.Year()
		month := date.Month()
		oldDate = time.Date(year, month, 1, 0, 0, 0, 0, date.Location())
		newDate = time.Date(year, month+1, 1, 0, 0, 0, 0, date.Location())

		// 0th day of month is last day of previous month
		days = int64(time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day())

	case Weekly:
		weekday := int(date.Weekday())
		newDate = time.Date(date.Year(), date.Month(), (7-weekday)+date.Day(), 0, 0, 0, 0, date.Location())
		oldDate = time.Date(date.Year(), date.Month(), (7-weekday)+date.Day()-7, 0, 0, 0, 0, date.Location())

		days = 7
	}

	resultMembers := client.SMembers(setName(org, currencySetKey))
	if err := resultMembers.Err(); err != nil {
		log.Error("Redis Error: %v", err)
		return data, err
	}

	currencies := resultMembers.Val()

	skip = false
	key = userKey(org, allTime)
	result = client.Get(key)

	if err := result.Err(); err != nil {
		if err == redis.Nil {
			skip = true
		} else {
			log.Error("Redis Error: %v", err)
			return data, err
		}
	}

	if !skip {
		if users, err := result.Int64(); err != nil {
			log.Error("Redis Error: %v", err)
			return data, err
		} else {
			data.TotalUsers = users
		}
	}

	skip = false
	key = subKey(org, mailinglistAllKey, allTime)
	result = client.Get(key)

	if err := result.Err(); err != nil {
		if err == redis.Nil {
			skip = true
		} else {
			log.Error("Redis Error: %v", err)
			return data, err
		}
	}

	if !skip {
		if subs, err := result.Int64(); err != nil {
			log.Error("Redis Error: %v", err)
			return data, err
		} else {
			data.TotalUsers = subs
		}
	}

	skip = false
	key = totalKey(org, ordersKey, allTime)
	result = client.Get(key)

	if err := result.Err(); err != nil {
		if err == redis.Nil {
			skip = true
		} else {
			log.Error("Redis Error: %v", err)
			return data, err
		}
	}

	if !skip {
		if orders, err := result.Int64(); err != nil {
			log.Error("Redis Error: %v", err)
			return data, err
		} else {
			data.TotalOrders = orders
		}
	}

	for _, cur := range currencies {
		currency := currency.Type(cur)

		skip = false
		keyId := salesKeyId(currency)
		key = totalKey(org, keyId, allTime)
		result = client.Get(key)

		if err := result.Err(); err != nil {
			if err == redis.Nil {
				skip = true
			} else {
				log.Error("Redis Error: %v", err)
				return data, err
			}
		}

		if !skip {
			if sales, err := result.Int64(); err != nil {
				log.Error("Redis Error: %v", err)
				return data, err
			} else {
				data.TotalSales[currency] = sales
			}
		}

		data.DailySales = make(currencyValues)
		data.DailyOrders = make([]int64, days)
		data.DailyUsers = make([]int64, days)
		data.DailySubscribers = make([]int64, days)

		currentDate := oldDate
		startDate := currentDate
		for currentDate.Before(newDate) {
			i := currentDate.Day() - startDate.Day()
			if currentDate.Month() != startDate.Month() {
				i += time.Date(currentDate.Year(), currentDate.Month(), 0, 0, 0, 0, 0, time.UTC).Day()
			}

			if data.DailySales[currency] == nil {
				data.DailySales[currency] = make([]int64, days)
			}

			skip = false
			key = totalKey(org, keyId, daily(currentDate))
			result = client.Get(key)

			if err := result.Err(); err != nil {
				if err == redis.Nil {
					skip = true
				} else {
					log.Error("Redis Error while getting %v: %v", key, err)
					return data, err
				}
			}

			if !skip {
				if sales, err := result.Int64(); err != nil {
					log.Error("Redis Error while getting %v: %v", key, err)
					return data, err
				} else {
					data.DailySales[currency][i] += sales
				}
			}

			skip = false
			key = totalKey(org, ordersKey, daily(currentDate))
			result = client.Get(key)

			if err := result.Err(); err != nil {
				if err == redis.Nil {
					skip = true
				} else {
					log.Error("Redis Error while getting %v: %v", key, err)
					return data, err
				}
			}

			if !skip {
				if orders, err := result.Int64(); err != nil {
					log.Error("Redis Error while getting %v: %v", key, err)
					return data, err
				} else {
					data.DailyOrders[i] += orders
				}
			}

			skip = false
			key = userKey(org, daily(currentDate))
			result = client.Get(key)

			if err := result.Err(); err != nil {
				if err == redis.Nil {
					skip = true
				} else {
					log.Error("Redis Error while getting %v: %v", key, err)
					return data, err
				}
			}

			if !skip {
				if users, err := result.Int64(); err != nil {
					log.Error("Redis Error while getting %v: %v", key, err)
					return data, err
				} else {
					data.DailyUsers[i] += users
				}
			}

			skip = false
			key = subKey(org, mailinglistAllKey, daily(currentDate))
			result = client.Get(key)

			if err := result.Err(); err != nil {
				if err == redis.Nil {
					skip = true
				} else {
					log.Error("Redis Error while getting %v: %v", key, err)
					return data, err
				}
			}

			if !skip {
				if subs, err := result.Int64(); err != nil {
					log.Error("Redis Error while getting %v: %v", key, err)
					return data, err
				} else {
					data.DailyUsers[i] += subs
				}
			}
			currentDate = currentDate.Add(time.Hour * 24)
		}
	}

	// var stors []store.Store
	// if _, err := store.Query(db).GetAll(&stors); err != nil {
	// 	return data, err
	// }
	// currentDate := oldDate
	// startDay := currentDate.Day()
	// for currentDate.Before(newDate) {
	// 	i := currentDate.Day() - startDay
	// 	totalSalesKey := totalKey(org, salesKeyId(stor.Currency), strconv.FormatInt(currentDate.Unix(), 10))
	// 	totalOrdersKey := totalKey(org, ordersKey, strconv.FormatInt(currentDate.Unix(), 10))

	// 	data.DailyOrders[i]
	// }

	// for _, stor := range stors {
	// }

	expiration := 0 * time.Minute

	// isToday := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	// now := time.Now()
	// today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// if isToday.Equal(today) {
	expiration = 15 * time.Minute
	// }

	item := &memcache.Item{
		Key:        dashboardKey,
		Object:     data,
		Expiration: expiration,
	}

	memcache.Gob.Set(ctx, item)

	return data, nil
}
