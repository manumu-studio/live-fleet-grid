// vessel.go - domain model for a tracked vessel and the JSON wire format
// broadcast to clients. Mirrors a VMS "tracked entity" (camera/sensor) record.
package main

// VesselStatus is the constrained set of operational states a vessel can be in.
// Kept as a string type (not an int enum) so the JSON wire format is self-describing.
type VesselStatus string

const (
	StatusUnderway VesselStatus = "UNDERWAY"
	StatusMoored   VesselStatus = "MOORED"
	StatusAnchored VesselStatus = "ANCHORED"
)

// Vessel is a single tracked entity. Lat/Lng are decimal degrees.
type Vessel struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	VesselType string       `json:"vesselType"`
	Status     VesselStatus `json:"status"`
	Lat        float64      `json:"lat"`
	Lng        float64      `json:"lng"`
}

// MessageType discriminates a full snapshot from an incremental update.
type MessageType string

const (
	MessageSnapshot MessageType = "snapshot"
	MessageUpdate   MessageType = "update"
)

// ServerMessage is the single envelope sent over the socket. The monotonic Seq
// lets the client detect dropped frames; SentAt (unix ms) supports latency math.
type ServerMessage struct {
	Type    MessageType `json:"type"`
	Seq     uint64      `json:"seq"`
	SentAt  int64       `json:"sentAt"`
	Vessels []Vessel    `json:"vessels"`
}
