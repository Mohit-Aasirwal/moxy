package crdt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Message represents a single chat entry.
type Message struct {
	ID        string `json:"id"`
	Sender    string `json:"sender"`
	Timestamp int64  `json:"timestamp"`
	Clock     uint64 `json:"clock"`
	Content   string `json:"content"`
	Signature []byte `json:"signature"`
}

// SyncReq represents an inventory of known message IDs.
type SyncReq struct {
	Type     string   `json:"type"` // "SYNC_REQ"
	Room     string   `json:"room"`
	KnownIDs []string `json:"known_ids"`
}

// SyncRes returns the missing message payloads.
type SyncRes struct {
	Type    string    `json:"type"` // "SYNC_RES"
	Missing []Message `json:"missing"`
}

// GenerateID produces a deterministic ID based on sender and logical clock.
func GenerateID(sender string, clock uint64) string {
	raw := fmt.Sprintf("%s:%d", sender, clock)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// ORSet represents an observed-remove set for messages (append-only here).
type ORSet struct {
	Adds      map[string]Message `json:"adds"`
	LastClock uint64             `json:"last_clock"` // Lamport clock tracking
}

func NewORSet() *ORSet {
	return &ORSet{
		Adds:      make(map[string]Message),
		LastClock: 0,
	}
}

// Merge combines another ORSet into this one, applying Lamport Clock rules.
func (set *ORSet) Merge(other *ORSet) {
	for id, msg := range other.Adds {
		if _, exists := set.Adds[id]; !exists {
			set.Adds[id] = msg
		}
	}
	
	if other.LastClock > set.LastClock {
		set.LastClock = other.LastClock
	}
}

// Add appends a new message. It modifies the current LastClock.
func (set *ORSet) Add(msg string, sender string, ts int64) Message {
	set.LastClock++
	
	m := Message{
		ID:        GenerateID(sender, set.LastClock),
		Sender:    sender,
		Timestamp: ts,
		Clock:     set.LastClock,
		Content:   msg,
	}
	set.Adds[m.ID] = m
	return m
}
