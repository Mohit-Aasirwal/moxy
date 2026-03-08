package engine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"moxy/pkg/crdt"
	"moxy/pkg/crypto"
	"moxy/pkg/network"
	"moxy/pkg/store"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
)

// Engine integrates the Libp2p node, CRDT state, Crypto, and BadgerDB.
// It exposes clear Go Channels for a TUI (like Bubbletea) to consume natively.
type Engine struct {
	Node     *network.Node
	Store    *store.Store
	Set      *crdt.ORSet
	PeerID   string
	Room     string
	Password string
	PrivKey  libp2pcrypto.PrivKey

	// Outbox is a channel through which the Engine outputs newly received
	// and verified messages so the UI can draw them to the screen.
	Outbox chan crdt.Message
}

const SyncProtocolID = "/moxy/sync/1.0.0"

// NewEngine initializes the database, loads history, and spins up the network.
func NewEngine(ctx context.Context, priv libp2pcrypto.PrivKey, peerID string, dbPath string, room string, password string, port int) (*Engine, error) {
	db, err := store.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("DB error: %w", err)
	}

	set := crdt.NewORSet()
	records, _ := db.GetAll()
	for _, rec := range records {
		var m crdt.Message
		json.Unmarshal(rec, &m)
		
		set.Adds[m.ID] = m
		if m.Clock > set.LastClock {
			set.LastClock = m.Clock
		}
	}

	node, err := network.NewNode(ctx, priv, port, room)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("Network error: %w", err)
	}

	e := &Engine{
		Node:     node,
		Store:    db,
		Set:      set,
		PeerID:   peerID,
		Room:     room,
		Password: password,
		PrivKey:  priv,
		Outbox:   make(chan crdt.Message, 100),
	}

	e.setupSyncStream()
	
	if err := node.StartMDNS(); err != nil {
		return nil, err
	}

	go e.listenPubSub(ctx)

	return e, nil
}

// Close gracefully shuts down the engine bindings
func (e *Engine) Close() {
	e.Store.Close()
	e.Node.Host.Close()
}

// History returns a slice of properly decrypted local DB messages.
func (e *Engine) History() []crdt.Message {
	symKey := crypto.DeriveSymmetricKey(e.Password, e.Room)
	var msgs []crdt.Message
	
	for _, m := range e.Set.Adds {
		cipherBytes, err := base64.StdEncoding.DecodeString(m.Content)
		if err == nil {
			plainBytes, decErr := crypto.DecryptMessage(symKey, cipherBytes)
			if decErr == nil {
				m.Content = string(plainBytes)
			}
		}
		msgs = append(msgs, m)
	}
	
	return msgs
}
