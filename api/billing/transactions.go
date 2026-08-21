package billing

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/transaction/util"
	"github.com/hanzoai/commerce/util/json/http"
)

// Transaction is one ledger entry as this surface reports it: cents, a currency
// code, and timestamps to the second in UTC.
type Transaction struct {
	Id       string `json:"id"`
	Type     string `json:"type"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Tags     string `json:"tags,omitempty"`
	Notes    string `json:"notes,omitempty"`
	// Metadata is whatever was attached to the entry, carried as the bytes it
	// arrived as. Raw JSON is the one free-form shape that also crosses the
	// internal plane, where a map has no type.
	Metadata  json.RawMessage `json:"metadata,omitempty"`
	CreatedAt string          `json:"createdAt"`
	ExpiresAt string          `json:"expiresAt,omitempty"`
}

// TransactionList is one page of a subject's ledger. Count is the size of the
// whole history, not of the page — that difference is how a reader knows there
// is more to ask for. Shaped like the plane's other list answers (billing.zap
// UsageList): the subject, the count, the rows.
type TransactionList struct {
	Transactions []Transaction `json:"transactions"`
	Count        int           `json:"count"`
	User         string        `json:"user"`
}

// errNoSubject is the ledger refusal: a history is one subject's, and none was
// named.
var errNoSubject = errors.New("transactions: no subject")

// IsTransactionRefusal reports whether err is that refusal.
// ListBillingTransactions renders it as its 400, and a caller that reached the
// core another way refuses the same ask the same way instead of reporting a
// fault the store never had.
func IsTransactionRefusal(err error) bool { return errors.Is(err, errNoSubject) }

// ListTransactions is a subject's ledger, newest first, one page of it — the
// QUERY, with no HTTP in it.
//
// It takes values rather than a request because the reader is not always a
// request: the same history is read over the internal plane by a peer that
// holds no ledger of its own, and a second copy of this query is how a billing
// page and a spend gate come to disagree about what a user has spent.
//
// It normalizes what it filters on, so the answer does not depend on how the
// asker capitalized the subject. An empty currency means every currency; a
// limit at or below zero means the default page of 100.
func ListTransactions(ctx context.Context, org *organization.Organization, user, cur string, limit, offset int) (*TransactionList, error) {
	if org == nil {
		return nil, errors.New("transactions: no organization")
	}
	user = strings.ToLower(strings.TrimSpace(user))
	if user == "" {
		return nil, errNoSubject
	}
	cur = strings.ToLower(strings.TrimSpace(cur))
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	datas, err := util.GetTransactions(org.Namespaced(ctx), user, "iam-user", org.TestMode())
	if err != nil {
		return nil, err
	}

	// Flatten all transactions across currencies.
	all := make([]Transaction, 0)
	for code, data := range datas.Data {
		if cur != "" && string(code) != cur {
			continue
		}
		for _, tx := range data.Transactions {
			r := Transaction{
				Id:        tx.Id(),
				Type:      string(tx.Type),
				Amount:    int64(tx.Amount),
				Currency:  string(tx.Currency),
				Tags:      tx.Tags,
				Notes:     tx.Notes,
				CreatedAt: tx.GetCreatedAt().Format("2006-01-02T15:04:05Z"),
			}
			if len(tx.Metadata) > 0 {
				if b, err := json.Marshal(tx.Metadata); err == nil {
					r.Metadata = b
				}
			}
			if !tx.ExpiresAt.IsZero() {
				r.ExpiresAt = tx.ExpiresAt.Format("2006-01-02T15:04:05Z")
			}
			all = append(all, r)
		}
	}

	// Sort newest first.
	slices.SortFunc(all, func(a, b Transaction) int {
		return cmp.Compare(b.CreatedAt, a.CreatedAt) // newest first
	})

	// Apply limit + offset.
	total := len(all)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	return &TransactionList{Transactions: all[offset:end], Count: total, User: user}, nil
}

// ListBillingTransactions returns transactions for an IAM user, newest first.
//
//	GET /v1/billing/transactions?user=hanzo/alice&limit=100&offset=0&currency=usd
//
// Response: { "transactions": [...], "count": N, "user": "hanzo/alice" }
func ListBillingTransactions(c *zip.Ctx) error {
	org := middleware.GetOrganization(c)

	user := strings.ToLower(strings.TrimSpace(c.Query("user")))
	if user == "" {
		return http.Fail(c, 400, "user query parameter is required", nil)
	}

	// Absent or unreadable paging leaves the values at zero, which the core
	// reads as its default page.
	limit, offset := 0, 0
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	list, err := ListTransactions(c.Context(), org, user, c.Query("currency"), limit, offset)
	if err != nil {
		return http.Fail(c, 500, "failed to query transactions", err)
	}

	return c.JSON(200, list)
}
