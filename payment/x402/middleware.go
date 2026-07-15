package x402

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/zap-proto/zip"
)

// contextKey is a private type to avoid collisions in context values.
type contextKey string

const (
	// PaymentReceiptKey is the request-local key for the payment receipt.
	PaymentReceiptKey = "x402.receipt"
)

// Middleware returns a zip middleware that enforces x402 payment for configured routes.
// Routes not listed in cfg.Routes pass through without payment.
//
// Usage:
//
//	router.Use(x402.Middleware(cfg, facilitator))
func Middleware(cfg *PaywallConfig, facilitator *Facilitator) zip.Handler {
	return func(c *zip.Ctx) error {
		// Find matching route config for this request path.
		route := matchRoute(cfg, c.Path())
		if route == nil {
			// No payment required for this route.
			return c.Next()
		}

		// Check for payment authorization header.
		authHeader := c.Header(HeaderPaymentAuthorization)
		if authHeader == "" {
			// No payment provided — return 402 with payment request.
			paymentReq := NewPaymentRequest(cfg, route, c.Path())
			c.SetHeader(HeaderPaymentRequest, paymentReq.MarshalHeader())
			return c.JSON(http.StatusPaymentRequired, map[string]any{
				"error":           "payment required",
				"payment_request": paymentReq,
			})
		}

		// Parse the payment authorization.
		auth, err := ParsePaymentAuthorization(authHeader)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]any{
				"error": "invalid payment authorization: " + err.Error(),
			})
		}

		// Check time bounds.
		if auth.IsExpired() {
			return c.JSON(http.StatusPaymentRequired, map[string]any{
				"error": "payment authorization expired",
			})
		}
		if auth.IsNotYetValid() {
			return c.JSON(http.StatusPaymentRequired, map[string]any{
				"error": "payment authorization not yet valid",
			})
		}

		// Validate the payment amount matches what we asked for.
		if auth.Value != route.Amount {
			return c.JSON(http.StatusPaymentRequired, map[string]any{
				"error": "payment amount mismatch",
			})
		}

		// Settle the payment via the facilitator.
		paymentReq := NewPaymentRequest(cfg, route, c.Path())
		receipt, err := facilitator.Settle(c.Context(), paymentReq, auth)
		if err != nil {
			return c.JSON(http.StatusPaymentRequired, map[string]any{
				"error": "payment settlement failed: " + err.Error(),
			})
		}

		if !receipt.Success {
			return c.JSON(http.StatusPaymentRequired, map[string]any{
				"error": "payment settlement rejected",
			})
		}

		// Attach the receipt to the context and response.
		receiptJSON, _ := json.Marshal(receipt)
		c.SetHeader(HeaderPaymentReceipt, string(receiptJSON))
		c.Locals(PaymentReceiptKey, receipt)

		return c.Next()
	}
}

// NetHTTPMiddleware returns a standard net/http middleware for x402 payment.
// For use outside Gin (e.g., chi, standard http.Handler chains).
func NetHTTPMiddleware(cfg *PaywallConfig, facilitator *Facilitator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route := matchRoute(cfg, r.URL.Path)
			if route == nil {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get(HeaderPaymentAuthorization)
			if authHeader == "" {
				paymentReq := NewPaymentRequest(cfg, route, r.URL.Path)
				w.Header().Set(HeaderPaymentRequest, paymentReq.MarshalHeader())
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusPaymentRequired)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":           "payment required",
					"payment_request": paymentReq,
				})
				return
			}

			auth, err := ParsePaymentAuthorization(authHeader)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "invalid payment authorization: " + err.Error(),
				})
				return
			}

			if auth.IsExpired() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusPaymentRequired)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "payment authorization expired",
				})
				return
			}

			if auth.Value != route.Amount {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusPaymentRequired)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "payment amount mismatch",
				})
				return
			}

			paymentReq := NewPaymentRequest(cfg, route, r.URL.Path)
			receipt, err := facilitator.Settle(r.Context(), paymentReq, auth)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusPaymentRequired)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "payment settlement failed: " + err.Error(),
				})
				return
			}

			if !receipt.Success {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusPaymentRequired)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "payment settlement rejected",
				})
				return
			}

			receiptJSON, _ := json.Marshal(receipt)
			w.Header().Set(HeaderPaymentReceipt, string(receiptJSON))
			next.ServeHTTP(w, r)
		})
	}
}

// GetReceipt retrieves the payment receipt from the zip context.
// Returns nil if no payment was processed for this request.
func GetReceipt(c *zip.Ctx) *PaymentReceipt {
	val := c.Locals(PaymentReceiptKey)
	if val == nil {
		return nil
	}
	receipt, ok := val.(*PaymentReceipt)
	if !ok {
		return nil
	}
	return receipt
}

// matchRoute finds the RouteConfig that matches the given path.
// Supports exact matches and prefix matches (paths ending with /*).
func matchRoute(cfg *PaywallConfig, path string) *RouteConfig {
	if cfg.Routes == nil {
		return nil
	}

	// Exact match first.
	if route, ok := cfg.Routes[path]; ok {
		return route
	}

	// Prefix match: check paths ending with /*
	for pattern, route := range cfg.Routes {
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(path, prefix) {
				return route
			}
		}
	}

	return nil
}
