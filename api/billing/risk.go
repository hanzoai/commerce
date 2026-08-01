// Copyright © 2026 Hanzo AI. MIT License.

package billing

// risk.go is the money plane's risk face: /v1/billing/risk.
//
// Every route here is a TYPED zip op, so the one declaration is the REST route,
// the OpenAPI operation, the MCP tool, the CLI command and the generated SDKs.
// Every one of them reaches /v1/risk for the judgement and none of them scores:
// Hanzo Risk decides, commerce enforces. What lives here is what only the money
// plane can do — hold a reserve exactly, refuse a payout, assemble a dispute
// defence out of the books.
//
// The ORG IS NEVER AN INPUT FIELD. It is bound from the validated principal by
// middleware.Bind and read with middleware.OrgFrom; the datastore built from it
// can only see that tenant's rows. An op that took a tenant as an argument would
// be a cross-tenant read the caller asserted for itself.
//
//go:generate go run github.com/zap-proto/zip/cmd/zipdoc

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/zap-proto/zip"

	"github.com/hanzoai/commerce/datastore"
	"github.com/hanzoai/commerce/middleware"
	"github.com/hanzoai/commerce/models/control"
	"github.com/hanzoai/commerce/models/outcome"
	"github.com/hanzoai/commerce/models/screen"
	"github.com/hanzoai/commerce/models/types/currency"
	"github.com/hanzoai/commerce/payment/processor"
	"github.com/hanzoai/commerce/risk"
	"github.com/hanzoai/commerce/util/permission"
)

// pageMax bounds a list read. A caller that names no limit gets this many, and
// one that names more gets this many — an unbounded list of a busy merchant's
// screens is a way to take the store down, not a feature.
const pageMax = 200

// RiskRoute registers the money plane's risk face.
//
// It takes the ROOT router and names the whole path in one literal, unlike the
// /v1/billing group beside it, for a mechanical reason: the doc generator
// resolves an op's identity from the group literal it can see in this file, so
// a prefix assembled from a parameter would file every doc comment under a path
// that does not exist. The group carries its own auth rather than inheriting
// the neighbouring group's by path prefix, so the gate does not depend on the
// order two registrations happened to run in.
func RiskRoute(root *zip.App) {
	g := root.Group("/v1/billing/risk")
	g.Use(middleware.TokenRequired(permission.Admin))
	g.Use(middleware.Bind())

	zip.Post(g, "/screen", riskScreen,
		zip.WithOperationID("riskScreen"),
		zip.WithSummary("Screen a money move and return the decision"),
		zip.WithTags("risk"))

	zip.Get(g, "/screens", riskScreens,
		zip.WithOperationID("riskScreens"),
		zip.WithSummary("List recorded screens"),
		zip.WithTags("risk"))

	zip.Get(g, "/screens/:id", riskScreenView,
		zip.WithOperationID("riskScreenView"),
		zip.WithSummary("Read one recorded screen"),
		zip.WithTags("risk"))

	zip.Get(g, "/controls", riskControls,
		zip.WithOperationID("riskControls"),
		zip.WithSummary("List the controls in force"),
		zip.WithTags("risk"))

	zip.Post(g, "/controls", riskControlPlace,
		zip.WithOperationID("riskControlPlace"),
		zip.WithSummary("Place a reserve, payout hold or block on a subject"),
		zip.WithTags("risk"),
		zip.WithStatus(http.StatusCreated))

	zip.Delete(g, "/controls/:id", riskControlRelease,
		zip.WithOperationID("riskControlRelease"),
		zip.WithSummary("Release a control"),
		zip.WithTags("risk"))

	zip.Get(g, "/merchants/:id", riskMerchant,
		zip.WithOperationID("riskMerchant"),
		zip.WithSummary("Read a merchant's standing from this org's own record"),
		zip.WithTags("risk"))

	zip.Post(g, "/merchants/:id/review", riskMerchantReview,
		zip.WithOperationID("riskMerchantReview"),
		zip.WithSummary("Review a merchant now, and act on the answer"),
		zip.WithTags("risk"))

	zip.Post(g, "/outcomes", riskOutcome,
		zip.WithOperationID("riskOutcome"),
		zip.WithSummary("Report how a scored money event turned out"),
		zip.WithTags("risk"),
		zip.WithStatus(http.StatusCreated))

	zip.Get(g, "/disputes/:id/evidence", riskEvidence,
		zip.WithOperationID("riskEvidence"),
		zip.WithSummary("Assemble a dispute defence from this org's record"),
		zip.WithTags("risk"))

	zip.Post(g, "/disputes/:id/submit", riskSubmit,
		zip.WithOperationID("riskSubmit"),
		zip.WithSummary("Submit an assembled dispute defence to the adjudicator"),
		zip.WithTags("risk"))
}

// -----------------------------------------------------------------------------
// the tenant seam
// -----------------------------------------------------------------------------

// screener resolves the org-scoped screener for this request. Every op starts
// here and there is no other way into the store from this file, so the tenant
// gate is one expression that cannot be forgotten one route at a time.
func screener(ctx context.Context) (*risk.Screener, error) {
	org, ok := middleware.OrgFrom(ctx)
	if !ok {
		return nil, zip.ErrForbidden("no organization on this request")
	}
	return &risk.Screener{
		DB: datastore.New(org.Namespaced(ctx)),
		By: middleware.WhoFrom(ctx),
	}, nil
}

// -----------------------------------------------------------------------------
// screen
// -----------------------------------------------------------------------------

// riskScreenIn is one money move put to the risk plane.
//
// Amount is EXACT MINOR UNITS of Currency — cents, not dollars, never a float.
// A money value that arrives as a float has already lost the cent it was going
// to lose, before any code here runs.
type riskScreenIn struct {
	// Stage is the lifecycle moment: payment, usage, payout, dispute or
	// merchant. It selects the feature window and rule set on the scoring plane.
	Stage string `json:"stage" validate:"required"`
	// SubjectKind is what is being judged: merchant, customer, account,
	// transaction or payout.
	SubjectKind string `json:"subjectKind" validate:"required"`
	// Subject is the subject's id within this org.
	Subject string `json:"subject" validate:"required"`
	// Amount is exact minor units. Zero is allowed: a merchant review judges no
	// particular move.
	Amount int64 `json:"amount,omitempty"`
	// Currency is the ISO code of Amount.
	Currency string `json:"currency,omitempty"`
	// Out is true when money is LEAVING — a payout or a refund. Reserves and
	// payout holds bear only on money that leaves.
	Out bool `json:"out,omitempty"`
	// Signals are the facts the scoring plane reads: ip, email, device, bin,
	// asn, ua and their kin.
	Signals map[string]string `json:"signals,omitempty"`
	// Reference is the money object being judged — a payment intent, a payout,
	// an order — so the record joins back to the books.
	Reference string `json:"reference,omitempty"`
	// Processor names the gateway, when there is one. A merchant who processes
	// payments elsewhere leaves it empty and is screened just the same.
	Processor string `json:"processor,omitempty"`
	// Idem makes a repeated screen of the same move return the first answer
	// rather than a second, possibly different, one.
	Idem string `json:"idem,omitempty"`
}

// riskScreenOut is the recorded judgement of one move.
//
// Example: {"stage":"payment","subjectKind":"customer","subject":"cus_1","amount":4200,"currency":"usd"}
// Response: {"id":"scr_1","action":"allow","allowed":4200,"held":0}
type riskScreenOut struct {
	// ID is the screen record, quotable in an appeal or a dispute.
	ID string `json:"id"`
	// Action is what the money plane did: allow, challenge, review, restrict or
	// block. Restrict and block mean the money did not move.
	Action string `json:"action"`
	// Moves states the same fact the way a caller acts on it.
	Moves bool `json:"moves"`
	// Score is the scoring plane's weight of evidence in [0,1].
	Score float64 `json:"score,omitempty"`
	// Agency is who acted: agent, human, bot or unknown.
	Agency string `json:"agency,omitempty"`
	// Decision is the /v1/risk decision this record is anchored to.
	Decision string `json:"decision,omitempty"`
	// Refusal states why the scoring plane could not judge. A screen carrying a
	// refusal was decided by the controls alone — it is not a clean result.
	Refusal string `json:"refusal,omitempty"`
	// Shadow marks a judgement that was recorded and deliberately not enforced.
	Shadow bool `json:"shadow,omitempty"`
	// Held and Allowed are exact minor units: what a reserve withheld and what
	// may still move. They always sum to the amount asked about.
	Held    int64 `json:"held"`
	Allowed int64 `json:"allowed"`
	// Reason is the strictest applying control's reason.
	Reason string `json:"reason,omitempty"`
	// Detail carries the evidence: the signals sent, the rules that hit and the
	// controls that bore on the move.
	Detail map[string]any `json:"detail,omitempty"`
	At     string         `json:"at,omitempty"`
}

// riskScreen scores one money move against this org's own model and controls,
// records the judgement, and returns it.
//
// It is processor-agnostic: it takes facts, not a payment. A merchant whose
// payments run through another processor calls this at authorization time and
// enforces the answer itself; a merchant on our own processors gets the same
// judgement automatically, from the same code, before the gateway is reached.
func riskScreen(ctx context.Context, in *riskScreenIn) (*riskScreenOut, error) {
	s, err := screener(ctx)
	if err != nil {
		return nil, err
	}
	rec, err := s.Screen(ctx, risk.Move{
		Stage:     risk.Stage(in.Stage),
		Subject:   risk.Subject{Kind: in.SubjectKind, ID: in.Subject},
		Amount:    currency.Cents(in.Amount),
		Currency:  currency.Type(in.Currency),
		Out:       in.Out,
		Signals:   in.Signals,
		Reference: in.Reference,
		Processor: in.Processor,
		Idem:      in.Idem,
	})
	if err != nil {
		return nil, badRequest(err)
	}
	return screenView(rec), nil
}

// riskScreensIn narrows a list of screens to one subject.
type riskScreensIn struct {
	SubjectKind string `json:"subjectKind,omitempty"`
	Subject     string `json:"subject,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

// riskScreenPage is a page of recorded screens, newest first.
type riskScreenPage struct {
	Screens []*riskScreenOut `json:"screens"`
}

// riskScreens lists the screens this org recorded, newest first.
func riskScreens(ctx context.Context, in *riskScreensIn) (*riskScreenPage, error) {
	s, err := screener(ctx)
	if err != nil {
		return nil, err
	}
	limit := in.Limit
	if limit <= 0 || limit > pageMax {
		limit = pageMax
	}
	out := &riskScreenPage{Screens: []*riskScreenOut{}}
	for _, rec := range screen.For(s.DB, in.SubjectKind, in.Subject, limit) {
		out.Screens = append(out.Screens, screenView(rec))
	}
	return out, nil
}

// riskRef names one record by its id.
type riskRef struct {
	ID string `json:"id" validate:"required"`
}

// riskScreenView reads one recorded screen.
func riskScreenView(ctx context.Context, in *riskRef) (*riskScreenOut, error) {
	s, err := screener(ctx)
	if err != nil {
		return nil, err
	}
	rec := screen.New(s.DB)
	if err := rec.GetById(in.ID); err != nil {
		return nil, zip.ErrNotFound("screen not found")
	}
	return screenView(rec), nil
}

func screenView(rec *screen.Screen) *riskScreenOut {
	out := &riskScreenOut{
		ID: rec.Id(), Action: rec.Action, Moves: risk.Action(rec.Action).Moves(),
		Score: rec.Score, Agency: rec.Agency, Decision: rec.Decision,
		Refusal: rec.Refusal, Shadow: rec.Shadow,
		Held: rec.Held, Allowed: rec.Allowed, Reason: rec.Reason, Detail: rec.Detail,
	}
	if at := rec.GetCreatedAt(); !at.IsZero() {
		out.At = at.UTC().Format(time.RFC3339)
	}
	return out
}

// -----------------------------------------------------------------------------
// controls
// -----------------------------------------------------------------------------

// riskControlIn places a standing restraint on one subject.
type riskControlIn struct {
	// Effect is reserve, hold or block. A reserve withholds a share of every
	// outbound move; a hold stops money leaving; a block stops it moving at all.
	Effect string `json:"effect" validate:"required"`
	// SubjectKind and Subject name what is restrained, inside this org.
	SubjectKind string `json:"subjectKind" validate:"required"`
	Subject     string `json:"subject" validate:"required"`
	// Rate is BASIS POINTS withheld, for a reserve: 2500 is a quarter. It is
	// basis points and not a fraction because money is integer arithmetic, and
	// a float rate drifts the withheld amount by a cent per move at scale.
	Rate int64 `json:"rate,omitempty"`
	// Until lapses the control, RFC 3339. Empty means it stands until released,
	// which is what a fraud restraint should do.
	Until  string `json:"until,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// riskControlOut is one standing restraint.
type riskControlOut struct {
	ID          string `json:"id"`
	Effect      string `json:"effect"`
	SubjectKind string `json:"subjectKind"`
	Subject     string `json:"subject"`
	Rate        int64  `json:"rate,omitempty"`
	Until       string `json:"until,omitempty"`
	Reason      string `json:"reason,omitempty"`
	// By is who placed it, from the validated principal.
	By         string `json:"by,omitempty"`
	Live       bool   `json:"live"`
	Released   bool   `json:"released,omitempty"`
	ReleasedAt string `json:"releasedAt,omitempty"`
	ReleasedBy string `json:"releasedBy,omitempty"`
}

// riskControlPage is every control the org has placed.
type riskControlPage struct {
	Controls []*riskControlOut `json:"controls"`
}

// riskControlsIn narrows the list to controls still in force.
type riskControlsIn struct {
	Live bool `json:"live,omitempty"`
}

// riskControls lists the controls this org has placed, and whether each still
// bears on a move.
func riskControls(ctx context.Context, in *riskControlsIn) (*riskControlPage, error) {
	s, err := screener(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := &riskControlPage{Controls: []*riskControlOut{}}
	for _, c := range control.All(s.DB) {
		if in.Live && !c.Live(now) {
			continue
		}
		out.Controls = append(out.Controls, controlView(c, now))
	}
	return out, nil
}

// riskControlPlace places a reserve, a payout hold or a block on one subject.
//
// Placing is idempotent while a control is in force: a monitor that runs every
// cycle does not accumulate a hundred identical holds on one merchant, and
// releasing takes one act rather than a hundred.
func riskControlPlace(ctx context.Context, in *riskControlIn) (*riskControlOut, error) {
	s, err := screener(ctx)
	if err != nil {
		return nil, err
	}
	var until time.Time
	if in.Until != "" {
		t, err := time.Parse(time.RFC3339, in.Until)
		if err != nil {
			return nil, zip.ErrBadRequest("until is not an RFC 3339 time")
		}
		until = t
	}
	c, err := risk.Place(s, risk.Subject{Kind: in.SubjectKind, ID: in.Subject}, in.Effect, in.Rate, until, in.Reason)
	if err != nil {
		return nil, badRequest(err)
	}
	return controlView(c, time.Now()), nil
}

// riskControlRelease lifts a control. Releasing one already released is not an
// error and does not rewrite who lifted it first.
func riskControlRelease(ctx context.Context, in *riskRef) (*riskControlOut, error) {
	s, err := screener(ctx)
	if err != nil {
		return nil, err
	}
	c := control.New(s.DB)
	if err := c.GetById(in.ID); err != nil {
		return nil, zip.ErrNotFound("control not found")
	}
	now := time.Now()
	c.Release(middleware.WhoFrom(ctx), now)
	if err := c.Update(); err != nil {
		return nil, zip.ErrInternal("failed to release the control")
	}
	return controlView(c, now), nil
}

func controlView(c *control.Control, now time.Time) *riskControlOut {
	out := &riskControlOut{
		ID: c.Id(), Effect: c.Effect, SubjectKind: c.SubjectKind, Subject: c.Subject,
		Rate: c.Rate, Reason: c.Reason, By: c.By, Live: c.Live(now), Released: c.Released,
		ReleasedBy: c.ReleasedBy,
	}
	if !c.Until.IsZero() {
		out.Until = c.Until.UTC().Format(time.RFC3339)
	}
	if !c.ReleasedAt.IsZero() {
		out.ReleasedAt = c.ReleasedAt.UTC().Format(time.RFC3339)
	}
	return out
}

// -----------------------------------------------------------------------------
// merchant standing
// -----------------------------------------------------------------------------

// riskMerchantIn names the merchant to read.
type riskMerchantIn struct {
	ID string `json:"id" validate:"required"`
}

// riskStandingOut is a merchant's standing: what this org's own record says
// about it, and what the risk plane made of that.
//
// The rates are BASIS POINTS of screened moves, not fractions — a dispute rate
// gets compared against a threshold and quoted in an appeal, and a float makes
// two services disagree in the fourth decimal about whether a merchant crossed
// one percent.
type riskStandingOut struct {
	Subject     string `json:"subject"`
	Screens     int    `json:"screens"`
	Refused     int    `json:"refused"`
	Disputes    int    `json:"disputes"`
	Lost        int    `json:"lost"`
	Refunds     int    `json:"refunds"`
	Failed      int    `json:"failed"`
	Negative    int    `json:"negative"`
	DisputeRate int64  `json:"disputeRate"`
	RefusalRate int64  `json:"refusalRate"`
	// VolumeIn, VolumeOut and Held are exact minor units.
	VolumeIn  int64             `json:"volumeIn"`
	VolumeOut int64             `json:"volumeOut"`
	Held      int64             `json:"held"`
	Controls  []*riskControlOut `json:"controls,omitempty"`
	// Screen is the merchant-stage judgement, present when the standing was
	// reviewed rather than merely counted.
	Screen *riskScreenOut `json:"screen,omitempty"`
	// Placed names a control the review placed.
	Placed string `json:"placed,omitempty"`
}

// riskMerchant reads a merchant's standing from this org's own record. It
// counts and reports; it scores nothing and changes nothing, so a console may
// poll it.
func riskMerchant(ctx context.Context, in *riskMerchantIn) (*riskStandingOut, error) {
	s, err := screener(ctx)
	if err != nil {
		return nil, err
	}
	st, err := risk.Count(s, risk.Subject{Kind: risk.KindMerchant, ID: in.ID})
	if err != nil {
		return nil, badRequest(err)
	}
	return standingView(st), nil
}

// riskReviewIn reviews one merchant now.
type riskReviewIn struct {
	ID string `json:"id" validate:"required"`
	// Act places the control the answer implies: a block on block, and on
	// restrict a reserve when Reserve is a rate, else a payout hold.
	Act bool `json:"act,omitempty"`
	// Reserve is BASIS POINTS to withhold when the answer restricts. Zero means
	// hold instead of reserving.
	Reserve int64 `json:"reserve,omitempty"`
}

// riskMerchantReview reviews a merchant now: it counts the standing, puts it to
// the risk plane as a merchant-stage question, records the judgement, and —
// when asked to act — places the control the answer implies.
//
// This is the continuous monitoring a platform runs on its merchants. It is a
// POST because it records a judgement and may restrain money.
func riskMerchantReview(ctx context.Context, in *riskReviewIn) (*riskStandingOut, error) {
	s, err := screener(ctx)
	if err != nil {
		return nil, err
	}
	st, err := risk.Monitor(ctx, s, risk.Subject{Kind: risk.KindMerchant, ID: in.ID}, in.Reserve, in.Act)
	if err != nil {
		return nil, badRequest(err)
	}
	return standingView(st), nil
}

func standingView(st *risk.Standing) *riskStandingOut {
	out := &riskStandingOut{
		Subject: st.Subject.ID, Screens: st.Screens, Refused: st.Refused,
		Disputes: st.Disputes, Lost: st.Lost, Refunds: st.Refunds,
		Failed: st.Failed, Negative: st.Negative,
		DisputeRate: st.DisputeRate, RefusalRate: st.RefusalRate,
		VolumeIn: int64(st.VolumeIn), VolumeOut: int64(st.VolumeOut), Held: int64(st.Held),
		Placed: st.Placed,
	}
	now := time.Now()
	for _, c := range st.Controls {
		out.Controls = append(out.Controls, controlView(c, now))
	}
	if st.Screen != nil {
		out.Screen = screenView(st.Screen)
	}
	return out
}

// -----------------------------------------------------------------------------
// outcomes
// -----------------------------------------------------------------------------

// riskOutcomeIn reports how a scored money event turned out.
type riskOutcomeIn struct {
	// Event is dispute, won, lost, refund, payoutfail, negative or abuse.
	Event string `json:"event" validate:"required"`
	// SubjectKind and Subject name what the outcome happened to.
	SubjectKind string `json:"subjectKind" validate:"required"`
	Subject     string `json:"subject" validate:"required"`
	// Amount is exact minor units of Currency.
	Amount   int64  `json:"amount,omitempty"`
	Currency string `json:"currency,omitempty"`
	// Screen anchors the outcome to the judgement it corrects. Naming one also
	// writes the label onto that screen, which is what the org's own model
	// learns from.
	Screen string `json:"screen,omitempty"`
	// Reference is the money object — a dispute, a payout, a charge.
	Reference string `json:"reference,omitempty"`
	Note      string `json:"note,omitempty"`
	// Idem makes a repeated report return the first record rather than a second.
	Idem string `json:"idem,omitempty"`
}

// riskOutcomeOut is the recorded outcome.
//
// Example: {"event":"dispute","subjectKind":"customer","subject":"cus_1","screen":"scr_1"}
// Response: {"id":"out_1","event":"dispute","reported":true}
type riskOutcomeOut struct {
	ID       string `json:"id"`
	Event    string `json:"event"`
	Screen   string `json:"screen,omitempty"`
	Decision string `json:"decision,omitempty"`
	// Reported states whether the scoring plane accepted the label, and Refusal
	// why it did not. An unreported outcome is still a durable record here, so a
	// scoring outage costs learning latency and never evidence.
	Reported bool   `json:"reported"`
	Refusal  string `json:"refusal,omitempty"`
	At       string `json:"at,omitempty"`
}

// riskOutcome records how a scored money event turned out and reports the label
// to the risk plane so the org's own model learns from its own outcomes.
//
// The record is written to this org's books FIRST and forwarded second. Wired
// the other way a scoring hiccup loses the label and the loss is invisible.
func riskOutcome(ctx context.Context, in *riskOutcomeIn) (*riskOutcomeOut, error) {
	s, err := screener(ctx)
	if err != nil {
		return nil, err
	}
	if !outcome.Events(in.Event) {
		return nil, zip.ErrBadRequest("event is not an outcome this plane records")
	}
	subject := risk.Subject{Kind: in.SubjectKind, ID: in.Subject}
	if err := subject.Valid(); err != nil {
		return nil, badRequest(err)
	}
	if prior, ok := outcome.ByIdem(s.DB, in.Idem); ok {
		return outcomeView(prior), nil
	}

	o := outcome.New(s.DB)
	o.Event = in.Event
	o.SubjectKind = subject.Kind
	o.Subject = subject.ID
	o.Amount = in.Amount
	o.Currency = currency.Type(in.Currency)
	o.Screen = in.Screen
	o.Reference = in.Reference
	o.Note = in.Note
	o.Idem = in.Idem
	o.By = middleware.WhoFrom(ctx)

	// The label rides the screen's decision, which is the judgement the model
	// made. An outcome on a move that was never screened has no judgement to
	// correct, and says so rather than inventing one.
	var rec *screen.Screen
	if in.Screen != "" {
		rec = screen.New(s.DB)
		if err := rec.GetById(in.Screen); err != nil {
			return nil, zip.ErrNotFound("screen not found")
		}
		o.Decision = rec.Decision
	}

	if err := o.Create(); err != nil {
		return nil, zip.ErrInternal("failed to record the outcome")
	}

	if rec != nil {
		rec.Outcome = in.Event
		rec.OutcomeAt = time.Now()
		if err := rec.Update(); err != nil {
			return nil, zip.ErrInternal("failed to label the screen")
		}
	}

	err = risk.Of().Label(ctx, &risk.Label{
		Decision: o.Decision,
		Subject:  subject,
		Outcome:  in.Event,
		Amount:   &risk.Money{Cents: currency.Cents(in.Amount), Currency: currency.Type(in.Currency)},
		Note:     in.Note,
	})
	switch {
	case err == nil:
		o.Reported = true
		o.ReportedAt = time.Now()
	case errors.Is(err, risk.ErrNoDecision):
		o.Refusal = "no decision"
	case errors.Is(err, risk.ErrAbsent):
		o.Refusal = risk.RefusalAbsent
	default:
		o.Refusal = risk.RefusalUnreachable
	}
	if err := o.Update(); err != nil {
		return nil, zip.ErrInternal("failed to record the report")
	}
	return outcomeView(o), nil
}

func outcomeView(o *outcome.Outcome) *riskOutcomeOut {
	out := &riskOutcomeOut{
		ID: o.Id(), Event: o.Event, Screen: o.Screen, Decision: o.Decision,
		Reported: o.Reported, Refusal: o.Refusal,
	}
	if at := o.GetCreatedAt(); !at.IsZero() {
		out.At = at.UTC().Format(time.RFC3339)
	}
	return out
}

// -----------------------------------------------------------------------------
// disputes
// -----------------------------------------------------------------------------

// riskEvidenceOut is a dispute defence assembled from this org's record.
type riskEvidenceOut struct {
	Evidence *risk.Evidence `json:"evidence"`
}

// riskEvidence assembles a dispute defence out of this org's own record: the
// charge, the risk judgement that admitted it, the rules that hit, the controls
// in force, and every later outcome for the same customer.
//
// What it cannot find it NAMES, in gaps. A blank evidence field looks like a
// fact that was checked and found empty; a stated gap tells the merchant what
// to add.
func riskEvidence(ctx context.Context, in *riskRef) (*riskEvidenceOut, error) {
	s, err := screener(ctx)
	if err != nil {
		return nil, err
	}
	e, err := risk.Assemble(s.DB, in.ID)
	if err != nil {
		return nil, zip.ErrNotFound("dispute not found")
	}
	return &riskEvidenceOut{Evidence: e}, nil
}

// riskSubmitIn submits the assembled defence for one dispute.
type riskSubmitIn struct {
	ID string `json:"id" validate:"required"`
	// Processor names which of the org's processors carries the submission. It
	// defaults to the one that took the charge.
	Processor string `json:"processor,omitempty"`
}

// riskSubmitOut is the adjudicator's receipt.
type riskSubmitOut struct {
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Accepted  bool   `json:"accepted"`
}

// riskSubmit submits an assembled dispute defence to the adjudicator that can
// carry it.
//
// TODAY IT ALWAYS REFUSES, and that is the honest answer rather than a defect
// hidden behind a 200. Two routes exist in principle: the processor that took
// the payment forwards evidence to the issuing network — no provider in this
// tree implements that yet — or a dispute network stops the chargeback before
// it becomes one, and we are a member of none. A merchant must not believe a
// defence was filed because a button returned success.
func riskSubmit(ctx context.Context, in *riskSubmitIn) (*riskSubmitOut, error) {
	s, err := screener(ctx)
	if err != nil {
		return nil, err
	}
	org, _ := middleware.OrgFrom(ctx)
	e, err := risk.Assemble(s.DB, in.ID)
	if err != nil {
		return nil, zip.ErrNotFound("dispute not found")
	}

	kind := processorType(in.Processor, e)
	receipt, err := risk.Submit(ctx, processorsForOrg(org), kind, e.Dispute.Provider, e)
	if err != nil {
		return nil, zip.Errorf(http.StatusNotImplemented, "%s", err.Error())
	}
	return &riskSubmitOut{Reference: receipt.Reference, Status: receipt.Status, Accepted: receipt.Accepted}, nil
}

// processorType is the processor asked to carry the submission: the one named,
// else the one that took the charge.
func processorType(named string, e *risk.Evidence) processor.ProcessorType {
	if named != "" {
		return processor.ProcessorType(named)
	}
	if e.Charge != nil {
		return processor.ProcessorType(e.Charge.Provider)
	}
	return ""
}

// badRequest renders a domain refusal as the caller's mistake it is. A subject
// kind this plane does not name and a reserve rate outside 0..100% are both
// things the caller can fix; anything else is ours.
func badRequest(err error) error {
	switch {
	case errors.Is(err, risk.ErrKind):
		return zip.ErrBadRequest(err.Error())
	default:
		return zip.ErrBadRequest(err.Error())
	}
}
