package identity

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

type Identity struct {
    PrivateKey string `json:"private_key"`
    PublicKey  string `json:"public_key"`
    PeerID     string `json:"peer_id"`
}

// GenerateKeyPair generates a new Ed25519 keypair and creates an Identity structure.
func GenerateKeyPair() (*Identity, crypto.PrivKey, error) {
    priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
    if err != nil {
        return nil, nil, err
    }

    privBytes, err := crypto.MarshalPrivateKey(priv)
    if err != nil {
        return nil, nil, err
    }

    pubBytes, err := crypto.MarshalPublicKey(pub)
    if err != nil {
        return nil, nil, err
    }

    peerID, err := peer.IDFromPublicKey(pub)
    if err != nil {
        return nil, nil, err
    }

    // Base64 encode for CLI display / JSON output
    // Alternatively, storing the raw crypto.PrivKey protobuf is standard.
    // For this design, we will stringify things safely.
    
    // We can use standard libp2p string formats.
    privStr := crypto.ConfigEncodeKey(privBytes)
    pubStr := crypto.ConfigEncodeKey(pubBytes)

    ident := &Identity{
        PrivateKey: privStr,
        PublicKey:  pubStr,
        PeerID:     peerID.String(),
    }
    return ident, priv, nil
}

// Save writes the Identity struct to the specified path.
func (id *Identity) Save(path string) error {
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0700); err != nil {
        return err
    }

    b, err := json.MarshalIndent(id, "", "  ")
    if err != nil {
        return err
    }

    return os.WriteFile(path, b, 0600)
}

// Load reads and parses an Identity struct from the specified path.
func Load(path string) (*Identity, crypto.PrivKey, error) {
    b, err := os.ReadFile(path)
    if err != nil {
        return nil, nil, fmt.Errorf("could not read identity file: %v", err)
    }

    var id Identity
    if err := json.Unmarshal(b, &id); err != nil {
        return nil, nil, fmt.Errorf("could not parse identity: %v", err)
    }

    privBytes, err := crypto.ConfigDecodeKey(id.PrivateKey)
    if err != nil {
        return nil, nil, fmt.Errorf("invalid private key encoding: %v", err)
    }

    priv, err := crypto.UnmarshalPrivateKey(privBytes)
    if err != nil {
        return nil, nil, fmt.Errorf("invalid private key bytes: %v", err)
    }

    return &id, priv, nil
}

// LoadOrGenerate tries to load an identity at the given path.
// If it does not exist, it physically generates a new one, saves it, and returns it.
func LoadOrGenerate(path string) (*Identity, crypto.PrivKey, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Does not exist, generate new!
		ident, priv, err := GenerateKeyPair()
		if err != nil {
			return nil, nil, err
		}
		
		// Ensure directory exists
		os.MkdirAll(filepath.Dir(path), 0755)
		
		if err := ident.Save(path); err != nil {
			return nil, nil, err
		}
		return ident, priv, nil
	}

	// Exists, just load
	return Load(path)
}
