package delay

import (
	"context"
	"sync"
	"time"
)

// A delayed task runs in a goroutine that outlives the call which queued it.
// The datastore it reaches does not: db.Manager.Close() shuts the namespace
// registry, and a task still running past that point gets back
// "db: namespaces closed" — a webhook emit or a counter shard failing against
// a database torn down underneath it. Nothing here was waiting for those
// goroutines, and nothing could tell them to stop: each one ran on a bare
// context.Background() and could sit in a retry backoff for
// DefaultRetryCount * DefaultRetryDelay after its caller had returned.
//
// This package spawns the goroutines, so this package owns them. Every task is
// counted before it starts and uncounted when it returns, and every wait it
// performs is interruptible. Drain then means what it says — stop the waiting,
// let the running tasks finish, and hand back control so the caller can close
// the database knowing nothing is still reading from it.
//
// The count is taken synchronously, in the goroutine that queues the task
// rather than in the one that runs it: a task which queues another task (the
// counter's concurrent-transaction retry) has the child counted before it
// uncounts itself, so one drain covers the whole chain instead of racing it.
//
// The stop signal is a channel rather than a cancelled context because "stop
// waiting to start" and "abandon the attempt in progress" are different things.
// A drain does the first and leaves the second alone: work already underway
// runs to completion, bounded only by the deadline the caller passes.
var (
	mu       sync.Mutex
	inFlight int
	idle     = closed()
	stopCh   = make(chan struct{})
)

func closed() chan struct{} {
	c := make(chan struct{})
	close(c)
	return c
}

// spawn runs fn in a counted background goroutine. The count is taken here, in
// the caller, so a Drain that starts immediately afterwards still sees it.
//
// sync.WaitGroup would be the obvious tool and is the wrong one: a Drain that
// times out has to abandon its wait, and a WaitGroup.Add racing a Wait that is
// still pending is a data race by contract. A count plus a channel closed at
// zero is selectable, so the wait can simply be abandoned.
func spawn(fn func()) {
	mu.Lock()
	if inFlight == 0 {
		idle = make(chan struct{})
	}
	inFlight++
	mu.Unlock()

	go func() {
		defer done()
		fn()
	}()
}

func done() {
	mu.Lock()
	inFlight--
	if inFlight == 0 {
		close(idle)
	}
	mu.Unlock()
}

// stopped reports the channel closed when a drain begins. Read it once per
// task; it is replaced after every drain.
func stopped() <-chan struct{} {
	mu.Lock()
	defer mu.Unlock()
	return stopCh
}

// pause waits for d and reports whether the wait finished, returning false as
// soon as stop is closed. It is how a task's initial delay and its retry
// backoff become interruptible: a drain no longer has to outwait
// DefaultRetryDelay to reclaim a goroutine that is only sleeping.
//
// A non-positive d has nothing to wait for and so is never interrupted. That
// distinction is the whole rule a drain enforces: work already accepted runs,
// work that is merely scheduled does not. An increment queued by the last
// request before shutdown is accepted work and must not be dropped on the
// floor; the same increment's jittered re-queue after a lost transaction race
// is a retry, and a retry against a database that is closing is not worth
// holding the shutdown for.
func pause(stop <-chan struct{}, d time.Duration) bool {
	if d <= 0 {
		return true
	}

	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-t.C:
		return true
	case <-stop:
		return false
	}
}

// Drain stops queued tasks from waiting any longer and waits for the ones
// already running to return, or for ctx to expire — whichever comes first. It
// returns ctx.Err() when the wait ran out of time, meaning tasks are still in
// flight and the datastore is not yet safe to close.
//
// Call it during shutdown after the HTTP listener has drained, so nothing new
// is being queued, and before closing the database. Both callers do exactly
// that: App.Shutdown and the test context's Close.
//
// Draining is not permanent. Once the wait is over the queue re-arms, so a
// process that drains for a checkpoint keeps working and a test binary running
// several contexts in sequence gets a clean queue for each.
func Drain(ctx context.Context) error {
	mu.Lock()
	close(stopCh)
	wait := idle
	mu.Unlock()

	var err error
	select {
	case <-wait:
	case <-ctx.Done():
		err = ctx.Err()
	}

	mu.Lock()
	stopCh = make(chan struct{})
	mu.Unlock()

	return err
}
