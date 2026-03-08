package crdt_test

import (
	"testing"
	"time"

	"moxy/pkg/crdt"
)

func TestMessageID_Generation(t *testing.T) {
	id1 := crdt.GenerateID("peerA", 1)
	id2 := crdt.GenerateID("peerA", 1)
	id3 := crdt.GenerateID("peerA", 2)
	id4 := crdt.GenerateID("peerB", 1)

	if id1 != id2 {
		t.Errorf("Expected deterministic ID generation, got %s and %s", id1, id2)
	}
	if id1 == id3 {
		t.Errorf("Expected different IDs for different clocks")
	}
	if id1 == id4 {
		t.Errorf("Expected different IDs for different senders")
	}
}

func TestORSet_AddAndMerge(t *testing.T) {
	setA := crdt.NewORSet()
	setB := crdt.NewORSet()

	msgA1 := setA.Add("hello from A", "peerA", time.Now().Unix())
	msgA2 := setA.Add("another from A", "peerA", time.Now().Unix())

	if setA.LastClock != 2 {
		t.Errorf("Expected setA LastClock to be 2, got %d", setA.LastClock)
	}

	// Merge A into B
	setB.Merge(setA)

	if len(setB.Adds) != 2 {
		t.Errorf("Expected setB to have 2 messages, got %d", len(setB.Adds))
	}
	if setB.LastClock != 2 {
		t.Errorf("Expected setB LastClock to be 2, got %d", setB.LastClock)
	}

	// Add independently to B (clock will increment from 2 -> 3)
	msgB1 := setB.Add("hello from B", "peerB", time.Now().Unix())

	// Merge back to A
	setA.Merge(setB)

	if len(setA.Adds) != 3 {
		t.Errorf("Expected setA to have 3 messages, got %d", len(setA.Adds))
	}
	if setA.LastClock != 3 {
		t.Errorf("Expected setA LastClock to be 3, got %d", setA.LastClock)
	}

	_, existsA1 := setA.Adds[msgA1.ID]
	_, existsA2 := setA.Adds[msgA2.ID]
	_, existsB1 := setA.Adds[msgB1.ID]

	if !existsA1 || !existsA2 || !existsB1 {
		t.Errorf("Merged set missing expected messages")
	}
}
