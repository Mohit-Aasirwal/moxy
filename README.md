<h1 align="center">🚀 Moxy</h1>

<div align="center">
  <strong>A resilient, zero-config, delay-tolerant, fully encrypted P2P terminal messenger.</strong>
</div>

<br />

Moxy is an interactive CLI chat application designed to operate brilliantly on ad-hoc local networks—without centralized servers or internet access. Using LibP2P GossipSub routing, Conflict-free Replicated Data Types (CRDTs), and robust ChaCha20 encryption, Moxy ensures your delay-tolerant message history is completely secure and eventually consistent.

---

### ✨ Features
- **Zero-Configuration Wizard**: Just run `./moxy` to enter a beautifully styled interactive prompt—no confusing terminal flags required!
- **Terminal User Interface (TUI)**: Powered by [Bubbletea](https://github.com/charmbracelet/bubbletea) and [Lipgloss](https://github.com/charmbracelet/lipgloss) for a stunning, responsive layout right in your console.
- **Delay-Tolerant Networking (DTN)**: Offline messages are securely stored in a local BadgerDB and synchronized sequentially the exact millisecond your node re-discovers its peers on the network.
- **End-to-End Encryption**: Symmetric payload encryption using ChaCha20, natively paired with Ed25519 cryptographic signatures to cryptographically guarantee message integrity.
- **Serverless Automation**: Automatic peer routing over the local network utilizing Multicast DNS (mDNS), combined with random kernel-assigned TCP ephemeral ports for clashless onboarding.

## 📥 Installation

Ensure you have **Go 1.24+** installed on your system.
```bash
git clone https://github.com/yourusername/moxy.git
cd moxy
go build -o moxy .
```

## 💻 Usage

### The Interactive Wizard (Recommended)
For a completely frictionless experience, launch the binary without any arguments:
```bash
./moxy
```
Moxy will beautifully prompt you for the **Room Name** and an optional **Secure Password**. Under the hood, it automatically generates your Ed25519 identity profile, binds to a free operating system port, intercepts your local network routing, and drops you straight into the interactive UI. We strongly recommend this method!

### Power User CLI Flags
If you want to spin up custom instances manually, use pre-existing keypairs, or launch Moxy natively in automated scripts:
```bash
./moxy chat --room "disaster" --password "super_secret" --identity ~/.moxy/hacker.json --port 8000
```
Use `./moxy --help` to see all available programmatic parameters.

## 🛠 Architecture Under the Hood
1. **Network Layer**: `go-libp2p` powers the mDNS discovery and GossipSub multi-peer routing across disjointed local subnets.
2. **State Layer**: An Append-Only Map CRDT utilizing Lamport Clocks precisely handles chronological ordering even when multiple delayed nodes merge differing network histories simultaneously.
3. **Storage Layer**: Physical persistence is maintained by `BadgerDB` efficiently storing encrypted Base64 binary blobs.
4. **Presentation Layer**: Built structurally with Bubbletea. The UI rendering components are safely decoupled from the underlying cryptographic CRDT maps to prevent accidental payload signature mutations when syncing.

## 🤝 Contributing
Pull requests are heavily encouraged! For major architectural changes or core protocol modifications, please open an issue first to discuss what you would like to change.

## 📜 License
[MIT](https://choosealicense.com/licenses/mit/)
