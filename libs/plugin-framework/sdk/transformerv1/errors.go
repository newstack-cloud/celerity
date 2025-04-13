package transformerv1

import (
	"fmt"
)

func errAbstractResourceTypeNotFound(abstractResourceType string) error {
	return fmt.Errorf(
		"abstract resource type not implemented in transformer plugin: %s",
		abstractResourceType,
	)
}
