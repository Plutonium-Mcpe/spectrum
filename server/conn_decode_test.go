package server

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/cooldogedev/spectrum/protocol"
	"github.com/golang/snappy"
)

// frame builds the wire form the backend sends: length prefix, marker, snappy payload.
func frame(raw []byte) []byte {
	body := append([]byte{packetDecodeNotNeeded}, snappy.Encode(nil, raw)...)
	out := make([]byte, 4, 4+len(body))
	binary.BigEndian.PutUint32(out, uint32(len(body)))
	return append(out, body...)
}

func fill(b byte, n int) []byte {
	raw := make([]byte, n)
	for i := range raw {
		raw[i] = b
	}
	return raw
}

func connOver(payloads ...[]byte) *Conn {
	var wire bytes.Buffer
	for _, p := range payloads {
		wire.Write(frame(p))
	}
	return &Conn{reader: protocol.NewReader(&wire), spawned: make(chan struct{})}
}

// Once the buffer is reused, a packet must still decode to exactly what was sent,
// whether it is bigger or smaller than the one before it.
func TestReadReusesBufferWithoutCorrupting(t *testing.T) {
	small, large, smallAgain := fill('a', 300), fill('b', 9000), fill('c', 120)
	c := connOver(small, large, smallAgain)
	close(c.spawned)

	for _, want := range [][]byte{small, large, smallAgain} {
		got, err := c.read()
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if !bytes.Equal(got.([]byte), want) {
			t.Fatalf("decoded %d bytes starting %q, want %d bytes starting %q", len(got.([]byte)), first(got.([]byte)), len(want), first(want))
		}
	}
}

// The buffer has to grow to the largest packet seen and then be kept, otherwise
// every large packet allocates again.
func TestReadGrowsAndKeepsTheBuffer(t *testing.T) {
	c := connOver(fill('a', 300), fill('b', 9000), fill('c', 120))
	close(c.spawned)

	if _, err := c.read(); err != nil {
		t.Fatalf("read: %v", err)
	}
	afterSmall := cap(c.decodeBuf)
	if _, err := c.read(); err != nil {
		t.Fatalf("read: %v", err)
	}
	afterLarge := cap(c.decodeBuf)
	if afterLarge <= afterSmall {
		t.Fatalf("buffer did not grow for the larger packet: %d -> %d", afterSmall, afterLarge)
	}
	if _, err := c.read(); err != nil {
		t.Fatalf("read: %v", err)
	}
	if cap(c.decodeBuf) != afterLarge {
		t.Fatalf("buffer shrank back to %d after a small packet, want %d", cap(c.decodeBuf), afterLarge)
	}
}

// Before the player spawns, packets are deferred and outlive the next read, so each
// one must get its own buffer.
func TestReadDoesNotReuseBeforeSpawn(t *testing.T) {
	first, second := fill('a', 500), fill('b', 500)
	c := connOver(first, second)

	kept, err := c.read()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, err := c.read(); err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(kept.([]byte), first) {
		t.Fatal("a packet held across a read was overwritten by the next one")
	}
	if c.decodeBuf != nil {
		t.Fatal("the shared buffer was populated before spawn")
	}
}

func first(b []byte) string {
	if len(b) > 4 {
		return string(b[:4])
	}
	return string(b)
}
