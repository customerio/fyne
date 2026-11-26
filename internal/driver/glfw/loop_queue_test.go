//go:build !no_glfw && !mobile

package glfw

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestMainQueue_StopAnswersEveryWaiter races callers that queue and wait,
// as DoAndWait does, against the loop stopping. Each caller must return, and
// its function run at most once: on the loop, or, refused, on the caller. Before the queue made the
// check and the send one step, a function sent between the loop's last look
// at the queue and its saying it had stopped was never answered.
func TestMainQueue_StopAnswersEveryWaiter(t *testing.T) {
	for round := 0; round < 200; round++ {
		q := newMainQueue()
		const callers = 32
		var wg sync.WaitGroup
		var ran [callers]atomic.Int32
		start := make(chan struct{})
		for i := 0; i < callers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				f := func() { ran[i].Add(1) }
				done := make(chan struct{})
				if q.post(funcData{f: f, done: done}) {
					<-done
				} else {
					f()
				}
			}(i)
		}
		close(start)
		// The loop serves a few before it stops, as it would.
		for i := 0; i < round%4; i++ {
			select {
			case f := <-q.out():
				f.f()
				if f.done != nil {
					f.done <- struct{}{}
				}
			default:
			}
		}
		q.stop()

		finished := make(chan struct{})
		go func() {
			wg.Wait()
			close(finished)
		}()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Fatalf("round %d: a caller was still waiting after the queue stopped", round)
		}
		for i := range ran {
			if n := ran[i].Load(); n > 1 {
				t.Fatalf("round %d: caller %d's function ran %d times", round, i, n)
			}
		}
	}
}

// TestMainQueue_RefusesAfterStop checks that a function offered once the loop
// has stopped is refused, so its caller runs it rather than waiting.
func TestMainQueue_RefusesAfterStop(t *testing.T) {
	q := newMainQueue()
	q.stop()
	if q.post(funcData{f: func() {}, done: make(chan struct{})}) {
		t.Fatal("post accepted a function after stop; its caller would wait forever")
	}
}
