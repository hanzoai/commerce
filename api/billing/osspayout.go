package billing

import (
	"cmp"
	"slices"
	"strings"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/log"
	"github.com/hanzoai/commerce/models/ossaccrual"
	"github.com/hanzoai/commerce/models/sbomrecord"
	"github.com/hanzoai/commerce/util/json/http"

	. "github.com/hanzoai/commerce/types"
)

// systemDB returns a datastore for the global "system" namespace, where SBOM
// records and OSS accruals live (platform-wide, not per-tenant).
func systemDB(c *zip.Ctx) *datastore.Datastore {
	ctx := c.Context()
	db := datastore.New(ctx)
	db.SetNamespace("system")
	return db
}

// ── SBOM ingest ───────────────────────────────────────────────────────────

type sbomComponent struct {
	PURL      string `json:"purl"`
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
	Version   string `json:"version"`
	Scope     string `json:"scope"` // direct | transitive
}

type sbomIngestRequest struct {
	ImageRef    string          `json:"imageRef"`
	ImageDigest string          `json:"imageDigest"`
	Service     string          `json:"service"`
	Format      string          `json:"format"` // cyclonedx | spdx
	Tool        string          `json:"tool"`   // syft
	Components  []sbomComponent `json:"components"`
}

// IngestSBOM stores the normalized SBOM for a built image.
//
//	POST /v1/billing/sbom
//
// Called by the arcd build pipeline after `docker push`: it runs
// `syft <image> -o cyclonedx-json`, normalizes the component graph to
// {purl, name, ecosystem, version, scope}, and POSTs it here keyed by the
// immutable image digest. Idempotent on ImageDigest — re-ingesting the same
// image updates the record in place.
func IngestSBOM(c *zip.Ctx) error {
	db := systemDB(c)

	var req sbomIngestRequest
	if err := c.Bind(&req); err != nil {
		return http.Fail(c, 400, "invalid request body", err)
	}
	req.ImageDigest = strings.TrimSpace(req.ImageDigest)
	if req.ImageDigest == "" {
		return http.Fail(c, 400, "imageDigest is required", nil)
	}

	in := sbomrecord.New(db)
	in.ImageRef = req.ImageRef
	in.ImageDigest = req.ImageDigest
	in.Service = req.Service
	in.Format = req.Format
	in.Tool = req.Tool
	in.Components = normalizeComponents(req.Components)

	rec, err := sbomrecord.Ingest(db, in)
	if err != nil {
		log.Error("sbom ingest failed for %s: %v", req.ImageDigest, err, c)
		return http.Fail(c, 500, "failed to store SBOM", err)
	}

	return c.JSON(201, map[string]any{
		"id":             rec.Id(),
		"imageDigest":    rec.ImageDigest,
		"service":        rec.Service,
		"componentCount": rec.ComponentCount,
	})
}

// normalizeComponents converts ingest components to model components, defaulting
// scope to "transitive" unless explicitly "direct". Shared by HTTP + ZAP paths.
func normalizeComponents(in []sbomComponent) []sbomrecord.Component {
	out := make([]sbomrecord.Component, 0, len(in))
	for _, comp := range in {
		scope := comp.Scope
		if scope != "direct" {
			scope = "transitive"
		}
		out = append(out, sbomrecord.Component{
			PURL:      strings.TrimSpace(comp.PURL),
			Name:      comp.Name,
			Ecosystem: comp.Ecosystem,
			Version:   comp.Version,
			Scope:     scope,
		})
	}
	return out
}

// ListSBOMs returns the stored SBOM records (metadata only, not components).
//
//	GET /v1/billing/sbom
func ListSBOMs(c *zip.Ctx) error {
	db := systemDB(c)
	records := make([]*sbomrecord.SBOMRecord, 0, 64)
	rootKey := db.NewKey("synckey", "", 1, nil)
	if _, err := sbomrecord.Query(db).Ancestor(rootKey).GetAll(&records); err != nil {
		return http.Fail(c, 500, "failed to query SBOMs", err)
	}
	items := make([]map[string]any, 0, len(records))
	for _, r := range records {
		items = append(items, map[string]any{
			"id":             r.Id(),
			"imageRef":       r.ImageRef,
			"imageDigest":    r.ImageDigest,
			"service":        r.Service,
			"format":         r.Format,
			"tool":           r.Tool,
			"componentCount": r.ComponentCount,
			"createdAt":      r.CreatedAt,
		})
	}
	return c.JSON(200, map[string]any{"count": len(items), "sboms": items})
}

// ── OSS accrual reads ───────────────────────────────────────────────────────

// ListOSSAccruals returns accrual ledger lines, optionally filtered by package
// PURL or spending org.
//
//	GET /v1/billing/oss-accruals?purl=&org=
func ListOSSAccruals(c *zip.Ctx) error {
	db := systemDB(c)
	q := ossaccrual.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil))
	if purl := strings.TrimSpace(c.Query("purl")); purl != "" {
		q = q.Filter("PURL=", purl)
	}
	if org := strings.TrimSpace(c.Query("org")); org != "" {
		q = q.Filter("SpendOrg=", org)
	}

	accruals := make([]*ossaccrual.OSSAccrual, 0, 256)
	if _, err := q.GetAll(&accruals); err != nil {
		return http.Fail(c, 500, "failed to query accruals", err)
	}

	items := make([]map[string]any, 0, len(accruals))
	for _, a := range accruals {
		items = append(items, map[string]any{
			"id":            a.Id(),
			"purl":          a.PURL,
			"name":          a.Name,
			"scope":         a.Scope,
			"spendOrg":      a.SpendOrg,
			"sourceTxnId":   a.SourceTxnID,
			"amount":        a.Amount,
			"currency":      a.Currency,
			"status":        a.Status,
			"fundingTarget": a.FundingTarget,
			"fundingKind":   a.FundingKind,
			"shareRatio":    a.ShareRatio,
			"createdAt":     a.CreatedAt,
		})
	}
	return c.JSON(200, map[string]any{"count": len(items), "accruals": items})
}

// maintainerSummary is the per-package rollup: total owed across all accrual
// lines for one package, plus its resolved funding target.
type maintainerSummary struct {
	PURL          string `json:"purl"`
	Name          string `json:"name"`
	Ecosystem     string `json:"ecosystem"`
	Scope         string `json:"scope"`
	FundingTarget string `json:"fundingTarget"`
	FundingKind   string `json:"fundingKind"`
	Currency      string `json:"currency"`
	AccruedCents  int64  `json:"accruedCents"`
	Lines         int    `json:"lines"`
	Status        string `json:"status"`
}

// GetOSSPayoutSummary rolls up accruals per package: the running total Hanzo
// owes each OSS package and where it would be paid. This is the payout-ready
// view — the disbursement job consumes it.
//
//	GET /v1/billing/oss-payout/summary?org=
func GetOSSPayoutSummary(c *zip.Ctx) error {
	db := systemDB(c)
	q := ossaccrual.Query(db).Ancestor(db.NewKey("synckey", "", 1, nil))
	if org := strings.TrimSpace(c.Query("org")); org != "" {
		q = q.Filter("SpendOrg=", org)
	}

	accruals := make([]*ossaccrual.OSSAccrual, 0, 256)
	if _, err := q.GetAll(&accruals); err != nil {
		return http.Fail(c, 500, "failed to query accruals", err)
	}

	byPURL := map[string]*maintainerSummary{}
	var totalCents int64
	heldCents := int64(0)
	for _, a := range accruals {
		s, ok := byPURL[a.PURL]
		if !ok {
			s = &maintainerSummary{
				PURL:          a.PURL,
				Name:          a.Name,
				Ecosystem:     a.Ecosystem,
				Scope:         string(a.Scope),
				FundingTarget: a.FundingTarget,
				FundingKind:   a.FundingKind,
				Currency:      string(a.Currency),
				Status:        string(a.Status),
			}
			byPURL[a.PURL] = s
		}
		s.AccruedCents += int64(a.Amount)
		s.Lines++
		totalCents += int64(a.Amount)
		if a.Status == ossaccrual.Held {
			heldCents += int64(a.Amount)
		}
	}

	maintainers := make([]*maintainerSummary, 0, len(byPURL))
	for _, s := range byPURL {
		maintainers = append(maintainers, s)
	}
	// Stable, useful order: largest owed first, then PURL.
	slices.SortStableFunc(maintainers, func(a, b *maintainerSummary) int {
		return cmp.Or(
			cmp.Compare(b.AccruedCents, a.AccruedCents), // highest first
			cmp.Compare(a.PURL, b.PURL),
		)
	})

	return c.JSON(200, map[string]any{
		"totalAccruedCents": totalCents,
		"heldCents":         heldCents,
		"packageCount":      len(maintainers),
		"maintainers":       maintainers,
		"metadata":          Map{"capFraction": 0.25},
	})
}
