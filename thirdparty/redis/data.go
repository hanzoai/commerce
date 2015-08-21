package redis

import (
	"time"

	"crowdstart.com/models/organization"
	"crowdstart.com/models/types/currency"
)

type currencyValues map[currency.Type]int64

type DashboardData struct {
	TotalSales  currencyValues
	TotalOrders int64

	DailySales  []currencyValues
	DailyOrders []currencyValues

	// DailyStoreSales  [](map[currency.Type]int64)
	// DailyStoreOrders [](map[currency.Type]int64)
}
type Type string

const (
	Monthly Type = "Monthly"
	Weekly       = "Weekly"
)

func GetDashboardData(t Type, date time.Time, org *organization.Organization) (DashboardData, error) {
	data := DashboardData{}

	var (
		newDate time.Time
		oldDate time.Time
		days    int64
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

	result := client.SMembers(setName(org, currencySetKey))
	if err := result.Err(); err != nil {
		return data, err
	}

	currencies := result.Val()

	for _, cur := range currencies {
		currency := currency.Type(cur)
		keyId := salesKeyId(currency)
		key := totalKey(org, keyId, AllTime(time.Now()))
		result := client.Get(key)

		if err := result.Err(); err != nil {
			return data, err
		}

		if sales, err := result.Int64(); err != nil {
			return data, err
		} else {
			data.TotalSales[currency] = sales
		}

		data.DailySales = make([]currencyValues, days)

		currentDate := oldDate
		startDay := currentDate.Day()
		for currentDate.Before(newDate) {
			i := currentDate.Day() - startDay
			if data.DailySales[i] != nil {
				data.DailySales[i] = make(currencyValues)
			}

			key := totalKey(org, keyId, Hourly(currentDate))
			result := client.Get(key)

			if err := result.Err(); err != nil {
				return data, err
			}

			if sales, err := result.Int64(); err != nil {
				return data, err
			} else {
				data.DailySales[i][currency] += sales
			}

			key = totalKey(org, ordersKey, Hourly(currentDate))
			result = client.Get(key)

			if err := result.Err(); err != nil {
				return data, err
			}

			if orders, err := result.Int64(); err != nil {
				return data, err
			} else {
				data.DailyOrders[i][currency] += orders
			}

			currentDate = currentDate.Add(time.Hour)
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

	return data, nil
}
