package core

import "errors"

var (
	// ErrFileSystemNotFound is an error when a file system could not be found
	// in the build engine for a provided "{scheme}://" value (e.g. "file://").
	ErrFileSystemNotFound = errors.New("file system not found")
)
