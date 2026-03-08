package engine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"moxy/pkg/crdt"
	"moxy/pkg/crypto"

	"github.com/libp2p/go-libp2p/core/peer"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
)

func (e *Engine) setupSyncStream() {
	// 1. Handle incoming direct peer synchronization requests
	e.Node.Host.SetStreamHandler(SyncProtocolID, func(s libp2pnetwork.Stream) {
		defer s.Close()
		var req crdt.SyncReq
		if err := json.NewDecoder(s).Decode(&req); err != nil {
			return
		}
		if req.Room != e.Room || req.Type != "SYNC_REQ" {
			return
		}

		knownMap := make(map[string]bool)
		for _, id := range req.KnownIDs {
			knownMap[id] = true
		}

		var missing []crdt.Message
		for id, msg := range e.Set.Adds {
			if !knownMap[id] {
				missing = append(missing, msg)
			}
		}

		res := crdt.SyncRes{
			Type:    "SYNC_RES",
			Missing: missing,
		}
		json.NewEncoder(s).Encode(res)
	})

	// 2. Perform outgoing sync checks whenever we connect to a new peer
	e.Node.OnPeerConnected = func(pid peer.ID) {
		s, err := e.Node.Host.NewStream(context.Background(), pid, SyncProtocolID)
		if err != nil {
			return
		}
		defer s.Close()

		var known []string
		for id := range e.Set.Adds {
			known = append(known, id)
		}

		req := crdt.SyncReq{
			Type:     "SYNC_REQ",
			Room:     e.Room,
			KnownIDs: known,
		}
		if err := json.NewEncoder(s).Encode(req); err != nil {
			return
		}

		var res crdt.SyncRes
		if err := json.NewDecoder(s).Decode(&res); err != nil {
			return
		}

		if res.Type == "SYNC_RES" {
			symKey := crypto.DeriveSymmetricKey(e.Password, e.Room)

			for _, m := range res.Missing {
				if _, exists := e.Set.Adds[m.ID]; !exists {
					msgHash := fmt.Sprintf("%s:%s:%d:%d:%s", m.ID, m.Sender, m.Timestamp, m.Clock, m.Content)
					valid, err := crypto.VerifySignature(m.Sender, []byte(msgHash), m.Signature)
					if err != nil || !valid {
						// Invalid
						continue
					}

					cipherBytes, err := base64.StdEncoding.DecodeString(m.Content)
					if err != nil { continue }
					
					plainBytes, err := crypto.DecryptMessage(symKey, cipherBytes)
					if err != nil { continue } // wrong password

					e.Set.Adds[m.ID] = m
					mBytes, _ := json.Marshal(m)
					e.Store.Set([]byte(m.ID), mBytes)
					if m.Clock > e.Set.LastClock {
						e.Set.LastClock = m.Clock
					}
					
					// Re-inject the decrypted text for the UI so the frontend doesn't need to decrypt
					m.Content = string(plainBytes)
					e.Outbox <- m 
				}
			}
		}
	}
}

func (e *Engine) listenPubSub(ctx context.Context) {
	for {
		msg, err := e.Node.Sub.Next(ctx)
		if err != nil {
			log.Println("Subscription err:", err)
			continue
		}

		if msg.ReceivedFrom == e.Node.Host.ID() {
			continue // ignore local bounces
		}

		var incoming crdt.Message
		if err := json.Unmarshal(msg.Data, &incoming); err != nil {
			continue
		}

		msgHash := fmt.Sprintf("%s:%s:%d:%d:%s", incoming.ID, incoming.Sender, incoming.Timestamp, incoming.Clock, incoming.Content)
		valid, sigErr := crypto.VerifySignature(incoming.Sender, []byte(msgHash), incoming.Signature)
		if sigErr != nil || !valid {
			continue // bad sig
		}

		if _, exists := e.Set.Adds[incoming.ID]; !exists {
			symKey := crypto.DeriveSymmetricKey(e.Password, e.Room)
			cipherBytes, decErr := base64.StdEncoding.DecodeString(incoming.Content)
			if decErr != nil { continue }
			
			plainBytes, encErr := crypto.DecryptMessage(symKey, cipherBytes)
			if encErr != nil { continue } // Wrong pass

			e.Set.Adds[incoming.ID] = incoming
			if incoming.Clock > e.Set.LastClock {
				e.Set.LastClock = incoming.Clock
			}
			
			e.Store.Set([]byte(incoming.ID), msg.Data)
			
			// Inject to UI
			incoming.Content = string(plainBytes)
			e.Outbox <- incoming
		}
	}
}

// Send injects a locally typed plaintext message into the CRDT, encrypts it, signs it, and broadcasts it
func (e *Engine) Send(plaintext string) error {
	symKey := crypto.DeriveSymmetricKey(e.Password, e.Room)
	cipherBytes, err := crypto.EncryptMessage(symKey, []byte(plaintext))
	if err != nil {
		return err
	}
	b64Content := base64.StdEncoding.EncodeToString(cipherBytes)

	// CRDT Object Generation
	msgObj := e.Set.Add(b64Content, e.PeerID, time.Now().Unix())

	// Sign
	msgHash := fmt.Sprintf("%s:%s:%d:%d:%s", msgObj.ID, msgObj.Sender, msgObj.Timestamp, msgObj.Clock, msgObj.Content)
	sig, err := crypto.SignPayload(e.PrivKey, []byte(msgHash))
	if err != nil {
		return err
	}
	msgObj.Signature = sig

	// RE-INJECT into CRDT Set memory map so peers requesting Sync receive the signature!
	e.Set.Adds[msgObj.ID] = msgObj

	// Publish to Network
	mBytes, _ := json.Marshal(msgObj)
	if err := e.Node.Topic.Publish(context.Background(), mBytes); err != nil {
		return err
	}

	// Persist internally
	e.Store.Set([]byte(msgObj.ID), mBytes)
	
	// Inject plaintext local version into UI queue so the user sees what they typed
	msgObj.Content = string(plaintext)
	e.Outbox <- msgObj

	return nil
}
