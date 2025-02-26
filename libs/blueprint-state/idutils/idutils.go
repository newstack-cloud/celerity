package idutils

import "fmt"

// ReourceInBlueprintID returns a unique identifier for a resource in the
// context of a blueprint instance.
// e.g. instance:123:resource:saveOrderFunction
func ReourceInBlueprintID(instanceID string, resourceName string) string {
	return fmt.Sprintf("instance:%s:resource:%s", instanceID, resourceName)
}

// ChildInBlueprintID returns a unique identifier for a child blueprint
// in the context of a parent blueprint.
// e.g. instance:123:child:coreInfra
func ChildInBlueprintID(instanceID string, childName string) string {
	return fmt.Sprintf("instance:%s:child:%s", instanceID, childName)
}

// LinkInBlueprintID returns a unique identifier for a link in the
// context of a blueprint instance.
// e.g. instance:123:link:saveOrderFunction::ordersTable_0
func LinkInBlueprintID(instanceID string, linkName string) string {
	return fmt.Sprintf("instance:%s:link:%s", instanceID, linkName)
}
