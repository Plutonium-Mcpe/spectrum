package session

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
)

type closeTrackingStream struct {
	io.ReadWriteCloser
	closed atomic.Bool
}

func (c *closeTrackingStream) Close() error {
	c.closed.Store(true)
	return nil
}

type cancellingTransport struct {
	stream *closeTrackingStream
	cancel func()
}

// Dial simulates a session being closed while the dial is in flight, which is the
// window dial() has to re-check: the first check happens before the lock is taken.
func (t cancellingTransport) Dial(context.Context, string) (io.ReadWriteCloser, error) {
	t.cancel()
	return t.stream, nil
}

// A connection dialled for a session that closed mid-dial must be closed here.
// CloseWithError has already run its sync.Once, so nothing else ever would, and
// the backend would keep the player's session open forever.
func TestDialClosesConnectionWhenSessionClosesMidDial(t *testing.T) {
	stream := &closeTrackingStream{}
	s := &Session{}
	s.ctx, s.cancelFunc = context.WithCancelCause(context.Background())
	s.transport = cancellingTransport{stream: stream, cancel: func() { s.cancelFunc(errors.New("closed by application")) }}

	conn, err := s.dial(context.Background(), "127.0.0.1:19132")
	if err == nil {
		t.Fatal("dial returned no error for a session that closed mid-dial")
	}
	if conn != nil {
		t.Fatal("dial returned a connection for a closed session")
	}
	if !stream.closed.Load() {
		t.Fatal("the dialled stream was left open")
	}
	if s.serverConn != nil {
		t.Fatal("a closed session was given a server connection")
	}
}
