package server

import (
	"errors"
	"sync"
	"testing"
)

// The read loop can reach handlePlayStatus before TransferContext has registered
// its callback. Before the fix that notification was lost for good: connected is
// closed once and nothing re-triggers it, so the transfer never finalized.
func TestOnConnectAfterSequenceCompletedStillFires(t *testing.T) {
	c := &Conn{connected: make(chan struct{})}

	close(c.connected)
	c.fireConnect(nil)

	fired := make(chan error, 1)
	c.OnConnect(func(err error) { fired <- err })

	select {
	case err := <-fired:
		if err != nil {
			t.Fatalf("callback got %v, want nil", err)
		}
	default:
		t.Fatal("a callback registered after completion must still be invoked")
	}
}

func TestOnConnectBeforeSequenceFiresOnce(t *testing.T) {
	c := &Conn{connected: make(chan struct{})}

	var calls int
	c.OnConnect(func(error) { calls++ })

	close(c.connected)
	c.fireConnect(nil)
	c.fireConnect(nil)
	c.fireConnect(errors.New("closed"))

	if calls != 1 {
		t.Fatalf("callback invoked %d times, want exactly 1", calls)
	}
}

// A failure before the sequence completes must reach the callback, so the
// transfer is reported as failed instead of hanging.
func TestFireConnectDeliversFailure(t *testing.T) {
	c := &Conn{connected: make(chan struct{})}

	got := make(chan error, 1)
	c.OnConnect(func(err error) { got <- err })

	want := errors.New("dial died")
	c.fireConnect(want)

	if err := <-got; !errors.Is(err, want) {
		t.Fatalf("callback got %v, want %v", err, want)
	}
}

// Registration racing the read loop must never lose the notification nor
// double-invoke it. Run under -race.
func TestOnConnectRacingReadLoop(t *testing.T) {
	for i := 0; i < 200; i++ {
		c := &Conn{connected: make(chan struct{})}

		var mu sync.Mutex
		calls := 0
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			close(c.connected)
			c.fireConnect(nil)
		}()
		go func() {
			defer wg.Done()
			c.OnConnect(func(error) {
				mu.Lock()
				calls++
				mu.Unlock()
			})
		}()

		wg.Wait()
		mu.Lock()
		got := calls
		mu.Unlock()
		if got != 1 {
			t.Fatalf("iteration %d: callback invoked %d times, want exactly 1", i, got)
		}
	}
}
