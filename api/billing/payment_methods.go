package billing

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/paymentmethod"
	"github.com/hanzoai/commerce/models/transaction"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/types"
	"github.com/hanzoai/commerce/util/json/http"
)

type createPaymentMethodRequest struct {
	CustomerId     string                            `json:"customerId"`
	Type           string                            `json:"type"` // "card" | "bank_account" | "balance"
	Card           *paymentmethod.CardDetails        `json:"card,omitempty"`
	BankAccount    *paymentmethod.BankAccountDetails `json:"bankAccount,omitempty"`
	BillingAddress *types.Address                    `json:"billingAddress,omitempty"`
	ProviderRef    string                            `json:"providerRef,omitempty"`
	ProviderType   string                            `json:"providerType,omitempty"`
	Metadata       map[string]interface{}            `json:"metadata,omitempty"`
}

// CreatePaymentMethod creates and attaches a payment method to a customer.
//
//	POST /api/v1/billing/payment-methods
func CreatePaymentMethod(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	var req createPaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	if req.CustomerId == "" {
		http.Fail(c, 400, "customerId is required", nil)
		return
	}

	pm := paymentmethod.New(db)
	pm.CustomerId = req.CustomerId
	pm.UserId = req.CustomerId
	if req.Type != "" {
		pm.Type = req.Type
	}
	pm.Card = req.Card
	pm.BankAccount = req.BankAccount
	pm.BillingAddress = req.BillingAddress
	pm.ProviderRef = req.ProviderRef
	pm.ProviderType = req.ProviderType

	if req.Card != nil {
		pm.Name = req.Card.Brand + " ending in " + req.Card.Last4
	}

	if req.Metadata != nil {
		pm.Metadata = req.Metadata
	}

	if err := pm.Create(); err != nil {
		log.Error("Failed to create payment method: %v", err, c)
		http.Fail(c, 500, "failed to create payment method", err)
		return
	}

	// Auto-grant starter credit when first payment method is added.
	// Non-fatal: if credit grant fails, payment method still succeeds.
	go func() {
		grantStarterCreditIfEligible(db, pm.UserId)
	}()

	c.JSON(201, paymentMethodResponse(pm))
}

// GetPaymentMethod retrieves a payment method by ID.
//
//	GET /api/v1/billing/payment-methods/:id
func GetPaymentMethod(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	pm := paymentmethod.New(db)
	if err := pm.GetById(c.Param("id")); err != nil {
		http.Fail(c, 404, "payment method not found", err)
		return
	}

	c.JSON(200, paymentMethodResponse(pm))
}

// ListPaymentMethods lists payment methods for a customer.
//
//	GET /api/v1/billing/payment-methods?customerId=...&type=...
func ListPaymentMethods(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	rootKey := db.NewKey("synckey", "", 1, nil)
	methods := make([]*paymentmethod.PaymentMethod, 0)
	q := paymentmethod.Query(db).Ancestor(rootKey)

	if customerId := c.Query("customerId"); customerId != "" {
		q = q.Filter("CustomerId=", customerId)
	}
	if pmType := c.Query("type"); pmType != "" {
		q = q.Filter("Type=", pmType)
	}

	iter := q.Order("-Created").Run()
	for {
		pm := paymentmethod.New(db)
		if _, err := iter.Next(pm); err != nil {
			break
		}
		methods = append(methods, pm)
	}

	results := make([]map[string]interface{}, len(methods))
	for i, pm := range methods {
		results[i] = paymentMethodResponse(pm)
	}
	c.JSON(200, results)
}

type updatePaymentMethodRequest struct {
	BillingAddress *types.Address         `json:"billingAddress,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// UpdatePaymentMethod updates a payment method.
//
//	PATCH /api/v1/billing/payment-methods/:id
func UpdatePaymentMethod(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	pm := paymentmethod.New(db)
	if err := pm.GetById(c.Param("id")); err != nil {
		http.Fail(c, 404, "payment method not found", err)
		return
	}

	var req updatePaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	if req.BillingAddress != nil {
		pm.BillingAddress = req.BillingAddress
	}
	if req.Metadata != nil {
		pm.Metadata = req.Metadata
	}

	if err := pm.Update(); err != nil {
		log.Error("Failed to update payment method: %v", err, c)
		http.Fail(c, 500, "failed to update payment method", err)
		return
	}

	c.JSON(200, paymentMethodResponse(pm))
}

// DetachPaymentMethod detaches (soft-deletes) a payment method.
//
//	DELETE /api/v1/billing/payment-methods/:id
func DetachPaymentMethod(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	pm := paymentmethod.New(db)
	if err := pm.GetById(c.Param("id")); err != nil {
		http.Fail(c, 404, "payment method not found", err)
		return
	}

	if err := pm.Delete(); err != nil {
		log.Error("Failed to detach payment method: %v", err, c)
		http.Fail(c, 500, "failed to detach payment method", err)
		return
	}

	c.JSON(200, gin.H{"deleted": true, "id": pm.Id()})
}

type setDefaultRequest struct {
	PaymentMethodId string `json:"paymentMethodId"`
}

// SetDefaultPaymentMethod sets the default payment method for a customer.
//
//	POST /api/v1/billing/customers/:id/default-payment-method
func SetDefaultPaymentMethod(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))
	customerId := c.Param("id")

	var req setDefaultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	// Unset any existing default for this customer
	rootKey := db.NewKey("synckey", "", 1, nil)
	iter := paymentmethod.Query(db).Ancestor(rootKey).
		Filter("CustomerId=", customerId).
		Filter("IsDefault=", true).
		Run()

	for {
		existing := paymentmethod.New(db)
		if _, err := iter.Next(existing); err != nil {
			break
		}
		existing.IsDefault = false
		_ = existing.Update()
	}

	// Set the new default
	pm := paymentmethod.New(db)
	if err := pm.GetById(req.PaymentMethodId); err != nil {
		http.Fail(c, 404, "payment method not found", err)
		return
	}

	pm.IsDefault = true
	if err := pm.Update(); err != nil {
		log.Error("Failed to set default payment method: %v", err, c)
		http.Fail(c, 500, "failed to set default", err)
		return
	}

	c.JSON(200, paymentMethodResponse(pm))
}

// grantStarterCreditIfEligible checks if the user already has a starter
// credit and grants one if not. Called asynchronously after first payment
// method creation.
func grantStarterCreditIfEligible(db *datastore.Datastore, userId string) {
	if userId == "" {
		return
	}

	rootKey := db.NewKey("synckey", "", 1, nil)

	// Check if starter credit was already granted.
	existingTrans := make([]*transaction.Transaction, 0)
	tq := transaction.Query(db).Ancestor(rootKey).
		Filter("DestinationId=", userId).
		Filter("Tags=", StarterCreditTag)
	if _, err := tq.Limit(1).GetAll(&existingTrans); err == nil && len(existingTrans) > 0 {
		return // already granted
	}

	trans := transaction.New(db)
	trans.Type = transaction.Deposit
	trans.DestinationId = userId
	trans.DestinationKind = "iam-user"
	trans.Currency = "usd"
	trans.Amount = currency.Cents(StarterCreditCents)
	trans.Notes = "Welcome credit: $5.00 USD (expires in 30 days)"
	trans.Tags = StarterCreditTag
	trans.ExpiresAt = time.Now().AddDate(0, 0, StarterCreditDays)
	trans.Metadata = types.Map{
		"creditType": "starter",
		"expiryDays": StarterCreditDays,
		"trigger":    "payment-method-added",
	}

	if err := trans.Create(); err != nil {
		log.Warn("Failed to auto-grant starter credit for user %s: %v", userId, err)
	}
}

func paymentMethodResponse(pm *paymentmethod.PaymentMethod) map[string]interface{} {
	resp := map[string]interface{}{
		"id":         pm.Id(),
		"customerId": pm.CustomerId,
		"type":       pm.Type,
		"isDefault":  pm.IsDefault,
		"created":    pm.Created,
	}
	if pm.Name != "" {
		resp["name"] = pm.Name
	}
	if pm.Card != nil {
		resp["card"] = pm.Card
	}
	if pm.BankAccount != nil {
		resp["bankAccount"] = pm.BankAccount
	}
	if pm.BillingAddress != nil {
		resp["billingAddress"] = pm.BillingAddress
	}
	if pm.ProviderRef != "" {
		resp["providerRef"] = pm.ProviderRef
	}
	if pm.Metadata != nil {
		resp["metadata"] = pm.Metadata
	}
	return resp
}
