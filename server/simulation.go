// simulation.go - the fleet simulator. Maintains ~12 vessels in memory and,
// once per tick, applies a small random-walk to each position and occasionally
// flips a status. It returns the changed vessels so the hub can broadcast a
// minimal "update" frame; a fresh client instead gets the full "snapshot".
package main

import (
	"encoding/json"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

const (
	fleetSize  = 12
	tickPeriod = time.Second
	// statusFlipChance is the per-vessel, per-tick probability of a status change.
	statusFlipChance = 0.08
	// walkStep bounds the per-tick lat/lng jitter (decimal degrees).
	walkStep = 0.01
)

var (
	vesselNames = []string{
		"Aurora", "Borealis", "Castor", "Drake", "Equinox", "Falcon",
		"Gannet", "Helios", "Iberia", "Juno", "Kraken", "Lyra",
	}
	vesselTypes = []string{"Cargo", "Tanker", "Patrol", "Ferry", "Research"}
	allStatuses = []VesselStatus{StatusUnderway, StatusMoored, StatusAnchored}
)

// Simulator owns fleet state. The mutex guards the vessels slice; seq is atomic
// so the monotonic sequence number is safe to read from a snapshot without the
// lock.
type Simulator struct {
	mu      sync.Mutex
	vessels []Vessel
	seq     atomic.Uint64
	rng     *rand.Rand
}

// newSimulator builds the initial fleet around a base position (mid-Atlantic).
func newSimulator() *Simulator {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	vessels := make([]Vessel, fleetSize)
	for i := 0; i < fleetSize; i++ {
		vessels[i] = Vessel{
			ID:         "vsl-" + itoa(i+1),
			Name:       vesselNames[i%len(vesselNames)],
			VesselType: vesselTypes[rng.Intn(len(vesselTypes))],
			Status:     allStatuses[rng.Intn(len(allStatuses))],
			Lat:        40.0 + rng.Float64()*8.0,
			Lng:        -30.0 + rng.Float64()*8.0,
		}
	}
	return &Simulator{vessels: vessels, rng: rng}
}

// snapshot returns a full "snapshot" message with the current sequence number.
// Sent as the first frame to every newly connected client.
func (s *Simulator) snapshot() ServerMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Vessel, len(s.vessels))
	copy(out, s.vessels)
	return ServerMessage{
		Type:    MessageSnapshot,
		Seq:     s.seq.Load(),
		SentAt:  time.Now().UnixMilli(),
		Vessels: out,
	}
}

// step advances the simulation by one tick and returns the vessels that
// changed. It does NOT assign a sequence number - seq is only consumed when a
// frame is actually broadcast, so the client's seq stream stays contiguous.
func (s *Simulator) step() []Vessel {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := make([]Vessel, 0, len(s.vessels))
	for i := range s.vessels {
		if s.advance(&s.vessels[i]) {
			changed = append(changed, s.vessels[i])
		}
	}
	return changed
}

// advance mutates one vessel in place and reports whether it changed. Moored
// vessels hold position; others random-walk. Any vessel may flip status.
func (s *Simulator) advance(v *Vessel) bool {
	changed := false

	if v.Status != StatusMoored {
		v.Lat += (s.rng.Float64()*2 - 1) * walkStep
		v.Lng += (s.rng.Float64()*2 - 1) * walkStep
		changed = true
	}

	if s.rng.Float64() < statusFlipChance {
		next := allStatuses[s.rng.Intn(len(allStatuses))]
		if next != v.Status {
			v.Status = next
			changed = true
		}
	}
	return changed
}

// broadcastTick advances the simulation by one tick and, if anything changed,
// consumes the next sequence number and returns the update frame plus true. If
// nothing changed it returns the zero message and false, and no sequence number
// is burned - keeping the client-visible seq strictly contiguous. Splitting this
// out of run keeps the per-tick semantics directly testable.
func (s *Simulator) broadcastTick() (ServerMessage, bool) {
	changed := s.step()
	if len(changed) == 0 {
		return ServerMessage{}, false
	}
	return ServerMessage{
		Type:    MessageUpdate,
		Seq:     s.seq.Add(1),
		SentAt:  time.Now().UnixMilli(),
		Vessels: changed,
	}, true
}

// run drives the simulation loop. Each tick it computes the changed vessels;
// only when something actually changed does it consume the next sequence
// number and broadcast. This keeps the client-visible seq strictly contiguous
// so genuine gap detection on the client is meaningful.
func (s *Simulator) run(hub *Hub, done <-chan struct{}) {
	ticker := time.NewTicker(tickPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			msg, ok := s.broadcastTick()
			if !ok {
				continue // nothing moved; do not burn a sequence number
			}
			if payload, err := json.Marshal(msg); err == nil {
				hub.broadcast(payload)
			}
		}
	}
}

// itoa is a tiny base-10 int-to-string helper to avoid importing strconv just
// for vessel ids.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
