package crypto

import (
	"crypto/rand"
	"errors"

	p2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// DeriveSymmetricKey hashes a human-readable password + room salt into a 32-byte ChaCha20 key.
func DeriveSymmetricKey(password, room string) []byte {
	// If password is completely blank, we effectively bypass E2EE for plaintext zones, but 
	// technically we'll just encrypt against an empty-string key deterministically.
	salt := []byte("meshrelief-v1-salt-" + room)
	// Argon2id: time=1, memory=64MB, threads=4, keyLen=32
	return argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, chacha20poly1305.KeySize)
}

// EncryptMessage symmetrically seals the plaintext.
func EncryptMessage(key []byte, plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(plaintext)+aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptMessage symmetrically opens the ciphertext.
func DecryptMessage(key []byte, ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aead.NonceSize() {
		return nil, errors.New("ciphertext too short to contain nonce")
	}

	nonce, encrypted := ciphertext[:aead.NonceSize()], ciphertext[aead.NonceSize():]
	return aead.Open(nil, nonce, encrypted, nil)
}

// SignPayload digitally signs a deterministic string block using the sender's Ed25519 Private Key.
func SignPayload(priv p2pcrypto.PrivKey, data []byte) ([]byte, error) {
	return priv.Sign(data)
}

// VerifySignature cryptographically validates a signature against the libp2p PeerID's inherently embedded public key!
func VerifySignature(peerIDStr string, data []byte, sig []byte) (bool, error) {
	pid, err := peer.Decode(peerIDStr)
	if err != nil {
		return false, err
	}

	pubKey, err := pid.ExtractPublicKey()
	if err != nil {
		return false, err
	}
	if pubKey == nil {
		return false, errors.New("cannot extract pubkey from peer id")
	}

	return pubKey.Verify(data, sig)
}
