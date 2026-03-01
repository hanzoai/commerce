package billing

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/middleware/iammiddleware"
	"github.com/hanzoai/commerce/models/spendalert"
	"github.com/hanzoai/commerce/util/json/http"
)

type createSpendAlertRequest struct {
	UserId    string `json:"userId"`
	Title     string `json:"title"`
	Threshold int64  `json:"threshold"`
	Currency  string `json:"currency"`
}

type updateSpendAlertRequest struct {
	Title     string `json:"title"`
	Threshold int64  `json:"threshold"`
}

// resolveSpendAlertUserId returns the userId to scope spend-alert queries.
// For IAM-authenticated requests the caller's subject is used as a safe
// default when no explicit user query param is provided.
func resolveSpendAlertUserId(c *gin.Context) string {
	user := strings.TrimSpace(c.Query("user"))
	if user != "" {
		return user
	}
	if claims := iammiddleware.GetIAMClaims(c); claims != nil {
		return claims.Subject
	}
	return ""
}

func spendAlertResponse(a *spendalert.SpendAlert) gin.H {
	return gin.H{
		"id":          a.Id(),
		"userId":      a.UserId,
		"title":       a.Title,
		"threshold":   a.Threshold,
		"currency":    a.Currency,
		"triggeredAt": a.TriggeredAt,
		"createdAt":   a.CreatedAt,
		"updatedAt":   a.UpdatedAt,
	}
}

// ListSpendAlerts returns all spend alerts for the given user.
//
//	GET /api/v1/billing/spend-alerts?user=:userId
func ListSpendAlerts(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	userId := resolveSpendAlertUserId(c)
	if userId == "" {
		http.Fail(c, 400, "user query parameter is required", nil)
		return
	}

	rootKey := db.NewKey("synckey", "", 1, nil)
	alerts := make([]*spendalert.SpendAlert, 0)
	q := spendalert.Query(db).Ancestor(rootKey).Filter("UserId=", userId)

	if _, err := q.GetAll(&alerts); err != nil {
		log.Error("Failed to list spend alerts: %v", err, c)
		http.Fail(c, 500, "failed to list spend alerts", err)
		return
	}

	items := make([]gin.H, 0, len(alerts))
	for _, a := range alerts {
		items = append(items, spendAlertResponse(a))
	}

	c.JSON(200, items)
}

// CreateSpendAlert creates a new spend alert for a user.
//
//	POST /api/v1/billing/spend-alerts
func CreateSpendAlert(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	var req createSpendAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	// Allow IAM-authenticated users to omit userId (use their own subject).
	if req.UserId == "" {
		if claims := iammiddleware.GetIAMClaims(c); claims != nil {
			req.UserId = claims.Subject
		}
	}

	if req.UserId == "" {
		http.Fail(c, 400, "userId is required", nil)
		return
	}

	if req.Threshold <= 0 {
		http.Fail(c, 400, "threshold must be positive", nil)
		return
	}

	cur := strings.ToLower(req.Currency)
	if cur == "" {
		cur = "usd"
	}

	a := spendalert.New(db)
	a.UserId = req.UserId
	a.Title = req.Title
	a.Threshold = req.Threshold
	a.Currency = cur

	if err := a.Create(); err != nil {
		log.Error("Failed to create spend alert: %v", err, c)
		http.Fail(c, 500, "failed to create spend alert", err)
		return
	}

	c.JSON(201, spendAlertResponse(a))
}

// UpdateSpendAlert updates title or threshold on an existing spend alert.
//
//	PATCH /api/v1/billing/spend-alerts/:id
func UpdateSpendAlert(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	id := c.Param("id")
	a := spendalert.New(db)
	if err := a.GetById(id); err != nil {
		http.Fail(c, 404, "spend alert not found", err)
		return
	}

	var req updateSpendAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		http.Fail(c, 400, "invalid request body", err)
		return
	}

	if req.Title != "" {
		a.Title = req.Title
	}

	if req.Threshold > 0 {
		a.Threshold = req.Threshold
	}

	if err := a.Update(); err != nil {
		log.Error("Failed to update spend alert: %v", err, c)
		http.Fail(c, 500, "failed to update spend alert", err)
		return
	}

	c.JSON(200, spendAlertResponse(a))
}

// DeleteSpendAlert deletes a spend alert by ID.
//
//	DELETE /api/v1/billing/spend-alerts/:id
func DeleteSpendAlert(c *gin.Context) {
	org := middleware.GetOrganization(c)
	db := datastore.New(org.Namespaced(c))

	id := c.Param("id")
	a := spendalert.New(db)
	if err := a.GetById(id); err != nil {
		http.Fail(c, 404, "spend alert not found", err)
		return
	}

	if err := a.Delete(); err != nil {
		log.Error("Failed to delete spend alert: %v", err, c)
		http.Fail(c, 500, "failed to delete spend alert", err)
		return
	}

	c.JSON(204, nil)
}
