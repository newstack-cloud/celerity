package core

import (
	"github.com/google/uuid"
	gonanoid "github.com/matoous/go-nanoid/v2"
)

// IDGenerator is an interface for generating globally unique IDs.
// ID generators are used in the deployment process of a blueprint container,
// if the state container should be responsible for assigning IDs, then you can choose to
// use an ID generator that produces an empty string.
type IDGenerator interface {
	GenerateID() (string, error)
}

// UUIDGenerator is an ID generator that produces UUIDs.
type UUIDGenerator struct{}

// NewNanoIDGenerator creates a new generator that produces v4 UUIDs.
func NewUUIDGenerator() IDGenerator {
	return &UUIDGenerator{}
}

// GenerateID generates a UUID v4.
func (u *UUIDGenerator) GenerateID() (string, error) {
	return uuid.NewString(), nil
}

// NanoIDGenerator is an ID generator that produces nano IDs.
type NanoIDGenerator struct{}

// NewNanoIDGenerator creates a new generator that produces nano IDs.
func NewNanoIDGenerator() IDGenerator {
	return &NanoIDGenerator{}
}

// GenerateID generates a NanoID.
func (n *NanoIDGenerator) GenerateID() (string, error) {
	return gonanoid.New()
}

// EmptyIDGenerator is an ID generator that produces empty strings.
type EmptyIDGenerator struct{}

// NewEmptyIDGenerator creates a new generator that produces empty strings.
// This is useful when the state container should be responsible for assigning IDs.
func NewEmptyIDGenerator() IDGenerator {
	return &EmptyIDGenerator{}
}

// GenerateID generates an empty string.
func (e *EmptyIDGenerator) GenerateID() (string, error) {
	return "", nil
}
