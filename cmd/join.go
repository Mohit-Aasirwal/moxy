package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"moxy/pkg/crdt"
	"moxy/pkg/crypto"
	"moxy/pkg/identity"
	"moxy/pkg/network"
	"moxy/pkg/store"

	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/spf13/cobra"
)

var (
	joinPortFlag     int
	joinIdentityFlag string
	joinPasswordFlag string
)

var joinCmd = &cobra.Command{
	Use:   "join [room]",
	Short: "Join a mesh room and listen for messages indefinitely",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		room := args[0]
		
		path := joinIdentityFlag
		home, _ := os.UserHomeDir()
		if path == "" {
			path = filepath.Join(home, ".moxy", "identity.json")
		}

		ident, priv, err := identity.Load(path)
		if err != nil {
			log.Fatalf("Could not load identity at %s. Did you run 'mesh init'? Error: %v", path, err)
		}

		// INIT STORAGE
		dbPath := filepath.Join(home, ".moxy", "store_"+ident.PeerID)
		db, err := store.Open(dbPath)
		if err != nil {
			log.Fatalf("DB error: %v", err)
		}
		defer db.Close()

		set := crdt.NewORSet()
		
		// Load existing DB inventories into local memory 
		records, _ := db.GetAll()
		for _, rec := range records {
			var m crdt.Message
			json.Unmarshal(rec, &m)
			set.Adds[m.ID] = m
			if m.Clock > set.LastClock {
				set.LastClock = m.Clock
			}
		}

		ctx := context.Background()
		node, err := network.NewNode(ctx, priv, joinPortFlag, room)
		if err != nil {
			log.Fatalf("Network error: %v", err)
		}

		const SyncProtocolID = "/moxy/sync/1.0.0"

		// 1. Handle incoming direct peer synchronization requests
		node.Host.SetStreamHandler(SyncProtocolID, func(s libp2pnetwork.Stream) {
			defer s.Close()
			var req crdt.SyncReq
			if err := json.NewDecoder(s).Decode(&req); err != nil {
				return
			}
			if req.Room != room || req.Type != "SYNC_REQ" {
				return
			}

			knownMap := make(map[string]bool)
			for _, id := range req.KnownIDs {
				knownMap[id] = true
			}

			var missing []crdt.Message
			for id, msg := range set.Adds {
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
		node.OnPeerConnected = func(pid peer.ID) {
			s, err := node.Host.NewStream(ctx, pid, SyncProtocolID)
			if err != nil {
				return
			}
			defer s.Close()

			var known []string
			for id := range set.Adds {
				known = append(known, id)
			}

			req := crdt.SyncReq{
				Type:     "SYNC_REQ",
				Room:     room,
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
				symKey := crypto.DeriveSymmetricKey(joinPasswordFlag, room)

				for _, m := range res.Missing {
					if _, exists := set.Adds[m.ID]; !exists {
						msgHash := fmt.Sprintf("%s:%s:%d:%d:%s", m.ID, m.Sender, m.Timestamp, m.Clock, m.Content)
						valid, err := crypto.VerifySignature(m.Sender, []byte(msgHash), m.Signature)
						if err != nil || !valid {
							fmt.Printf("\n[SEC-WARN] Dropped invalid signature from %s during sync. Err: %v\n", m.Sender[:8], err)
							continue
						}

						cipherBytes, err := base64.StdEncoding.DecodeString(m.Content)
						if err != nil {
							continue
						}
						plainBytes, err := crypto.DecryptMessage(symKey, cipherBytes)
						if err != nil {
							fmt.Printf("\n[SEC-WARN] Decryption failed for %s. Wrong password?\n", m.ID)
							continue
						}

						set.Adds[m.ID] = m
						mBytes, _ := json.Marshal(m)
						db.Set([]byte(m.ID), mBytes)
						if m.Clock > set.LastClock {
							set.LastClock = m.Clock
						}
						fmt.Printf("\n[SYNC] Restored %s: %s\n", m.Sender[:8], string(plainBytes))
					}
				}
			}
		}

		// Now safe to begin discovering peers and accepting streams
		if err := node.StartMDNS(); err != nil {
			log.Fatalf("Failed to start mDNS: %v", err)
		}

		fmt.Printf("Joined room: %s as %s (Messages loaded: %d)\n", room, ident.PeerID, len(set.Adds))

		// 3. Listen to live PubSub messages passively
		for {
			msg, err := node.Sub.Next(ctx)
			if err != nil {
				log.Println("Subscription err:", err)
				continue
			}

			if msg.ReceivedFrom == node.Host.ID() {
				continue
			}

			var incoming crdt.Message
			if err := json.Unmarshal(msg.Data, &incoming); err != nil {
				continue
			}

			msgHash := fmt.Sprintf("%s:%s:%d:%d:%s", incoming.ID, incoming.Sender, incoming.Timestamp, incoming.Clock, incoming.Content)
			valid, sigErr := crypto.VerifySignature(incoming.Sender, []byte(msgHash), incoming.Signature)
			if sigErr != nil || !valid {
				fmt.Printf("\n[SEC-WARN] Dropped invalid signature from %s. Err: %v\n", incoming.Sender[:8], sigErr)
				continue
			}

			if _, exists := set.Adds[incoming.ID]; !exists {
				symKey := crypto.DeriveSymmetricKey(joinPasswordFlag, room)
				cipherBytes, decErr := base64.StdEncoding.DecodeString(incoming.Content)
				if decErr != nil { continue }
				plainBytes, encErr := crypto.DecryptMessage(symKey, cipherBytes)
				if encErr != nil {
					fmt.Printf("\n[SEC-WARN] Decryption failed for incoming payload. Wrong password?\n")
					continue
				}

				set.Adds[incoming.ID] = incoming
				if incoming.Clock > set.LastClock {
				    set.LastClock = incoming.Clock
				}
				
				db.Set([]byte(incoming.ID), msg.Data)
				fmt.Printf("\n[RCV] %s: %s\n", incoming.Sender[:8], string(plainBytes))
			}
		}
	},
}

func init() {
	joinCmd.Flags().IntVarP(&joinPortFlag, "port", "p", 0, "Listen port")
	joinCmd.Flags().StringVarP(&joinIdentityFlag, "identity", "i", "", "Path to identity file (default: ~/.moxy/identity.json)")
	joinCmd.Flags().StringVar(&joinPasswordFlag, "password", "", "Symmetric room password for E2EE (optional)")
	rootCmd.AddCommand(joinCmd)
}
