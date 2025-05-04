package testutils

import "fmt"

type FailingIDGenerator struct{}

func (f *FailingIDGenerator) GenerateID() (string, error) {
	return "", fmt.Errorf("failed to generate ID")
}
