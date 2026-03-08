package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"moxy/pkg/crdt"
	"moxy/pkg/crypto"
	"moxy/pkg/identity"
	"moxy/pkg/network"
	"moxy/pkg/store"

	"encoding/base64"
	"encoding/json"

	"github.com/spf13/cobra"
)

var (
	sendRoomFlag     string
	sendPortFlag     int
	sendIdentityFlag string
	sendPasswordFlag string
)

var sendCmd = &cobra.Command{
	Use:   "send [message]",
	Short: "Send a message to the mesh room",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()
		
		path := sendIdentityFlag
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

		ctx := context.Background()
		node, err := network.NewNode(ctx, priv, sendPortFlag, sendRoomFlag)
		if err != nil {
			log.Fatalf("Network error: %v", err)
		}

		if err := node.StartMDNS(); err != nil {
			log.Fatalf("Failed to start mDNS: %v", err)
		}

		// Wait for peers before publishing short-lived message
		fmt.Printf("Searching for peers in '%s' on LAN via mDNS...\n", sendRoomFlag)
		
		// Poll topic peers until we find someone or timeout
		timeout := time.After(15 * time.Second)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		
		foundPeer := false
	WaitLoop:
		for {
			select {
			case <-timeout:
				log.Println("Warning: No peers found within 15 seconds. Publishing into the void...")
				break WaitLoop
			case <-ticker.C:
				peers := node.Topic.ListPeers()
				if len(peers) > 0 {
					foundPeer = true
					fmt.Printf("Connected to %d peer(s)! Transmitting...\n", len(peers))
					
					// Small buffer to ensure GossipSub meshes correctly
					time.Sleep(1 * time.Second)
					break WaitLoop
				}
			}
		}

		// Keep a memory ORSet for timestamp generation
		// Read LastClock to properly sequence Lamport Clock for sender
		set := crdt.NewORSet()
		records, _ := db.GetAll()
		for _, rec := range records {
			var m crdt.Message
			json.Unmarshal(rec, &m)
			if m.Clock > set.LastClock {
				set.LastClock = m.Clock
			}
		}

		// 1. Encrypt Payload
		symKey := crypto.DeriveSymmetricKey(sendPasswordFlag, sendRoomFlag)
		cipherBytes, err := crypto.EncryptMessage(symKey, []byte(args[0]))
		if err != nil {
			log.Fatalf("Failed to encrypt payload: %v", err)
		}
		b64Content := base64.StdEncoding.EncodeToString(cipherBytes)

		// 2. CRDT Clock Setup & Object Generation
		msgObj := set.Add(b64Content, ident.PeerID, time.Now().Unix())

		// 3. Digital Signatures
		msgHash := fmt.Sprintf("%s:%s:%d:%d:%s", msgObj.ID, msgObj.Sender, msgObj.Timestamp, msgObj.Clock, msgObj.Content)
		sig, err := crypto.SignPayload(priv, []byte(msgHash))
		if err != nil {
			log.Fatalf("Failed to sign message block: %v", err)
		}
		msgObj.Signature = sig

		// 4. Publish to Distributed Network Stream
		mBytes, _ := json.Marshal(msgObj)
		if err := node.Topic.Publish(ctx, mBytes); err != nil {
			log.Fatalf("Failed to publish: %v", err)
		}

		// Persist internally
		db.Set([]byte(msgObj.ID), mBytes)

		if foundPeer {
			fmt.Printf("Message Sent [%s] (Encrypted & Signed)\n", msgObj.ID)
		}
		
		// Sleep shortly to ensure network flush
		time.Sleep(500 * time.Millisecond)
	},
}

func init() {
	sendCmd.Flags().StringVarP(&sendRoomFlag, "room", "r", "disaster", "Room name to join")
	sendCmd.Flags().IntVarP(&sendPortFlag, "port", "p", 0, "Listen port")
	sendCmd.Flags().StringVarP(&sendIdentityFlag, "identity", "i", "", "Path to identity file (default: ~/.moxy/identity.json)")
	sendCmd.Flags().StringVar(&sendPasswordFlag, "password", "", "Symmetric room password for E2EE (optional)")
	rootCmd.AddCommand(sendCmd)
}
