// simulation_test.go - unit tests for the fleet simulator and the env-driven
// origin allow-list. The simulation tests assert the two contract guarantees the
// client relies on: the broadcast sequence number strictly increments, and every
// broadcast carries a non-empty changed-set of vessels with valid fields.
package main

import (
	"net/http"
	"testing"
)

// validStatuses mirrors the allowed VesselStatus set for assertion.
var validStatuses = map[VesselStatus]struct{}{
	StatusUnderway: {},
	StatusMoored:   {},
	StatusAnchored: {},
}

// TestBroadcastTickSeqStrictlyIncrements drives the simulator through many ticks
// and asserts that every broadcast frame's seq is exactly the previous seq + 1.
// Ticks that produce no change are skipped (no seq burned), which is the
// contiguous-stream guarantee the client's gap detection depends on.
func TestBroadcastTickSeqStrictlyIncrements(t *testing.T) {
	sim := newSimulator()

	var prevSeq uint64
	broadcasts := 0
	const ticks = 500

	for i := 0; i < ticks; i++ {
		msg, ok := sim.broadcastTick()
		if !ok {
			continue // nothing changed this tick; no seq consumed
		}
		broadcasts++

		if msg.Type != MessageUpdate {
			t.Fatalf("broadcast %d: expected type %q, got %q", broadcasts, MessageUpdate, msg.Type)
		}
		if msg.Seq != prevSeq+1 {
			t.Fatalf("broadcast %d: seq not strictly incrementing: got %d, want %d", broadcasts, msg.Seq, prevSeq+1)
		}
		prevSeq = msg.Seq
	}

	if broadcasts == 0 {
		t.Fatalf("expected at least one broadcast across %d ticks, got none", ticks)
	}
}

// TestBroadcastTickProducesValidChangedSet asserts that whenever a tick
// broadcasts, the frame carries a non-empty set of vessels and every vessel has
// a valid status and lat/lng within sane geographic bounds.
func TestBroadcastTickProducesValidChangedSet(t *testing.T) {
	sim := newSimulator()
	broadcasts := 0

	for i := 0; i < 500 && broadcasts < 20; i++ {
		msg, ok := sim.broadcastTick()
		if !ok {
			continue
		}
		broadcasts++

		if len(msg.Vessels) == 0 {
			t.Fatalf("broadcast %d: changed-set is empty", broadcasts)
		}
		for _, v := range msg.Vessels {
			if v.ID == "" {
				t.Errorf("broadcast %d: vessel has empty id", broadcasts)
			}
			if _, ok := validStatuses[v.Status]; !ok {
				t.Errorf("broadcast %d: vessel %s has invalid status %q", broadcasts, v.ID, v.Status)
			}
			if v.Lat < -90 || v.Lat > 90 {
				t.Errorf("broadcast %d: vessel %s lat out of bounds: %f", broadcasts, v.ID, v.Lat)
			}
			if v.Lng < -180 || v.Lng > 180 {
				t.Errorf("broadcast %d: vessel %s lng out of bounds: %f", broadcasts, v.ID, v.Lng)
			}
		}
	}

	if broadcasts == 0 {
		t.Fatal("expected at least one broadcast, got none")
	}
}

// TestSnapshotReflectsCurrentSeq verifies a snapshot taken after some broadcasts
// reports the latest sequence number, so a freshly connected client starts its
// gap cursor at the right place.
func TestSnapshotReflectsCurrentSeq(t *testing.T) {
	sim := newSimulator()
	var last uint64
	for i := 0; i < 200; i++ {
		if msg, ok := sim.broadcastTick(); ok {
			last = msg.Seq
		}
	}
	snap := sim.snapshot()
	if snap.Type != MessageSnapshot {
		t.Fatalf("expected snapshot type, got %q", snap.Type)
	}
	if snap.Seq != last {
		t.Fatalf("snapshot seq %d does not match last broadcast seq %d", snap.Seq, last)
	}
	if len(snap.Vessels) != fleetSize {
		t.Fatalf("snapshot should carry the full fleet of %d, got %d", fleetSize, len(snap.Vessels))
	}
}

// TestParseAllowedOriginsDefaultsToDevOrigins asserts that an unset/blank value
// yields exactly the localhost dev origins.
func TestParseAllowedOriginsDefaultsToDevOrigins(t *testing.T) {
	for _, raw := range []string{"", "   ", " , , "} {
		set := parseAllowedOrigins(raw)
		if len(set) != len(defaultDevOrigins) {
			t.Fatalf("raw %q: expected %d dev origins, got %d", raw, len(defaultDevOrigins), len(set))
		}
		for _, want := range defaultDevOrigins {
			if _, ok := set[want]; !ok {
				t.Errorf("raw %q: missing dev origin %q", raw, want)
			}
		}
	}
}

// TestParseAllowedOriginsTrimsAndIgnoresEmpties verifies robust parsing of a
// comma-separated allow-list with stray spaces and empty entries.
func TestParseAllowedOriginsTrimsAndIgnoresEmpties(t *testing.T) {
	set := parseAllowedOrigins(" https://lfg.manumustudio.com , , http://localhost:5173 ")
	if len(set) != 2 {
		t.Fatalf("expected 2 origins, got %d: %v", len(set), set)
	}
	for _, want := range []string{"https://lfg.manumustudio.com", "http://localhost:5173"} {
		if _, ok := set[want]; !ok {
			t.Errorf("missing origin %q", want)
		}
	}
}

// TestOriginCheckerAllowsAndRejects exercises the CheckOrigin func: the
// production domain and dev origin are allowed, an unknown origin is rejected,
// and a no-Origin (non-browser) request is allowed.
func TestOriginCheckerAllowsAndRejects(t *testing.T) {
	allowed := parseAllowedOrigins("https://lfg.manumustudio.com")
	check := newOriginChecker(allowed)

	cases := []struct {
		origin string
		want   bool
	}{
		{"https://lfg.manumustudio.com", true},
		{"https://evil.example.com", false},
		{"http://localhost:5173", false}, // not in this allow-list
		{"", true},                       // non-browser client, no Origin header
	}
	for _, c := range cases {
		req, err := http.NewRequest(http.MethodGet, "/ws", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		if got := check(req); got != c.want {
			t.Errorf("origin %q: got %v, want %v", c.origin, got, c.want)
		}
	}
}
