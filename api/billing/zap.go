package billing

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"

	. "github.com/hanzoai/commerce/types"
)

// ── ZAP-over-HTTP dispatch ──────────────────────────────────────────────
//
// Single endpoint that speaks the ZAP JSON envelope protocol.
//
//	POST /api/v1/zap
//
// Request:  {"method":"billing.getBalance","id":"req-1","params":{...}}
// Response: {"id":"req-1","result":{...}} or {"id":"req-1","error":{...}}

type zapRequest struct {
	Method string          `json:"method"`
	ID     string          `json:"id"`
	Params json.RawMessage `json:"params"`
}

type zapResponse struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *zapError   `json:"error,omitempty"`
}

type zapError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ZapDispatch is the single ZAP-over-HTTP endpoint for billing.
func ZapDispatch(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, zapResponse{
			Error: &zapError{Code: -32700, Message: "read error: " + err.Error()},
		})
		return
	}

	var req zapRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(400, zapResponse{
			Error: &zapError{Code: -32700, Message: "parse error: " + err.Error()},
		})
		return
	}

	if req.Method == "" {
		c.JSON(400, zapResponse{
			ID:    req.ID,
			Error: &zapError{Code: -32600, Message: "method is required"},
		})
		return
	}

	var result interface{}
	var zapErr *zapError

	switch req.Method {
	case "billing.getBalance":
		result, zapErr = zapGetBalance(c, req.Params)
	case "billing.getBalanceAll":
		result, zapErr = zapGetBalanceAll(c, req.Params)
	case "billing.getUsage":
		result, zapErr = zapGetUsage(c, req.Params)
	case "billing.recordUsage":
		result, zapErr = zapRecordUsage(c, req.Params)
	case "billing.deposit":
		result, zapErr = zapDeposit(c, req.Params)
	default:
		zapErr = &zapError{Code: -32601, Message: "method not found: " + req.Method}
	}

	if zapErr != nil {
		c.JSON(200, zapResponse{ID: req.ID, Error: zapErr})
		return
	}

	c.JSON(200, zapResponse{ID: req.ID, Result: result})
}

// ── ZAP method handlers ─────────────────────────────────────────────────

type zapBalanceParams struct {
	User     string `json:"user"`
	Currency string `json:"currency"`
}

func zapGetBalance(c *gin.Context, params json.RawMessage) (interface{}, *zapError) {
	var p zapBalanceParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &zapError{Code: -32602, Message: "invalid params: " + err.Error()}
	}
	if p.User == "" {
		return nil, &zapError{Code: -32602, Message: "user is required"}
	}

	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)

	cur := currency.Type(strings.ToLower(p.Currency))
	if cur == "" {
		cur = "usd"
	}

	datas, err := util.GetTransactionsByCurrency(ctx, p.User, "iam-user", cur, !org.Live)
	if err != nil {
		return nil, &zapError{Code: -32000, Message: "balance query failed: " + err.Error()}
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

	return gin.H{
		"user":      p.User,
		"currency":  cur,
		"balance":   balance,
		"holds":     holds,
		"available": available,
	}, nil
}

func zapGetBalanceAll(c *gin.Context, params json.RawMessage) (interface{}, *zapError) {
	var p struct {
		User string `json:"user"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &zapError{Code: -32602, Message: "invalid params: " + err.Error()}
	}
	if p.User == "" {
		return nil, &zapError{Code: -32602, Message: "user is required"}
	}

	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)

	datas, err := util.GetTransactions(ctx, p.User, "iam-user", !org.Live)
	if err != nil {
		return nil, &zapError{Code: -32000, Message: "balance query failed: " + err.Error()}
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

	return gin.H{
		"user":     p.User,
		"balances": balances,
	}, nil
}

func zapGetUsage(c *gin.Context, params json.RawMessage) (interface{}, *zapError) {
	var p zapBalanceParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &zapError{Code: -32602, Message: "invalid params: " + err.Error()}
	}
	if p.User == "" {
		return nil, &zapError{Code: -32602, Message: "user is required"}
	}

	org := middleware.GetOrganization(c)
	ctx := org.Namespaced(c)
	db := datastore.New(ctx)

	rootKey := db.NewKey("synckey", "", 1, nil)

	transs := make([]*transaction.Transaction, 0)
	q := transaction.Query(db).Ancestor(rootKey).
		Filter("Test=", !org.Live).
		Filter("SourceKind=", "iam-user").
		Filter("SourceId=", p.User).
		Filter("Tags=", "api-usage")

	cur := currency.Type(strings.ToLower(p.Currency))
	if cur != "" {
		q = q.Filter("Currency=", cur)
	}

	if _, err := q.GetAll(&transs); err != nil {
		return nil, &zapError{Code: -32000, Message: "usage query failed: " + err.Error()}
	}

	items := make([]gin.H, 0, len(transs))
	for _, t := range transs {
		items = append(items, gin.H{
			"transactionId": t.Id(),
			"amount":        t.Amount,
			"currency":      t.Currency,
			"notes":         t.Notes,
			"metadata":      t.Metadata,
			"createdAt":     t.CreatedAt,
		})
	}

	return gin.H{
		"user":  p.User,
		"count": len(items),
		"usage": items,
	}, nil
}

func zapRecordUsage(c *gin.Context, params json.RawMessage) (interface{}, *zapError) {
	var req usageRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &zapError{Code: -32602, Message: "invalid params: " + err.Error()}
	}

	if req.User == "" {
		return nil, &zapError{Code: -32602, Message: "user is required"}
	}

	if req.Amount <= 0 {
		return gin.H{"user": req.User, "amount": 0, "status": "skipped"}, nil
	}

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	cur := currency.Type(strings.ToLower(req.Currency))
	if cur == "" {
		cur = "usd"
	}

	notes := fmt.Sprintf("API usage: %s (%d tokens)", req.Model, req.TotalTokens)

	trans := transaction.New(db)
	trans.Type = transaction.Withdraw
	trans.SourceId = req.User
	trans.SourceKind = "iam-user"
	trans.Currency = cur
	trans.Amount = currency.Cents(req.Amount)
	trans.Notes = notes
	trans.Tags = "api-usage"
	trans.Metadata = Map{
		"model":            req.Model,
		"provider":         req.Provider,
		"promptTokens":     req.PromptTokens,
		"completionTokens": req.CompletionTokens,
		"totalTokens":      req.TotalTokens,
		"requestId":        req.RequestID,
		"premium":          req.Premium,
		"stream":           req.Stream,
		"status":           req.Status,
		"clientIp":         req.ClientIP,
	}

	if !org.Live {
		trans.Test = true
	}

	if err := trans.Create(); err != nil {
		log.Error("ZAP: Failed to record usage: %v", err, c)
		return nil, &zapError{Code: -32000, Message: "failed to record usage: " + err.Error()}
	}

	return gin.H{
		"transactionId": trans.Id(),
		"user":          req.User,
		"amount":        req.Amount,
		"currency":      cur,
		"type":          "withdraw",
	}, nil
}

func zapDeposit(c *gin.Context, params json.RawMessage) (interface{}, *zapError) {
	var req depositRequest
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &zapError{Code: -32602, Message: "invalid params: " + err.Error()}
	}

	if req.User == "" {
		return nil, &zapError{Code: -32602, Message: "user is required"}
	}

	if req.Amount <= 0 {
		return nil, &zapError{Code: -32602, Message: "amount must be positive"}
	}

	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	cur := currency.Type(strings.ToLower(req.Currency))
	if cur == "" {
		cur = "usd"
	}

	notes := req.Notes
	if notes == "" {
		notes = fmt.Sprintf("Deposit: %d cents %s", req.Amount, cur)
	}

	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = req.User
	trans.DestinationKind = "iam-user"
	trans.Currency = cur
	trans.Amount = currency.Cents(req.Amount)
	trans.Notes = notes
	trans.Tags = req.Tags

	if req.ExpiresIn > 0 {
		trans.ExpiresAt = time.Now().AddDate(0, 0, req.ExpiresIn)
	}

	if !org.Live {
		trans.Test = true
	}

	if err := trans.Create(); err != nil {
		log.Error("ZAP: Failed to create deposit: %v", err, c)
		return nil, &zapError{Code: -32000, Message: "failed to create deposit: " + err.Error()}
	}

	resp := gin.H{
		"transactionId": trans.Id(),
		"user":          req.User,
		"amount":        req.Amount,
		"currency":      cur,
		"type":          "deposit",
		"tags":          req.Tags,
	}
	if !trans.ExpiresAt.IsZero() {
		resp["expiresAt"] = trans.ExpiresAt
	}

	return resp, nil
}
