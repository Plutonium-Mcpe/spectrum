package session

import (
	"context"
	"errors"
	"testing"
)

func newTestSession(registry *Registry) *Session {
	s := &Session{registry: registry}
	s.ctx, s.cancelFunc = context.WithCancelCause(context.Background())
	return s
}

// A session that dies during the login sequence closes before it is ever
// registered, so its RemoveSession finds nothing. Registering it afterwards would
// leave a dead session in the registry that nothing can ever take out, since
// CloseWithError only runs once.
func TestRegisterUndoesItselfWhenTheSessionAlreadyClosed(t *testing.T) {
	registry := NewRegistry()
	s := newTestSession(registry)
	s.cancelFunc(errors.New("failed to read packet from client"))

	if err := s.register("xuid"); err == nil {
		t.Fatal("register accepted a closed session")
	}
	if len(registry.GetSessions()) != 0 {
		t.Fatalf("registry holds %d session(s), want 0", len(registry.GetSessions()))
	}
}

func TestRegisterKeepsALiveSession(t *testing.T) {
	registry := NewRegistry()
	s := newTestSession(registry)

	if err := s.register("xuid"); err != nil {
		t.Fatalf("register refused a live session: %v", err)
	}
	if registry.GetSession("xuid") != s {
		t.Fatal("the session was not registered")
	}
}

// A close that lands after the registration removes the entry itself.
func TestCloseAfterRegisterRemovesTheSession(t *testing.T) {
	registry := NewRegistry()
	s := newTestSession(registry)
	if err := s.register("xuid"); err != nil {
		t.Fatalf("register refused a live session: %v", err)
	}

	registry.RemoveSessionIf("xuid", s)
	if len(registry.GetSessions()) != 0 {
		t.Fatalf("registry holds %d session(s), want 0", len(registry.GetSessions()))
	}
}

// A player who reconnects registers a new session under the same xuid. The late
// close of the previous one must not unregister the live session.
func TestRemoveSessionIfSparesAReconnectedPlayer(t *testing.T) {
	registry := NewRegistry()
	previous := newTestSession(registry)
	current := newTestSession(registry)

	registry.AddSession("xuid", previous)
	registry.AddSession("xuid", current)
	registry.RemoveSessionIf("xuid", previous)

	if registry.GetSession("xuid") != current {
		t.Fatal("the reconnected player's session was unregistered")
	}
}
