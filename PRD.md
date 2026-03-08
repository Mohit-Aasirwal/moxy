Good. We’re doing this properly.

This will not be a fluffy “idea doc.”
This is a **real PRD + technical spec** for a production-grade MVP.

Project codename: **MeshRelief**

---

# 1. Product Requirements Document (PRD)

## 1.1 Vision

MeshRelief is a **resilient, encrypted, delay-tolerant CLI messaging system** that enables communication during internet shutdowns, disasters, or infrastructure collapse.

It forms a **self-healing peer-to-peer mesh network** using local connectivity and synchronizes globally when any node regains internet access.

---

## 1.2 Problem Statement

During disasters or political shutdowns:

* Internet may be unavailable.
* Cellular networks may fail.
* Centralized servers become unreachable.
* Communication blackouts cause real harm.

Existing messaging systems:

* Depend on centralized infra.
* Do not tolerate long partitions.
* Are not mesh-native.

---

## 1.3 Target Users

Primary:

* Emergency responders
* Disaster volunteers
* Civilian groups in blackout regions

Secondary:

* Activists in censored regions
* Remote communities
* Field research teams

---

## 1.4 Non-Goals (Important)

* No media streaming
* No voice/video
* No UI beyond CLI
* No centralized servers
* No blockchain
* No token economy

If you add these early, you’ll kill the project.

---

# 2. Functional Requirements

## 2.1 Identity

* Each node generates a persistent Ed25519 keypair.
* PeerID derived from public key.
* Identity stored locally.

Command:

```bash
mesh init
```

---

## 2.2 Network Formation

Nodes must:

* Discover peers on same LAN via mDNS.
* Form P2P connections via libp2p.
* Join named “rooms” (pubsub topics).

Command:

```bash
mesh join disaster-room
```

---

## 2.3 Messaging

Users can:

```bash
mesh send "Water available at Sector 9"
```

System must:

* Encrypt message at application layer.
* Assign logical timestamp.
* Store locally.
* Broadcast via pubsub.

---

## 2.4 Delay-Tolerant Sync

When nodes reconnect:

* Exchange message inventories.
* Sync missing messages.
* Resolve conflicts using CRDT logic.

Must support:

* Network partitions
* Offline periods
* Rejoins

---

## 2.5 Internet Bridge Mode

If node regains internet:

* Connect to global DHT via Tor or public bootstrap nodes.
* Sync remote state.
* Propagate to local mesh.

Command:

```bash
mesh sync
```

---

# 3. Non-Functional Requirements

| Category   | Requirement                       |
| ---------- | --------------------------------- |
| Latency    | Sub-500ms LAN message propagation |
| Resilience | Survive 48hr offline partition    |
| Security   | E2EE + forward secrecy            |
| Storage    | Local append-only log             |
| Binary     | Single static executable          |
| Memory     | <150MB RAM usage                  |
| Platform   | Linux, Mac, Windows               |

---

# 4. System Architecture

We design this layered.

## 4.1 Layered Architecture

```text
CLI Layer
↓
Application Layer (Messaging Engine + CRDT)
↓
Sync Protocol Layer
↓
P2P Networking Layer (libp2p)
↓
Transport Security (Noise)
↓
Network Interfaces (LAN, Tor)
```

---

# 5. Low-Level Architecture

Now we get serious.

---

## 5.1 Module Breakdown

### 5.1.1 CLI Module

Responsible for:

* Command parsing
* Rendering messages
* Triggering sync

Tech:

* Cobra (Go CLI framework)

---

### 5.1.2 Identity Module

Responsibilities:

* Keypair generation
* PeerID derivation
* Persistent storage

Data format:

```json
{
  "private_key": "...",
  "public_key": "...",
  "peer_id": "Qm..."
}
```

Stored in:

```
~/.meshrelief/identity.json
```

---

### 5.1.3 P2P Networking Layer

Tech:

* go-libp2p
* GossipSub
* Kademlia DHT

Features:

* mDNS discovery
* Peer connection management
* Topic subscription

Connection lifecycle:

1. Discover peer
2. Secure channel via Noise
3. Subscribe to topic
4. Begin pubsub

---

### 5.1.4 Messaging Engine

Core structure:

```go
type Message struct {
    ID        string
    Sender    string
    Timestamp int64
    Clock     uint64
    Content   string
    Signature []byte
}
```

Message ID:

```
SHA256(Sender + Clock)
```

Clock:
Lamport clock.

---

## 5.2 CRDT Design

We use:

> Observed-Remove Set (OR-Set)

Why:

* Messages are append-only.
* Supports concurrent insertions.

Local state:

```go
type ORSet struct {
    Adds map[string]Message
}
```

Merge rule:

* Union of message IDs.
* If duplicate → ignore.

Lamport update:

```
localClock = max(localClock, incomingClock) + 1
```

---

## 5.3 Storage Engine

Use:

* BadgerDB or BoltDB (embedded KV store)

Structure:

```
messages/
    messageID → serialized message
metadata/
    lastClock
    joinedRooms
```

Append-only.
Never delete.

---

## 5.4 Sync Protocol (Critical)

Pubsub is NOT enough.

We define explicit sync protocol:

### SYNC_REQ

```json
{
  "type": "SYNC_REQ",
  "room": "disaster",
  "known_ids": ["id1","id2"]
}
```

### SYNC_RES

```json
{
  "type": "SYNC_RES",
  "missing": [Message, Message]
}
```

Algorithm:

1. Node A connects.
2. Sends inventory (hash set).
3. Node B computes difference.
4. Sends missing messages.
5. A merges into CRDT.

Time complexity:
O(n) per sync.

Optimization later:

* Bloom filters.

---

# 6. Encryption Model

## 6.1 Transport Security

libp2p uses:

* Noise protocol
* Ephemeral keys
* Perfect forward secrecy

---

## 6.2 Application-Level E2EE

Each room has:

```go
type Room struct {
    Name string
    SymmetricKey []byte
}
```

Encryption:

* AES-256-GCM
* Nonce = random 12 bytes

Before publish:

```
ciphertext = AES(roomKey, plaintext)
```

---

## 6.3 Signature

Each message signed:

```
signature = Ed25519(privateKey, messageHash)
```

Verification on receipt:

* Validate signature
* Reject if invalid

---

# 7. Tor Bridge Mode

If internet available:

1. Start Tor client.
2. Connect via SOCKS5.
3. Bootstrap to public DHT peers.
4. Join global topic.
5. Sync state.

Tor mode optional:

```
mesh sync --tor
```

---

# 8. Failure Modes

You must design for:

* Split-brain partitions
* Malicious peers
* Replay attacks
* Message flooding
* Clock skew

Mitigation:

| Threat         | Solution                        |
| -------------- | ------------------------------- |
| Replay         | Message ID dedupe               |
| Flooding       | Rate limit per peer             |
| Malicious spam | Reputation system (later)       |
| Clock abuse    | Ignore absurd future timestamps |

---

# 9. Performance Model

Assume:

* 100 nodes local mesh
* 10,000 messages total

Storage footprint:
~2MB

Sync cost:
Full inventory exchange ≈ 100KB

---

# 10. CLI Commands Spec

```bash
mesh init
mesh join <room>
mesh leave <room>
mesh send "<text>"
mesh peers
mesh sync
mesh status
mesh export-log
```

---

# 11. Observability

Add:

* Peer connection logs
* Sync metrics
* Message propagation latency
* Debug mode flag

---

# 12. Deployment Strategy

* Static Go binary
* No external dependencies
* Cross-compiled builds

Distribution:

* GitHub releases
* GPG signed binaries

---

# 13. Roadmap

Phase 1:
LAN messaging + CRDT

Phase 2:
Explicit sync protocol

Phase 3:
Encryption + signatures

Phase 4:
Tor bridge

Phase 5:
Bluetooth mesh

---

# 14. Hard Engineering Truth

The hardest parts:

1. Sync correctness
2. CRDT merge integrity
3. Tor integration stability
4. Partition simulation testing

Not:

* CLI
* Pubsub

If sync logic is wrong, everything collapses.

---

# 15. Future Expansion

If this works:

* Mobile client
* Emergency broadcast mode
* GPS coordinate tagging
* Encrypted file transfer
* Satellite relay integration
* Community governance model

---

# Final Reality Check

This is distributed systems + networking + cryptography.

If you execute properly:
You will understand P2P systems better than 90% of engineers.

If you cut corners:
It becomes a glorified LAN chat.

---
