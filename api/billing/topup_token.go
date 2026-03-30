package billing

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/thirdparty/kms"
	jsonhttp "github.com/hanzoai/commerce/util/json/http"
)

type topupTokenRequest struct {
	SourceID    string `json:"sourceId"`    // Square Web Payments SDK nonce
	AmountCents int64  `json:"amountCents"`
	UserID      string `json:"userId"`
	Currency    string `json:"currency,omitempty"`
}

// TopupWithToken charges a Square Web Payments SDK nonce and credits user balance.
// Use this for one-time top-ups without saving a payment method first.
//
//	POST /api/v1/billing/topup/token
//
// Body: { sourceId, amountCents, userId?, currency? }
// Returns: { transactionId, balanceCents, status }
func TopupWithToken(c *gin.Context) {
	org := middleware.GetOrganization(c)

	if v, ok := c.Get("kms"); ok {
		if kmsClient, ok := v.(*kms.CachedClient); ok {
			if err := kms.Hydrate(kmsClient, org); err != nil {
				log.Error("KMS hydration failed for org %q: %v", org.Name, err, c)
			}
		}
	}

	db := datastore.New(org.Namespaced(c))

	var req topupTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonhttp.Fail(c, 400, "invalid request body", err)
		return
	}

	if req.SourceID == "" {
		jsonhttp.Fail(c, 400, "sourceId is required", nil)
		return
	}

	// Fall back to the authenticated IAM user if userId not provided.
	if req.UserID == "" {
		if email := strings.TrimSpace(c.GetString("iam_email")); email != "" {
			req.UserID = email
		} else if sub := strings.TrimSpace(c.GetString("iam_user_id")); sub != "" {
			req.UserID = sub
		}
	}
	if req.UserID == "" {
		jsonhttp.Fail(c, 400, "userId is required", nil)
		return
	}
	if req.AmountCents <= 0 {
		jsonhttp.Fail(c, 400, "amountCents must be positive", nil)
		return
	}

	cur := currency.Type(strings.ToLower(req.Currency))
	if cur == "" {
		cur = currency.USD
	}

	ctx := middleware.GetContext(c)
	chargeReq := processor.PaymentRequest{
		Token:       req.SourceID,
		Amount:      currency.Cents(req.AmountCents),
		Currency:    cur,
		Description: fmt.Sprintf("Top-up %d %s for user %s", req.AmountCents, cur, req.UserID),
	}

	proc, err := processor.SelectProcessor(ctx, chargeReq)
	if err != nil {
		log.Error("No processor available for token topup: %v", err, c)
		jsonhttp.Fail(c, 422, "no payment processor available", err)
		return
	}

	result, err := proc.Charge(ctx, chargeReq)
	if err != nil {
		log.Error("Charge failed for token topup (user=%s): %v", req.UserID, err, c)
		jsonhttp.Fail(c, 402, "charge failed", err)
		return
	}
	if !result.Success {
		msg := result.ErrorMessage
		if msg == "" {
			msg = "charge declined"
		}
		jsonhttp.Fail(c, 402, msg, nil)
		return
	}

	// Credit the user's balance.
	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = req.UserID
	trans.DestinationKind = "iam-user"
	trans.Currency = cur
	trans.Amount = currency.Cents(req.AmountCents)
	trans.Notes = fmt.Sprintf("Top-up via %s (ref: %s)", proc.Type(), result.ProcessorRef)
	trans.Tags = "topup"

	if !org.Live {
		trans.Test = true
	}

	if err := trans.Create(); err != nil {
		// Charge succeeded but credit failed — log for manual reconciliation.
		log.Error("RECONCILE: charge succeeded (ref=%s) but deposit failed for user %s: %v",
			result.ProcessorRef, req.UserID, err, c)
		jsonhttp.Fail(c, 500, "charge succeeded but balance credit failed; contact support", err)
		return
	}

	var balanceCents currency.Cents
	if datas, err := util.GetTransactionsByCurrency(org.Namespaced(c), req.UserID, "iam-user", cur, !org.Live); err == nil {
		if data, ok := datas.Data[cur]; ok {
			balanceCents = data.Balance
		}
	}

	c.JSON(200, gin.H{
		"transactionId": trans.Id(),
		"balanceCents":  balanceCents,
		"status":        "ok",
	})
}
