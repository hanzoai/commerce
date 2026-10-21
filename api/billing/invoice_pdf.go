package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/checkout"
	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/util/json/http"
)

// Document is a rendered attachment: the bytes and the base name they are
// offered under, without the extension the endpoint appends. The name travels with
// the bytes because it is derived from the row, and a reader that rebuilt it
// would need its own copy of that rule.
type Document struct {
	Filename string `json:"filename"`
	Body     []byte `json:"body"`
}

// InvoicePDF renders one invoice as an attachment — the RENDER, with no HTTP in
// it.
//
// It takes values so a caller that is not a request can ask: the download is
// served by a peer process that holds no ledger, and the alternative is shipping
// the invoice row across and re-rendering it there, which is a second renderer
// and therefore a second-looking invoice.
//
// Bytes cross this seam happily because the render is a PURE function of the
// row — one page, no timestamps, no random ids — so the same invoice renders the
// same bytes however often it is asked for, and a retry costs a re-render and
// nothing else.
//
// The org scopes the lookup at the storage layer, so an id from another tenant
// resolves to nothing rather than being found and then filtered.
//
// brand is the slug of the brand that bills (hanzo, lux, zoo, pars): its mark
// heads the page and its name and address close it. The caller knows it from the
// request; an unknown or empty slug is Hanzo.
func InvoicePDF(ctx context.Context, org *organization.Organization, id, brand string) (*Document, error) {
	if org == nil {
		return nil, errors.New("invoice pdf: no organization")
	}
	inv := billinginvoice.New(datastore.New(org.Namespaced(ctx)))
	if err := inv.GetById(id); err != nil {
		return nil, fmt.Errorf("invoice not found: %w", err)
	}

	orgName := org.FullName
	if orgName == "" {
		orgName = org.Name
	}

	name := inv.NumberStr
	if name == "" {
		name = "invoice-" + inv.Id()
	}
	return &Document{Filename: name, Body: renderInvoicePDF(inv, orgName, brand)}, nil
}

// DownloadInvoicePDF renders and serves an invoice as a PDF.
//
//	GET /v1/billing/invoices/:id/pdf
//
// Tenant isolation: the invoice is loaded from the caller's OWN org namespace
// (datastore.New(org.Namespaced(c.Context()))). GetById scopes the lookup to that
// namespace at the storage layer, so an org can only ever fetch its own
// invoice — a foreign id resolves to nothing and returns 404. It is mounted on
// the user group so a normal authenticated org member can download their own
// invoice. The brand that bills is the one whose host the request came to.
func DownloadInvoicePDF(c *zip.Ctx) error {
	// #146 class: never panic on a missing org (co-resident embed path — see ListInvoices).
	// No validated org ⇒ a clean 401 (no namespace to resolve the invoice in), never a panic.
	org, ok := middleware.GetOrganizationOK(c)
	if !ok || org == nil {
		return http.Fail(c, 401, "authentication required", nil)
	}

	doc, err := InvoicePDF(c.Context(), org, c.Param("id"), checkout.BrandSlugForHost(checkout.RequestHost(c)))
	if err != nil {
		return http.Fail(c, 404, "invoice not found", err)
	}

	c.SetHeader("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, doc.Filename))
	c.SetHeader("Content-Type", "application/pdf")
	return c.Bytes(200, doc.Body)
}

// ── Layout ───────────────────────────────────────────────────────────────────
//
// A US Letter page in two inks, black and a gray ladder, set in Zen: the
// brand's mark and name top left, the number and its status top right, who is
// billed and when beneath, the line items as a ruled table, the totals
// right-aligned under it, and the brand's legal name and address at the foot.
// Rows that do not fit carry on to the next page under the table's head again.
// The text stays uncompressed, so the number and the amounts are readable in
// the raw bytes.

const (
	pageW, pageH = 612.0, 792.0
	margin       = 48.0
	edge         = pageW - margin // the right edge every right-aligned figure meets

	qtyX    = 372.0 // right edges of the table's figure columns
	unitX   = 468.0
	labelX  = 340.0 // where the totals' labels start
	footY   = 46.0
	floorY  = 104.0 // nothing but the footer sits below this
	rowStep = 22.0
)

// The gray ladder: ink, quiet ink, labels, rules.
const (
	ink   = 0.0
	quiet = 0.35
	label = 0.47
	hair  = 0.86
)

// issuer is the brand that bills: what heads the invoice and what closes it.
type issuer struct {
	name    string // set beside the mark
	legal   string // the footer's first words; the name when the brand states none
	address string
	site    string
	mark    *mark
}

// Only Hanzo's legal name and address are stated, so only Hanzo's footer
// carries them; the others close with their name and site.
var issuers = map[string]issuer{
	"hanzo": {name: "Hanzo", legal: "Hanzo Industries Inc.", address: "995 Market St, San Francisco, CA 94103", site: "hanzo.ai", mark: hanzoMark},
	"lux":   {name: "Lux", legal: "Lux", site: "lux.network", mark: luxMark},
	"zoo":   {name: "Zoo", legal: "Zoo", site: "zoo.ngo", mark: zooMark},
	"pars":  {name: "Pars", legal: "Pars", site: "pars.network"},
}

func issuerFor(brand string) issuer {
	if is, ok := issuers[strings.ToLower(strings.TrimSpace(brand))]; ok {
		return is
	}
	return issuers["hanzo"]
}

// statusPill is how each status is labelled: paid is the one solid pill.
func statusPill(s billinginvoice.Status) (text string, gray float64, solid bool) {
	switch s {
	case billinginvoice.Paid:
		return "PAID", ink, true
	case billinginvoice.Open:
		return "DUE", ink, false
	case "":
		return "DRAFT", label, false
	}
	return strings.ToUpper(string(s)), label, false
}

// caps sets a small tracked label.
func caps(c *canvas, x, y float64, s string) { c.text(regular, 7.5, x, y, label, 0.9, s) }

func capsRight(c *canvas, x, y float64, s string) { c.textRight(regular, 7.5, x, y, label, 0.9, s) }

// renderInvoicePDF draws the invoice billed by brand to orgName.
func renderInvoicePDF(inv *billinginvoice.BillingInvoice, orgName, brand string) []byte {
	is := issuerFor(brand)
	number := inv.NumberStr
	if number == "" {
		number = "INV"
	}
	billed := strings.TrimSpace(orgName)
	if billed == "" {
		billed = inv.CustomerEmail
	}
	if billed == "" {
		billed = "-"
	}
	money := func(cents int64) string { return fmtAmount(inv.Currency, cents) }

	var pages []*canvas
	var c *canvas
	var y float64

	// brandLine sets the mark and the brand's name with the mark's foot at y.
	brandLine := func(y, size, nameSize float64) {
		x := margin
		if is.mark != nil {
			c.mark(is.mark, x, y, size)
			x += size + 10
		}
		c.text(semibold, nameSize, x, y+size/2-nameSize*0.35, ink, 0, is.name)
	}
	tableHead := func() {
		caps(c, margin, y, "DESCRIPTION")
		capsRight(c, qtyX, y, "QTY")
		capsRight(c, unitX, y, "UNIT PRICE")
		capsRight(c, edge, y, "AMOUNT")
		y -= 9
		c.rule(margin, edge, y, 0.75, 0.15)
		y -= rowStep - 5
	}
	newPage := func() {
		c = &canvas{}
		pages = append(pages, c)
		if len(pages) > 1 {
			// A continued page: the brand, the number, and the table's head again.
			brandLine(730, 18, 11)
			c.textRight(semibold, 11, edge, 734, ink, 0, number)
			c.textRight(regular, 8, edge, 722, label, 0, "continued")
			c.rule(margin, edge, 708, 0.5, hair)
			y = 684
			tableHead()
		}
	}

	// ── Page one: the head ────────────────────────────────────────────────
	newPage()
	brandLine(718, 26, 15)
	capsRight(c, edge, 738, "INVOICE")
	c.textRight(semibold, 22, edge, 714, ink, 0, number)
	text, gray, solid := statusPill(inv.Status)
	c.pill(text, edge, 688, gray, solid)
	c.rule(margin, edge, 668, 0.5, hair)

	// Who is billed, and the dates that matter to them.
	y = 640
	caps(c, margin, y, "BILLED TO")
	c.text(semibold, 11, margin, y-16, ink, 0, fit(semibold, 11, billed, 200))
	if inv.CustomerEmail != "" && inv.CustomerEmail != billed {
		c.text(regular, 9, margin, y-30, quiet, 0, fit(regular, 9, inv.CustomerEmail, 200))
	}
	cols := []struct {
		x           float64
		head, value string
	}{
		{268, "ISSUED", fmtDate(inv.CreatedAt)},
		{360, "", ""},
		{452, "PERIOD", fmtPeriod(inv.PeriodStart, inv.PeriodEnd)},
	}
	switch {
	case inv.Status == billinginvoice.Paid && !inv.PaidAt.IsZero():
		cols[1].head, cols[1].value = "PAID", fmtDate(inv.PaidAt)
	case inv.Status == billinginvoice.Void && !inv.VoidedAt.IsZero():
		cols[1].head, cols[1].value = "VOIDED", fmtDate(inv.VoidedAt)
	default:
		cols[1].head, cols[1].value = "DUE", fmtDate(inv.DueDate)
	}
	for _, col := range cols {
		caps(c, col.x, y, col.head)
		c.text(regular, 10, col.x, y-16, ink, 0, fit(regular, 10, col.value, edge-col.x))
	}

	// The one figure a reader came for, in words.
	y = 574
	headline := money(inv.AmountDue) + " due"
	switch {
	case inv.Status == billinginvoice.Paid:
		paid := inv.AmountPaid
		if paid == 0 {
			paid = inv.AmountDue
		}
		headline = money(paid) + " paid"
		if !inv.PaidAt.IsZero() {
			headline += " " + fmtDate(inv.PaidAt)
		}
	case inv.Status == billinginvoice.Void:
		headline = "Voided"
	case !inv.DueDate.IsZero():
		headline += " " + fmtDate(inv.DueDate)
	}
	c.text(semibold, 16, margin, y, ink, 0, headline)

	// ── The line items ────────────────────────────────────────────────────
	y = 532
	tableHead()
	descW := qtyX - margin - 56
	for _, li := range inv.LineItems {
		if y < floorY+8 {
			newPage()
		}
		c.text(regular, 10, margin, y, ink, 0, fit(regular, 10, li.Description, descW))
		if li.Quantity != 0 {
			c.textRight(regular, 10, qtyX, y, quiet, 0, fmt.Sprintf("%d", li.Quantity))
		}
		if li.UnitPrice != 0 {
			c.textRight(regular, 10, unitX, y, quiet, 0, money(li.UnitPrice))
		}
		c.textRight(regular, 10, edge, y, ink, 0, money(li.Amount))
		c.rule(margin, edge, y-8, 0.5, hair)
		y -= rowStep
	}

	// ── The totals, right-aligned under the amounts ───────────────────────
	type total struct {
		label string
		cents int64
	}
	totals := []total{{"Subtotal", inv.Subtotal}}
	if inv.Discount != 0 {
		l := "Discount"
		if inv.DiscountName != "" {
			l += " (" + inv.DiscountName + ")"
		}
		totals = append(totals, total{l, -inv.Discount})
	}
	if inv.Tax != 0 || inv.TaxPercent != 0 {
		l := "Tax"
		if inv.TaxPercent != 0 {
			l += " (" + strconvPercent(inv.TaxPercent) + ")"
		}
		totals = append(totals, total{l, inv.Tax})
	}
	if inv.CreditApplied != 0 {
		totals = append(totals, total{"Credit applied", -inv.CreditApplied})
	}
	need := float64(len(totals))*18 + 48
	if inv.AmountPaid != 0 {
		need += 18
	}
	if y-need < floorY {
		newPage()
	}
	y -= 4
	for _, t := range totals {
		c.text(regular, 10, labelX, y, quiet, 0, fit(regular, 10, t.label, unitX-labelX))
		c.textRight(regular, 10, edge, y, ink, 0, money(t.cents))
		y -= 18
	}
	y += 6
	c.rule(labelX, edge, y, 0.75, 0.15)
	y -= 18
	due := "Amount due"
	if inv.Status == billinginvoice.Paid {
		due = "Total"
	}
	c.text(semibold, 12, labelX, y, ink, 0, due)
	c.textRight(semibold, 12, edge, y, ink, 0, money(inv.AmountDue))
	if inv.AmountPaid != 0 {
		y -= 18
		c.text(regular, 10, labelX, y, quiet, 0, "Paid")
		c.textRight(regular, 10, edge, y, ink, 0, money(-inv.AmountPaid))
	}

	// ── The foot of every page ────────────────────────────────────────────
	foot := is.legal
	if is.address != "" {
		foot += "  ·  " + is.address
	}
	for i, p := range pages {
		p.rule(margin, edge, footY+16, 0.5, hair)
		p.text(regular, 7.5, margin, footY, label, 0, foot)
		right := is.site
		if len(pages) > 1 {
			right = fmt.Sprintf("%s  ·  Page %d of %d", is.site, i+1, len(pages))
		}
		p.textRight(regular, 7.5, edge, footY, label, 0, right)
	}

	streams := make([]string, len(pages))
	for i, p := range pages {
		streams[i] = p.b.String()
	}
	return writePDF(is.name+" invoice "+number, streams)
}

// strconvPercent prints a tax rate as a person writes it: 8.25%, 20%.
func strconvPercent(p float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", p), "0"), ".") + "%"
}

// fmtAmount renders a cents figure in the invoice's own currency ("$33.34",
// "-$5.00").
//
// The rendering is currency.Type's — the hand-rolled spelling this replaces
// printed cents/100, a dot, then cents%100, which needs a manual sign fix
// because Go's % keeps the sign of the dividend.
//
// An invoice written before the currency column existed carries none, and that
// column's default is usd, so an unset currency reads as usd here rather than
// rendering a bare number with no symbol at all.
func fmtAmount(cur currency.Type, cents int64) string {
	if cur == "" {
		cur = currency.USD
	}
	return cur.ToString(currency.Cents(cents))
}

// fmtDate renders a UTC calendar date, "Jan 2, 2026", or "-" for none.
func fmtDate(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format("Jan 2, 2006")
}

// fmtPeriod renders a billing period, its year said once when both ends share it:
// "Jan 1 – Feb 1, 2026", "Dec 15, 2025 – Jan 15, 2026".
func fmtPeriod(start, end time.Time) string {
	switch {
	case start.IsZero() && end.IsZero():
		return "-"
	case start.IsZero() || end.IsZero():
		return fmtDate(start) + " – " + fmtDate(end)
	case start.UTC().Year() == end.UTC().Year():
		return start.UTC().Format("Jan 2") + " – " + end.UTC().Format("Jan 2, 2006")
	}
	return fmtDate(start) + " – " + fmtDate(end)
}
