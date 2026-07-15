package order

import (
	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/order"
	"github.com/hanzoai/commerce/models/payment"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/models/wallet"
	// "github.com/hanzoai/commerce/util/json"
	"github.com/hanzoai/commerce/util/json/http"
)

type StatusResponse struct {
	Id            string         `json:"id"`
	Total         currency.Cents `json:"total"`
	Paid          currency.Cents `json:"paid"`
	Currency      currency.Type  `json:"currency"`
	Status        order.Status   `json:"status"`
	PaymentStatus payment.Status `json:"paymentStatus"`
	Wallet        *wallet.Wallet `json:"wallet,omitempty"`
}

func Status(c *zip.Ctx) error {
	id := c.Param("orderid")
	// Per-org store (Red MED-1): read the order from the caller org's store, not
	// the shared systemDB (datastore.New drops the namespace + binds systemDB).
	org := middleware.GetOrganization(c)
	db := datastore.NewNamespaced(org.Namespaced(c.Context()))
	ord := order.New(db)

	// Ensure order exists
	if err := ord.GetById(id); err != nil {
		return http.Fail(c, 404, "No order found with id: "+id, err)
	}

	wal, err := ord.GetOrCreateWallet(db)
	if err != nil {
		log.Warn("Order '%v' has no wallet due to error: '%v'", ord.Id_, err, c)
	}

	res := &StatusResponse{
		Id:            ord.Id_,
		Total:         ord.Total,
		Paid:          ord.Paid,
		Currency:      ord.Currency,
		Status:        ord.Status,
		PaymentStatus: ord.PaymentStatus,
		Wallet:        wal,
	}

	return http.Render(c, 200, res)
}
