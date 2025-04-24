package manage

import "fmt"

// EventNotFound is an error type
// that indicates an event with the specified ID was not found.
type EventNotFound struct {
	ID string
}

func (e EventNotFound) Error() string {
	return fmt.Sprintf("Event with ID %s not found", e.ID)
}

// EventNotFoundError creates a new EventNotFound error with the specified ID.
// An error can be checked against this type using:
//
//	var eventNotFoundErr *manage.EventNotFound
//	if errors.As(err, &eventNotFoundErr) {
//		// Handle the error
//	}
func EventNotFoundError(id string) error {
	return &EventNotFound{ID: id}
}
