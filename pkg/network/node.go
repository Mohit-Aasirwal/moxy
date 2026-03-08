package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

// MDNSTag is the discovery rendezvous point
const MDNSTag = "moxy-disaster-protocol-v1"

type Node struct {
	Host             host.Host
	PubSub           *pubsub.PubSub
	RoomName         string
	Topic            *pubsub.Topic
	Sub              *pubsub.Subscription
	OnPeerConnected  func(peer.ID) // Callback for sync protocol
	
	peerLock sync.RWMutex
	Peers    map[peer.ID]bool
}

// discoveryNotifee satisfies mdns.Notifee interface
type discoveryNotifee struct {
	node *Node
}

func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	// Attempt connecting to found peers on LAN
	if pi.ID == n.node.Host.ID() {
		return
	}
	
	n.node.peerLock.Lock()
	if n.node.Peers[pi.ID] {
		n.node.peerLock.Unlock()
		return // Already connecting or connected
	}
	n.node.Peers[pi.ID] = true
	n.node.peerLock.Unlock()

	// Asynchronously connect 
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		err := n.node.Host.Connect(ctx, pi)
		if err != nil {
			fmt.Printf("Failed to connect to discovered peer: %v\n", err)
			return
		}
		
		if n.node.OnPeerConnected != nil {
			n.node.OnPeerConnected(pi.ID)
		}
	}()
}

// NewNode instantiates a libp2p host and configures pubsub and mdns.
func NewNode(ctx context.Context, priv crypto.PrivKey, port int, room string) (*Node, error) {
	addr := fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)
	
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(addr),
		libp2p.Identity(priv),
	)
	if err != nil {
		return nil, err
	}

	// Setup PubSub (GossipSub protocol)
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		return nil, err
	}

	// Join the specified room topic
	topic, err := ps.Join(room)
	if err != nil {
		return nil, err
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	node := &Node{
		Host:     h,
		PubSub:   ps,
		RoomName: room,
		Topic:    topic,
		Sub:      sub,
		Peers:    make(map[peer.ID]bool),
	}

	return node, nil
}

// StartMDNS initializes local discovery. Must be called after callbacks are bound!
func (n *Node) StartMDNS() error {
	mdnsService := mdns.NewMdnsService(n.Host, MDNSTag, &discoveryNotifee{node: n})
	return mdnsService.Start()
}
