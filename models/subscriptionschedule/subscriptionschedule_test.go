package subscriptionschedule

import (
	"testing"
	"time"
)

// --- Release ---

func TestRelease_FromNotStarted(t *testing.T) {
	s := &SubscriptionSchedule{Status: NotStarted}
	if err := s.Release(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != Released {
		t.Errorf("expected %s, got %s", Released, s.Status)
	}
}

func TestRelease_FromActive(t *testing.T) {
	s := &SubscriptionSchedule{Status: Active}
	if err := s.Release(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != Released {
		t.Errorf("expected %s, got %s", Released, s.Status)
	}
}

func TestRelease_InvalidStatus_Completed(t *testing.T) {
	s := &SubscriptionSchedule{Status: Completed}
	err := s.Release()
	if err == nil {
		t.Fatal("expected error releasing from Completed")
	}
}

func TestRelease_InvalidStatus_Released(t *testing.T) {
	s := &SubscriptionSchedule{Status: Released}
	err := s.Release()
	if err == nil {
		t.Fatal("expected error releasing already-released schedule")
	}
}

func TestRelease_InvalidStatus_Canceled(t *testing.T) {
	s := &SubscriptionSchedule{Status: SSCanceled}
	err := s.Release()
	if err == nil {
		t.Fatal("expected error releasing canceled schedule")
	}
}

// --- Cancel ---

func TestCancel_FromNotStarted(t *testing.T) {
	s := &SubscriptionSchedule{Status: NotStarted}
	if err := s.Cancel(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != SSCanceled {
		t.Errorf("expected %s, got %s", SSCanceled, s.Status)
	}
}

func TestCancel_FromActive(t *testing.T) {
	s := &SubscriptionSchedule{Status: Active}
	if err := s.Cancel(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != SSCanceled {
		t.Errorf("expected %s, got %s", SSCanceled, s.Status)
	}
}

func TestCancel_InvalidStatus_Completed(t *testing.T) {
	s := &SubscriptionSchedule{Status: Completed}
	err := s.Cancel()
	if err == nil {
		t.Fatal("expected error canceling completed schedule")
	}
}

func TestCancel_InvalidStatus_Released(t *testing.T) {
	s := &SubscriptionSchedule{Status: Released}
	err := s.Cancel()
	if err == nil {
		t.Fatal("expected error canceling released schedule")
	}
}

func TestCancel_InvalidStatus_Canceled(t *testing.T) {
	s := &SubscriptionSchedule{Status: SSCanceled}
	err := s.Cancel()
	if err == nil {
		t.Fatal("expected error canceling already-canceled schedule")
	}
}

// --- Complete ---

func TestComplete_FromActive(t *testing.T) {
	s := &SubscriptionSchedule{Status: Active}
	s.Complete()
	if s.Status != Completed {
		t.Errorf("expected %s, got %s", Completed, s.Status)
	}
}

func TestComplete_FromNotStarted(t *testing.T) {
	s := &SubscriptionSchedule{Status: NotStarted}
	s.Complete()
	if s.Status != Completed {
		t.Errorf("expected %s, got %s", Completed, s.Status)
	}
}

func TestComplete_Idempotent(t *testing.T) {
	s := &SubscriptionSchedule{Status: Completed}
	s.Complete()
	if s.Status != Completed {
		t.Errorf("expected %s, got %s", Completed, s.Status)
	}
}

// --- Start ---

func TestStart_SetsActiveAndSubscriptionId(t *testing.T) {
	s := &SubscriptionSchedule{Status: NotStarted}
	s.Start("sub_abc")
	if s.Status != Active {
		t.Errorf("expected %s, got %s", Active, s.Status)
	}
	if s.SubscriptionId != "sub_abc" {
		t.Errorf("expected sub_abc, got %s", s.SubscriptionId)
	}
}

func TestStart_OverwritesSubscriptionId(t *testing.T) {
	s := &SubscriptionSchedule{
		Status:         NotStarted,
		SubscriptionId: "old_sub",
	}
	s.Start("new_sub")
	if s.SubscriptionId != "new_sub" {
		t.Errorf("expected new_sub, got %s", s.SubscriptionId)
	}
}

// --- Status constants ---

func TestStatusConstants(t *testing.T) {
	cases := []struct {
		status Status
		want   string
	}{
		{NotStarted, "not_started"},
		{Active, "active"},
		{Completed, "completed"},
		{Released, "released"},
		{SSCanceled, "canceled"},
	}
	for _, tc := range cases {
		if string(tc.status) != tc.want {
			t.Errorf("status %q != %q", tc.status, tc.want)
		}
	}
}

// --- Full lifecycle ---

func TestFullLifecycle_StartThenComplete(t *testing.T) {
	s := &SubscriptionSchedule{Status: NotStarted}
	s.Start("sub_lifecycle")
	if s.Status != Active {
		t.Fatalf("expected Active after Start, got %s", s.Status)
	}
	s.Complete()
	if s.Status != Completed {
		t.Errorf("expected Completed, got %s", s.Status)
	}
}

func TestFullLifecycle_StartThenCancel(t *testing.T) {
	s := &SubscriptionSchedule{Status: NotStarted}
	s.Start("sub_cancel")
	if err := s.Cancel(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != SSCanceled {
		t.Errorf("expected %s, got %s", SSCanceled, s.Status)
	}
}

func TestFullLifecycle_StartThenRelease(t *testing.T) {
	s := &SubscriptionSchedule{Status: NotStarted}
	s.Start("sub_release")
	if err := s.Release(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Status != Released {
		t.Errorf("expected %s, got %s", Released, s.Status)
	}
}

func TestFullLifecycle_CompletedCannotCancel(t *testing.T) {
	s := &SubscriptionSchedule{Status: NotStarted}
	s.Start("sub_done")
	s.Complete()
	err := s.Cancel()
	if err == nil {
		t.Fatal("expected error canceling completed schedule")
	}
}

func TestFullLifecycle_CompletedCannotRelease(t *testing.T) {
	s := &SubscriptionSchedule{Status: NotStarted}
	s.Start("sub_done2")
	s.Complete()
	err := s.Release()
	if err == nil {
		t.Fatal("expected error releasing completed schedule")
	}
}

// --- Kind ---

func TestKind(t *testing.T) {
	s := &SubscriptionSchedule{}
	if s.Kind() != "subscription-schedule" {
		t.Errorf("expected 'subscription-schedule', got %q", s.Kind())
	}
}

// --- Validator ---

func TestValidator_ReturnsNil(t *testing.T) {
	s := &SubscriptionSchedule{}
	if s.Validator() != nil {
		t.Error("expected nil validator")
	}
}

// --- Release from empty status ---

func TestRelease_InvalidStatus_Empty(t *testing.T) {
	s := &SubscriptionSchedule{Status: ""}
	err := s.Release()
	if err == nil {
		t.Fatal("expected error releasing from empty status")
	}
}

func TestRelease_InvalidStatus_Unknown(t *testing.T) {
	s := &SubscriptionSchedule{Status: Status("suspended")}
	err := s.Release()
	if err == nil {
		t.Fatal("expected error releasing from unknown status")
	}
}

// --- Cancel from empty/unknown status ---

func TestCancel_InvalidStatus_Empty(t *testing.T) {
	s := &SubscriptionSchedule{Status: ""}
	err := s.Cancel()
	if err == nil {
		t.Fatal("expected error canceling from empty status")
	}
}

func TestCancel_InvalidStatus_Unknown(t *testing.T) {
	s := &SubscriptionSchedule{Status: Status("paused")}
	err := s.Cancel()
	if err == nil {
		t.Fatal("expected error canceling from unknown status")
	}
}

// --- Complete from all terminal states (idempotent) ---

func TestComplete_FromReleased(t *testing.T) {
	s := &SubscriptionSchedule{Status: Released}
	s.Complete()
	if s.Status != Completed {
		t.Errorf("expected %s, got %s", Completed, s.Status)
	}
}

func TestComplete_FromCanceled(t *testing.T) {
	s := &SubscriptionSchedule{Status: SSCanceled}
	s.Complete()
	if s.Status != Completed {
		t.Errorf("expected %s, got %s", Completed, s.Status)
	}
}

// --- Start sets status regardless of current ---

func TestStart_FromActive(t *testing.T) {
	s := &SubscriptionSchedule{Status: Active, SubscriptionId: "old_sub"}
	s.Start("new_sub")
	if s.Status != Active {
		t.Errorf("expected %s, got %s", Active, s.Status)
	}
	if s.SubscriptionId != "new_sub" {
		t.Errorf("expected new_sub, got %s", s.SubscriptionId)
	}
}

func TestStart_EmptySubscriptionId(t *testing.T) {
	s := &SubscriptionSchedule{Status: NotStarted}
	s.Start("")
	if s.Status != Active {
		t.Errorf("expected %s, got %s", Active, s.Status)
	}
	if s.SubscriptionId != "" {
		t.Errorf("expected empty subscriptionId, got %q", s.SubscriptionId)
	}
}

// --- Phase struct ---

func TestPhaseStruct(t *testing.T) {
	now := time.Now()
	end := now.Add(30 * 24 * time.Hour)
	phase := Phase{
		PlanId:            "plan_abc",
		StartDate:         now,
		EndDate:           end,
		ProrationBehavior: "create_prorations",
		Items: []PhaseItem{
			{PriceId: "price_1", Quantity: 2},
			{PriceId: "price_2", Quantity: 1},
		},
	}
	if phase.PlanId != "plan_abc" {
		t.Errorf("expected plan_abc, got %s", phase.PlanId)
	}
	if len(phase.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(phase.Items))
	}
	if phase.Items[0].PriceId != "price_1" {
		t.Errorf("expected price_1, got %s", phase.Items[0].PriceId)
	}
	if phase.Items[0].Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", phase.Items[0].Quantity)
	}
	if phase.ProrationBehavior != "create_prorations" {
		t.Errorf("expected create_prorations, got %s", phase.ProrationBehavior)
	}
}

func TestPhaseItemZeroValue(t *testing.T) {
	pi := PhaseItem{}
	if pi.PriceId != "" {
		t.Errorf("expected empty, got %q", pi.PriceId)
	}
	if pi.Quantity != 0 {
		t.Errorf("expected 0, got %d", pi.Quantity)
	}
}

// --- SubscriptionSchedule with phases ---

func TestSubscriptionScheduleWithPhases(t *testing.T) {
	now := time.Now()
	s := &SubscriptionSchedule{
		Status:    NotStarted,
		StartDate: now,
		Phases: []Phase{
			{PlanId: "plan_a", StartDate: now, EndDate: now.Add(30 * 24 * time.Hour)},
			{PlanId: "plan_b", StartDate: now.Add(30 * 24 * time.Hour), EndDate: now.Add(60 * 24 * time.Hour)},
		},
	}
	if len(s.Phases) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(s.Phases))
	}
	if s.Phases[0].PlanId != "plan_a" {
		t.Errorf("expected plan_a, got %s", s.Phases[0].PlanId)
	}
}

// --- EndBehavior field ---

func TestEndBehavior(t *testing.T) {
	s := &SubscriptionSchedule{EndBehavior: "cancel"}
	if s.EndBehavior != "cancel" {
		t.Errorf("expected 'cancel', got %q", s.EndBehavior)
	}
}

// --- Terminal states cannot release or cancel (comprehensive) ---

func TestReleasedThenCancel(t *testing.T) {
	s := &SubscriptionSchedule{Status: Active}
	if err := s.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	err := s.Cancel()
	if err == nil {
		t.Fatal("expected error canceling released schedule")
	}
}

func TestCanceledThenRelease(t *testing.T) {
	s := &SubscriptionSchedule{Status: Active}
	if err := s.Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	err := s.Release()
	if err == nil {
		t.Fatal("expected error releasing canceled schedule")
	}
}
