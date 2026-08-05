// Package resources is the merchant CRUD table — the surfaces a shop owner
// edits: products and their variants, collections, discounts, sales channels,
// stock locations, subscribers, webhooks.
//
// It is its OWN package, and that is the whole point. The table used to live
// inside api.Route, so reaching it meant importing api — which drags checkout,
// subscriptions, and thirdparty/netlify behind it. hanzoai/cloud embeds commerce
// as a plugin and carries none of that, so it could not mount the merchant
// surface at all: every admin data view 404'd in production while sign-in,
// catalog and billing answered 200.
//
// A leaf that imports only the models and the REST binder can be mounted by
// both shapes commerce runs in. Standalone, api.Route calls it and the
// resources answer at `/v1/<kind>`. Embedded, cloud's commerce plugin calls it
// on the `/v1/commerce` group and they answer at `/v1/commerce/<kind>` — the
// endpoint every Hanzo surface is told to use.
//
// Every gate arrives as a parameter rather than being rebuilt here, so the two
// shapes cannot come to disagree about who may write a product.
package resources

import (
	"github.com/zap-proto/zip"

	return_ "github.com/hanzoai/commerce/models/return"

	"github.com/hanzoai/commerce/demo/disclosure"
	"github.com/hanzoai/commerce/demo/tokentransaction"
	"github.com/hanzoai/commerce/models/collection"
	"github.com/hanzoai/commerce/models/discount"
	"github.com/hanzoai/commerce/models/movie"
	"github.com/hanzoai/commerce/models/note"
	"github.com/hanzoai/commerce/models/product"
	"github.com/hanzoai/commerce/models/saleschannel"
	"github.com/hanzoai/commerce/models/site"
	"github.com/hanzoai/commerce/models/stocklocation"
	"github.com/hanzoai/commerce/models/submission"
	"github.com/hanzoai/commerce/models/subscriber"
	"github.com/hanzoai/commerce/models/transfer"
	"github.com/hanzoai/commerce/models/variant"
	"github.com/hanzoai/commerce/models/wallet"
	"github.com/hanzoai/commerce/models/watchlist"
	"github.com/hanzoai/commerce/models/webhook"

	"github.com/hanzoai/commerce/util/rest"
)

// Route registers the merchant resources on api.
//
// productEvents is the storefront's product.created/updated publisher. It is a
// parameter because it belongs to the standalone's event loop, not to the
// table: an embed that publishes nowhere passes nil and still gets its CRUD.
func Route(api zip.Router, tokenRequired, adminRequired, requireAccess, productEvents zip.Handler) {
	productMW := []zip.Handler{tokenRequired, requireAccess}
	if productEvents != nil {
		productMW = append(productMW, productEvents)
	}

	rest.New(collection.Collection{}).Route(api, tokenRequired, requireAccess)
	rest.New(discount.Discount{}).Route(api, tokenRequired, requireAccess)
	rest.New(movie.Movie{}).Route(api, tokenRequired)
	rest.New(note.Note{}).Route(api, tokenRequired)
	rest.New(product.Product{}).Route(api, productMW...)
	rest.New(return_.Return{}).Route(api, tokenRequired)
	rest.New(site.Site{}).Route(api, tokenRequired)
	rest.New(submission.Submission{}).Route(api, tokenRequired)
	rest.New(subscriber.Subscriber{}).Route(api, tokenRequired)
	// Transfer is the PAYMENT ANNOTATION on a payable, so writing one settles
	// money we owe. Admin-gated, not any token holder.
	rest.New(transfer.Transfer{}).Route(api, adminRequired)
	rest.New(variant.Variant{}).Route(api, tokenRequired, requireAccess)
	rest.New(wallet.Wallet{}).Route(api, adminRequired)
	rest.New(watchlist.Watchlist{}).Route(api, tokenRequired)
	rest.New(webhook.Webhook{}).Route(api, adminRequired)

	rest.New(saleschannel.SalesChannel{}).Route(api, tokenRequired, requireAccess)
	rest.New(stocklocation.StockLocation{}).Route(api, tokenRequired, requireAccess)

	rest.New(disclosure.Disclosure{}).Route(api, tokenRequired)
	rest.New(tokentransaction.Transaction{}).Route(api, tokenRequired)
}
