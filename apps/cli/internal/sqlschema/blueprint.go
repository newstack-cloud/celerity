package sqlschema

import (
	"fmt"
	"path/filepath"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
)

const resourceTypeSqlDatabase = "celerity/sqlDatabase"

// CollectDatabaseResources extracts celerity/sqlDatabase resources from a blueprint
// and resolves schema/migration paths relative to the project root.
func CollectDatabaseResources(bp *schema.Blueprint, projectRoot string) []DatabaseResource {
	if bp.Resources == nil {
		return nil
	}

	var resources []DatabaseResource
	for name, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != resourceTypeSqlDatabase {
			continue
		}

		dr := DatabaseResource{
			ResourceName: name,
			Name:         specString(resource.Spec, "name"),
			Engine:       specString(resource.Spec, "engine"),
			SchemaPath:   specString(resource.Spec, "schemaPath"),
			MigrationsPath: specString(resource.Spec, "migrationsPath"),
			AuthMode:     specString(resource.Spec, "authMode"),
		}

		if dr.Name == "" {
			dr.Name = name
		}
		if dr.Engine == "" {
			dr.Engine = "postgres"
		}

		if dr.SchemaPath != "" {
			dr.SchemaPath = resolvePath(projectRoot, dr.SchemaPath)
		}
		if dr.MigrationsPath != "" {
			dr.MigrationsPath = resolvePath(projectRoot, dr.MigrationsPath)
		}

		resources = append(resources, dr)
	}

	return resources
}

// HasSqlDatabaseResources returns true if the blueprint contains any
// celerity/sqlDatabase resources.
func HasSqlDatabaseResources(bp *schema.Blueprint) bool {
	if bp.Resources == nil {
		return false
	}
	for _, resource := range bp.Resources.Values {
		if resource.Type != nil && resource.Type.Value == resourceTypeSqlDatabase {
			return true
		}
	}
	return false
}

func specString(spec *core.MappingNode, field string) string {
	if spec == nil || spec.Fields == nil {
		return ""
	}
	node := spec.Fields[field]
	if node == nil || node.Scalar == nil {
		return ""
	}
	if node.Scalar.StringValue != nil {
		return *node.Scalar.StringValue
	}
	return node.Scalar.ToString()
}

func resolvePath(projectRoot, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	// Strip leading "./" for cleanliness.
	cleaned := filepath.Clean(p)
	resolved := filepath.Join(projectRoot, cleaned)
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return resolved
	}
	return abs
}

// ResourceTypeSqlDatabase is the blueprint resource type for SQL databases.
// Exported for use by the compose generator.
func ResourceType() string {
	return resourceTypeSqlDatabase
}

// DefaultPostgresCredentials returns the default local dev credentials
// for PostgreSQL containers.
func DefaultPostgresCredentials() (user, password, database string) {
	return "celerity", "celerity", "celerity"
}

// FormatConnectionString builds a PostgreSQL connection string from components.
func FormatConnectionString(host, port, user, password, database string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, password, host, port, database)
}
