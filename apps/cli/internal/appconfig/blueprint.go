package appconfig

import (
	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/schema"
)

const resourceTypeConfig = "celerity/config"

// CollectStoreInfo extracts StoreInfo from celerity/config resources in the blueprint.
// Determines StoreKind based on the presence of the plaintext field.
func CollectStoreInfo(bp *schema.Blueprint) []StoreInfo {
	if bp.Resources == nil {
		return nil
	}

	var stores []StoreInfo
	for name, resource := range bp.Resources.Values {
		if resource.Type == nil || resource.Type.Value != resourceTypeConfig {
			continue
		}

		storeName := extractSpecStringField(resource, "name")
		if storeName == "" {
			storeName = name
		}

		plaintextKeys := extractSpecStringArray(resource, "plaintext")
		storeKind := StoreKindSecretsManager
		if len(plaintextKeys) > 0 {
			storeKind = StoreKindParameterStore
		}

		keyMap := make(map[string]bool, len(plaintextKeys))
		for _, k := range plaintextKeys {
			keyMap[k] = true
		}

		stores = append(stores, StoreInfo{
			ResourceName:  name,
			StoreName:     storeName,
			StoreKind:     storeKind,
			PlaintextKeys: keyMap,
		})
	}

	return stores
}

// FindStoreInfo looks up a store by spec.name first, then by resource name.
func FindStoreInfo(stores []StoreInfo, name string) *StoreInfo {
	for i := range stores {
		if stores[i].StoreName == name {
			return &stores[i]
		}
	}
	for i := range stores {
		if stores[i].ResourceName == name {
			return &stores[i]
		}
	}
	return nil
}

func extractSpecStringField(resource *schema.Resource, field string) string {
	if resource.Spec == nil || resource.Spec.Fields == nil {
		return ""
	}
	return extractScalarString(resource.Spec.Fields[field])
}

func extractSpecStringArray(resource *schema.Resource, field string) []string {
	if resource.Spec == nil || resource.Spec.Fields == nil {
		return nil
	}

	node := resource.Spec.Fields[field]
	if node == nil || node.Items == nil {
		return nil
	}

	var result []string
	for _, item := range node.Items {
		s := extractScalarString(item)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

func extractScalarString(node *core.MappingNode) string {
	if node == nil || node.Scalar == nil {
		return ""
	}
	if node.Scalar.StringValue != nil {
		return *node.Scalar.StringValue
	}
	return node.Scalar.ToString()
}
