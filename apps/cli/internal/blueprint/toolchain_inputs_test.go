package blueprint

import (
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
	"github.com/stretchr/testify/assert"
)

func resourceOfType(resourceType string, spec map[string]*core.MappingNode) *schema.Resource {
	r := &schema.Resource{Type: &schema.ResourceTypeWrapper{Value: resourceType}}
	if spec != nil {
		r.Spec = &core.MappingNode{Fields: spec}
	}
	return r
}

func blueprintWith(resources map[string]*schema.Resource) *schema.Blueprint {
	return &schema.Blueprint{Resources: &schema.ResourceMap{Values: resources}}
}

func stringNode(value string) *core.MappingNode {
	return &core.MappingNode{Scalar: &core.ScalarValue{StringValue: &value}}
}

func TestResourceKinds_sortedAndDeduplicated(t *testing.T) {
	bp := blueprintWith(map[string]*schema.Resource{
		"filesBucket":  resourceOfType("celerity/bucket", nil),
		"imagesBucket": resourceOfType("celerity/bucket", nil),
		"usersStore":   resourceOfType("celerity/datastore", nil),
		"workQueue":    resourceOfType("celerity/queue", nil),
	})

	assert.Equal(t, []string{"bucket", "datastore", "queue"}, ResourceKinds(bp))
}

// Configuration reaches a handler through the config store, and the API and
// handler types describe the application rather than something it reaches.
func TestResourceKinds_ignoresTypesWithNoResourceLayer(t *testing.T) {
	bp := blueprintWith(map[string]*schema.Resource{
		"appConfig": resourceOfType("celerity/config", nil),
		"api":       resourceOfType("celerity/api", nil),
		"getUsers":  resourceOfType("celerity/handler", nil),
	})

	assert.Empty(t, ResourceKinds(bp))
}

func TestResourceKinds_nilBlueprint(t *testing.T) {
	assert.Nil(t, ResourceKinds(nil))
	assert.Nil(t, ResourceKinds(&schema.Blueprint{}))
}

func TestSQLEngines_sortedAndDeduplicated(t *testing.T) {
	bp := blueprintWith(map[string]*schema.Resource{
		"primary": resourceOfType("celerity/sqlDatabase", map[string]*core.MappingNode{
			"engine": stringNode("postgres"),
		}),
		"replica": resourceOfType("celerity/sqlDatabase", map[string]*core.MappingNode{
			"engine": stringNode("postgres"),
		}),
		"legacy": resourceOfType("celerity/sqlDatabase", map[string]*core.MappingNode{
			"engine": stringNode("mysql"),
		}),
		"notADatabase": resourceOfType("celerity/datastore", nil),
	})

	assert.Equal(t, []string{"mysql", "postgres"}, SQLEngines(bp))
}

func TestSQLEngines_databaseWithNoDeclaredEngine(t *testing.T) {
	bp := blueprintWith(map[string]*schema.Resource{
		"primary": resourceOfType("celerity/sqlDatabase", nil),
	})
	assert.Empty(t, SQLEngines(bp))
}
