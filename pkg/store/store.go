package store

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

type Store struct {
    db *badger.DB
}

// Open initializes the badger database at the given path
func Open(path string) (*Store, error) {
    opts := badger.DefaultOptions(path)
    // Reduce logs
    opts.Logger = nil

    db, err := badger.Open(opts)
    if err != nil {
        return nil, fmt.Errorf("failed to open badger db: %w", err)
    }

    return &Store{db: db}, nil
}

// Close shuts down the store
func (s *Store) Close() error {
    if s.db != nil {
         return s.db.Close()
    }
    return nil
}

// Set stores a key-value pair
func (s *Store) Set(key, value []byte) error {
    return s.db.Update(func(txn *badger.Txn) error {
        return txn.Set(key, value)
    })
}

// Get retrieves a value by key. Returns nil if not found.
func (s *Store) Get(key []byte) ([]byte, error) {
    var valCopy []byte
    err := s.db.View(func(txn *badger.Txn) error {
        item, err := txn.Get(key)
        if err != nil {
            return err
        }
        err = item.Value(func(val []byte) error {
            valCopy = append([]byte{}, val...)
            return nil
        })
        return err
    })

    if err == badger.ErrKeyNotFound {
        return nil, nil // Interpret as explicit empty/nil for key not found for easier logic wrapper handling
    }
    if err != nil {
        return nil, err
    }
    return valCopy, nil
}

// GetAll returns all values currently in the DB.
func (s *Store) GetAll() ([][]byte, error) {
    var records [][]byte
    err := s.db.View(func(txn *badger.Txn) error {
        it := txn.NewIterator(badger.DefaultIteratorOptions)
        defer it.Close()
        
        for it.Rewind(); it.Valid(); it.Next() {
            item := it.Item()
            err := item.Value(func(v []byte) error {
                records = append(records, append([]byte{}, v...))
                return nil
            })
            if err != nil {
                return err
            }
        }
        return nil
    })
    
    return records, err
}
