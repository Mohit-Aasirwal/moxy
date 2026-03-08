package store_test

import (
	"bytes"
	"os"
	"testing"

	"moxy/pkg/store"
)

func TestStore_SetAndGet(t *testing.T) {
	dir, err := os.MkdirTemp("", "badger-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	key := []byte("test-key")
	val := []byte("test-val")

	err = db.Set(key, val)
	if err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}

	got, err := db.Get(key)
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}
	if !bytes.Equal(got, val) {
		t.Errorf("Expected %s, got %s", val, got)
	}

	// Test missing key
	missing, err := db.Get([]byte("not-found"))
	if err != nil {
		t.Errorf("Unexpected error for missing key: %v", err)
	}
	if missing != nil {
		t.Errorf("Expected nil for missing key, got %v", missing)
	}
}
