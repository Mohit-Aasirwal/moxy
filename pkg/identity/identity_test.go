package identity_test

import (
    "os"
    "path/filepath"
    "testing"

    "moxy/pkg/identity"
)

func TestGenerateAndSaveLoad(t *testing.T) {
    ident, priv, err := identity.GenerateKeyPair()
    if err != nil {
        t.Fatalf("Failed to generate keypair: %v", err)
    }

    if ident.PeerID == "" {
        t.Errorf("Generated Identity is missing PeerID")
    }
    if priv == nil {
        t.Errorf("Private key is missing")
    }

    dir, err := os.MkdirTemp("", "mesh_test")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(dir)

    keyPath := filepath.Join(dir, "identity.json")

    err = ident.Save(keyPath)
    if err != nil {
        t.Fatalf("Failed to save identity: %v", err)
    }

    loadedIdent, loadedPriv, err := identity.Load(keyPath)
    if err != nil {
        t.Fatalf("Failed to load identity: %v", err)
    }

    if loadedIdent.PeerID != ident.PeerID || loadedIdent.PublicKey != ident.PublicKey {
        t.Errorf("Mismatch between saved and loaded identities")
    }
    if loadedPriv == nil {
         t.Errorf("Loaded private key is nil")
    }

    b1, _ := priv.Raw()
    b2, _ := loadedPriv.Raw()
    if string(b1) != string(b2) {
        t.Errorf("Mismatch in raw bytes of loaded vs saved private key")
    }
}
