package billing

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

// GetBalance returns the current balance for an IAM user.
//
//	GET /api/v1/billing/balance?user=hanzo/alice&currency=usd
//
// All amounts in cents. available = balance - holds.
func GetBalance(c *gin.Context) {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)

	user := strings.TrimSpace(c.Query("user"))
	if user == "" {
		http.Fail(c, 400, "user query parameter is required", nil)
		return
	}

	cur := currency.Type(strings.ToLower(c.DefaultQuery("currency", "usd")))

	datas, err := util.GetTransactionsByCurrency(ctx, user, "iam-user", cur, !org.Live)
	if err != nil {
		http.Fail(c, 500, "failed to query balance", err)
		return
	}

	var balance, holds currency.Cents
	if data, ok := datas.Data[cur]; ok {
		balance = data.Balance
		holds = data.Holds
	}

	available := balance - holds
	if available < 0 {
		available = 0
	}

	c.JSON(200, gin.H{
		"user":      user,
		"currency":  cur,
		"balance":   balance,
		"holds":     holds,
		"available": available,
	})
}

// GetBalanceAll returns balances across all currencies for an IAM user.
//
//	GET /api/v1/billing/balance/all?user=hanzo/alice
func GetBalanceAll(c *gin.Context) {
	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)

	user := strings.TrimSpace(c.Query("user"))
	if user == "" {
		http.Fail(c, 400, "user query parameter is required", nil)
		return
	}

	datas, err := util.GetTransactions(ctx, user, "iam-user", !org.Live)
	if err != nil {
		http.Fail(c, 500, "failed to query balance", err)
		return
	}

	balances := make([]gin.H, 0, len(datas.Data))
	for cur, data := range datas.Data {
		available := data.Balance - data.Holds
		if available < 0 {
			available = 0
		}
		balances = append(balances, gin.H{
			"currency":  cur,
			"balance":   data.Balance,
			"holds":     data.Holds,
			"available": available,
		})
	}

	c.JSON(200, gin.H{
		"user":      user,
		"balances":  balances,
	})
}
