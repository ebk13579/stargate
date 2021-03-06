// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import (
	"context"

	"github.com/zeebo/errs"
)

// Invalid is the class of error that is returned for invalid records.
var Invalid = errs.Class("invalid")

// Record is a key/value store record.
type Record struct {
	SatelliteAddress     string
	MacaroonHead         []byte // 32 bytes probably
	EncryptedSecretKey   []byte
	EncryptedAccessGrant []byte
	Public               bool // if true, knowledge of secret key is not required
}

// KeyHash is the key portion of the key/value store.
type KeyHash [32]byte

// KV is an abstract key/value store of KeyHash to Records.
type KV interface {
	// Put stores the record in the key/value store.
	// It is an error if the key already exists.
	Put(ctx context.Context, keyHash KeyHash, record *Record) (err error)

	// Get retrieves the record from the key/value store.
	// It returns nil if the key does not exist.
	// If the record is invalid, the error contains why.
	Get(ctx context.Context, keyHash KeyHash) (record *Record, err error)

	// Delete removes the record from the key/value store.
	// It is not an error if the key does not exist.
	Delete(ctx context.Context, keyHash KeyHash) error

	// Invalidate causes the record to become invalid.
	// It is not an error if the key does not exist.
	// It does not update the invalid reason if the record is already invalid.
	Invalidate(ctx context.Context, keyHash KeyHash, reason string) error
}
