package billing

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/models/billinginvoice"
	"github.com/hanzoai/commerce/models/organization"
	"github.com/hanzoai/commerce/util/nscontext"
	"github.com/hanzoai/commerce/util/test/ae"
)

// TestRenderInvoicePDF_ValidStructure proves the hand-rolled writer emits a
// well-formed, deterministic PDF whose number and amount-due are greppable in
// the uncompressed content stream.
func TestRenderInvoicePDF_ValidStructure(t *testing.T) {
	inv := &billinginvoice.BillingInvoice{}
	inv.SetNumber(42)
	inv.Status = billinginvoice.Open
	inv.PeriodStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	inv.PeriodEnd = time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	inv.LineItems = []billinginvoice.LineItem{
		{Description: "Pro subscription", Amount: 2000, Quantity: 1, UnitPrice: 2000},
		{Description: "API usage (metered)", Amount: 1234, Quantity: 100, UnitPrice: 12},
	}
	inv.Subtotal = 3234
	inv.Tax = 100
	inv.Discount = 0
	inv.CreditApplied = 0
	inv.AmountDue = 3334

	pdf := renderInvoicePDF(inv, "Acme Inc", "hanzo")

	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("PDF must start with %%PDF-")
	}
	if !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Fatalf("PDF must end with %%%%EOF")
	}
	if len(pdf) < 400 {
		t.Fatalf("PDF suspiciously short: %d bytes", len(pdf))
	}
	if !bytes.Contains(pdf, []byte("INV-0042")) {
		t.Fatalf("PDF missing invoice number INV-0042")
	}
	if !bytes.Contains(pdf, []byte("$33.34")) {
		t.Fatalf("PDF missing amount-due $33.34")
	}
	if !bytes.Contains(pdf, []byte("Acme Inc")) {
		t.Fatalf("PDF missing org name")
	}

	// Deterministic: same invoice → identical bytes (idempotent render).
	if pdf2 := renderInvoicePDF(inv, "Acme Inc", "hanzo"); !bytes.Equal(pdf, pdf2) {
		t.Fatalf("render is not deterministic")
	}
}

// TestRenderInvoicePDF_EscapesAndDefaults proves PDF string metacharacters in
// user-controlled text are escaped (no stream break), and that an invoice with
// no brand is billed by Hanzo.
func TestRenderInvoicePDF_EscapesAndDefaults(t *testing.T) {
	inv := &billinginvoice.BillingInvoice{}
	inv.SetNumber(1)
	inv.Status = billinginvoice.Open
	inv.LineItems = []billinginvoice.LineItem{
		{Description: "weird (name) with \\ and ) paren", Amount: 500},
	}
	inv.Subtotal = 500
	inv.AmountDue = 500

	pdf := renderInvoicePDF(inv, "", "")
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) || !bytes.HasSuffix(pdf, []byte("%%EOF\n")) {
		t.Fatalf("escaped PDF is malformed")
	}
	if !bytes.Contains(pdf, []byte("(Hanzo)")) {
		t.Fatalf("an invoice with no brand must be billed by Hanzo")
	}
	// The raw '(' / ')' / '\' must appear only in escaped form inside the item text.
	if !bytes.Contains(pdf, []byte(`\(name\)`)) {
		t.Fatalf("parens in item text were not escaped")
	}
	if !bytes.Contains(pdf, []byte(`\\`)) {
		t.Fatalf("backslash in item text was not escaped")
	}
}

// TestRenderInvoicePDF_Issuer proves each brand bills under its own name and
// mark: Hanzo's footer carries its legal name and address, and a Lux or Zoo
// invoice names neither Hanzo nor Hanzo's address.
func TestRenderInvoicePDF_Issuer(t *testing.T) {
	inv := &billinginvoice.BillingInvoice{}
	inv.SetNumber(3)
	inv.Status = billinginvoice.Paid
	inv.LineItems = []billinginvoice.LineItem{{Description: "Pro", Amount: 2000}}
	inv.Subtotal, inv.AmountDue, inv.AmountPaid = 2000, 2000, 2000

	hanzo := renderInvoicePDF(inv, "Acme Inc", "hanzo")
	for _, want := range []string{"(Hanzo)", "Hanzo Industries Inc.", "995 Market St, San Francisco, CA 94103", "(PAID)", "/FontFile2", "Zen-Regular", "Zen-SemiBold"} {
		if !bytes.Contains(hanzo, []byte(want)) {
			t.Errorf("hanzo invoice lacks %q", want)
		}
	}
	for brand, name := range map[string]string{"lux": "(Lux)", "zoo": "(Zoo)", "LUX ": "(Lux)"} {
		pdf := renderInvoicePDF(inv, "Acme Inc", brand)
		if !bytes.Contains(pdf, []byte(name)) {
			t.Errorf("%s invoice lacks its name %s", brand, name)
		}
		if bytes.Contains(pdf, []byte("Hanzo")) || bytes.Contains(pdf, []byte("995 Market")) {
			t.Errorf("%s invoice names Hanzo", brand)
		}
	}
	if bytes.Equal(renderInvoicePDF(inv, "Acme Inc", "lux"), renderInvoicePDF(inv, "Acme Inc", "zoo")) {
		t.Fatalf("lux and zoo invoices are the same file")
	}
}

// TestRenderInvoicePDF_Pages proves a long invoice carries on to further pages
// and numbers them, rather than writing rows off the bottom of one.
func TestRenderInvoicePDF_Pages(t *testing.T) {
	inv := &billinginvoice.BillingInvoice{}
	inv.SetNumber(9)
	inv.Status = billinginvoice.Open
	for i := 0; i < 60; i++ {
		inv.LineItems = append(inv.LineItems, billinginvoice.LineItem{Description: "Usage", Amount: 100, Quantity: 1, UnitPrice: 100})
	}
	inv.Subtotal, inv.AmountDue = 6000, 6000
	pdf := renderInvoicePDF(inv, "Acme Inc", "hanzo")
	if !bytes.Contains(pdf, []byte("/Count 3")) {
		t.Fatalf("60 rows should take three pages")
	}
	if !bytes.Contains(pdf, []byte("Page 3 of 3")) || !bytes.Contains(pdf, []byte("(continued)")) {
		t.Fatalf("continued pages are not marked")
	}
}

// TestRenderInvoicePDF_Latin1 proves a name with accents keeps them, encoded in
// the pages' Windows-1252 as octal escapes, and a character outside it reads "?".
func TestRenderInvoicePDF_Latin1(t *testing.T) {
	inv := &billinginvoice.BillingInvoice{}
	inv.SetNumber(5)
	inv.LineItems = []billinginvoice.LineItem{{Description: "Caf\u00e9 \u2014 \u6771", Amount: 100}}
	pdf := renderInvoicePDF(inv, "Zo\u00eb GmbH", "hanzo")
	for _, want := range []string{`Zo\353 GmbH`, `Caf\351 \227 ?`} {
		if !bytes.Contains(pdf, []byte(want)) {
			t.Errorf("PDF lacks %s", want)
		}
	}
}

// pdfSeed scopes a request to org `ns`, matching the gateway plumbing so
// DownloadInvoicePDF resolves the ae datastore in that namespace.
func pdfSeed(ns string) func(*zip.Ctx) {
	return func(c *zip.Ctx) {
		org := &organization.Organization{}
		org.Name = ns
		c.Locals("organization", org)
		c.SetContext(nscontext.WithNamespace(context.Background(), ns))
	}
}

// TestDownloadInvoicePDF_TenantIsolation is the RED-focus test: an org can
// download ITS OWN invoice (200, application/pdf), but a DIFFERENT org handed
// the exact same invoice id gets 404 — the invoice is invisible outside its
// owning namespace.
func TestDownloadInvoicePDF_TenantIsolation(t *testing.T) {
	tc := ae.NewContext()
	defer tc.Close()

	// org A seeds an invoice in its OWN namespace.
	dbA := datastore.New(nscontext.WithNamespace(context.Background(), "pdf-org-a"))
	invA := billinginvoice.New(dbA)
	invA.UserId = "pdf-org-a/alice"
	invA.Status = billinginvoice.Open
	invA.SetNumber(7)
	invA.LineItems = []billinginvoice.LineItem{{Description: "Pro subscription", Amount: 5000}}
	invA.RecalculateSubtotal()
	invA.AmountDue = 5000
	if err := invA.Create(); err != nil {
		t.Fatalf("seed invoice: %v", err)
	}
	idA := invA.Id()

	// org A downloads its OWN invoice: 200 + PDF.
	{
		w := driveSeeded(pdfSeed("pdf-org-a"), "/v1/billing/invoices/:id/pdf",
			httptest.NewRequest(http.MethodGet, "/v1/billing/invoices/"+idA+"/pdf", nil), DownloadInvoicePDF)
		if w.StatusCode != 200 {
			t.Fatalf("org A own download: status = %d, body=%s", w.StatusCode, func() string { b, _ := io.ReadAll(w.Body); return string(b) }())
		}
		if ct := w.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/pdf") {
			t.Fatalf("Content-Type = %q, want application/pdf", ct)
		}
		if cd := w.Header.Get("Content-Disposition"); !strings.Contains(cd, "INV-0007.pdf") {
			t.Fatalf("Content-Disposition = %q, want attachment INV-0007.pdf", cd)
		}
		if !bytes.HasPrefix(func() []byte { b, _ := io.ReadAll(w.Body); return b }(), []byte("%PDF-")) {
			t.Fatalf("org A body is not a PDF")
		}
	}

	// org B handed org A's id: MUST 404 (cross-tenant read is impossible).
	{
		w := driveSeeded(pdfSeed("pdf-org-b"), "/v1/billing/invoices/:id/pdf",
			httptest.NewRequest(http.MethodGet, "/v1/billing/invoices/"+idA+"/pdf", nil), DownloadInvoicePDF)
		if w.StatusCode != 404 {
			t.Fatalf("CROSS-TENANT LEAK: org B got status %d for org A's invoice %s (want 404); body=%s",
				w.StatusCode, idA, func() string { b, _ := io.ReadAll(w.Body); return string(b) }())
		}
		if bytes.HasPrefix(func() []byte { b, _ := io.ReadAll(w.Body); return b }(), []byte("%PDF-")) {
			t.Fatalf("CROSS-TENANT LEAK: org B received PDF bytes for org A's invoice")
		}
	}
}
