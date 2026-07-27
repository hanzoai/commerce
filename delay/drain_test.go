package delay

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// drainCtx bounds a Drain in tests. Generous enough that a slow machine does
// not fail a passing case, tight enough that a hang is reported as a failure
// rather than by the package timeout.
func drainCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), 5*time.Second)
}

// A task that is still running must be finished before Drain returns. This is
// the whole point: the caller closes the datastore next, and a task reading
// from it after that gets "db: namespaces closed".
func TestDrainWaitsForRunningTask(t *testing.T) {
	var done atomic.Bool
	release := make(chan struct{})

	f := Func("drain-waits", func(ctx context.Context) error {
		<-release
		done.Store(true)
		return nil
	})

	if err := f.Call(context.Background()); err != nil {
		t.Fatalf("Call: %v", err)
	}

	// Let the task get as far as blocking, then let Drain start racing it.
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(release)
	}()

	ctx, cancel := drainCtx(t)
	defer cancel()
	if err := Drain(ctx); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	if !done.Load() {
		t.Fatal("Drain returned while the task was still running")
	}
}

// A task queued but not yet started must not start after a Drain: its work is
// exactly what the drain exists to prevent.
func TestDrainCancelsPendingDelay(t *testing.T) {
	var ran atomic.Bool

	f := Func("drain-pending", func(ctx context.Context) error {
		ran.Store(true)
		return nil
	}).After(time.Hour)

	if err := f.Call(context.Background()); err != nil {
		t.Fatalf("Call: %v", err)
	}

	ctx, cancel := drainCtx(t)
	defer cancel()

	start := time.Now()
	if err := Drain(ctx); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Drain waited out the task's delay (%v)", elapsed)
	}

	if ran.Load() {
		t.Fatal("a task pending behind a delay ran anyway")
	}
}

// A failing task retries with a backoff. Draining must cut the backoff short
// rather than hold the shutdown for DefaultRetryCount * DefaultRetryDelay —
// which was 15 seconds of a goroutine sleeping against a database that was
// about to be closed.
func TestDrainInterruptsRetryBackoff(t *testing.T) {
	attempts := make(chan struct{}, 8)

	f := Func("drain-retries", func(ctx context.Context) error {
		attempts <- struct{}{}
		return errors.New("always fails")
	})

	if err := f.Call(context.Background()); err != nil {
		t.Fatalf("Call: %v", err)
	}

	select {
	case <-attempts:
	case <-time.After(2 * time.Second):
		t.Fatal("task never ran")
	}

	// The task is now sleeping out DefaultRetryDelay before attempt two.
	ctx, cancel := drainCtx(t)
	defer cancel()

	start := time.Now()
	if err := Drain(ctx); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= DefaultRetryDelay {
		t.Fatalf("Drain waited out the retry backoff (%v)", elapsed)
	}

	select {
	case <-attempts:
		t.Fatal("a retry ran after the drain began")
	default:
	}
}

// The counter re-queues a shard write that lost a transaction race, so a task
// can queue a task. The child registers while the parent is still counted,
// which is what lets one Drain cover the chain. The invariant that matters is
// not that the child runs but that it never runs *after* Drain returns — the
// caller closes the datastore on that signal.
func TestDrainCoversRequeuedTask(t *testing.T) {
	var childRan, ranAfterDrain, drained atomic.Bool

	release := make(chan struct{})

	child := Func("drain-chain-child", func(ctx context.Context) error {
		if drained.Load() {
			ranAfterDrain.Store(true)
		}
		childRan.Store(true)
		return nil
	})

	parent := Func("drain-chain-parent", func(ctx context.Context) error {
		<-release
		return child.Call(ctx)
	})

	if err := parent.Call(context.Background()); err != nil {
		t.Fatalf("Call: %v", err)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(release)
	}()

	ctx, cancel := drainCtx(t)
	defer cancel()
	if err := Drain(ctx); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	drained.Store(true)

	if !childRan.Load() {
		t.Fatal("the re-queued task was dropped; accepted work must run")
	}

	// Give a task that escaped the wait every chance to show itself.
	time.Sleep(200 * time.Millisecond)
	if ranAfterDrain.Load() {
		t.Fatal("a re-queued task ran after Drain returned")
	}
}

// Work accepted before a drain runs. Dropping it would trade one silent
// failure for another: the last request's counter increment simply vanishing.
func TestDrainRunsWorkQueuedBeforeIt(t *testing.T) {
	var ran atomic.Bool

	f := Func("drain-accepted", func(context.Context) error {
		ran.Store(true)
		return nil
	})

	if err := f.Call(context.Background()); err != nil {
		t.Fatalf("Call: %v", err)
	}

	ctx, cancel := drainCtx(t)
	defer cancel()
	if err := Drain(ctx); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	if !ran.Load() {
		t.Fatal("a task queued before the drain never ran")
	}
}

// Drain reports the timeout instead of pretending the queue is empty: the
// caller needs to know the datastore is not yet safe to close.
func TestDrainReportsTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	f := Func("drain-timeout", func(ctx context.Context) error {
		<-release
		return nil
	})

	if err := f.Call(context.Background()); err != nil {
		t.Fatalf("Call: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := Drain(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Drain err = %v, want context.DeadlineExceeded", err)
	}
}

// Draining is not permanent. A test binary closes one shared context and opens
// the next; a process that drains for a checkpoint keeps serving.
func TestQueueRearmsAfterDrain(t *testing.T) {
	ctx, cancel := drainCtx(t)
	defer cancel()
	if err := Drain(ctx); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	var ran atomic.Bool
	f := Func("drain-rearm", func(context.Context) error {
		ran.Store(true)
		return nil
	})

	if err := f.Call(context.Background()); err != nil {
		t.Fatalf("Call: %v", err)
	}

	ctx2, cancel2 := drainCtx(t)
	defer cancel2()
	if err := Drain(ctx2); err != nil {
		t.Fatalf("Drain after re-arm: %v", err)
	}

	if !ran.Load() {
		t.Fatal("a task queued after a drain never ran")
	}
}

func TestAfterDoesNotMutateOriginal(t *testing.T) {
	f := Func("after-immutable", func(context.Context) error { return nil })
	f2 := f.After(time.Minute)

	if f2.delay != time.Minute {
		t.Fatalf("After delay = %v, want 1m", f2.delay)
	}
	if f.delay != 0 {
		t.Fatalf("After mutated the original: delay = %v", f.delay)
	}
}
