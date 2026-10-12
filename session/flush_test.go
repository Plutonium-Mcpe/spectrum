package session

import (
	"testing"
	"time"

	"github.com/cooldogedev/spectrum/util"
)

// Avec un delai, les demandes de flush d'un meme tick doivent se fondre en une seule :
// c'est tout l'interet, une seule compression au lieu d'une par lot.
func TestScheduleFlushArmsASingleTimer(t *testing.T) {
	s := &Session{opts: util.Opts{FlushDelay: 50 * time.Millisecond}}

	for i := 0; i < 10; i++ {
		if err := s.scheduleFlush(); err != nil {
			t.Fatalf("scheduleFlush: %v", err)
		}
	}

	s.flushMu.Lock()
	armed := s.flushTimer
	s.flushMu.Unlock()
	if armed == nil {
		t.Fatal("aucun flush arme")
	}
	// Un seul minuteur pour dix demandes : les suivantes n'en creent pas d'autre.
	for i := 0; i < 5; i++ {
		_ = s.scheduleFlush()
		s.flushMu.Lock()
		same := s.flushTimer == armed
		s.flushMu.Unlock()
		if !same {
			t.Fatal("une demande supplementaire a arme un second flush")
		}
	}
	s.stopFlushTimer()
}

// Une session fermee ne doit rien laisser arme.
func TestStopFlushTimerClearsThePendingFlush(t *testing.T) {
	s := &Session{opts: util.Opts{FlushDelay: time.Hour}}
	if err := s.scheduleFlush(); err != nil {
		t.Fatalf("scheduleFlush: %v", err)
	}
	s.stopFlushTimer()

	s.flushMu.Lock()
	defer s.flushMu.Unlock()
	if s.flushTimer != nil {
		t.Fatal("un flush est reste arme apres la fermeture")
	}
}
